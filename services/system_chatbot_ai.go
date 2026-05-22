package services

import (
	"fmt"
	"strings"
)

type SystemChatbotInput struct {
	Question string
	History  []SystemChatbotMessage
}

type SystemChatbotMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func buildSystemChatbotPrompt(input SystemChatbotInput) string {
	parts := []string{
		"Anda adalah Qwen, asisten AI umum seperti ChatGPT.",
		"Jawab pertanyaan pengguna dari topik global apa pun selama aman dan legal.",
		"Jangan mengaitkan jawaban dengan aplikasi internal kecuali pengguna memang membahas hal itu.",
		"Gunakan Bahasa Indonesia yang natural, jelas, dan langsung ke inti.",
		"Jika pertanyaan meminta langkah, berikan langkah yang praktis.",
		"Jika pertanyaan butuh informasi terbaru atau sangat bergantung waktu, jelaskan bahwa data terbaru perlu diverifikasi.",
		"Jika pertanyaan berbahaya, ilegal, atau melanggar privasi, tolak singkat dan arahkan ke alternatif aman.",
		"Jangan balas dalam format JSON, array, object, atau markdown code block.",
	}

	history := normalizeSystemChatbotHistory(input.History)
	if len(history) > 0 {
		parts = append(parts, "", "Riwayat percakapan terakhir:")
		parts = append(parts, history...)
	}

	parts = append(parts,
		"",
		fmt.Sprintf("Pertanyaan terbaru: %s", strings.TrimSpace(input.Question)),
		"",
		"Jawab sebagai asisten AI umum. Berikan jawaban lengkap secukupnya tanpa menyebut aplikasi internal.",
	)

	return strings.Join(parts, "\n")
}

func GenerateSystemChatbotAnswer(input SystemChatbotInput) (string, error) {
	systemMessage := "Anda adalah Qwen, asisten AI umum yang menjawab pertanyaan global dengan jelas, praktis, dan aman."
	return callHuggingFaceText(buildSystemChatbotPrompt(input), systemMessage, 0.55)
}

func normalizeSystemChatbotHistory(history []SystemChatbotMessage) []string {
	if len(history) == 0 {
		return nil
	}

	start := 0
	if len(history) > 12 {
		start = len(history) - 12
	}

	lines := make([]string, 0, len(history)-start)
	for _, item := range history[start:] {
		content := strings.TrimSpace(item.Content)
		if content == "" {
			continue
		}
		if len(content) > 1200 {
			content = content[:1200] + "..."
		}

		role := "Pengguna"
		switch strings.ToLower(strings.TrimSpace(item.Role)) {
		case "assistant", "bot", "ai":
			role = "Asisten"
		case "user", "pengguna":
			role = "Pengguna"
		default:
			role = "Pesan"
		}
		lines = append(lines, fmt.Sprintf("%s: %s", role, content))
	}

	return lines
}
