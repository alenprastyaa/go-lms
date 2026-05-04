package utils

import "testing"

func TestToInt(t *testing.T) {
	tests := []struct {
		name string
		in   string
		def  int
		want int
	}{
		{"valid", "10", 1, 10},
		{"zero", "0", 7, 7},
		{"negative", "-5", 7, 7},
		{"invalid", "abc", 9, 9},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToInt(tt.in, tt.def); got != tt.want {
				t.Fatalf("ToInt() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestToString(t *testing.T) {
	if got := ToString(nil); got != "" {
		t.Fatalf("ToString(nil) = %q, want empty", got)
	}
	if got := ToString(123); got != "123" {
		t.Fatalf("ToString(123) = %q, want 123", got)
	}
}

func TestStringPtr(t *testing.T) {
	if got := StringPtr("   "); got != nil {
		t.Fatalf("StringPtr(blank) expected nil")
	}
	got := StringPtr("  user  ")
	if got == nil || *got != "user" {
		t.Fatalf("StringPtr(trimmed) = %#v", got)
	}
}

