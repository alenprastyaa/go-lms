package utils

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
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

type UploadedAsset struct {
	URL         string
	FileName    string
	ContentType string
	Size        int
	PreviewURL  string
	PreviewName string
	PreviewType string
	PreviewSize int
}

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

func SaveUploadedChatAttachment(c *fiber.Ctx, fh *multipart.FileHeader) (*UploadedAsset, error) {
	file, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	contentType := strings.TrimSpace(fh.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	normalizedContent, normalizedName, normalizedType, err := normalizeChatAttachmentForUpload(content, fh.Filename, contentType)
	if err != nil {
		return nil, err
	}

	url, err := uploadBytesToR2(c.Context(), normalizedContent, normalizedName, normalizedType)
	if err != nil {
		return nil, err
	}

	asset := &UploadedAsset{
		URL:         url,
		FileName:    normalizedName,
		ContentType: normalizedType,
		Size:        len(normalizedContent),
	}

	if isPDFChatAttachment(normalizedType, normalizedName) {
		if previewContent, previewName, previewType, previewErr := generatePDFPreview(content, normalizedName); previewErr == nil {
			if previewURL, uploadErr := uploadBytesToR2(c.Context(), previewContent, previewName, previewType); uploadErr == nil {
				asset.PreviewURL = previewURL
				asset.PreviewName = previewName
				asset.PreviewType = previewType
				asset.PreviewSize = len(previewContent)
			}
		}
	}

	return asset, nil
}

func isPDFChatAttachment(contentType, fileName string) bool {
	if strings.Contains(strings.ToLower(strings.TrimSpace(contentType)), "application/pdf") {
		return true
	}
	return strings.HasSuffix(strings.ToLower(strings.TrimSpace(fileName)), ".pdf")
}

func generatePDFPreview(content []byte, originalName string) ([]byte, string, string, error) {
	if len(content) == 0 {
		return nil, "", "", fmt.Errorf("pdf content is empty")
	}

	tempDir, err := os.MkdirTemp("", "chat-pdf-preview-*")
	if err != nil {
		return nil, "", "", err
	}
	defer os.RemoveAll(tempDir)

	inputPath := filepath.Join(tempDir, "input.pdf")
	if err := os.WriteFile(inputPath, content, 0o600); err != nil {
		return nil, "", "", err
	}

	outputPrefix := filepath.Join(tempDir, "preview")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"pdftoppm",
		"-f", "1",
		"-l", "1",
		"-singlefile",
		"-jpeg",
		"-scale-to", "480",
		inputPath,
		outputPrefix,
	)
	if output, runErr := cmd.CombinedOutput(); runErr != nil {
		return nil, "", "", fmt.Errorf("failed to generate pdf preview: %w", runErr)
	} else if len(output) == 0 {
		// no-op
	}

	previewPath := outputPrefix + ".jpg"
	previewContent, err := os.ReadFile(previewPath)
	if err != nil {
		return nil, "", "", err
	}

	baseName := strings.TrimSuffix(strings.TrimSpace(filepath.Base(originalName)), filepath.Ext(originalName))
	if baseName == "" {
		baseName = "pdf-preview"
	}

	return previewContent, fmt.Sprintf("%s-preview.jpg", baseName), "image/jpeg", nil
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

func normalizeChatAttachmentForUpload(content []byte, originalName, contentType string) ([]byte, string, string, error) {
	if len(content) == 0 {
		return content, originalName, "application/octet-stream", nil
	}

	detectedType := strings.ToLower(strings.TrimSpace(contentType))
	if detectedType == "" || detectedType == "application/octet-stream" {
		detectedType = strings.ToLower(http.DetectContentType(content))
	}

	if !strings.HasPrefix(detectedType, "image/") {
		return content, originalName, detectedType, nil
	}

	baseName := strings.TrimSuffix(strings.TrimSpace(filepath.Base(originalName)), filepath.Ext(originalName))
	if baseName == "" {
		baseName = "image"
	}

	switch {
	case strings.Contains(detectedType, "image/webp"):
		return content, fmt.Sprintf("%s.webp", baseName), "image/webp", nil
	case strings.Contains(detectedType, "image/png"):
		normalized, err := encodeChatImage(content, "png")
		if err != nil {
			return nil, "", "", err
		}
		return normalized, fmt.Sprintf("%s.png", baseName), "image/png", nil
	case strings.Contains(detectedType, "image/gif"):
		normalized, err := encodeChatImage(content, "png")
		if err != nil {
			return nil, "", "", err
		}
		return normalized, fmt.Sprintf("%s.png", baseName), "image/png", nil
	case strings.Contains(detectedType, "image/jpeg"), strings.Contains(detectedType, "image/jpg"):
		normalized, err := encodeChatImage(content, "jpeg")
		if err != nil {
			return nil, "", "", err
		}
		return normalized, fmt.Sprintf("%s.jpg", baseName), "image/jpeg", nil
	default:
		img, _, err := image.Decode(bytes.NewReader(content))
		if err != nil {
			return nil, "", "", fmt.Errorf("format gambar chat tidak didukung server. Gunakan JPG, PNG, atau WebP")
		}
		if hasTransparency(img) {
			normalized, encodeErr := encodeImageByFormat(img, "png")
			if encodeErr != nil {
				return nil, "", "", encodeErr
			}
			return normalized, fmt.Sprintf("%s.png", baseName), "image/png", nil
		}
		normalized, encodeErr := encodeImageByFormat(img, "jpeg")
		if encodeErr != nil {
			return nil, "", "", encodeErr
		}
		return normalized, fmt.Sprintf("%s.jpg", baseName), "image/jpeg", nil
	}
}

func encodeChatImage(content []byte, format string) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("format gambar chat tidak didukung server. Gunakan JPG, PNG, atau WebP")
	}
	return encodeImageByFormat(img, format)
}

func encodeImageByFormat(img image.Image, format string) ([]byte, error) {
	var buf bytes.Buffer

	switch format {
	case "png":
		encoder := png.Encoder{CompressionLevel: png.BestCompression}
		if err := encoder.Encode(&buf, img); err != nil {
			return nil, err
		}
	case "jpeg":
		if err := jpeg.Encode(&buf, flattenImageIfNeeded(img), &jpeg.Options{Quality: 86}); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("format output gambar tidak didukung")
	}

	return buf.Bytes(), nil
}

func hasTransparency(img image.Image) bool {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 1 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 1 {
			_, _, _, alpha := img.At(x, y).RGBA()
			if alpha < 0xffff {
				return true
			}
		}
	}
	return false
}

func flattenImageIfNeeded(img image.Image) image.Image {
	if !hasTransparency(img) {
		return img
	}

	bounds := img.Bounds()
	canvas := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 1 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 1 {
			canvas.Set(x, y, color.White)
		}
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y += 1 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 1 {
			canvas.Set(x, y, img.At(x, y))
		}
	}

	return canvas
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
