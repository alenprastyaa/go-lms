package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const defaultGeminiModel = "gemini-2.5-flash-lite"

type geminiGenerateContentRequest struct {
	SystemInstruction *geminiContent           `json:"system_instruction,omitempty"`
	Contents          []geminiContent          `json:"contents"`
	GenerationConfig  geminiGenerationConfig   `json:"generationConfig"`
	SafetySettings    []geminiSafetySetting    `json:"safetySettings,omitempty"`
	Tools             []map[string]interface{} `json:"tools,omitempty"`
	ToolConfig        map[string]interface{}   `json:"toolConfig,omitempty"`
	Labels            map[string]string        `json:"labels,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text,omitempty"`
}

type geminiGenerationConfig struct {
	Temperature      float64 `json:"temperature,omitempty"`
	ResponseMimeType string  `json:"responseMimeType,omitempty"`
}

type geminiSafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

type geminiGenerateContentResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
		Index        int    `json:"index"`
	} `json:"candidates"`
	PromptFeedback struct {
		BlockReason        string `json:"blockReason"`
		BlockReasonMessage string `json:"blockReasonMessage"`
	} `json:"promptFeedback"`
	UsageMetadata struct {
		PromptTokenCount        int `json:"promptTokenCount"`
		CachedContentTokenCount int `json:"cachedContentTokenCount"`
		CandidatesTokenCount    int `json:"candidatesTokenCount"`
		ToolUsePromptTokenCount int `json:"toolUsePromptTokenCount"`
		ThoughtsTokenCount      int `json:"thoughtsTokenCount"`
		TotalTokenCount         int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
	Error *struct {
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

func geminiAPIKey() string {
	for _, key := range []string{"GEMINI_API_KEY", "GOOGLE_API_KEY", "GOOGLE_AI_STUDIO_API_KEY"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func geminiModel() string {
	if value := strings.TrimSpace(os.Getenv("GEMINI_MODEL")); value != "" {
		return value
	}
	return defaultGeminiModel
}

func callGeminiText(prompt, systemMessage string, temperature float64, responseMimeType string) (string, error) {
	apiKey := geminiAPIKey()
	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY belum diatur di server")
	}

	model := geminiModel()
	requestBody := geminiGenerateContentRequest{
		SystemInstruction: nil,
		Contents: []geminiContent{
			{
				Role:  "user",
				Parts: []geminiPart{{Text: strings.TrimSpace(prompt)}},
			},
		},
		GenerationConfig: geminiGenerationConfig{
			Temperature:      temperature,
			ResponseMimeType: responseMimeType,
		},
	}
	if trimmed := strings.TrimSpace(systemMessage); trimmed != "" {
		requestBody.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: trimmed}},
		}
	}

	rawBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	endpoint := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		url.PathEscape(model),
		url.QueryEscape(apiKey),
	)

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(rawBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

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

	var parsed geminiGenerateContentResponse
	if len(rawResp) > 0 {
		_ = json.Unmarshal(rawResp, &parsed)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("request Gemini gagal dengan status %d: %s", resp.StatusCode, normalizeGeminiError(rawResp, parsed))
	}

	text := extractGeminiResponseText(parsed)
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("Gemini tidak mengembalikan teks jawaban")
	}

	log.Printf(
		"Gemini token usage model=%s prompt=%d cached=%d candidates=%d thoughts=%d total=%d",
		model,
		parsed.UsageMetadata.PromptTokenCount,
		parsed.UsageMetadata.CachedContentTokenCount,
		parsed.UsageMetadata.CandidatesTokenCount,
		parsed.UsageMetadata.ThoughtsTokenCount,
		parsed.UsageMetadata.TotalTokenCount,
	)

	return strings.TrimSpace(text), nil
}

func normalizeGeminiError(raw []byte, parsed geminiGenerateContentResponse) string {
	if parsed.Error != nil {
		msg := strings.TrimSpace(parsed.Error.Message)
		if msg != "" {
			return msg
		}
	}
	if parsed.PromptFeedback.BlockReasonMessage != "" {
		return strings.TrimSpace(parsed.PromptFeedback.BlockReasonMessage)
	}
	if parsed.PromptFeedback.BlockReason != "" {
		return strings.TrimSpace(parsed.PromptFeedback.BlockReason)
	}
	if trimmed := strings.TrimSpace(string(raw)); trimmed != "" {
		return trimmed
	}
	return "respons kosong"
}

func extractGeminiResponseText(parsed geminiGenerateContentResponse) string {
	if len(parsed.Candidates) == 0 {
		return ""
	}

	parts := parsed.Candidates[0].Content.Parts
	if len(parts) == 0 {
		return ""
	}

	var builder strings.Builder
	for _, part := range parts {
		text := strings.TrimSpace(part.Text)
		if text == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(text)
	}

	return builder.String()
}
