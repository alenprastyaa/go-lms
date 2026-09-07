package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Lapisan tool calling untuk asisten yang dapat menjalankan aksi di dalam aplikasi.
// Berkas ini hanya mengurus percakapan dengan model; keputusan boleh atau tidaknya
// sebuah aksi dijalankan sepenuhnya berada di sisi controller.

type AgentTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type AgentMessage struct {
	Role       string          `json:"role"`
	Content    string          `json:"content"`
	ToolCalls  []AgentToolCall `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Name       string          `json:"name,omitempty"`
}

type AgentToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type AgentResult struct {
	Content   string
	ToolCalls []AgentToolCall
}

type agentRequest struct {
	Model       string         `json:"model"`
	Messages    []AgentMessage `json:"messages"`
	Tools       []agentToolDef `json:"tools,omitempty"`
	ToolChoice  string         `json:"tool_choice,omitempty"`
	Temperature float64        `json:"temperature,omitempty"`
}

type agentToolDef struct {
	Type     string    `json:"type"`
	Function AgentTool `json:"function"`
}

type agentResponse struct {
	Choices []struct {
		Message struct {
			Content   string          `json:"content"`
			ToolCalls []AgentToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// RunAgentTurn mengirim percakapan beserta daftar tool yang boleh dipakai, lalu
// mengembalikan balasan model: berupa teks biasa, atau permintaan memanggil tool.
func RunAgentTurn(messages []AgentMessage, tools []AgentTool) (AgentResult, error) {
	apiKey := openRouterAPIKey()
	if apiKey == "" {
		return AgentResult{}, fmt.Errorf("OPENROUTER_API_KEY belum diatur di server")
	}

	defs := make([]agentToolDef, 0, len(tools))
	for _, tool := range tools {
		defs = append(defs, agentToolDef{Type: "function", Function: tool})
	}

	reqBody := agentRequest{
		Model:       openRouterModel(),
		Messages:    messages,
		Tools:       defs,
		Temperature: 0.2,
	}
	if len(defs) > 0 {
		reqBody.ToolChoice = "auto"
	}

	rawBody, err := json.Marshal(reqBody)
	if err != nil {
		return AgentResult{}, err
	}

	req, err := http.NewRequest(http.MethodPost, openRouterAPIURL(), bytes.NewReader(rawBody))
	if err != nil {
		return AgentResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("HTTP-Referer", "https://school-system.local")
	req.Header.Set("X-Title", "School System LMS")

	client := &http.Client{Timeout: openRouterTimeout()}
	resp, err := client.Do(req)
	if err != nil {
		return AgentResult{}, err
	}
	defer resp.Body.Close()

	rawResp, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return AgentResult{}, readErr
	}

	var parsed agentResponse
	if len(rawResp) > 0 {
		_ = json.Unmarshal(rawResp, &parsed)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := ""
		if parsed.Error != nil {
			msg = strings.TrimSpace(parsed.Error.Message)
		}
		if msg == "" {
			msg = strings.TrimSpace(string(rawResp))
		}
		if len(msg) > 400 {
			msg = msg[:400]
		}
		return AgentResult{}, fmt.Errorf("AI menolak permintaan (status %d): %s", resp.StatusCode, msg)
	}

	if len(parsed.Choices) == 0 {
		return AgentResult{}, fmt.Errorf("AI tidak mengembalikan balasan")
	}

	choice := parsed.Choices[0].Message
	return AgentResult{
		Content:   strings.TrimSpace(choice.Content),
		ToolCalls: choice.ToolCalls,
	}, nil
}
