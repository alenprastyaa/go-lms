package services

import (
	"strings"
	"testing"
)

func TestNormalizeQuestionTextImprovesAwkwardContext(t *testing.T) {
	raw := "Sebuah perusahaan percetakan memproduksi mug. Biaya produksi (dalam rupiah) untuk x mug per hari dimodelkan oleh fungsi kuadrat C(x) = $x^2 - 100x + 5000$."

	got := normalizeQuestionText(raw)

	if strings.Contains(strings.ToLower(got), "untuk x mug per hari dimodelkan oleh") {
		t.Fatalf("normalizeQuestionText() still contains awkward phrase: %q", got)
	}
	if strings.Contains(got, "$") {
		t.Fatalf("normalizeQuestionText() still contains dollar delimiter: %q", got)
	}
	if !strings.Contains(got, "per hari untuk memproduksi x mug") {
		t.Fatalf("normalizeQuestionText() did not improve phrasing: %q", got)
	}
	if !strings.Contains(strings.ToLower(got), "pangkat") {
		t.Fatalf("normalizeQuestionText() did not convert power notation: %q", got)
	}
}

func TestBuildQuestionBankPromptIncludesNaturalLanguageGuidance(t *testing.T) {
	prompt := buildQuestionBankPrompt(QuestionBankAIInput{
		SubjectName:   "Matematika",
		ClassName:     "X",
		Topic:         "Fungsi Kuadrat",
		QuestionType:  "MCQ",
		QuestionCount: 5,
	})

	expectations := []string{
		"Kalimat soal harus terdengar seperti soal buku latihan atau ujian yang ditulis guru",
		"Hindari frasa kaku, berulang, atau janggal",
		"Untuk soal matematika atau kontekstual, pilih narasi yang mengalir alami",
	}

	for _, expected := range expectations {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt does not contain %q", expected)
		}
	}
}

func TestBuildQuestionBankPromptIncludesQuestionStyleInstruction(t *testing.T) {
	prompt := buildQuestionBankPrompt(QuestionBankAIInput{
		SubjectName:   "Matematika",
		ClassName:     "X",
		Topic:         "Fungsi Kuadrat",
		QuestionType:  "MCQ",
		QuestionCount: 5,
		QuestionStyle: "MULTI_STEP",
	})

	if !strings.Contains(prompt, "Model soal: soal beranak/bertahap") {
		t.Fatalf("prompt does not contain multi-step question style instruction: %q", prompt)
	}
}

func TestCleanOptionLabelDoesNotPanicOnShortOrSpacedLabels(t *testing.T) {
	cases := map[string]string{
		"A ":            "A",
		"A. Jakarta":    "Jakarta",
		"b) Bandung":    "Bandung",
		"C: Surabaya":   "Surabaya",
		"Pilihan bebas": "Pilihan bebas",
		"":              "",
	}

	for input, expected := range cases {
		if got := cleanOptionLabel(input); got != expected {
			t.Fatalf("cleanOptionLabel(%q) = %q, want %q", input, got, expected)
		}
	}
}
