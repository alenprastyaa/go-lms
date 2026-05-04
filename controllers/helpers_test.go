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

