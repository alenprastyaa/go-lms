package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func SaveUploadedFile(c *fiber.Ctx, fh *multipart.FileHeader) (string, error) {
	remoteURL, err := UploadToAlentest(c, fh)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(remoteURL) == "" {
		return "", fmt.Errorf("upload response did not contain file url")
	}
	return remoteURL, nil
}

func UploadLocalFileToAlentest(filePath, fileName, mimeType string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	baseName := strings.TrimSpace(fileName)
	if baseName == "" {
		baseName = filepath.Base(filePath)
	}
	if strings.TrimSpace(mimeType) == "" {
		mimeType = "application/octet-stream"
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", baseName)
	if err != nil {
		return "", err
	}
	if _, err = io.Copy(part, file); err != nil {
		return "", err
	}
	if err = writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, "https://alentest.my.id/file/api/upload-file", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("remote upload failed with status %d", resp.StatusCode)
	}

	url := extractUploadedURL(raw)
	if strings.TrimSpace(url) == "" {
		return "", fmt.Errorf("upload response did not contain file url")
	}
	return url, nil
}

func UploadToAlentest(c *fiber.Ctx, fh *multipart.FileHeader) (string, error) {
	file, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", fh.Filename)
	if err != nil {
		return "", err
	}
	if _, err = io.Copy(part, file); err != nil {
		return "", err
	}
	if err = writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, "https://alentest.my.id/file/api/upload-file", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("remote upload failed with status %d", resp.StatusCode)
	}

	url := extractUploadedURL(raw)
	if strings.TrimSpace(url) == "" {
		return "", fmt.Errorf("upload response did not contain file url")
	}
	return url, nil
}

func extractUploadedURL(raw []byte) string {
	var any map[string]interface{}
	if err := json.Unmarshal(raw, &any); err != nil {
		return ""
	}
	return findURL(any)
}

func findURL(v interface{}) string {
	switch t := v.(type) {
	case string:
		if strings.HasPrefix(t, "http://") || strings.HasPrefix(t, "https://") {
			return t
		}
		return ""
	case map[string]interface{}:
		for _, key := range []string{"url", "path"} {
			if s, ok := t[key].(string); ok && strings.TrimSpace(s) != "" {
				return s
			}
		}
		for _, key := range []string{"data", "result"} {
			if nested, ok := t[key]; ok {
				if url := findURL(nested); url != "" {
					return url
				}
			}
		}
	case []interface{}:
		for _, item := range t {
			if url := findURL(item); url != "" {
				return url
			}
		}
	}
	return ""
}
