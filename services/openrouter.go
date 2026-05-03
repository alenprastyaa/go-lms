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

const defaultOpenRouterModel = "nvidia/nemotron-3-nano-omni-30b-a3b-reasoning:free"

const defaultOpenRouterAPIURL = "https://openrouter.ai/api/v1/chat/completions"

type openRouterCompletionResponse struct {
	Choices []openRouterChoice `json:"choices"`
}

type openRouterChoice struct {
	Message openRouterMessage `json:"message"`
	Text    json.RawMessage   `json:"text"`
}

type openRouterMessage struct {
	Content   json.RawMessage `json:"content"`
	Reasoning string          `json:"reasoning"`
}

func openRouterModel() string {
	if model := strings.TrimSpace(os.Getenv("OPENROUTER_MODEL")); model != "" {
		return model
	}
	return defaultOpenRouterModel
}

func openRouterAPIURL() string {
	if endpoint := strings.TrimSpace(os.Getenv("OPENROUTER_API_URL")); endpoint != "" {
		return endpoint
	}
	return defaultOpenRouterAPIURL
}

func normalizeOpenRouterError(message string) string {
	normalized := strings.TrimSpace(message)
	if normalized == "" {
		return "Request OpenRouter gagal"
	}

	lowered := strings.ToLower(normalized)
	switch {
	case strings.Contains(lowered, "user not found"):
		return "OPENROUTER_API_KEY tidak valid atau akun OpenRouter untuk API key ini tidak ditemukan"
	case strings.Contains(lowered, "invalid api key"), strings.Contains(lowered, "unauthorized"):
		return "OPENROUTER_API_KEY tidak valid atau akses OpenRouter ditolak"
	default:
		return normalized
	}
}

func callOpenRouter(prompt, systemMessage string, temperature float64) (string, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	if apiKey == "" {
		return "", fmt.Errorf("OPENROUTER_API_KEY belum diatur di server")
	}

	basePayload := map[string]interface{}{
		"model":       openRouterModel(),
		"temperature": temperature,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": systemMessage,
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	callProvider := func(payload map[string]interface{}) (*openRouterCompletionResponse, error) {
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterAPIURL(), bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("HTTP-Referer", strings.TrimSpace(os.Getenv("OPENROUTER_SITE_URL")))
		if strings.TrimSpace(req.Header.Get("HTTP-Referer")) == "" {
			req.Header.Set("HTTP-Referer", "http://localhost:8080")
		}
		req.Header.Set("X-Title", envOrDefault("OPENROUTER_APP_NAME", "School System"))

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var parsed openRouterCompletionResponse
		if err := json.Unmarshal(raw, &parsed); err != nil {
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return nil, fmt.Errorf("request OpenRouter gagal dengan status %d", resp.StatusCode)
			}
			return nil, err
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("%s", normalizeOpenRouterError(extractOpenRouterErrorMessage(raw, parsed)))
		}

		return &parsed, nil
	}

	payloadWithResponseFormat := cloneMap(basePayload)
	payloadWithResponseFormat["response_format"] = map[string]string{"type": "json_object"}

	var completion *openRouterCompletionResponse
	var err error
	completion, err = callProvider(payloadWithResponseFormat)
	if err != nil {
		completion, err = callProvider(basePayload)
		if err != nil {
			return "", err
		}
	}

	text := extractOpenRouterResponseText(completion)
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("OpenRouter tidak mengembalikan isi yang dapat diproses")
	}

	return text, nil
}

func cloneMap(src map[string]interface{}) map[string]interface{} {
	dst := make(map[string]interface{}, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func extractOpenRouterErrorMessage(raw []byte, parsed openRouterCompletionResponse) string {
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err == nil {
		if errObj, ok := payload["error"].(map[string]interface{}); ok {
			if msg, ok := errObj["message"].(string); ok && strings.TrimSpace(msg) != "" {
				return msg
			}
		}
		if msg, ok := payload["message"].(string); ok && strings.TrimSpace(msg) != "" {
			return msg
		}
	}

	if len(parsed.Choices) > 0 && strings.TrimSpace(parsed.Choices[0].Message.Reasoning) != "" {
		return parsed.Choices[0].Message.Reasoning
	}

	return "Request OpenRouter gagal"
}

func extractOpenRouterResponseText(parsed *openRouterCompletionResponse) string {
	if parsed == nil || len(parsed.Choices) == 0 {
		return ""
	}

	choice := parsed.Choices[0]
	if text := rawMessageToString(choice.Message.Content); strings.TrimSpace(text) != "" {
		return strings.TrimSpace(text)
	}

	if strings.TrimSpace(choice.Message.Reasoning) != "" {
		return strings.TrimSpace(choice.Message.Reasoning)
	}

	if text := rawMessageToString(choice.Text); strings.TrimSpace(text) != "" {
		return strings.TrimSpace(text)
	}

	return ""
}

func rawMessageToString(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return strings.TrimSpace(asString)
	}

	var array []map[string]interface{}
	if err := json.Unmarshal(raw, &array); err == nil {
		var builder strings.Builder
		for _, item := range array {
			if text, ok := item["text"].(string); ok {
				builder.WriteString(text)
			}
		}
		return strings.TrimSpace(builder.String())
	}

	var anyValue interface{}
	if err := json.Unmarshal(raw, &anyValue); err == nil {
		switch v := anyValue.(type) {
		case map[string]interface{}:
			if text, ok := v["text"].(string); ok {
				return strings.TrimSpace(text)
			}
		case []interface{}:
			var builder strings.Builder
			for _, item := range v {
				if mp, ok := item.(map[string]interface{}); ok {
					if text, ok := mp["text"].(string); ok {
						builder.WriteString(text)
					}
				}
			}
			return strings.TrimSpace(builder.String())
		}
	}

	return strings.TrimSpace(string(raw))
}
