package utils

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gofiber/fiber/v2"
)

func SaveUploadedFile(c *fiber.Ctx, fh *multipart.FileHeader) (string, error) {
	remoteURL, err := UploadToR2(c, fh)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(remoteURL) == "" {
		return "", fmt.Errorf("upload response did not contain file url")
	}
	return remoteURL, nil
}

func UploadLocalFileToAlentest(filePath, fileName, mimeType string) (string, error) {
	baseName := strings.TrimSpace(fileName)
	if baseName == "" {
		baseName = filepath.Base(filePath)
	}
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	return uploadBytesToR2(context.Background(), content, baseName, mimeType)
}

func UploadToR2(c *fiber.Ctx, fh *multipart.FileHeader) (string, error) {
	file, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	contentType := fh.Header.Get("Content-Type")
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}
	content, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	return uploadBytesToR2(c.Context(), content, fh.Filename, contentType)
}

func uploadBytesToR2(ctx context.Context, content []byte, originalName, contentType string) (string, error) {
	endpoint := strings.TrimSpace(os.Getenv("R2_ENDPOINT"))
	bucket := strings.TrimSpace(os.Getenv("R2_BUCKET"))
	accessKey := strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID"))
	secretKey := strings.TrimSpace(os.Getenv("R2_SECRET_ACCESS_KEY"))
	publicBase := strings.TrimSpace(os.Getenv("R2_PUBLIC_BASE_URL"))

	if endpoint == "" || bucket == "" || accessKey == "" || secretKey == "" {
		return "", fmt.Errorf("missing R2 config: set R2_ENDPOINT, R2_BUCKET, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY")
	}

	cfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion("auto"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return "", err
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	compressed, resolvedType := maybeCompressUpload(content, contentType)
	key := buildObjectKey(originalName)
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(compressed),
		ContentType: aws.String(resolvedType),
	})
	if err != nil {
		return "", err
	}

	if publicBase != "" {
		return strings.TrimRight(publicBase, "/") + "/" + key, nil
	}
	return strings.TrimRight(endpoint, "/") + "/" + bucket + "/" + key, nil
}

func maybeCompressUpload(content []byte, contentType string) ([]byte, string) {
	if len(content) == 0 {
		return content, "application/octet-stream"
	}

	detectedType := strings.ToLower(strings.TrimSpace(contentType))
	if detectedType == "" || detectedType == "application/octet-stream" {
		detectedType = strings.ToLower(http.DetectContentType(content))
	}

	switch {
	case strings.Contains(detectedType, "image/jpeg"), strings.Contains(detectedType, "image/jpg"):
		optimized, ok := recompressJPEG(content)
		if ok {
			return optimized, "image/jpeg"
		}
	case strings.Contains(detectedType, "image/png"):
		optimized, ok := recompressPNG(content)
		if ok {
			return optimized, "image/png"
		}
	case strings.Contains(detectedType, "image/gif"):
		optimized, ok := recompressGIF(content)
		if ok {
			return optimized, "image/gif"
		}
	}

	if detectedType == "" {
		detectedType = "application/octet-stream"
	}
	return content, detectedType
}

func recompressJPEG(content []byte) ([]byte, bool) {
	img, _, err := image.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, false
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 72}); err != nil {
		return nil, false
	}
	if buf.Len() >= len(content) {
		return nil, false
	}
	return buf.Bytes(), true
}

func recompressPNG(content []byte) ([]byte, bool) {
	img, _, err := image.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, false
	}
	var buf bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	if err := encoder.Encode(&buf, img); err != nil {
		return nil, false
	}
	if buf.Len() >= len(content) {
		return nil, false
	}
	return buf.Bytes(), true
}

func recompressGIF(content []byte) ([]byte, bool) {
	img, err := gif.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, false
	}
	var buf bytes.Buffer
	if err := gif.Encode(&buf, img, nil); err != nil {
		return nil, false
	}
	if buf.Len() >= len(content) {
		return nil, false
	}
	return buf.Bytes(), true
}

var nonAlphaNum = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func buildObjectKey(name string) string {
	base := strings.TrimSpace(filepath.Base(name))
	if base == "" || base == "." || base == "/" {
		base = "file"
	}
	safe := strings.ToLower(base)
	safe = strings.ReplaceAll(safe, " ", "-")
	safe = nonAlphaNum.ReplaceAllString(safe, "-")
	safe = strings.Trim(safe, "-")
	if safe == "" {
		safe = "file"
	}
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), safe)
}
