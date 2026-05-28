package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"bionicpro-reports/internal/config"
)

type S3 struct {
	client      *minio.Client
	bucket      string
	cdnBaseURL  string
	dataVersion string
	cacheCtl    string
}

func NewS3(cfg config.Config) (*S3, error) {
	client, err := minio.New(cfg.S3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		Secure: cfg.S3UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("init s3 client: %w", err)
	}

	return &S3{
		client:      client,
		bucket:      cfg.S3Bucket,
		cdnBaseURL:  strings.TrimRight(cfg.CDNBaseURL, "/"),
		dataVersion: cfg.DataVersion,
		cacheCtl:    fmt.Sprintf("public, max-age=%d", cfg.CDNMaxAgeSec),
	}, nil
}

func (s *S3) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("check bucket: %w", err)
	}
	if exists {
		return s.ensureReadPolicy(ctx)
	}
	if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("create bucket: %w", err)
	}
	return s.ensureReadPolicy(ctx)
}

func (s *S3) ObjectKey(userID string) string {
	sum := sha256.Sum256([]byte(userID))
	return path.Join(s.dataVersion, hex.EncodeToString(sum[:])+".json")
}

func (s *S3) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err == nil {
		return true, nil
	}
	resp := minio.ToErrorResponse(err)
	if resp.Code == "NoSuchKey" || resp.StatusCode == 404 {
		return false, nil
	}
	return false, fmt.Errorf("stat object: %w", err)
}

func (s *S3) SaveReport(ctx context.Context, key string, payload []byte) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(payload), int64(len(payload)), minio.PutObjectOptions{
		ContentType:  "application/json",
		CacheControl: s.cacheCtl,
	})
	if err != nil {
		return fmt.Errorf("put object: %w", err)
	}
	return nil
}

func (s *S3) CDNURL(key string) string {
	return s.cdnBaseURL + "/" + key
}

func (s *S3) ensureReadPolicy(ctx context.Context) error {
	// Nginx CDN reads objects anonymously from MinIO.
	policy := fmt.Sprintf(`{
  "Version":"2012-10-17",
  "Statement":[
    {
      "Effect":"Allow",
      "Principal":{"AWS":["*"]},
      "Action":["s3:GetBucketLocation","s3:ListBucket"],
      "Resource":["arn:aws:s3:::%s"]
    },
    {
      "Effect":"Allow",
      "Principal":{"AWS":["*"]},
      "Action":["s3:GetObject"],
      "Resource":["arn:aws:s3:::%s/*"]
    }
  ]
}`, s.bucket, s.bucket)
	if err := s.client.SetBucketPolicy(ctx, s.bucket, policy); err != nil {
		return fmt.Errorf("set bucket policy: %w", err)
	}
	return nil
}
