package objectstorage

import (
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Client provides the application with an S3-compatible object store for
// profile photos. The photo upload endpoint will use this client in ticket 4.
type Client struct {
	client *minio.Client
	bucket string
}

func New(ctx context.Context, endpoint, accessKey, secretKey, bucket string, useSSL bool) (*Client, error) {
	if endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		return nil, fmt.Errorf("object storage endpoint, credentials, and bucket are required")
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure:       useSSL,
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		return nil, fmt.Errorf("create MinIO client: %w", err)
	}

	if err := ensureBucket(ctx, client, bucket); err != nil {
		return nil, err
	}

	return &Client{client: client, bucket: bucket}, nil
}

// MinIO can take a few seconds to become ready after its container starts.
// Retrying here keeps startup reliable without depending on shell utilities
// inside the storage image for a Docker healthcheck.
func ensureBucket(ctx context.Context, client *minio.Client, bucket string) error {
	for {
		exists, err := client.BucketExists(ctx, bucket)
		if err == nil {
			if exists {
				return nil
			}
			makeErr := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
			if makeErr == nil {
				return nil
			}
			err = fmt.Errorf("create photo bucket: %w", makeErr)
		} else {
			err = fmt.Errorf("check photo bucket: %w", err)
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("initialize photo bucket: %w", err)
		case <-time.After(time.Second):
		}
	}
}

func (c *Client) Bucket() string {
	return c.bucket
}
