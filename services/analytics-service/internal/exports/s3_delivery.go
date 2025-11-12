// Package exports provides Linode Object Storage (S3-compatible) adapter for CSV delivery.
package exports

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// S3Delivery handles CSV uploads to Linode Object Storage (S3-compatible) and signed URL generation.
type S3Delivery struct {
	client     *s3.Client
	bucket     string
	region     string
	signedURLTTL time.Duration
	logger     *zap.Logger
}

// NewS3Delivery creates a new Linode Object Storage delivery adapter.
// Linode Object Storage is S3-compatible and uses the AWS SDK v2.
func NewS3Delivery(endpoint, accessKey, secretKey, bucket, region string, signedURLTTL time.Duration, logger *zap.Logger) (*S3Delivery, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	// Override endpoint for Linode Object Storage (S3-compatible)
	if endpoint != "" {
		cfg.BaseEndpoint = aws.String(endpoint)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.UsePathStyle = true // Required for Linode Object Storage
		}
	})

	return &S3Delivery{
		client:      client,
		bucket:      bucket,
		region:      region,
		signedURLTTL: signedURLTTL,
		logger:      logger,
	}, nil
}

// UploadCSV uploads CSV data to S3 and returns the signed URL and checksum.
func (s *S3Delivery) UploadCSV(ctx context.Context, orgID, jobID uuid.UUID, csvData []byte) (string, string, error) {
	// Calculate SHA-256 checksum
	hash := sha256.Sum256(csvData)
	checksum := hex.EncodeToString(hash[:])

	// Generate object key: analytics/exports/{org_id}/{job_id}.csv
	key := fmt.Sprintf("analytics/exports/%s/%s.csv", orgID.String(), jobID.String())

	// Upload to Linode Object Storage
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(csvData),
		ContentType:   aws.String("text/csv"),
		ContentLength: aws.Int64(int64(len(csvData))),
		Metadata: map[string]string{
			"checksum": checksum,
			"org-id":   orgID.String(),
			"job-id":   jobID.String(),
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("upload CSV to Linode Object Storage: %w", err)
	}

	// Generate signed URL
	signedURL, err := s.GenerateSignedURL(ctx, key)
	if err != nil {
		return "", "", fmt.Errorf("generate signed URL: %w", err)
	}

	s.logger.Info("uploaded CSV to Linode Object Storage",
		zap.String("org_id", orgID.String()),
		zap.String("job_id", jobID.String()),
		zap.String("key", key),
		zap.String("checksum", checksum),
		zap.Int("size_bytes", len(csvData)),
	)

	return signedURL, checksum, nil
}

// GenerateSignedURL generates a presigned GET URL for downloading an object from Linode Object Storage.
func (s *S3Delivery) GenerateSignedURL(ctx context.Context, key string) (string, error) {
	presigner := s3.NewPresignClient(s.client)
	
	getRequest, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = s.signedURLTTL
	})
	if err != nil {
		return "", fmt.Errorf("presign get request: %w", err)
	}

	return getRequest.URL, nil
}

