package preprocessor

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ClientManager manages a pool of preprocessor gRPC clients.
// It maintains one client per unique endpoint for connection reuse.
type ClientManager struct {
	clients map[string]*Client
	mu      sync.RWMutex
	logger  *zap.Logger
	timeout time.Duration
}

// NewClientManager creates a new ClientManager.
func NewClientManager(logger *zap.Logger) *ClientManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ClientManager{
		clients: make(map[string]*Client),
		logger:  logger,
		timeout: 5 * time.Second,
	}
}

// WithTimeout sets the default timeout for new clients.
func (m *ClientManager) WithTimeout(timeout time.Duration) *ClientManager {
	m.timeout = timeout
	return m
}

// GetOrCreate returns an existing client for the endpoint or creates a new one.
func (m *ClientManager) GetOrCreate(endpoint string) (*Client, error) {
	// Fast path: check if client exists
	m.mu.RLock()
	client, exists := m.clients[endpoint]
	m.mu.RUnlock()

	if exists {
		return client, nil
	}

	// Slow path: create new client
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if client, exists := m.clients[endpoint]; exists {
		return client, nil
	}

	// Create new client
	cfg := DefaultClientConfig(endpoint)
	cfg.Timeout = m.timeout

	client, err := NewClient(cfg, m.logger.With(zap.String("endpoint", endpoint)))
	if err != nil {
		return nil, fmt.Errorf("failed to create preprocessor client for %s: %w", endpoint, err)
	}

	m.clients[endpoint] = client
	m.logger.Info("created new preprocessor client",
		zap.String("endpoint", endpoint),
		zap.Int("total_clients", len(m.clients)),
	)

	return client, nil
}

// Get returns an existing client for the endpoint or nil if not found.
func (m *ClientManager) Get(endpoint string) *Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.clients[endpoint]
}

// Close closes all managed clients.
func (m *ClientManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for endpoint, client := range m.clients {
		if err := client.Close(); err != nil {
			m.logger.Error("failed to close preprocessor client",
				zap.String("endpoint", endpoint),
				zap.Error(err),
			)
			lastErr = err
		}
	}

	m.clients = make(map[string]*Client)
	return lastErr
}

// Count returns the number of managed clients.
func (m *ClientManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.clients)
}
