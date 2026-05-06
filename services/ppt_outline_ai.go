package services

import (
	"encoding/json"
	"fmt"
	"strings"
)

type PowerPointAIInput struct {
	SubjectName            string
	ClassName              string
	Topic                  string
	MaterialTitle          string
	SlideCount             int
	TeacherSummary         string
	LearningGoals          string
	AdditionalInstructions string
}

type PowerPointAIOutline struct {
	PresentationTitle string
	Summary           string
	Slides            []PowerPointSlide
}

type powerPointAIResponse struct {
	PresentationTitle string                 `json:"presentation_title"`
	Summary           string                 `json:"summary"`
	Slides            []powerPointAIRawSlide `json:"slides"`
}

type powerPointAIRawSlide struct {
	Title        string   `json:"title"`
	Bullets      []string `json:"bullets"`
	SpeakerNotes string   `json:"speaker_notes"`
}

func buildPowerPointPrompt(input PowerPointAIInput) string {
	parts := []string{
		"Anda adalah asisten guru yang membuat outline presentasi PowerPoint untuk LMS sekolah.",
		"Gunakan Bahasa Indonesia yang formal, jelas, ringkas, dan cocok untuk siswa sekolah.",
		fmt.Sprintf("Mata pelajaran: %s.", fallbackText(input.SubjectName, "-")),
		fmt.Sprintf("Kelas: %s.", fallbackText(input.ClassName, "-")),
		fmt.Sprintf("Judul presentasi: %s.", strings.TrimSpace(input.MaterialTitle)),
		fmt.Sprintf("Topik utama: %s.", strings.TrimSpace(input.Topic)),
		fmt.Sprintf("Jumlah slide: %d.", input.SlideCount),
	}

	if strings.TrimSpace(input.TeacherSummary) != "" {
		parts = append(parts, fmt.Sprintf("Ringkasan awal dari guru: %s.", strings.TrimSpace(input.TeacherSummary)))
	}
	if strings.TrimSpace(input.LearningGoals) != "" {
		parts = append(parts, fmt.Sprintf("Tujuan pembelajaran: %s.", strings.TrimSpace(input.LearningGoals)))
	}
	if strings.TrimSpace(input.AdditionalInstructions) != "" {
		parts = append(parts, fmt.Sprintf("Instruksi tambahan: %s.", strings.TrimSpace(input.AdditionalInstructions)))
	}

	parts = append(parts,
		"Setiap slide wajib memiliki judul singkat dan 3 sampai 5 poin bullet yang padat.",
		"Jangan gunakan markdown. Jangan gunakan tabel. Jangan gunakan penjelasan di luar JSON.",
		"Kembalikan JSON valid saja dengan struktur:",
		`{"presentation_title":"...","summary":"...","slides":[{"title":"...","bullets":["...","..."],"speaker_notes":"..."}]}`,
		"speaker_notes boleh singkat, maksimal 2 kalimat, dan opsional.",
	)

	return strings.Join(parts, "\n")
}

func GeneratePowerPointOutlineWithHuggingFace(input PowerPointAIInput) (*PowerPointAIOutline, error) {
	if input.SlideCount < 3 {
		input.SlideCount = 3
	}
	if input.SlideCount > 15 {
		input.SlideCount = 15
	}

	text, err := callHuggingFace(
		buildPowerPointPrompt(input),
		"Anda adalah asisten guru yang membuat outline presentasi dan wajib mengembalikan JSON valid tanpa markdown.",
		0.7,
	)
	if err != nil {
		return nil, err
	}

	var parsed powerPointAIResponse
	if err := json.Unmarshal([]byte(extractJSONObject(text)), &parsed); err != nil {
		return nil, fmt.Errorf("hasil Hugging Face tidak bisa diparsing sebagai JSON presentasi: %w", err)
	}

	slides := normalizePowerPointSlides(parsed.Slides, input.MaterialTitle, input.SlideCount)
	if len(slides) == 0 {
		return nil, fmt.Errorf("hasil Hugging Face tidak valid untuk dijadikan presentasi")
	}

	presentationTitle := strings.TrimSpace(parsed.PresentationTitle)
	if presentationTitle == "" {
		presentationTitle = strings.TrimSpace(input.MaterialTitle)
	}
	if presentationTitle == "" {
		presentationTitle = strings.TrimSpace(input.Topic)
	}
	if presentationTitle == "" {
		presentationTitle = "Materi Pembelajaran"
	}

	summary := strings.TrimSpace(parsed.Summary)
	if summary == "" {
		summary = strings.TrimSpace(input.TeacherSummary)
	}
	if summary == "" {
		summary = strings.TrimSpace(input.Topic)
	}
	if summary == "" {
		summary = "Materi presentasi pembelajaran hasil generate AI."
	}

	return &PowerPointAIOutline{
		PresentationTitle: presentationTitle,
		Summary:           summary,
		Slides:            slides,
	}, nil
}

func normalizePowerPointSlides(slides []powerPointAIRawSlide, fallbackTitle string, slideCount int) []PowerPointSlide {
	normalized := make([]PowerPointSlide, 0, len(slides))
	for index, slide := range slides {
		title := strings.TrimSpace(slide.Title)
		if title == "" {
			title = fmt.Sprintf("%s %d", fallbackTitle, index+1)
		}
		bullets := normalizeSlideBullets(slide.Bullets)
		if len(bullets) == 0 {
			continue
		}
		notes := strings.TrimSpace(slide.SpeakerNotes)
		if len(notes) > 0 {
			notes = strings.TrimSpace(notes)
		}
		normalized = append(normalized, PowerPointSlide{
			Title:        title,
			Bullets:      bullets,
			SpeakerNotes: notes,
		})
	}
	if len(normalized) > slideCount {
		normalized = normalized[:slideCount]
	}
	return normalized
}
