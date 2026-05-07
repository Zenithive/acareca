package upload

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/google/uuid"
	"github.com/iamarpitzala/acareca/pkg/config"
)

type R2StorageProvider struct {
	client    *s3.S3
	bucket    string
	publicURL string
	config    config.Config
	creds     *credentials.Credentials
	endpoint  string
}

func NewR2StorageProvider(cfg *config.Config) (*R2StorageProvider, error) {

	// R2 endpoint format: https://<account_id>.r2.cloudflarestorage.com
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.R2AccountID)

	creds := credentials.NewStaticCredentials(cfg.R2AccessKeyID, cfg.R2SecretAccessKey, "")

	// Create AWS session for R2
	sess, err := session.NewSession(&aws.Config{
		Region:           aws.String("auto"),
		Endpoint:         aws.String(endpoint),
		Credentials:      creds,
		S3ForcePathStyle: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("create R2 session: %w", err)
	}

	return &R2StorageProvider{
		client:    s3.New(sess),
		bucket:    cfg.R2BucketName,
		publicURL: cfg.R2PublicURL,
		creds:     creds,
		endpoint:  endpoint,
	}, nil
}

// Upload uploads a file to R2
func (p *R2StorageProvider) Upload(ctx context.Context, file multipart.File, header *multipart.FileHeader, objectKey string) (string, string, error) {
	hash := sha256.New()
	teeReader := io.TeeReader(file, hash)

	content, err := io.ReadAll(teeReader)
	if err != nil {
		return "", "", fmt.Errorf("read file content: %w", err)
	}

	_, err = p.client.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(p.bucket),
		Key:         aws.String(objectKey),
		Body:        aws.ReadSeekCloser(bytes.NewReader(content)),
		ContentType: aws.String(header.Header.Get("Content-Type")),
		Metadata: map[string]*string{
			"original-filename": aws.String(header.Filename),
			"upload-date":       aws.String(time.Now().Format(time.RFC3339)),
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("upload to R2: %w", err)
	}

	checksum := hex.EncodeToString(hash.Sum(nil))
	return objectKey, checksum, nil
}

// Download retrieves a file from R2
func (p *R2StorageProvider) Download(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	result, err := p.client.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return nil, fmt.Errorf("download from R2: %w", err)
	}

	return result.Body, nil
}

// Delete removes a file from R2
func (p *R2StorageProvider) Delete(ctx context.Context, objectKey string) error {
	_, err := p.client.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("delete from R2: %w", err)
	}

	return nil
}

// GetURL returns the public URL for accessing a file
func (p *R2StorageProvider) GetURL(objectKey string) string {
	if p.publicURL != "" {
		return fmt.Sprintf("%s/%s", p.publicURL, objectKey)
	}
	return ""
}

// GeneratePresignedURL generates a presigned URL for temporary access
func (p *R2StorageProvider) GeneratePresignedURL(objectKey string, expiresIn time.Duration) (string, error) {
	req, _ := p.client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(objectKey),
	})

	url, err := req.Presign(expiresIn)
	if err != nil {
		return "", fmt.Errorf("generate presigned URL: %w", err)
	}

	return url, nil
}

// GeneratePresignedUploadURL generates a presigned URL for direct client upload to R2.
//
// R2 requires x-amz-content-sha256=UNSIGNED-PAYLOAD to be present as a signed
// query parameter. The AWS SDK does not add it automatically for presigned PUT
// requests, so we inject it into the request's query string *before* calling
// req.Presign() so it is included in the string-to-sign and therefore part of
// the final signature. Setting it as a header after presigning does not work
// because headers are not transmitted by the client in a direct presigned upload.
func (p *R2StorageProvider) GeneratePresignedUploadURL(objectKey string, contentType string, expiresIn time.Duration) (string, error) {
	req, _ := p.client.PutObjectRequest(&s3.PutObjectInput{
		Bucket:      aws.String(p.bucket),
		Key:         aws.String(objectKey),
		ContentType: aws.String(contentType),
	})

	// Inject x-amz-content-sha256 into the query string before signing so
	// R2 receives it as a signed parameter in the presigned URL.
	q := req.HTTPRequest.URL.Query()
	q.Set("x-amz-content-sha256", "UNSIGNED-PAYLOAD")
	req.HTTPRequest.URL.RawQuery = q.Encode()

	url, err := req.Presign(expiresIn)
	if err != nil {
		return "", fmt.Errorf("generate presigned upload URL: %w", err)
	}

	return url, nil
}

// GenerateObjectKey generates a unique object key for a file
func (p *R2StorageProvider) GenerateObjectKey(ownerID uuid.UUID, filename string) string {
	ext := filepath.Ext(filename)

	timestamp := time.Now().Format("20060102")
	uniqueID := uuid.New().String()[:8]

	return fmt.Sprintf("%s/%s/%s%s", ownerID.String(), timestamp, uniqueID, ext)
}

// HeadObject checks if an object exists in R2 and returns its size in bytes
func (p *R2StorageProvider) HeadObject(ctx context.Context, objectKey string) (int64, error) {
	result, err := p.client.HeadObjectWithContext(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return 0, fmt.Errorf("head object: %w", err)
	}

	if result.ContentLength == nil {
		return 0, nil
	}
	return *result.ContentLength, nil
}

// ListObjects lists objects in R2 with a prefix
func (p *R2StorageProvider) ListObjects(ctx context.Context, prefix string, maxKeys int64) ([]*s3.Object, error) {
	result, err := p.client.ListObjectsV2WithContext(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(p.bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int64(maxKeys),
	})
	if err != nil {
		return nil, fmt.Errorf("list objects: %w", err)
	}

	return result.Contents, nil
}

// CopyObject copies an object within R2
func (p *R2StorageProvider) CopyObject(ctx context.Context, sourceKey, destKey string) error {
	_, err := p.client.CopyObjectWithContext(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(p.bucket),
		CopySource: aws.String(fmt.Sprintf("%s/%s", p.bucket, sourceKey)),
		Key:        aws.String(destKey),
	})
	if err != nil {
		return fmt.Errorf("copy object: %w", err)
	}

	return nil
}
