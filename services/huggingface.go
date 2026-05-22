package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultHuggingFaceModel = "Qwen/Qwen2.5-7B-Instruct"

const defaultHuggingFaceAPIURL = "https://router.huggingface.co/v1/chat/completions"

type huggingFaceCompletionResponse struct {
	Choices []huggingFaceChoice `json:"choices"`
	Error   *huggingFaceError   `json:"error"`
	Usage   huggingFaceUsage    `json:"usage"`
}

type huggingFaceChoice struct {
	Message huggingFaceMessage `json:"message"`
	Text    json.RawMessage    `json:"text"`
}

type huggingFaceMessage struct {
	Content json.RawMessage `json:"content"`
	Refusal string          `json:"refusal"`
}

type huggingFaceError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

type huggingFaceUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func huggingFaceAPIKey() string {
	for _, key := range []string{"HF_API_KEY", "HUGGINGFACE_API_KEY", "HF_TOKEN"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func huggingFaceModel() string {
	if model := strings.TrimSpace(os.Getenv("HUGGINGFACE_MODEL")); model != "" {
		return model
	}
	if model := strings.TrimSpace(os.Getenv("HF_MODEL")); model != "" {
		return model
	}
	return defaultHuggingFaceModel
}

func huggingFaceAPIURL() string {
	if endpoint := strings.TrimSpace(os.Getenv("HUGGINGFACE_API_URL")); endpoint != "" {
		return endpoint
	}
	if endpoint := strings.TrimSpace(os.Getenv("HF_API_URL")); endpoint != "" {
		return endpoint
	}
	return defaultHuggingFaceAPIURL
}

func normalizeHuggingFaceError(message string) string {
	normalized := strings.TrimSpace(message)
	if normalized == "" {
		return "Request Hugging Face gagal"
	}

	lowered := strings.ToLower(normalized)
	switch {
	case strings.Contains(lowered, "invalid token"), strings.Contains(lowered, "authentication"), strings.Contains(lowered, "unauthorized"):
		return "HF_API_KEY tidak valid atau akses Hugging Face ditolak"
	case strings.Contains(lowered, "insufficient"), strings.Contains(lowered, "permission"):
		return "HF_API_KEY tidak memiliki izin untuk memakai Inference Providers di Hugging Face"
	default:
		return normalized
	}
}

func callHuggingFace(prompt, systemMessage string, temperature float64) (string, error) {
	return callHuggingFaceWithOptions(prompt, systemMessage, temperature, true)
}

func callHuggingFaceText(prompt, systemMessage string, temperature float64) (string, error) {
	return callHuggingFaceWithOptions(prompt, systemMessage, temperature, false)
}

func callHuggingFaceWithOptions(prompt, systemMessage string, temperature float64, preferJSON bool) (string, error) {
	apiKey := huggingFaceAPIKey()
	if apiKey == "" {
		return "", fmt.Errorf("HF_API_KEY belum diatur di server")
	}

	model := huggingFaceModel()
	basePayload := map[string]interface{}{
		"model":       model,
		"temperature": temperature,
		"max_tokens":  1800,
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

	callProvider := func(payload map[string]interface{}) (*huggingFaceCompletionResponse, error) {
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, huggingFaceAPIURL(), bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		var parsed huggingFaceCompletionResponse
		if err := json.Unmarshal(raw, &parsed); err != nil {
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				return nil, providerStatusError("Hugging Face", resp.StatusCode, raw)
			}
			return nil, err
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, providerStatusError("Hugging Face", resp.StatusCode, []byte(extractHuggingFaceErrorMessage(raw, parsed)))
		}

		return &parsed, nil
	}

	var completion *huggingFaceCompletionResponse
	var err error
	if preferJSON {
		payloadWithResponseFormat := cloneMap(basePayload)
		payloadWithResponseFormat["response_format"] = map[string]string{"type": "json_object"}

		completion, err = callProvider(payloadWithResponseFormat)
		if err != nil {
			completion, err = callProvider(basePayload)
			if err != nil {
				return "", err
			}
		}
	} else {
		completion, err = callProvider(basePayload)
		if err != nil {
			return "", err
		}
	}

	text := extractHuggingFaceResponseText(completion)
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("Hugging Face tidak mengembalikan isi yang dapat diproses")
	}

	if completion != nil && (completion.Usage.PromptTokens > 0 || completion.Usage.CompletionTokens > 0 || completion.Usage.TotalTokens > 0) {
		log.Printf(
			"Hugging Face token usage model=%s prompt=%d completion=%d total=%d",
			model,
			completion.Usage.PromptTokens,
			completion.Usage.CompletionTokens,
			completion.Usage.TotalTokens,
		)
	}

	return text, nil
}

func providerStatusError(provider string, statusCode int, raw []byte) error {
	message := strings.TrimSpace(string(raw))
	if len(message) > 500 {
		message = message[:500] + "..."
	}

	if statusCode == http.StatusPaymentRequired {
		return fmt.Errorf("%s mengembalikan 402 Payment Required. Aktifkan billing/kredit Inference Providers, atau gunakan provider Qwen cadangan", provider)
	}

	if message == "" {
		return fmt.Errorf("request %s gagal dengan status %d", provider, statusCode)
	}

	return fmt.Errorf("request %s gagal dengan status %d: %s", provider, statusCode, normalizeHuggingFaceError(message))
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

func extractHuggingFaceErrorMessage(raw []byte, parsed huggingFaceCompletionResponse) string {
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err == nil {
		if errObj, ok := payload["error"].(map[string]interface{}); ok {
			if msg, ok := errObj["message"].(string); ok && strings.TrimSpace(msg) != "" {
				return msg
			}
		}
		if msg, ok := payload["error"].(string); ok && strings.TrimSpace(msg) != "" {
			return msg
		}
		if msg, ok := payload["message"].(string); ok && strings.TrimSpace(msg) != "" {
			return msg
		}
	}

	if parsed.Error != nil && strings.TrimSpace(parsed.Error.Message) != "" {
		return parsed.Error.Message
	}

	if len(parsed.Choices) > 0 && strings.TrimSpace(parsed.Choices[0].Message.Refusal) != "" {
		return parsed.Choices[0].Message.Refusal
	}

	return "Request Hugging Face gagal"
}

func extractHuggingFaceResponseText(parsed *huggingFaceCompletionResponse) string {
	if parsed == nil || len(parsed.Choices) == 0 {
		return ""
	}

	choice := parsed.Choices[0]
	if text := rawMessageToString(choice.Message.Content); strings.TrimSpace(text) != "" {
		return strings.TrimSpace(text)
	}

	if strings.TrimSpace(choice.Message.Refusal) != "" {
		return strings.TrimSpace(choice.Message.Refusal)
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
