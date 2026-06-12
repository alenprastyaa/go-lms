package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultHuggingFaceModel = "deepseek-ai/DeepSeek-V4-Pro"

const defaultHuggingFaceAPIURL = "https://router.huggingface.co/v1/chat/completions"

const huggingFaceModelTimeout = 35 * time.Second

var defaultHuggingFaceModels = []string{
	"deepseek-ai/DeepSeek-V4-Pro",
	"deepseek-ai/DeepSeek-V4-Flash",
	"Qwen/Qwen2.5-7B-Instruct",
	"mistralai/Mistral-7B-Instruct-v0.3",
	"Qwen/Qwen2.5-3B-Instruct",
}

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
	models := huggingFaceModels()
	if len(models) > 0 {
		return models[0]
	}
	return defaultHuggingFaceModel
}

func huggingFaceModels() []string {
	values := make([]string, 0, len(defaultHuggingFaceModels)+1)
	appendModels := func(raw string) {
		for _, item := range strings.FieldsFunc(raw, func(r rune) bool {
			return r == ',' || r == '\n' || r == ';'
		}) {
			model := strings.TrimSpace(item)
			if model != "" {
				values = append(values, model)
			}
		}
	}

	appendModels(os.Getenv("HUGGINGFACE_MODELS"))
	appendModels(os.Getenv("HF_MODELS"))
	if model := strings.TrimSpace(os.Getenv("HUGGINGFACE_MODEL")); model != "" {
		values = append(values, model)
	}
	if model := strings.TrimSpace(os.Getenv("HF_MODEL")); model != "" {
		values = append(values, model)
	}
	values = append(values, defaultHuggingFaceModels...)

	seen := map[string]bool{}
	models := make([]string, 0, len(values))
	for _, model := range values {
		key := strings.ToLower(model)
		if seen[key] {
			continue
		}
		seen[key] = true
		models = append(models, model)
	}
	if len(models) == 0 {
		return []string{defaultHuggingFaceModel}
	}
	return models
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
	return callHuggingFaceWithOptions(prompt, systemMessage, temperature, true, 1800)
}

func callHuggingFaceText(prompt, systemMessage string, temperature float64) (string, error) {
	return callHuggingFaceWithOptions(prompt, systemMessage, temperature, false, 1800)
}

func callHuggingFaceJSONWithMaxTokens(prompt, systemMessage string, temperature float64, maxTokens int) (string, error) {
	return callHuggingFaceWithOptions(prompt, systemMessage, temperature, true, maxTokens)
}

func callHuggingFaceWithOptions(prompt, systemMessage string, temperature float64, preferJSON bool, maxTokens int) (string, error) {
	apiKey := huggingFaceAPIKey()
	if apiKey == "" {
		return "", fmt.Errorf("HF_API_KEY belum diatur di server")
	}
	if maxTokens <= 0 {
		maxTokens = 1800
	}

	basePayload := map[string]interface{}{
		"temperature": temperature,
		"max_tokens":  maxTokens,
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

	callProvider := func(model string, payload map[string]interface{}) (*huggingFaceCompletionResponse, error) {
		payload = cloneMap(payload)
		payload["model"] = model
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}

		ctx, cancel := context.WithTimeout(context.Background(), huggingFaceModelTimeout)
		defer cancel()

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

	var lastErr error
	models := huggingFaceModels()
	for index, model := range models {
		var completion *huggingFaceCompletionResponse
		var err error
		if preferJSON {
			payloadWithResponseFormat := cloneMap(basePayload)
			payloadWithResponseFormat["response_format"] = map[string]string{"type": "json_object"}

			completion, err = callProvider(model, payloadWithResponseFormat)
			if shouldRetryHuggingFaceWithoutResponseFormat(err) {
				completion, err = callProvider(model, basePayload)
			}
		} else {
			completion, err = callProvider(model, basePayload)
		}
		if err != nil {
			lastErr = err
			if index < len(models)-1 {
				log.Printf("Hugging Face model fallback model=%s next=%s error=%v", model, models[index+1], err)
				continue
			}
			break
		}

		text := extractHuggingFaceResponseText(completion)
		if strings.TrimSpace(text) == "" {
			lastErr = fmt.Errorf("Hugging Face model %s tidak mengembalikan isi yang dapat diproses", model)
			if index < len(models)-1 {
				log.Printf("Hugging Face model fallback model=%s next=%s error=%v", model, models[index+1], lastErr)
				continue
			}
			break
		}

		if index > 0 {
			log.Printf("Hugging Face fallback succeeded model=%s attempt=%d", model, index+1)
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

	if lastErr != nil {
		return "", fmt.Errorf("semua model Hugging Face gagal dipakai: %w", lastErr)
	}
	return "", fmt.Errorf("semua model Hugging Face gagal dipakai")
}

func providerStatusError(provider string, statusCode int, raw []byte) error {
	message := strings.TrimSpace(string(raw))
	if len(message) > 500 {
		message = message[:500] + "..."
	}
	return &providerHTTPError{provider: provider, statusCode: statusCode, message: message}
}

type providerHTTPError struct {
	provider   string
	statusCode int
	message    string
}

func (e *providerHTTPError) Error() string {
	if e == nil {
		return "request provider gagal"
	}
	if e.statusCode == http.StatusPaymentRequired {
		return fmt.Sprintf("%s mengembalikan 402 Payment Required. Model berikutnya akan dicoba jika tersedia.", e.provider)
	}
	if strings.TrimSpace(e.message) == "" {
		return fmt.Sprintf("request %s gagal dengan status %d", e.provider, e.statusCode)
	}
	return fmt.Sprintf("request %s gagal dengan status %d: %s", e.provider, e.statusCode, normalizeHuggingFaceError(e.message))
}

func shouldRetryHuggingFaceWithoutResponseFormat(err error) bool {
	var httpErr *providerHTTPError
	if !errors.As(err, &httpErr) {
		return false
	}
	if httpErr.statusCode != http.StatusBadRequest {
		return false
	}
	lowered := strings.ToLower(httpErr.message)
	return strings.Contains(lowered, "response_format") || strings.Contains(lowered, "json_object")
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
