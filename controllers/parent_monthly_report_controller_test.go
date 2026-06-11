package controllers

import (
	"strings"
	"testing"
	"time"
)

func TestParentMonthlyReportPhonePrefersLinkedParentPhone(t *testing.T) {
	studentPhone := "081111111111"
	parentPhone := "+62 812-3456-7890"

	got, err := parentMonthlyReportPhone(parentMonthlyReportStudent{
		StudentPhone:      &studentPhone,
		LinkedParentPhone: &parentPhone,
	})
	if err != nil {
		t.Fatalf("parentMonthlyReportPhone returned error: %v", err)
	}
	if got != "6281234567890" {
		t.Fatalf("phone = %q, want linked parent number normalized", got)
	}
}

func TestParentMonthlyReportPhoneFallsBackToStudentPhone(t *testing.T) {
	studentPhone := "0812 0000 1111"

	got, err := parentMonthlyReportPhone(parentMonthlyReportStudent{StudentPhone: &studentPhone})
	if err != nil {
		t.Fatalf("parentMonthlyReportPhone returned error: %v", err)
	}
	if got != "6281200001111" {
		t.Fatalf("phone = %q, want student phone normalized", got)
	}
}

func TestSummarizeParentMonthlyGrades(t *testing.T) {
	scoreA := 80.0
	scoreB := 95.5

	got := summarizeParentMonthlyGrades([]parentMonthlyGradeRow{
		{Score: &scoreA, IsSubmitted: true},
		{Score: &scoreB, IsSubmitted: true},
		{Score: nil, IsSubmitted: false},
	})

	if got.TotalAssignments != 3 {
		t.Fatalf("TotalAssignments = %d, want 3", got.TotalAssignments)
	}
	if got.SubmittedCount != 2 {
		t.Fatalf("SubmittedCount = %d, want 2", got.SubmittedCount)
	}
	if got.PendingCount != 1 {
		t.Fatalf("PendingCount = %d, want 1", got.PendingCount)
	}
	if got.GradedCount != 2 {
		t.Fatalf("GradedCount = %d, want 2", got.GradedCount)
	}
	if got.AverageScore == nil || *got.AverageScore != 87.75 {
		t.Fatalf("AverageScore = %v, want 87.75", got.AverageScore)
	}
}

func TestBuildParentMonthlyReportWhatsAppMessageContainsStudentSummaryAndPDFLink(t *testing.T) {
	average := 88.25
	message := buildParentMonthlyReportWhatsAppMessage(
		parentMonthlyReportStudent{
			StudentName: "Budi Santoso",
			ClassName:   "X IPA 1",
		},
		parentMonthlyAttendanceSummary{
			PresentCount:  18,
			LateCount:     2,
			RecordedCount: 20,
		},
		parentMonthlyGradeSummary{
			TotalAssignments: 4,
			GradedCount:      3,
			AverageScore:     &average,
		},
		"Juni 2026",
		"https://cdn.example.test/laporan.pdf",
	)

	wants := []string{
		"Yth. Bapak/Ibu Orang Tua/Wali Budi Santoso",
		"bulan *Juni 2026*",
		"Nama siswa: *Budi Santoso*",
		"Kelas: *X IPA 1*",
		"- Hadir: *18*",
		"- Terlambat: *2*",
		"- Total catatan: *20*",
		"- Tugas/ujian bulan ini: *4*",
		"- Sudah dinilai: *3*",
		"- Rata-rata nilai: *88.25*",
		"https://cdn.example.test/laporan.pdf",
	}
	for _, want := range wants {
		if !strings.Contains(message, want) {
			t.Fatalf("message missing %q:\n%s", want, message)
		}
	}
}

func TestParseParentReportMonth(t *testing.T) {
	got, err := parseParentReportMonth("2026-06", "")
	if err != nil {
		t.Fatalf("parseParentReportMonth returned error: %v", err)
	}
	want := time.Date(2026, 6, 1, 0, 0, 0, 0, jakartaLocation())
	if !got.Equal(want) {
		t.Fatalf("month = %s, want %s", got, want)
	}

	got, err = parseParentReportMonth("", "2026-05-20")
	if err != nil {
		t.Fatalf("parseParentReportMonth from date returned error: %v", err)
	}
	want = time.Date(2026, 5, 1, 0, 0, 0, 0, jakartaLocation())
	if !got.Equal(want) {
		t.Fatalf("month from date = %s, want %s", got, want)
	}

	if _, err := parseParentReportMonth("06-2026", ""); err == nil {
		t.Fatal("expected invalid month error")
	}
}
