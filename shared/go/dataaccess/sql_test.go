package dataaccess

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ai-aas/shared-go/config"
)

type trackingDriver struct {
	mu       sync.Mutex
	pingErr  error
	lastConn *stubConn
}

type stubConn struct {
	driver *trackingDriver
	closed bool
}

func (d *trackingDriver) Open(name string) (driver.Conn, error) {
	conn := &stubConn{driver: d}
	d.mu.Lock()
	d.lastConn = conn
	d.mu.Unlock()
	return conn, nil
}

func (c *stubConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *stubConn) Close() error {
	c.driver.mu.Lock()
	c.closed = true
	c.driver.mu.Unlock()
	return nil
}

func (c *stubConn) Begin() (driver.Tx, error) {
	return nil, errors.New("not implemented")
}

func (c *stubConn) Ping(ctx context.Context) error {
	c.driver.mu.Lock()
	err := c.driver.pingErr
	c.driver.mu.Unlock()
	return err
}

func (c *stubConn) ResetSession(context.Context) error                                        { return nil }
func (c *stubConn) IsValid() bool                                                             { return true }
func (c *stubConn) CheckNamedValue(*driver.NamedValue) error                                  { return nil }
func (c *stubConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return nil, errors.New("not implemented")
}
func (c *stubConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return nil, errors.New("not implemented")
}

var testDriver = &trackingDriver{}

func init() {
	sql.Register("stub", testDriver)
}

func resetDriver() {
	testDriver.mu.Lock()
	defer testDriver.mu.Unlock()
	testDriver.pingErr = nil
	testDriver.lastConn = nil
}

func TestConfigureSQLAppliesSettings(t *testing.T) {
	resetDriver()

	db, err := sql.Open("stub", "config")
	if err != nil {
		t.Fatalf("open stub db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	cfg := config.DatabaseConfig{
		MaxIdleConns:    3,
		MaxOpenConns:    5,
		ConnMaxLifetime: time.Minute,
	}

	ConfigureSQL(db, cfg)

	stats := db.Stats()
	if stats.MaxOpenConnections != cfg.MaxOpenConns {
		t.Fatalf("expected max open %d, got %d", cfg.MaxOpenConns, stats.MaxOpenConnections)
	}
}

func TestOpenSQLSuccess(t *testing.T) {
	resetDriver()
	ctx := context.Background()
	cfg := config.DatabaseConfig{
		DSN:             "success",
		MaxIdleConns:    2,
		MaxOpenConns:    4,
		ConnMaxLifetime: 2 * time.Minute,
	}

	db, err := OpenSQL(ctx, "stub", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	stats := db.Stats()
	if stats.MaxOpenConnections != cfg.MaxOpenConns {
		t.Fatalf("expected max open %d, got %d", cfg.MaxOpenConns, stats.MaxOpenConnections)
	}
	testDriver.mu.Lock()
	conn := testDriver.lastConn
	testDriver.mu.Unlock()
	if conn == nil {
		t.Fatalf("expected connection to be recorded")
	}
}

func TestOpenSQLPingFailureClosesConnection(t *testing.T) {
	resetDriver()
	testDriver.mu.Lock()
	testDriver.pingErr = errors.New("ping failed")
	testDriver.mu.Unlock()

	_, err := OpenSQL(context.Background(), "stub", config.DatabaseConfig{DSN: "fail"})
	if err == nil {
		t.Fatalf("expected ping error")
	}

	testDriver.mu.Lock()
	conn := testDriver.lastConn
	testDriver.mu.Unlock()
	if conn == nil {
		t.Fatalf("expected connection record")
	}
	if !conn.closed {
		t.Fatalf("expected connection to be closed on ping failure")
	}
}

