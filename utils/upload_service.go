package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

// Layanan unggah berkas eksternal.
//
// Seluruh unggahan aplikasi diarahkan ke sini lebih dahulu. Bila layanan sedang
// bermasalah atau sengaja dimatikan lewat UPLOAD_API_URL kosong, unggahan
// kembali memakai R2 seperti sebelumnya.

const defaultUploadServiceURL = "https://upload-file.applicationservice.id/api/upload-file"

type uploadServiceResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
	Data    struct {
		Filename string `json:"filename"`
		URL      string `json:"url"`
		Size     int64  `json:"size"`
	} `json:"data"`
}

func uploadServiceURL() string {
	value, exists := os.LookupEnv("UPLOAD_API_URL")
	if !exists {
		return defaultUploadServiceURL
	}
	// Diisi kosong secara sengaja berarti layanan eksternal dimatikan.
	return strings.TrimSpace(value)
}

func uploadServiceTimeout() time.Duration {
	if value := strings.TrimSpace(os.Getenv("UPLOAD_API_TIMEOUT_SECONDS")); value != "" {
		var seconds int
		if _, err := fmt.Sscanf(value, "%d", &seconds); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return 60 * time.Second
}

// uploadBytesToService mengunggah berkas dengan key form "file" dan mengembalikan
// URL publik dari data.url pada respons layanan.
func uploadBytesToService(ctx context.Context, content []byte, originalName, contentType string) (string, error) {
	endpoint := uploadServiceURL()
	if endpoint == "" {
		return "", fmt.Errorf("layanan unggah eksternal tidak aktif")
	}
	if len(content) == 0 {
		return "", fmt.Errorf("berkas kosong")
	}

	name := strings.TrimSpace(originalName)
	if name == "" {
		name = "file"
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", name)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(content); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: uploadServiceTimeout()}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var parsed uploadServiceResponse
	_ = json.Unmarshal(raw, &parsed)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(parsed.Message)
		if message == "" {
			message = strings.TrimSpace(string(raw))
		}
		if len(message) > 300 {
			message = message[:300]
		}
		return "", fmt.Errorf("layanan unggah menolak berkas (status %d): %s", resp.StatusCode, message)
	}

	url := strings.TrimSpace(parsed.Data.URL)
	if url == "" {
		return "", fmt.Errorf("respons layanan unggah tidak memuat url")
	}
	return url, nil
}

// uploadBytes adalah pintu masuk tunggal seluruh unggahan berkas aplikasi.
func uploadBytes(ctx context.Context, content []byte, originalName, contentType string) (string, error) {
	if uploadServiceURL() != "" {
		url, err := uploadBytesToService(ctx, content, originalName, contentType)
		if err == nil {
			return url, nil
		}
		// Layanan eksternal gagal; lanjutkan ke R2 agar unggahan tetap berhasil.
		log.Printf("upload service gagal, beralih ke R2: %v", err)
	}
	return uploadBytesToR2(ctx, content, originalName, contentType)
}
