package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Config mirrors citenexus's storage/location.py S3 location: credentials are
// referenced by env-var NAME, never a value — the value is resolved only at
// connect time and never stored on this struct, so it's always safe to log/repr.
type S3Config struct {
	Bucket       string
	Prefix       string
	EndpointURL  string // optional, e.g. http://localhost:9000 for MinIO
	Region       string // default "us-east-1"
	AccessKeyEnv string // default "AWS_ACCESS_KEY_ID"
	SecretKeyEnv string // default "AWS_SECRET_ACCESS_KEY"
}

// S3Backend implements Backend against an S3-compatible endpoint.
type S3Backend struct {
	client *s3.Client
	bucket string
	prefix string
}

// NewS3Backend resolves credentials from the configured env-var names and
// builds a client. Falls back to the default AWS credential chain if the env
// vars aren't set.
func NewS3Backend(ctx context.Context, cfg S3Config) (*S3Backend, error) {
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}
	accessKeyEnv := cfg.AccessKeyEnv
	if accessKeyEnv == "" {
		accessKeyEnv = "AWS_ACCESS_KEY_ID"
	}
	secretKeyEnv := cfg.SecretKeyEnv
	if secretKeyEnv == "" {
		secretKeyEnv = "AWS_SECRET_ACCESS_KEY"
	}

	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region)}
	if ak, sk := os.Getenv(accessKeyEnv), os.Getenv(secretKeyEnv); ak != "" && sk != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(ak, sk, "")))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.EndpointURL != "" {
			o.BaseEndpoint = aws.String(cfg.EndpointURL)
			o.UsePathStyle = true // MinIO / non-AWS endpoints need path-style
		}
	})

	return &S3Backend{client: client, bucket: cfg.Bucket, prefix: cfg.Prefix}, nil
}

func (s *S3Backend) key(k string) string {
	if s.prefix == "" {
		return k
	}
	return strings.TrimSuffix(s.prefix, "/") + "/" + k
}

func (s *S3Backend) PutBytes(key string, data []byte) error {
	_, err := s.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(key)),
		Body:   bytes.NewReader(data),
	})
	return err
}

func (s *S3Backend) GetBytes(key string) ([]byte, error) {
	out, err := s.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(key)),
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	return io.ReadAll(out.Body)
}

func (s *S3Backend) Exists(key string) bool {
	_, err := s.client.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(key)),
	})
	return err == nil
}

func (s *S3Backend) ListPrefix(prefix string) ([]string, error) {
	ctx := context.Background()
	var keys []string
	fullPrefix := s.key(prefix)
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(fullPrefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			k := aws.ToString(obj.Key)
			if s.prefix != "" {
				k = strings.TrimPrefix(k, strings.TrimSuffix(s.prefix, "/")+"/")
			}
			keys = append(keys, k)
		}
	}
	return keys, nil
}

func (s *S3Backend) DeletePrefix(prefix string) error {
	keys, err := s.ListPrefix(prefix)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	ctx := context.Background()
	var objIDs []types.ObjectIdentifier
	for _, k := range keys {
		objIDs = append(objIDs, types.ObjectIdentifier{Key: aws.String(s.key(k))})
	}
	const batchSize = 1000
	for i := 0; i < len(objIDs); i += batchSize {
		end := min(i+batchSize, len(objIDs))
		_, err := s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(s.bucket),
			Delete: &types.Delete{Objects: objIDs[i:end]},
		})
		if err != nil {
			return err
		}
	}
	return nil
}
