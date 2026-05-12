package controllers

import "testing"

func TestCleanAIAnswerFlattensJSONArrayLines(t *testing.T) {
	raw := "[\"$Bank Soal\"]\n[\"membantu membuat quiz dengan mudah.\"]"

	got := cleanAIAnswer(raw)
	want := "$Bank Soal membantu membuat quiz dengan mudah."
	if got != want {
		t.Fatalf("cleanAIAnswer() = %q, want %q", got, want)
	}
}

func TestCleanAIAnswerFormatsStructuredJSON(t *testing.T) {
	raw := `{"answer":"Gunakan menu Bank Soal dulu.","steps":["Buat soal","Pilih ke quiz"],"follow_up":"Mau saya jelaskan contoh alurnya?"}`

	got := cleanAIAnswer(raw)
	want := "Gunakan menu Bank Soal dulu.\n\nLangkah singkat:\n1. Buat soal\n2. Pilih ke quiz\n\nMau saya jelaskan contoh alurnya?"
	if got != want {
		t.Fatalf("cleanAIAnswer() = %q, want %q", got, want)
	}
}

func TestIsUsableAIAnswerRejectsFragment(t *testing.T) {
	cases := []string{
		`["$Bank Soal"]`,
		`+`,
		`["+"]`,
		`$Bank Soal`,
	}

	for _, input := range cases {
		if isUsableAIAnswer(cleanAIAnswer(input)) {
			t.Fatalf("expected answer %q to be rejected", input)
		}
	}
}

func TestIsUsableAIAnswerAcceptsNormalSentence(t *testing.T) {
	answer := "Untuk membuat quiz, buka Bank Soal dulu lalu pilih soal yang ingin diterbitkan."
	if !isUsableAIAnswer(answer) {
		t.Fatalf("expected normal answer to be accepted")
	}
}
