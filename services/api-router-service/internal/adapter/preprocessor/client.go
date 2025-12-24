// Package preprocessor provides a gRPC client for the preprocessor service.
//
// The preprocessor service applies model-specific chat templates using HuggingFace
// AutoTokenizer before forwarding requests to inference backends.
package preprocessor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/adapter/preprocessor/pb"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// ClientConfig holds configuration options for the preprocessor gRPC client.
type ClientConfig struct {
	// Endpoint is the gRPC server address (host:port)
	Endpoint string

	// Timeout is the default timeout for RPC calls
	Timeout time.Duration

	// KeepaliveTime is the interval between keepalive pings
	KeepaliveTime time.Duration

	// KeepaliveTimeout is the timeout for keepalive ping acknowledgment
	KeepaliveTimeout time.Duration

	// MaxRecvMsgSize is the maximum message size the client can receive (default: 16MB)
	MaxRecvMsgSize int
}

// DefaultClientConfig returns a ClientConfig with sensible defaults.
func DefaultClientConfig(endpoint string) *ClientConfig {
	return &ClientConfig{
		Endpoint:         endpoint,
		Timeout:          5 * time.Second, // Preprocessing should be fast
		KeepaliveTime:    30 * time.Second,
		KeepaliveTimeout: 10 * time.Second,
		MaxRecvMsgSize:   16 * 1024 * 1024, // 16MB
	}
}

// Client wraps a gRPC connection to the preprocessor service.
type Client struct {
	conn   *grpc.ClientConn
	client pb.PreprocessorServiceClient
	logger *zap.Logger
	config *ClientConfig
	mu     sync.RWMutex
	closed bool
}

// NewClient creates a new gRPC client connected to the preprocessor service.
func NewClient(cfg *ClientConfig, logger *zap.Logger) (*Client, error) {
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

	logger.Info("connecting to preprocessor gRPC server",
		zap.String("endpoint", cfg.Endpoint),
		zap.Duration("timeout", cfg.Timeout),
	)

	conn, err := grpc.NewClient(cfg.Endpoint, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to preprocessor at %s: %w", cfg.Endpoint, err)
	}

	client := pb.NewPreprocessorServiceClient(conn)

	return &Client{
		conn:   conn,
		client: client,
		logger: logger,
		config: cfg,
	}, nil
}

// Message represents a chat message in OpenAI format.
type Message struct {
	Role    string
	Content string
}

// PreprocessRequest contains the request parameters for preprocessing.
type PreprocessRequest struct {
	ModelID     string
	EngineType  string
	Messages    []Message
	MaxTokens   int32
	Temperature float32
	TopP        float32
	Stream      bool
}

// PreprocessResponse contains the preprocessed output.
type PreprocessResponse struct {
	FormattedPrompt  string
	InputIDs         []int32
	InputShape       []int64
	PromptTokenCount int32
	ErrorMessage     string
}

// Preprocess applies model-specific chat template to messages.
func (c *Client) Preprocess(ctx context.Context, req *PreprocessRequest) (*PreprocessResponse, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, fmt.Errorf("client is closed")
	}
	c.mu.RUnlock()

	// Apply timeout if not already set in context
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.Timeout)
		defer cancel()
	}

	// Convert messages to proto format
	protoMessages := make([]*pb.Message, len(req.Messages))
	for i, msg := range req.Messages {
		protoMessages[i] = &pb.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Build proto request
	protoReq := &pb.PreprocessRequest{
		ModelId:     req.ModelID,
		EngineType:  req.EngineType,
		Messages:    protoMessages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stream:      req.Stream,
	}

	c.logger.Debug("sending preprocess request",
		zap.String("model_id", req.ModelID),
		zap.String("engine_type", req.EngineType),
		zap.Int("num_messages", len(req.Messages)),
	)

	resp, err := c.client.Preprocess(ctx, protoReq)
	if err != nil {
		return nil, fmt.Errorf("preprocess RPC failed: %w", err)
	}

	// Check for application-level error
	if resp.ErrorMessage != "" {
		return nil, fmt.Errorf("preprocessing failed: %s", resp.ErrorMessage)
	}

	return &PreprocessResponse{
		FormattedPrompt:  resp.FormattedPrompt,
		InputIDs:         resp.InputIds,
		InputShape:       resp.InputShape,
		PromptTokenCount: resp.PromptTokenCount,
	}, nil
}

// HealthCheck checks if the preprocessor service is ready.
func (c *Client) HealthCheck(ctx context.Context) error {
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

	resp, err := c.client.HealthCheck(ctx, &pb.HealthCheckRequest{})
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	if !resp.Ready {
		return fmt.Errorf("preprocessor is not ready: %s", resp.Message)
	}

	return nil
}

// Close closes the gRPC connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true

	if c.conn != nil {
		c.logger.Info("closing preprocessor gRPC connection")
		if err := c.conn.Close(); err != nil {
			return fmt.Errorf("failed to close gRPC connection: %w", err)
		}
	}

	return nil
}

// Endpoint returns the configured endpoint for this client.
func (c *Client) Endpoint() string {
	return c.config.Endpoint
}
