package dataaccess

import (
	"context"
	"database/sql"

	"github.com/ai-aas/shared-go/config"
)

// OpenSQL opens a database connection using the provided driver and configuration.
func OpenSQL(ctx context.Context, driver string, cfg config.DatabaseConfig) (*sql.DB, error) {
	db, err := sql.Open(driver, cfg.DSN)
	if err != nil {
		return nil, err
	}
	ConfigureSQL(db, cfg)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// ConfigureSQL applies pooling parameters to the provided database handle.
func ConfigureSQL(db *sql.DB, cfg config.DatabaseConfig) {
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
}
