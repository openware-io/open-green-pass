package infra

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// EvidenceObjectStore wraps the S3-compatible store used for immutable
// evidence bodies. Metadata stays in PostgreSQL; this only owns objects.
type EvidenceObjectStore struct {
	client *minio.Client
	bucket string
}

func NewEvidenceObjectStore(endpoint, accessKey, secretKey, bucket string) (*EvidenceObjectStore, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return nil, errors.New("object store endpoint is empty")
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "http://" + endpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("invalid object store endpoint: %w", err)
	}
	bucket = strings.TrimSpace(bucket)
	if bucket == "" {
		bucket = "gp-evidence"
	}
	client, err := minio.New(parsed.Host, &minio.Options{
		Creds:        credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure:       parsed.Scheme == "https",
		BucketLookup: minio.BucketLookupPath,
		Region:       "us-east-1",
	})
	if err != nil {
		return nil, err
	}
	return &EvidenceObjectStore{client: client, bucket: bucket}, nil
}

func (s *EvidenceObjectStore) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{Region: "us-east-1"})
}

func (s *EvidenceObjectStore) PresignPut(ctx context.Context, key string, expiry time.Duration) (string, error) {
	u, err := s.client.PresignedPutObject(ctx, s.bucket, key, expiry)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (s *EvidenceObjectStore) PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error) {
	u, err := s.client.PresignedGetObject(ctx, s.bucket, key, expiry, nil)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (s *EvidenceObjectStore) Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, reader, size, minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (s *EvidenceObjectStore) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	return s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
}

func (s *EvidenceObjectStore) URI(key string) string {
	return "s3://" + s.bucket + "/" + strings.TrimPrefix(key, "/")
}
