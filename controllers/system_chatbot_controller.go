package controllers

import (
	"encoding/json"
	"fmt"
	"lms/services"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"
	"lms/utils"
)

type chatbotIntent struct {
	keywords []string
	answer   string
}

func interfaceToTrimmedString(value interface{}) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func interfaceSliceToStrings(value interface{}) []string {
	rawItems, ok := value.([]interface{})
	if !ok {
		return nil
	}

	items := make([]string, 0, len(rawItems))
	for _, item := range rawItems {
		text := interfaceToTrimmedString(item)
		if text != "" {
			items = append(items, text)
		}
	}
	return items
}

func flattenJSONArrayStrings(value interface{}) []string {
	switch typed := value.(type) {
	case []interface{}:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			items = append(items, flattenJSONArrayStrings(item)...)
		}
		return items
	case map[string]interface{}:
		if structured := formatStructuredAIAnswer(typed); structured != "" {
			return []string{structured}
		}
		return nil
	default:
		text := interfaceToTrimmedString(typed)
		if text == "" {
			return nil
		}
		return []string{text}
	}
}

func formatStructuredAIAnswer(payload map[string]interface{}) string {
	if len(payload) == 0 {
		return ""
	}

	mainAnswer := ""
	for _, key := range []string{"answer", "jawaban", "response", "message", "content", "jawaban_inti"} {
		if value := interfaceToTrimmedString(payload[key]); value != "" {
			mainAnswer = value
			break
		}
	}

	steps := []string{}
	for _, key := range []string{"langkah_singkat", "steps", "langkah", "instructions"} {
		if items := interfaceSliceToStrings(payload[key]); len(items) > 0 {
			steps = items
			break
		}
	}

	followUp := ""
	for _, key := range []string{"pertanyaan_lanjutan", "follow_up", "next_question"} {
		if value := interfaceToTrimmedString(payload[key]); value != "" {
			followUp = value
			break
		}
	}

	parts := make([]string, 0, 3)
	if mainAnswer != "" {
		parts = append(parts, mainAnswer)
	}
	if len(steps) > 0 {
		lines := make([]string, 0, len(steps)+1)
		lines = append(lines, "Langkah singkat:")
		for index, step := range steps {
			lines = append(lines, fmt.Sprintf("%d. %s", index+1, step))
		}
		parts = append(parts, strings.Join(lines, "\n"))
	}
	if followUp != "" {
		parts = append(parts, followUp)
	}

	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func cleanAIAnswer(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}

	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	var payload map[string]interface{}
	if json.Unmarshal([]byte(text), &payload) == nil {
		structured := formatStructuredAIAnswer(payload)
		if structured != "" {
			text = structured
		} else {
			for _, key := range []string{"answer", "jawaban", "response", "message", "content"} {
				if value, ok := payload[key].(string); ok && strings.TrimSpace(value) != "" {
					text = strings.TrimSpace(value)
					break
				}
			}
		}
	}

	var rawArray interface{}
	if json.Unmarshal([]byte(text), &rawArray) == nil {
		if items := flattenJSONArrayStrings(rawArray); len(items) > 0 {
			text = strings.Join(items, " ")
		}
	}

	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	noiseLine := regexp.MustCompile(`^\s*[\{\}\[\],"]+\s*$`)
	jsonArrayLineCount := 0
	nonEmptyLineCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			cleaned = append(cleaned, "")
			continue
		}
		nonEmptyLineCount++
		if noiseLine.MatchString(trimmed) {
			continue
		}
		var lineArray interface{}
		if json.Unmarshal([]byte(trimmed), &lineArray) == nil {
			if items := flattenJSONArrayStrings(lineArray); len(items) > 0 {
				jsonArrayLineCount++
				cleaned = append(cleaned, strings.Join(items, " "))
				continue
			}
		}
		cleaned = append(cleaned, line)
	}

	text = strings.TrimSpace(strings.Join(cleaned, "\n"))
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	if jsonArrayLineCount > 0 && jsonArrayLineCount == nonEmptyLineCount {
		text = strings.Join(strings.Fields(text), " ")
	}

	return strings.TrimSpace(text)
}

func isUsableAIAnswer(answer string) bool {
	text := strings.TrimSpace(answer)
	if text == "" {
		return false
	}

	normalized := strings.Trim(text, `"'[]{}(),`)
	if normalized == "" {
		return false
	}

	letterCount := 0
	for _, r := range normalized {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r >= 128 {
			letterCount++
		}
	}
	if letterCount < 6 {
		return false
	}

	suspiciousPatterns := []*regexp.Regexp{
		regexp.MustCompile(`^\$?[A-Za-z0-9_ -]{1,20}$`),
		regexp.MustCompile(`^\[[^\]]+\]$`),
	}
	for _, pattern := range suspiciousPatterns {
		if pattern.MatchString(text) {
			return false
		}
	}

	return true
}

func fallbackSystemChatbotAnswer(question string, counts struct {
	Teachers int `json:"teachers"`
	Students int `json:"students"`
	Classes  int `json:"classes"`
	Subjects int `json:"subjects"`
}) string {
	_ = counts
	_ = question
	return "Maaf, saya belum bisa menjawab dengan yakin saat ini. Coba kirim pertanyaan yang lebih spesifik, atau pecah jadi satu pertanyaan per pesan supaya saya bisa bantu lebih tepat."
}

func (a *AppContext) AskSystemChatbot(c *fiber.Ctx) error {
	var body struct {
		Question string `json:"question"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Format pertanyaan tidak valid")
	}

	question := strings.ToLower(strings.TrimSpace(body.Question))
	if question == "" {
		return utils.Error(c, 400, "Pertanyaan tidak boleh kosong")
	}
	if len(question) > 500 {
		return utils.Error(c, 400, "Pertanyaan terlalu panjang, mohon ringkas maksimal 500 karakter")
	}

	answer, aiErr := services.GenerateSystemChatbotAnswer(services.SystemChatbotInput{
		Question: question,
	})
	answer = cleanAIAnswer(answer)
	if !isUsableAIAnswer(answer) || aiErr != nil {
		answer = fallbackSystemChatbotAnswer(question, struct {
			Teachers int `json:"teachers"`
			Students int `json:"students"`
			Classes  int `json:"classes"`
			Subjects int `json:"subjects"`
		}{})
	}

	return utils.Success(c, 200, "Jawaban chatbot berhasil dibuat", fiber.Map{
		"role_hint": "Asisten ini bisa membantu pertanyaan umum lintas topik, bukan hanya topik sekolah.",
		"answer":    answer,
		"suggestions": []string{
			"Jelaskan topik ini dengan bahasa sederhana.",
			"Buat langkah-langkah singkat untuk masalah ini.",
			"Ringkas jawaban ini dalam 3 poin.",
			"Beri contoh praktis yang mudah diikuti.",
		},
	})
}
