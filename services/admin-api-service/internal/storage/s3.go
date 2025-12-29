// Package storage provides object storage operations for model files.
package storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Config holds S3 client configuration
type S3Config struct {
	Endpoint       string
	AccessKey      string
	SecretKey      string
	Bucket         string
	Region         string
	ForcePathStyle bool
}

// S3Client provides S3 operations for model files
type S3Client struct {
	client *s3.Client
	bucket string
}

// NewS3Client creates a new S3 client
func NewS3Client(ctx context.Context, cfg S3Config) (*S3Client, error) {
	// Create AWS credentials
	creds := credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")

	// Load AWS config
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(creds),
	)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	// Create S3 client with custom endpoint if provided
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.ForcePathStyle
	})

	return &S3Client{
		client: client,
		bucket: cfg.Bucket,
	}, nil
}

// MovePrefix moves all objects from oldPrefix to newPrefix
// This is used when renaming a model to migrate its cached files
func (c *S3Client) MovePrefix(ctx context.Context, oldPrefix, newPrefix string) (int64, int, error) {
	// Ensure prefixes end with /
	if !strings.HasSuffix(oldPrefix, "/") {
		oldPrefix += "/"
	}
	if !strings.HasSuffix(newPrefix, "/") {
		newPrefix += "/"
	}

	// List all objects with the old prefix
	objects, totalSize, err := c.listAllObjects(ctx, oldPrefix)
	if err != nil {
		return 0, 0, fmt.Errorf("list objects: %w", err)
	}

	if len(objects) == 0 {
		// No files to move
		return 0, 0, nil
	}

	// Copy each object to the new prefix
	for _, obj := range objects {
		oldKey := aws.ToString(obj.Key)
		// Replace old prefix with new prefix
		newKey := strings.Replace(oldKey, oldPrefix, newPrefix, 1)

		// Copy object
		_, err := c.client.CopyObject(ctx, &s3.CopyObjectInput{
			Bucket:     aws.String(c.bucket),
			CopySource: aws.String(fmt.Sprintf("%s/%s", c.bucket, oldKey)),
			Key:        aws.String(newKey),
		})
		if err != nil {
			return 0, 0, fmt.Errorf("copy object %s to %s: %w", oldKey, newKey, err)
		}
	}

	// Delete old objects after successful copy
	for _, obj := range objects {
		_, err := c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(c.bucket),
			Key:    obj.Key,
		})
		if err != nil {
			return 0, 0, fmt.Errorf("delete old object %s: %w", aws.ToString(obj.Key), err)
		}
	}

	return totalSize, len(objects), nil
}

// listAllObjects lists all objects with the given prefix
func (c *S3Client) listAllObjects(ctx context.Context, prefix string) ([]types.Object, int64, error) {
	var allObjects []types.Object
	var totalSize int64

	paginator := s3.NewListObjectsV2Paginator(c.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("list page: %w", err)
		}

		for _, obj := range page.Contents {
			allObjects = append(allObjects, obj)
			totalSize += aws.ToInt64(obj.Size)
		}
	}

	return allObjects, totalSize, nil
}

// PrefixExists checks if any objects exist with the given prefix
func (c *S3Client) PrefixExists(ctx context.Context, prefix string) (bool, error) {
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	result, err := c.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(c.bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(1),
	})
	if err != nil {
		return false, fmt.Errorf("list objects: %w", err)
	}

	return len(result.Contents) > 0, nil
}
