package filestorage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type SaveOptions struct {
	Prefix      string
	MaxSize     int64
	AllowedExts map[string]bool
}

func SaveUploadedFile(ctx context.Context, file *multipart.FileHeader, opts SaveOptions) (string, error) {
	if file.Size > opts.MaxSize {
		return "", fmt.Errorf("%s file must be %dMB or less", opts.Prefix, opts.MaxSize/(1024*1024))
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !opts.AllowedExts[ext] {
		return "", errors.New("file type is not allowed")
	}

	fileName := randomHex(16) + ext
	return saveToS3(ctx, file, opts.Prefix, fileName)
}

func saveToS3(ctx context.Context, file *multipart.FileHeader, prefix string, fileName string) (string, error) {
	endpoint := strings.TrimSpace(os.Getenv("S3_ENDPOINT"))
	accessKey := strings.TrimSpace(os.Getenv("S3_ACCESS_KEY"))
	secretKey := strings.TrimSpace(os.Getenv("S3_SECRET_KEY"))
	bucket := strings.TrimSpace(os.Getenv("S3_BUCKET"))
	if endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		return "", errors.New("S3_ENDPOINT, S3_ACCESS_KEY, S3_SECRET_KEY and S3_BUCKET are required")
	}

	useSSL := strings.EqualFold(os.Getenv("S3_USE_SSL"), "true")
	region := strings.TrimSpace(os.Getenv("S3_REGION"))
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
		Region: region,
	})
	if err != nil {
		return "", err
	}

	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return "", err
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: region}); err != nil {
			return "", err
		}
	}
	if strings.EqualFold(os.Getenv("S3_PUBLIC_READ"), "true") {
		if err := client.SetBucketPolicy(ctx, bucket, publicReadPolicy(bucket)); err != nil {
			return "", err
		}
	}

	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	key := strings.Trim(prefix, "/") + "/" + fileName
	_, err = client.PutObject(ctx, bucket, key, src, file.Size, minio.PutObjectOptions{
		ContentType: file.Header.Get("Content-Type"),
	})
	if err != nil {
		return "", err
	}

	return publicURL(endpoint, bucket, key, useSSL), nil
}

func publicURL(endpoint, bucket, key string, useSSL bool) string {
	base := strings.TrimRight(os.Getenv("S3_PUBLIC_URL"), "/")
	if base != "" {
		return base + "/" + escapeKey(key)
	}
	scheme := "http"
	if useSSL {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/%s/%s", scheme, endpoint, bucket, escapeKey(key))
}

func escapeKey(key string) string {
	parts := strings.Split(key, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func publicReadPolicy(bucket string) string {
	return fmt.Sprintf(`{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {"AWS": ["*"]},
      "Action": ["s3:GetObject"],
      "Resource": ["arn:aws:s3:::%s/*"]
    }
  ]
}`, bucket)
}

func randomHex(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}
