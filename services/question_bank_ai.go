package services

import (
	"encoding/json"
	"fmt"
	"strings"
)

type QuestionBankAIInput struct {
	SubjectName            string
	ClassName              string
	GradeLabel             string
	PhaseName              string
	CurriculumName         string
	Topic                  string
	QuestionType           string
	QuestionCount          int
	Difficulty             string
	AdditionalInstructions string
}

type QuestionBankAIItem struct {
	QuestionType  string   `json:"question_type"`
	QuestionText  string   `json:"question_text"`
	Options       []string `json:"options"`
	CorrectOption *int     `json:"correct_option"`
	Rubric        *string  `json:"rubric"`
}

type questionBankAIResponse struct {
	Items []questionBankAIRawItem `json:"items"`
}

type questionBankAIRawItem struct {
	QuestionType  string          `json:"question_type"`
	QuestionText  string          `json:"question_text"`
	Options       json.RawMessage `json:"options"`
	CorrectOption json.RawMessage `json:"correct_option"`
	Rubric        string          `json:"rubric"`
}

func buildQuestionBankPrompt(input QuestionBankAIInput) string {
	normalizedType := normalizeQuestionType(input.QuestionType)
	parts := []string{
		"Anda adalah penyusun bank soal untuk LMS sekolah.",
		"Buat soal dalam Bahasa Indonesia yang jelas, natural, dan sesuai konteks sekolah.",
		fmt.Sprintf("Mapel: %s.", fallbackText(input.SubjectName, "-")),
		fmt.Sprintf("Kelas: %s.", fallbackText(input.ClassName, "-")),
	}

	if strings.TrimSpace(input.GradeLabel) != "" {
		parts = append(parts, fmt.Sprintf("Jenjang/kelas target tambahan: %s.", strings.TrimSpace(input.GradeLabel)))
	}
	if strings.TrimSpace(input.PhaseName) != "" {
		parts = append(parts, fmt.Sprintf("Fase belajar: %s.", strings.TrimSpace(input.PhaseName)))
	}
	if strings.TrimSpace(input.CurriculumName) != "" {
		parts = append(parts, fmt.Sprintf("Kurikulum: %s.", strings.TrimSpace(input.CurriculumName)))
	}

	parts = append(parts,
		fmt.Sprintf("Topik: %s.", strings.TrimSpace(input.Topic)),
		fmt.Sprintf("Tipe soal: %s.", normalizedType),
		fmt.Sprintf("Jumlah soal: %d.", input.QuestionCount),
		fmt.Sprintf("Tingkat kesulitan: %s.", fallbackText(strings.TrimSpace(input.Difficulty), "MENENGAH")),
	)

	if strings.TrimSpace(input.AdditionalInstructions) != "" {
		parts = append(parts, fmt.Sprintf("Instruksi tambahan: %s.", strings.TrimSpace(input.AdditionalInstructions)))
	}

	if normalizedType == "MCQ" {
		parts = append(parts, "Setiap soal MCQ harus punya tepat 5 opsi jawaban dari A sampai E dan satu correct_option berbasis indeks 0 sampai 4.")
	} else {
		parts = append(parts, "Setiap soal essay harus memiliki rubric singkat untuk membantu penilaian guru.")
	}

	parts = append(parts,
		"Kembalikan JSON saja tanpa markdown, tanpa penjelasan tambahan.",
		"Struktur JSON wajib:",
		fmt.Sprintf(`{"items":[{"question_type":"%s","question_text":"...","options":["..."],"correct_option":0,"rubric":null}]}`, normalizedType),
		"Untuk ESSAY, isi options dengan null dan correct_option dengan null.",
	)

	return strings.Join(parts, "\n")
}

func normalizeQuestionType(value string) string {
	upper := strings.ToUpper(strings.TrimSpace(value))
	if upper == "ESSAY" {
		return "ESSAY"
	}
	return "MCQ"
}

func fallbackText(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return fallback
}

func GenerateQuestionBankItemsWithOpenRouter(input QuestionBankAIInput) ([]QuestionBankAIItem, error) {
	if input.QuestionCount <= 0 {
		input.QuestionCount = 5
	}

	text, err := callOpenRouter(
		buildQuestionBankPrompt(input),
		"Anda adalah penyusun bank soal sekolah yang harus mengembalikan JSON valid tanpa markdown.",
		0.7,
	)
	if err != nil {
		return nil, err
	}

	var parsed questionBankAIResponse
	if err := json.Unmarshal([]byte(extractJSONObject(text)), &parsed); err != nil {
		return nil, fmt.Errorf("hasil OpenRouter tidak bisa diparsing sebagai JSON bank soal: %w", err)
	}

	return normalizeQuestionBankItems(parsed.Items, input.QuestionType), nil
}

func extractJSONObject(raw string) string {
	direct := strings.TrimSpace(raw)
	if direct == "" {
		return ""
	}

	if strings.HasPrefix(direct, "{") && strings.HasSuffix(direct, "}") {
		return direct
	}

	if start := strings.Index(direct, "{"); start >= 0 {
		if end := strings.LastIndex(direct, "}"); end > start {
			return strings.TrimSpace(direct[start : end+1])
		}
	}

	if fenced := fencedJSON(direct); fenced != "" {
		return fenced
	}

	return direct
}

func fencedJSON(raw string) string {
	start := strings.Index(strings.ToLower(raw), "```json")
	if start < 0 {
		start = strings.Index(raw, "```")
		if start < 0 {
			return ""
		}
	}

	candidate := raw[start:]
	candidate = strings.TrimPrefix(candidate, "```json")
	candidate = strings.TrimPrefix(candidate, "```")
	candidate = strings.TrimSpace(candidate)
	if end := strings.LastIndex(candidate, "```"); end >= 0 {
		candidate = candidate[:end]
	}
	return strings.TrimSpace(candidate)
}

func normalizeQuestionBankItems(items []questionBankAIRawItem, questionType string) []QuestionBankAIItem {
	normalizedType := normalizeQuestionType(questionType)
	normalized := make([]QuestionBankAIItem, 0, len(items))

	for _, item := range items {
		questionText := strings.TrimSpace(item.QuestionText)
		if questionText == "" {
			continue
		}

		if normalizedType == "MCQ" {
			options := normalizeStringArray(item.Options, 5)
			correctOption, ok := parseIntegerFromRawMessage(item.CorrectOption)
			if len(options) != 5 || !ok || correctOption < 0 || correctOption > 4 {
				continue
			}

			correct := correctOption
			rubric := (*string)(nil)

			normalized = append(normalized, QuestionBankAIItem{
				QuestionType:  "MCQ",
				QuestionText:  questionText,
				Options:       options,
				CorrectOption: &correct,
				Rubric:        rubric,
			})
			continue
		}

		rubric := strings.TrimSpace(item.Rubric)
		if rubric == "" {
			rubric = "Jawaban dinilai berdasarkan ketepatan konsep, kelengkapan penjelasan, dan kejelasan alasan."
		}

		normalized = append(normalized, QuestionBankAIItem{
			QuestionType:  "ESSAY",
			QuestionText:  questionText,
			Options:       nil,
			CorrectOption: nil,
			Rubric:        &rubric,
		})
	}

	return normalized
}

func normalizeStringArray(raw json.RawMessage, limit int) []string {
	raw = bytesTrimSpace(raw)
	if len(raw) == 0 || strings.EqualFold(string(raw), "null") {
		return nil
	}

	var values []string
	if err := json.Unmarshal(raw, &values); err == nil {
		return trimAndLimitStrings(values, limit)
	}

	var interfaces []interface{}
	if err := json.Unmarshal(raw, &interfaces); err == nil {
		values = make([]string, 0, len(interfaces))
		for _, item := range interfaces {
			values = append(values, fmt.Sprint(item))
		}
		return trimAndLimitStrings(values, limit)
	}

	return nil
}

func trimAndLimitStrings(values []string, limit int) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		item := strings.TrimSpace(value)
		if item == "" {
			continue
		}
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func parseIntegerFromRawMessage(raw json.RawMessage) (int, bool) {
	raw = bytesTrimSpace(raw)
	if len(raw) == 0 || strings.EqualFold(string(raw), "null") {
		return 0, false
	}

	var number float64
	if err := json.Unmarshal(raw, &number); err == nil {
		return int(number), true
	}

	var integer int
	if err := json.Unmarshal(raw, &integer); err == nil {
		return integer, true
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		var parsed int
		if _, scanErr := fmt.Sscanf(strings.TrimSpace(text), "%d", &parsed); scanErr == nil {
			return parsed, true
		}
	}

	return 0, false
}

func bytesTrimSpace(raw json.RawMessage) json.RawMessage {
	return json.RawMessage(strings.TrimSpace(string(raw)))
}
