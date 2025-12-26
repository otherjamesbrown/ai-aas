// Package triton provides protocol translation and client implementations for Triton Inference Server.
//
// This file implements the gRPC client for communicating with Triton backends using
// the bidirectional streaming ModelStreamInfer RPC, which is required for TensorRT-LLM
// models running with "decoupled transaction policy".
package triton

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/adapter/triton/pb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// GRPCClientConfig holds configuration options for TritonGRPCClient.
type GRPCClientConfig struct {
	// Endpoint is the gRPC server address (host:port)
	Endpoint string

	// Timeout is the default timeout for unary RPC calls (e.g., health checks)
	Timeout time.Duration

	// KeepaliveTime is the interval between keepalive pings
	KeepaliveTime time.Duration

	// KeepaliveTimeout is the timeout for keepalive ping acknowledgment
	KeepaliveTimeout time.Duration

	// MaxRecvMsgSize is the maximum message size the client can receive (default: 16MB)
	MaxRecvMsgSize int
}

// DefaultGRPCClientConfig returns a GRPCClientConfig with sensible defaults.
func DefaultGRPCClientConfig(endpoint string) *GRPCClientConfig {
	return &GRPCClientConfig{
		Endpoint:         endpoint,
		Timeout:          30 * time.Second,
		KeepaliveTime:    30 * time.Second,
		KeepaliveTimeout: 10 * time.Second,
		MaxRecvMsgSize:   16 * 1024 * 1024, // 16MB
	}
}

// TritonGRPCClient wraps a gRPC connection to a Triton Inference Server.
// It provides methods for streaming inference and health checks.
type TritonGRPCClient struct {
	conn    *grpc.ClientConn
	client  pb.GRPCInferenceServiceClient
	logger  *zap.Logger
	config  *GRPCClientConfig
	mu      sync.RWMutex
	closed  bool
}

// NewTritonGRPCClient creates a new gRPC client connected to the specified Triton endpoint.
func NewTritonGRPCClient(cfg *GRPCClientConfig, logger *zap.Logger) (*TritonGRPCClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("endpoint is required")
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	// Configure gRPC dial options
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                cfg.KeepaliveTime,
			Timeout:             cfg.KeepaliveTimeout,
			PermitWithoutStream: true,
		}),
	}

	if cfg.MaxRecvMsgSize > 0 {
		opts = append(opts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(cfg.MaxRecvMsgSize)))
	}

	logger.Info("connecting to Triton gRPC server",
		zap.String("endpoint", cfg.Endpoint),
		zap.Duration("timeout", cfg.Timeout),
	)

	conn, err := grpc.NewClient(cfg.Endpoint, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Triton gRPC server at %s: %w", cfg.Endpoint, err)
	}

	client := pb.NewGRPCInferenceServiceClient(conn)

	return &TritonGRPCClient{
		conn:   conn,
		client: client,
		logger: logger,
		config: cfg,
	}, nil
}

// StreamInfer initiates a bidirectional streaming inference call.
// The returned stream can be used to send requests and receive responses.
// The caller is responsible for sending requests on the stream and closing it when done.
//
// Usage:
//
//	stream, err := client.StreamInfer(ctx)
//	if err != nil {
//	    return err
//	}
//	// Send request
//	if err := stream.Send(req); err != nil {
//	    return err
//	}
//	// Close send side to signal we're done sending
//	if err := stream.CloseSend(); err != nil {
//	    return err
//	}
//	// Receive responses
//	for {
//	    resp, err := stream.Recv()
//	    if err == io.EOF {
//	        break
//	    }
//	    if err != nil {
//	        return err
//	    }
//	    // Process response
//	}
func (c *TritonGRPCClient) StreamInfer(ctx context.Context) (pb.GRPCInferenceService_ModelStreamInferClient, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("client is closed")
	}
	c.mu.RUnlock()

	stream, err := c.client.ModelStreamInfer(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start streaming inference: %w", err)
	}

	return stream, nil
}

// HealthCheck checks if the Triton server is ready to accept inference requests.
// It uses the ServerReady RPC which indicates the server is ready for inferencing.
func (c *TritonGRPCClient) HealthCheck(ctx context.Context) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return fmt.Errorf("client is closed")
	}
	c.mu.RUnlock()

	// Apply timeout if not already set in context
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.Timeout)
		defer cancel()
	}

	resp, err := c.client.ServerReady(ctx, &pb.ServerReadyRequest{})
	if err != nil {
		return fmt.Errorf("server ready check failed: %w", err)
	}

	if !resp.Ready {
		return fmt.Errorf("server is not ready")
	}

	return nil
}

// ModelReady checks if a specific model is ready to accept inference requests.
func (c *TritonGRPCClient) ModelReady(ctx context.Context, modelName string, modelVersion string) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return fmt.Errorf("client is closed")
	}
	c.mu.RUnlock()

	// Apply timeout if not already set in context
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.Timeout)
		defer cancel()
	}

	resp, err := c.client.ModelReady(ctx, &pb.ModelReadyRequest{
		Name:    modelName,
		Version: modelVersion,
	})
	if err != nil {
		return fmt.Errorf("model ready check failed for %s: %w", modelName, err)
	}

	if !resp.Ready {
		return fmt.Errorf("model %s is not ready", modelName)
	}

	return nil
}

// Close closes the gRPC connection.
// After Close is called, the client should not be used.
func (c *TritonGRPCClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true

	if c.conn != nil {
		c.logger.Info("closing Triton gRPC connection")
		if err := c.conn.Close(); err != nil {
			return fmt.Errorf("failed to close gRPC connection: %w", err)
		}
	}

	return nil
}

// GetClient returns the underlying gRPC client for advanced use cases.
// Most users should use the higher-level methods like StreamInfer.
func (c *TritonGRPCClient) GetClient() pb.GRPCInferenceServiceClient {
	return c.client
}

// Endpoint returns the configured endpoint for this client.
func (c *TritonGRPCClient) Endpoint() string {
	return c.config.Endpoint
}
