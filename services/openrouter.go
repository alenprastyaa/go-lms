package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultOpenRouterAPIURL = "https://openrouter.ai/api/v1/chat/completions"
	defaultOpenRouterModel  = "google/gemini-2.5-flash-lite"
)

type openRouterChatCompletionRequest struct {
	Model       string                  `json:"model"`
	Messages    []openRouterChatMessage `json:"messages"`
	Temperature float64                 `json:"temperature,omitempty"`
	// OpenRouter docs: max_tokens is deprecated; prefer max_completion_tokens.
	MaxTokens           *int                      `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int                      `json:"max_completion_tokens,omitempty"`
	ResponseFormat      *openRouterResponseFormat `json:"response_format,omitempty"`
}

type openRouterChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openRouterResponseFormat struct {
	Type string `json:"type"`
}

type openRouterChatCompletionResponse struct {
	Choices []struct {
		Message      openRouterChatMessage `json:"message"`
		FinishReason string                `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func openRouterAPIKey() string {
	if value := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")); value != "" {
		return value
	}
	return ""
}

func openRouterAPIURL() string {
	if value := strings.TrimSpace(os.Getenv("OPENROUTER_API_URL")); value != "" {
		return value
	}
	return defaultOpenRouterAPIURL
}

func openRouterModel() string {
	model := strings.TrimSpace(os.Getenv("OPENROUTER_MODEL"))
	if model == "" {
		model = strings.TrimSpace(os.Getenv("GEMINI_MODEL"))
	}
	if model == "" {
		return defaultOpenRouterModel
	}

	// Convenience: allow "gemini-2.5-flash-lite" and map to OpenRouter id.
	if !strings.Contains(model, "/") && strings.HasPrefix(strings.ToLower(model), "gemini") {
		return "google/" + model
	}
	return model
}

func openRouterForceJSON() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("OPENROUTER_FORCE_JSON")))
	return value == "1" || value == "true" || value == "yes"
}

func callOpenRouterText(purpose string, prompt, systemMessage string, temperature float64, maxTokens int) (string, error) {
	apiKey := openRouterAPIKey()
	if apiKey == "" {
		return "", fmt.Errorf("OPENROUTER_API_KEY belum diatur di server")
	}

	model := openRouterModel()
	reqBody := openRouterChatCompletionRequest{
		Model: model,
		Messages: []openRouterChatMessage{
			{Role: "system", Content: strings.TrimSpace(systemMessage)},
			{Role: "user", Content: strings.TrimSpace(prompt)},
		},
		Temperature: temperature,
	}
	if maxTokens > 0 {
		reqBody.MaxTokens = &maxTokens
		reqBody.MaxCompletionTokens = &maxTokens
	}
	if openRouterForceJSON() {
		reqBody.ResponseFormat = &openRouterResponseFormat{Type: "json_object"}
	}

	rawBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, openRouterAPIURL(), bytes.NewReader(rawBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("HTTP-Referer", "https://school-system.local")
	req.Header.Set("X-Title", "School System LMS")

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	rawResp, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return "", readErr
	}

	var parsed openRouterChatCompletionResponse
	if len(rawResp) > 0 {
		_ = json.Unmarshal(rawResp, &parsed)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(parsed.Error.Message)
		if msg == "" {
			msg = strings.TrimSpace(string(rawResp))
		}
		if msg == "" {
			msg = "respons kosong"
		}
		return "", fmt.Errorf("request OpenRouter gagal dengan status %d: %s", resp.StatusCode, msg)
	}

	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("OpenRouter tidak mengembalikan pilihan jawaban")
	}
	answer := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if answer == "" {
		return "", fmt.Errorf("OpenRouter tidak mengembalikan teks jawaban")
	}

	log.Printf(
		"OpenRouter token usage purpose=%s model=%s prompt=%d completion=%d total=%d max=%d finish=%s",
		strings.TrimSpace(purpose),
		model,
		parsed.Usage.PromptTokens,
		parsed.Usage.CompletionTokens,
		parsed.Usage.TotalTokens,
		maxTokens,
		strings.TrimSpace(parsed.Choices[0].FinishReason),
	)

	return answer, nil
}
