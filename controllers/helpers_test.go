package controllers

import "testing"

func TestDetectChatMessageType(t *testing.T) {
	tests := []struct {
		name       string
		explicit   string
		mime       string
		nameFile   string
		urlFile    string
		wantResult string
	}{
		{"explicit", "voice", "", "", "", "VOICE"},
		{"image by mime", "", "image/png", "a.png", "", "IMAGE"},
		{"file by attachment", "", "application/pdf", "a.pdf", "", "FILE"},
		{"text no attachment", "", "", "", "", "TEXT"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectChatMessageType(tt.explicit, tt.mime, tt.nameFile, tt.urlFile)
			if got != tt.wantResult {
				t.Fatalf("got %q want %q", got, tt.wantResult)
			}
		})
	}
}

func TestGeneralHelpers(t *testing.T) {
	if got := nullIfEmpty("   "); got != nil {
		t.Fatalf("nullIfEmpty blank expected nil")
	}
	if got := nullIfZero(0); got != nil {
		t.Fatalf("nullIfZero(0) expected nil")
	}
	if got := ternaryString(true, "y", "n"); got != "y" {
		t.Fatalf("ternaryString expected y")
	}
	if got := asString("  test "); got != "test" {
		t.Fatalf("asString trim mismatch")
	}
	if got := toJSONRaw(map[string]any{"a": 1}); got == nil {
		t.Fatalf("toJSONRaw should not be nil")
	}
	if got := fallbackTopic(" "); got != "materi" {
		t.Fatalf("fallbackTopic mismatch")
	}
	if got := fallbackTitle("", "Matematika"); got != "Materi Matematika" {
		t.Fatalf("fallbackTitle mismatch: %q", got)
	}
	if got := max(2, 9); got != 9 {
		t.Fatalf("max mismatch")
	}
	if got := toIntAny(float64(8)); got != 8 {
		t.Fatalf("toIntAny mismatch")
	}
}

func TestNormalizeQuestionBankImportContentSkipsTemplateExample(t *testing.T) {
	content := `TEMPLATE BANK SOAL
CONTOH SOAL
1. Siapakah proklamator kemerdekaan Republik Indonesia?
A. Ir. Soekarno
B. Mohammad Hatta
C. Ir. Soekarno dan Mohammad Hatta
D. Soeharto
E. BJ. Habibie
Jawaban: C

ISI SOAL ANDA DI BAWAH INI
1. Pertanyaan asli dari guru?
A. Opsi A
B. Opsi B
C. Opsi C
D. Opsi D
E. Opsi E
Jawaban: A`

	normalized := normalizeQuestionBankImportContent(content)
	items := parseNumberedQuestionTemplate(normalized)

	if len(items) != 1 {
		t.Fatalf("got %d imported items, want 1", len(items))
	}
	if got := items[0]["question_text"]; got != "Pertanyaan asli dari guru?" {
		t.Fatalf("got question %q, want teacher question only", got)
	}
}

func TestParseNumberedQuestionTemplateReadsAnswerVariants(t *testing.T) {
	content := `1. Soal jawaban B?
A. Salah A
B. Benar B
C. Salah C
D. Salah D
E. Salah E
Jawaban : B

2. Soal jawaban C?
A. Salah A
B. Salah B
C. Benar C
D. Salah D
E. Salah E
Jawaban：C

3. Soal jawaban D?
A. Salah A
B. Salah B
C. Salah C
D. Benar D
E. Salah E
Jawaban:
D`

	items := parseNumberedQuestionTemplate(content)
	if len(items) != 3 {
		t.Fatalf("got %d imported items, want 3", len(items))
	}

	wants := []int{1, 2, 3}
	for i, want := range wants {
		got, ok := items[i]["correct_option"].(int)
		if !ok || got != want {
			t.Fatalf("item %d got correct_option %v, want %d", i+1, items[i]["correct_option"], want)
		}
	}
}

func TestParseNumberedQuestionTemplateSupportsCompactAndSplitNumbers(t *testing.T) {
	content := `1.Siapakah presiden pertama Republik Indonesia?
A. Soeharto
B. Mohammad Hatta
C. Ir. Soekarno
D. B.J. Habibie
E. Susilo Bambang Yudhoyono
Jawaban: C

2.
Ibu kota Indonesia saat ini adalah...
A. Bandung
B. Surabaya
C. Medan
D. Jakarta
E. Makassar
Jawaban: D`

	items := parseNumberedQuestionTemplate(content)
	if len(items) != 2 {
		t.Fatalf("got %d imported items, want 2", len(items))
	}
	if got := items[0]["question_text"]; got != "Siapakah presiden pertama Republik Indonesia?" {
		t.Fatalf("got first question %q", got)
	}
	if got := items[1]["question_text"]; got != "Ibu kota Indonesia saat ini adalah..." {
		t.Fatalf("got second question %q", got)
	}
	if got := items[0]["correct_option"]; got != 2 {
		t.Fatalf("got first correct_option %v, want 2", got)
	}
	if got := items[1]["correct_option"]; got != 3 {
		t.Fatalf("got second correct_option %v, want 3", got)
	}
}

func TestParseNumberedQuestionTemplateSkipsMCQWithoutValidAnswer(t *testing.T) {
	content := `1. Soal tanpa jawaban valid?
A. Opsi A
B. Opsi B
C. Opsi C
D. Opsi D
E. Opsi E
Jawaban:`

	items := parseNumberedQuestionTemplate(content)
	if len(items) != 0 {
		t.Fatalf("got %d imported items, want 0", len(items))
	}
}

func TestParseSimpleMCQTemplateDoesNotDefaultAnswersToA(t *testing.T) {
	content := `1. Soal tanpa kunci legacy?
A. Opsi A
B. Opsi B
C. Opsi C
D. Opsi D
E. Opsi E`

	items := parseSimpleMCQTemplate(content)
	if len(items) != 0 {
		t.Fatalf("got %d legacy items, want 0 without answer key", len(items))
	}
}

func TestLooksLikeModernQuestionBankTemplate(t *testing.T) {
	if !looksLikeModernQuestionBankTemplate("1. Soal\nJawaban: B") {
		t.Fatalf("modern template with Jawaban label should be detected")
	}
	if looksLikeModernQuestionBankTemplate("KUNCI JAWABAN\n1. B") {
		t.Fatalf("legacy answer key format should not be treated as modern")
	}
}
