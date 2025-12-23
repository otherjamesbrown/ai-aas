package triton

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestDefaultGRPCClientConfig(t *testing.T) {
	endpoint := "localhost:8001"
	cfg := DefaultGRPCClientConfig(endpoint)

	if cfg.Endpoint != endpoint {
		t.Errorf("expected endpoint %q, got %q", endpoint, cfg.Endpoint)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("expected timeout %v, got %v", 30*time.Second, cfg.Timeout)
	}
	if cfg.KeepaliveTime != 30*time.Second {
		t.Errorf("expected keepalive time %v, got %v", 30*time.Second, cfg.KeepaliveTime)
	}
	if cfg.KeepaliveTimeout != 10*time.Second {
		t.Errorf("expected keepalive timeout %v, got %v", 10*time.Second, cfg.KeepaliveTimeout)
	}
	if cfg.MaxRecvMsgSize != 16*1024*1024 {
		t.Errorf("expected max recv msg size %d, got %d", 16*1024*1024, cfg.MaxRecvMsgSize)
	}
}

func TestNewTritonGRPCClient_NilConfig(t *testing.T) {
	_, err := NewTritonGRPCClient(nil, nil)
	if err == nil {
		t.Error("expected error for nil config, got nil")
	}
}

func TestNewTritonGRPCClient_EmptyEndpoint(t *testing.T) {
	cfg := &GRPCClientConfig{}
	_, err := NewTritonGRPCClient(cfg, nil)
	if err == nil {
		t.Error("expected error for empty endpoint, got nil")
	}
}

func TestNewTritonGRPCClient_ValidConfig(t *testing.T) {
	// This test creates a client with a non-existent endpoint
	// The connection won't actually be established until a call is made
	cfg := DefaultGRPCClientConfig("localhost:9999")
	logger := zap.NewNop()

	client, err := NewTritonGRPCClient(cfg, logger)
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}
	defer client.Close()

	if client.Endpoint() != "localhost:9999" {
		t.Errorf("expected endpoint %q, got %q", "localhost:9999", client.Endpoint())
	}
}

func TestTritonGRPCClient_CloseTwice(t *testing.T) {
	cfg := DefaultGRPCClientConfig("localhost:9999")
	client, err := NewTritonGRPCClient(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	// First close should succeed
	if err := client.Close(); err != nil {
		t.Errorf("first close failed: %v", err)
	}

	// Second close should be idempotent
	if err := client.Close(); err != nil {
		t.Errorf("second close failed: %v", err)
	}
}

func TestTritonGRPCClient_MethodsAfterClose(t *testing.T) {
	cfg := DefaultGRPCClientConfig("localhost:9999")
	client, err := NewTritonGRPCClient(cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	client.Close()

	ctx := context.Background()

	// StreamInfer should fail after close
	_, err = client.StreamInfer(ctx)
	if err == nil {
		t.Error("expected error calling StreamInfer after close")
	}

	// HealthCheck should fail after close
	err = client.HealthCheck(ctx)
	if err == nil {
		t.Error("expected error calling HealthCheck after close")
	}

	// ModelReady should fail after close
	err = client.ModelReady(ctx, "test-model", "")
	if err == nil {
		t.Error("expected error calling ModelReady after close")
	}
}
