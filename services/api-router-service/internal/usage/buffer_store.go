// Package usage provides disk-based buffering for usage records.
//
// Purpose:
//   This package implements persistent buffering for usage records when Kafka
//   is unavailable, ensuring at-least-once delivery guarantees.
//
// Key Responsibilities:
//   - Persist usage records to disk when Kafka is unavailable
//   - Load buffered records on startup
//   - Provide retry mechanism for failed publishes
//   - Clean up successfully published records
//
// Requirements Reference:
//   - specs/006-api-router-service/spec.md#US-004 (Accurate, timely usage accounting)
//   - specs/006-api-router-service/spec.md#NFR-006 (At-least-once delivery)
//   - specs/006-api-router-service/spec.md#NFR-015 (24-hour buffer retention)
//
package usage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// BufferStore provides disk-based buffering for usage records.
type BufferStore struct {
	dir      string
	logger   *zap.Logger
	mu       sync.RWMutex
	maxSize  int // Maximum number of records to buffer
	maxAge   time.Duration // Maximum age of buffered records
}

// BufferStoreConfig configures the buffer store.
type BufferStoreConfig struct {
	Dir     string        // Directory to store buffered records
	MaxSize int           // Maximum number of records to buffer (0 = unlimited)
	MaxAge  time.Duration // Maximum age of buffered records (0 = no expiration)
	Logger  *zap.Logger
}

// NewBufferStore creates a new buffer store.
func NewBufferStore(cfg BufferStoreConfig) (*BufferStore, error) {
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(cfg.Dir, 0755); err != nil {
		return nil, fmt.Errorf("create buffer directory: %w", err)
	}

	return &BufferStore{
		dir:    cfg.Dir,
		logger: cfg.Logger.With(zap.String("component", "usage-buffer-store")),
		maxSize: cfg.MaxSize,
		maxAge:  cfg.MaxAge,
	}, nil
}

// Store stores a usage record to disk buffer.
func (s *BufferStore) Store(record *UsageRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check size limit
	if s.maxSize > 0 {
		count, err := s.countRecords()
		if err != nil {
			s.logger.Warn("failed to count records", zap.Error(err))
		} else if count >= s.maxSize {
			return fmt.Errorf("buffer store is full (%d records)", count)
		}
	}

	// Serialize record
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal record: %w", err)
	}

	// Write to file (one file per record, named by record ID)
	filename := filepath.Join(s.dir, record.RecordID+".json")
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("write buffer file: %w", err)
	}

	s.logger.Debug("usage record buffered",
		zap.String("record_id", record.RecordID),
		zap.String("request_id", record.RequestID),
	)

	return nil
}

// Load loads all buffered records from disk.
func (s *BufferStore) Load() ([]*UsageRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read buffer directory: %w", err)
	}

	var records []*UsageRecord
	now := time.Now()

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Skip non-JSON files
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		// Check age if maxAge is set
		if s.maxAge > 0 {
			info, err := entry.Info()
			if err != nil {
				s.logger.Warn("failed to get file info", zap.String("file", entry.Name()), zap.Error(err))
				continue
			}
			if now.Sub(info.ModTime()) > s.maxAge {
				s.logger.Debug("skipping expired buffer file", zap.String("file", entry.Name()))
				continue
			}
		}

		// Read and parse record
		filename := filepath.Join(s.dir, entry.Name())
		data, err := os.ReadFile(filename)
		if err != nil {
			s.logger.Warn("failed to read buffer file", zap.String("file", entry.Name()), zap.Error(err))
			continue
		}

		var record UsageRecord
		if err := json.Unmarshal(data, &record); err != nil {
			s.logger.Warn("failed to unmarshal buffer file", zap.String("file", entry.Name()), zap.Error(err))
			continue
		}

		records = append(records, &record)
	}

	s.logger.Info("loaded buffered usage records",
		zap.Int("count", len(records)),
	)

	return records, nil
}

// Remove removes a buffered record by record ID.
func (s *BufferStore) Remove(recordID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := filepath.Join(s.dir, recordID+".json")
	if err := os.Remove(filename); err != nil {
		if os.IsNotExist(err) {
			return nil // Already removed
		}
		return fmt.Errorf("remove buffer file: %w", err)
	}

	s.logger.Debug("removed buffered usage record",
		zap.String("record_id", recordID),
	)

	return nil
}

// Count returns the number of buffered records.
func (s *BufferStore) Count() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.countRecords()
}

// countRecords counts records without holding the lock (caller must hold lock).
func (s *BufferStore) countRecords() (int, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			count++
		}
	}

	return count, nil
}

// Cleanup removes expired records and returns the number removed.
func (s *BufferStore) Cleanup() (int, error) {
	if s.maxAge == 0 {
		return 0, nil // No expiration
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return 0, fmt.Errorf("read buffer directory: %w", err)
	}

	now := time.Now()
	removed := 0

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if now.Sub(info.ModTime()) > s.maxAge {
			filename := filepath.Join(s.dir, entry.Name())
			if err := os.Remove(filename); err == nil {
				removed++
				s.logger.Debug("removed expired buffer file", zap.String("file", entry.Name()))
			}
		}
	}

	if removed > 0 {
		s.logger.Info("cleaned up expired buffered records", zap.Int("removed", removed))
	}

	return removed, nil
}

// Clear removes all buffered records.
func (s *BufferStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return fmt.Errorf("read buffer directory: %w", err)
	}

	removed := 0
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			filename := filepath.Join(s.dir, entry.Name())
			if err := os.Remove(filename); err == nil {
				removed++
			}
		}
	}

	s.logger.Info("cleared all buffered records", zap.Int("removed", removed))
	return nil
}

