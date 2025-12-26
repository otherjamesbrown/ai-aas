package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "valid config",
			cfg: Config{
				HTTPPort:                8084,
				DatabaseURL:             "postgres://localhost/test",
				IngestionBatchSize:      1000,
				IngestionWorkers:        4,
				AggregationWorkers:      2,
				ExportWorkerConcurrency: 2,
			},
			wantErr: "",
		},
		{
			name: "invalid port - zero",
			cfg: Config{
				HTTPPort:                0,
				DatabaseURL:             "postgres://localhost/test",
				IngestionBatchSize:      1000,
				IngestionWorkers:        4,
				AggregationWorkers:      2,
				ExportWorkerConcurrency: 2,
			},
			wantErr: "HTTP_PORT must be between 1 and 65535",
		},
		{
			name: "invalid port - too high",
			cfg: Config{
				HTTPPort:                70000,
				DatabaseURL:             "postgres://localhost/test",
				IngestionBatchSize:      1000,
				IngestionWorkers:        4,
				AggregationWorkers:      2,
				ExportWorkerConcurrency: 2,
			},
			wantErr: "HTTP_PORT must be between 1 and 65535",
		},
		{
			name: "missing database URL",
			cfg: Config{
				HTTPPort:                8084,
				DatabaseURL:             "",
				IngestionBatchSize:      1000,
				IngestionWorkers:        4,
				AggregationWorkers:      2,
				ExportWorkerConcurrency: 2,
			},
			wantErr: "DATABASE_URL is required",
		},
		{
			name: "invalid batch size - zero",
			cfg: Config{
				HTTPPort:                8084,
				DatabaseURL:             "postgres://localhost/test",
				IngestionBatchSize:      0,
				IngestionWorkers:        4,
				AggregationWorkers:      2,
				ExportWorkerConcurrency: 2,
			},
			wantErr: "INGESTION_BATCH_SIZE must be positive",
		},
		{
			name: "invalid batch size - negative",
			cfg: Config{
				HTTPPort:                8084,
				DatabaseURL:             "postgres://localhost/test",
				IngestionBatchSize:      -10,
				IngestionWorkers:        4,
				AggregationWorkers:      2,
				ExportWorkerConcurrency: 2,
			},
			wantErr: "INGESTION_BATCH_SIZE must be positive",
		},
		{
			name: "invalid ingestion workers",
			cfg: Config{
				HTTPPort:                8084,
				DatabaseURL:             "postgres://localhost/test",
				IngestionBatchSize:      1000,
				IngestionWorkers:        0,
				AggregationWorkers:      2,
				ExportWorkerConcurrency: 2,
			},
			wantErr: "INGESTION_WORKERS must be positive",
		},
		{
			name: "invalid aggregation workers",
			cfg: Config{
				HTTPPort:                8084,
				DatabaseURL:             "postgres://localhost/test",
				IngestionBatchSize:      1000,
				IngestionWorkers:        4,
				AggregationWorkers:      0,
				ExportWorkerConcurrency: 2,
			},
			wantErr: "AGGREGATION_WORKERS must be positive",
		},
		{
			name: "invalid export worker concurrency",
			cfg: Config{
				HTTPPort:                8084,
				DatabaseURL:             "postgres://localhost/test",
				IngestionBatchSize:      1000,
				IngestionWorkers:        4,
				AggregationWorkers:      2,
				ExportWorkerConcurrency: -1,
			},
			wantErr: "EXPORT_WORKER_CONCURRENCY must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	// Save and restore environment
	origEnv := os.Environ()
	defer func() {
		os.Clearenv()
		for _, e := range origEnv {
			kv := splitEnv(e)
			os.Setenv(kv[0], kv[1])
		}
	}()

	t.Run("loads valid config from environment", func(t *testing.T) {
		os.Clearenv()
		os.Setenv("DATABASE_URL", "postgres://test:test@localhost/analytics")
		os.Setenv("HTTP_PORT", "9090")
		os.Setenv("INGESTION_BATCH_SIZE", "500")
		os.Setenv("INGESTION_WORKERS", "8")
		os.Setenv("AGGREGATION_WORKERS", "4")
		os.Setenv("EXPORT_WORKER_CONCURRENCY", "3")
		os.Setenv("ROLLUP_INTERVAL", "30m")

		cfg, err := Load()
		require.NoError(t, err)
		assert.Equal(t, 9090, cfg.HTTPPort)
		assert.Equal(t, "postgres://test:test@localhost/analytics", cfg.DatabaseURL)
		assert.Equal(t, 500, cfg.IngestionBatchSize)
		assert.Equal(t, 8, cfg.IngestionWorkers)
		assert.Equal(t, 4, cfg.AggregationWorkers)
		assert.Equal(t, 3, cfg.ExportWorkerConcurrency)
		assert.Equal(t, 30*time.Minute, cfg.RollupInterval)
	})

	t.Run("uses defaults when not set", func(t *testing.T) {
		os.Clearenv()
		os.Setenv("DATABASE_URL", "postgres://localhost/test")

		cfg, err := Load()
		require.NoError(t, err)
		assert.Equal(t, "analytics-service", cfg.ServiceName)
		assert.Equal(t, "development", cfg.Environment)
		assert.Equal(t, 8084, cfg.HTTPPort)
		assert.Equal(t, 1000, cfg.IngestionBatchSize)
		assert.Equal(t, 4, cfg.IngestionWorkers)
		assert.Equal(t, 2, cfg.AggregationWorkers)
		assert.Equal(t, 1*time.Hour, cfg.RollupInterval)
		assert.Equal(t, 5*time.Minute, cfg.FreshnessCacheTTL)
		assert.Equal(t, true, cfg.EnableRBAC)
	})

	t.Run("fails when required field missing", func(t *testing.T) {
		os.Clearenv()
		// DATABASE_URL is required but not set

		cfg, err := Load()
		assert.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("fails validation on invalid values", func(t *testing.T) {
		os.Clearenv()
		os.Setenv("DATABASE_URL", "postgres://localhost/test")
		os.Setenv("HTTP_PORT", "99999") // Invalid port

		cfg, err := Load()
		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "HTTP_PORT")
	})
}

func TestMustLoad(t *testing.T) {
	// Save and restore environment
	origEnv := os.Environ()
	defer func() {
		os.Clearenv()
		for _, e := range origEnv {
			kv := splitEnv(e)
			os.Setenv(kv[0], kv[1])
		}
	}()

	t.Run("panics on invalid config", func(t *testing.T) {
		os.Clearenv()
		// DATABASE_URL is required but not set

		assert.Panics(t, func() {
			MustLoad()
		})
	})

	t.Run("returns config on valid environment", func(t *testing.T) {
		os.Clearenv()
		os.Setenv("DATABASE_URL", "postgres://localhost/test")

		assert.NotPanics(t, func() {
			cfg := MustLoad()
			assert.NotNil(t, cfg)
		})
	})
}

// splitEnv splits KEY=VALUE into [KEY, VALUE]
func splitEnv(e string) []string {
	for i := 0; i < len(e); i++ {
		if e[i] == '=' {
			return []string{e[:i], e[i+1:]}
		}
	}
	return []string{e, ""}
}
