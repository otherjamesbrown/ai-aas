// Package storage provides S3-compatible object storage operations.
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Client provides S3-compatible object storage operations
type S3Client struct {
	client     *s3.Client
	bucket     string
	endpoint   string
	pathPrefix string
}

// S3Config configures the S3 client
type S3Config struct {
	Endpoint       string
	AccessKey      string
	SecretKey      string
	Bucket         string
	Region         string
	PathPrefix     string // Optional prefix for all paths (e.g., "models/")
	ForcePathStyle bool   // Use path-style URLs (required for Linode Object Storage)
}

// NewS3Client creates a new S3 client
func NewS3Client(ctx context.Context, cfg S3Config) (*S3Client, error) {
	if cfg.Region == "" {
		cfg.Region = "us-east-1" // Default region
	}

	// Build AWS config
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey,
			cfg.SecretKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	// Create S3 client with custom endpoint
	// For S3-compatible services like Linode Object Storage, we need to disable
	// request checksum calculation as they don't fully support the newer AWS SDK v2
	// checksum handling which causes XAmzContentSHA256Mismatch errors.
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.ForcePathStyle
		// Disable request checksum calculation for non-AWS S3-compatible services
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})

	return &S3Client{
		client:     client,
		bucket:     cfg.Bucket,
		endpoint:   cfg.Endpoint,
		pathPrefix: cfg.PathPrefix,
	}, nil
}

// fullPath returns the full S3 path with prefix
func (c *S3Client) fullPath(path string) string {
	if c.pathPrefix == "" {
		return path
	}
	return strings.TrimSuffix(c.pathPrefix, "/") + "/" + strings.TrimPrefix(path, "/")
}

// Bucket returns the bucket name
func (c *S3Client) Bucket() string {
	return c.bucket
}

// Endpoint returns the S3 endpoint
func (c *S3Client) Endpoint() string {
	return c.endpoint
}

// Upload uploads a file to S3
func (c *S3Client) Upload(ctx context.Context, localPath, remotePath string) error {
	// Read the entire file into memory
	// Using bytes.NewReader which is seekable, required for AWS SDK v2 to compute
	// the content SHA256 correctly for S3-compatible services like Linode Object Storage
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	key := c.fullPath(remotePath)

	_, err = c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(data),
		ContentLength: aws.Int64(int64(len(data))),
	})
	if err != nil {
		return fmt.Errorf("put object: %w", err)
	}

	return nil
}

// UploadBytes uploads data from a byte slice to S3
func (c *S3Client) UploadBytes(ctx context.Context, data []byte, remotePath string) error {
	key := c.fullPath(remotePath)

	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(data),
		ContentLength: aws.Int64(int64(len(data))),
	})
	if err != nil {
		return fmt.Errorf("put object: %w", err)
	}

	return nil
}

// UploadMultipart uploads a large file using multipart upload
func (c *S3Client) UploadMultipart(ctx context.Context, localPath, remotePath string, partSize int64, onProgress func(uploaded, total int64)) error {
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	totalSize := info.Size()
	if partSize < 5*1024*1024 {
		partSize = 5 * 1024 * 1024 // Minimum part size is 5MB
	}

	key := c.fullPath(remotePath)

	// Start multipart upload
	createResp, err := c.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("create multipart upload: %w", err)
	}

	uploadID := createResp.UploadId
	var completedParts []types.CompletedPart
	var uploaded int64
	partNumber := int32(1)

	// Upload parts
	for uploaded < totalSize {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			c.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(c.bucket),
				Key:      aws.String(key),
				UploadId: uploadID,
			})
			return ctx.Err()
		default:
		}

		remaining := totalSize - uploaded
		currentPartSize := partSize
		if remaining < currentPartSize {
			currentPartSize = remaining
		}

		buffer := make([]byte, currentPartSize)
		n, err := f.Read(buffer)
		if err != nil && err != io.EOF {
			// Abort upload on error
			c.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(c.bucket),
				Key:      aws.String(key),
				UploadId: uploadID,
			})
			return fmt.Errorf("read file: %w", err)
		}

		partResp, err := c.client.UploadPart(ctx, &s3.UploadPartInput{
			Bucket:        aws.String(c.bucket),
			Key:           aws.String(key),
			UploadId:      uploadID,
			PartNumber:    aws.Int32(partNumber),
			Body:          bytes.NewReader(buffer[:n]),
			ContentLength: aws.Int64(int64(n)),
		})
		if err != nil {
			c.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket:   aws.String(c.bucket),
				Key:      aws.String(key),
				UploadId: uploadID,
			})
			return fmt.Errorf("upload part %d: %w", partNumber, err)
		}

		completedParts = append(completedParts, types.CompletedPart{
			ETag:       partResp.ETag,
			PartNumber: aws.Int32(partNumber),
		})

		uploaded += int64(n)
		if onProgress != nil {
			onProgress(uploaded, totalSize)
		}
		partNumber++
	}

	// Complete multipart upload
	_, err = c.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(c.bucket),
		Key:      aws.String(key),
		UploadId: uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completedParts,
		},
	})
	if err != nil {
		return fmt.Errorf("complete multipart upload: %w", err)
	}

	return nil
}

// Download downloads a file from S3
func (c *S3Client) Download(ctx context.Context, remotePath, localPath string) error {
	key := c.fullPath(remotePath)

	resp, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("get object: %w", err)
	}
	defer resp.Body.Close()

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	f, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("copy data: %w", err)
	}

	return nil
}

// Delete deletes an object from S3
func (c *S3Client) Delete(ctx context.Context, remotePath string) error {
	key := c.fullPath(remotePath)

	_, err := c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete object: %w", err)
	}

	return nil
}

// ListObjects lists objects with a given prefix
func (c *S3Client) ListObjects(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	key := c.fullPath(prefix)

	var objects []ObjectInfo
	paginator := s3.NewListObjectsV2Paginator(c.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucket),
		Prefix: aws.String(key),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list objects: %w", err)
		}

		for _, obj := range page.Contents {
			objects = append(objects, ObjectInfo{
				Key:          *obj.Key,
				Size:         *obj.Size,
				LastModified: *obj.LastModified,
				ETag:         strings.Trim(*obj.ETag, "\""),
			})
		}
	}

	return objects, nil
}

// ObjectInfo represents information about an S3 object
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified interface{}
	ETag         string
}

// Exists checks if an object exists
func (c *S3Client) Exists(ctx context.Context, remotePath string) (bool, error) {
	key := c.fullPath(remotePath)

	_, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// Check if it's a not found error
		var nsk *types.NotFound
		if ok := errors.As(err, &nsk); ok {
			return false, nil
		}
		return false, fmt.Errorf("head object: %w", err)
	}

	return true, nil
}

// GetObjectSize returns the size of an object
func (c *S3Client) GetObjectSize(ctx context.Context, remotePath string) (int64, error) {
	key := c.fullPath(remotePath)

	resp, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return 0, fmt.Errorf("head object: %w", err)
	}

	return *resp.ContentLength, nil
}
