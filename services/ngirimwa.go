package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultNgirimWABaseURL = "https://dash.ngirimwa.com/api/v1"

type ngirimWASendMessageRequest struct {
	To      string `json:"to"`
	Message string `json:"message"`
}

type ngirimWASendMessageResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
	Error   interface{} `json:"error"`
}

func ngirimWAAPIKey() string {
	return strings.TrimSpace(os.Getenv("NGIRIMWA_API_KEY"))
}

func ngirimWABaseURL() string {
	if value := strings.TrimSpace(os.Getenv("NGIRIMWA_BASE_URL")); value != "" {
		return strings.TrimRight(value, "/")
	}
	return defaultNgirimWABaseURL
}

func SendWhatsAppMessage(to, message string) (*ngirimWASendMessageResponse, error) {
	apiKey := ngirimWAAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("NGIRIMWA_API_KEY belum diatur di server")
	}

	payload := ngirimWASendMessageRequest{
		To:      strings.TrimSpace(to),
		Message: strings.TrimSpace(message),
	}
	if payload.To == "" {
		return nil, fmt.Errorf("nomor tujuan WhatsApp kosong")
	}
	if payload.Message == "" {
		return nil, fmt.Errorf("isi pesan WhatsApp kosong")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ngirimWABaseURL()+"/messages/send", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var parsed ngirimWASendMessageResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("request NgirimWA gagal dengan status %d", resp.StatusCode)
		}
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !parsed.Success {
		errMessage := strings.TrimSpace(parsed.Message)
		if errMessage == "" {
			errMessage = normalizeNgirimWAError(parsed.Error)
		}
		if errMessage == "" {
			errMessage = fmt.Sprintf("request NgirimWA gagal dengan status %d", resp.StatusCode)
		}
		return &parsed, fmt.Errorf("%s", errMessage)
	}

	return &parsed, nil
}

func normalizeNgirimWAError(raw interface{}) string {
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	case map[string]interface{}:
		if msg, ok := v["message"].(string); ok {
			return strings.TrimSpace(msg)
		}
		if errValue, ok := v["error"].(string); ok {
			return strings.TrimSpace(errValue)
		}
	}
	return ""
}
