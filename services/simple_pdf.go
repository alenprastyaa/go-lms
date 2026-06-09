package services

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

type MonthlyStudentReportPDFInput struct {
	SchoolName       string
	StudentName      string
	ClassName        string
	MonthLabel       string
	PresentCount     int
	LateCount        int
	AbsentCount      int
	RecordedCount    int
	TotalAssignments int
	SubmittedCount   int
	PendingCount     int
	GradedCount      int
	AverageScore     string
	GradeRows        []MonthlyStudentReportPDFGradeRow
}

type MonthlyStudentReportPDFGradeRow struct {
	Subject string
	Title   string
	Score   string
	Status  string
}

func BuildSimpleTextPDF(title string, lines []string) []byte {
	normalizedLines := make([]string, 0, len(lines)+1)
	if strings.TrimSpace(title) != "" {
		normalizedLines = append(normalizedLines, title, "")
	}
	for _, line := range lines {
		normalizedLines = append(normalizedLines, wrapPDFLine(line, 88)...)
	}
	if len(normalizedLines) == 0 {
		normalizedLines = append(normalizedLines, "Laporan")
	}
	if len(normalizedLines) > 48 {
		normalizedLines = normalizedLines[:48]
		normalizedLines = append(normalizedLines, "... laporan dipersingkat untuk tampilan PDF.")
	}

	var content bytes.Buffer
	content.WriteString("BT\n/F1 11 Tf\n50 790 Td\n15 TL\n")
	for _, line := range normalizedLines {
		content.WriteString("(")
		content.WriteString(escapePDFText(line))
		content.WriteString(") Tj\nT*\n")
	}
	content.WriteString("ET\n")

	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", content.Len(), content.String()),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}

	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.4\n")
	offsets := make([]int, 0, len(objects)+1)
	offsets = append(offsets, 0)
	for index, object := range objects {
		offsets = append(offsets, pdf.Len())
		pdf.WriteString(strconv.Itoa(index + 1))
		pdf.WriteString(" 0 obj\n")
		pdf.WriteString(object)
		pdf.WriteString("\nendobj\n")
	}

	xrefOffset := pdf.Len()
	pdf.WriteString("xref\n")
	pdf.WriteString(fmt.Sprintf("0 %d\n", len(objects)+1))
	pdf.WriteString("0000000000 65535 f \n")
	for _, offset := range offsets[1:] {
		pdf.WriteString(fmt.Sprintf("%010d 00000 n \n", offset))
	}
	pdf.WriteString("trailer\n")
	pdf.WriteString(fmt.Sprintf("<< /Size %d /Root 1 0 R >>\n", len(objects)+1))
	pdf.WriteString("startxref\n")
	pdf.WriteString(strconv.Itoa(xrefOffset))
	pdf.WriteString("\n%%EOF\n")
	return pdf.Bytes()
}

func BuildMonthlyStudentReportPDF(input MonthlyStudentReportPDFInput) []byte {
	canvas := newPDFCanvas()
	canvas.rect(0, 0, 595, 128, colorRGB{15, 23, 42})
	canvas.rect(0, 120, 595, 8, colorRGB{16, 185, 129})
	canvas.text(42, 48, 21, "F2", colorRGB{255, 255, 255}, "Laporan Bulanan Siswa")
	canvas.text(42, 72, 11, "F1", colorRGB{203, 213, 225}, fallbackPDFText(input.SchoolName, "Sekolah"))
	canvas.text(42, 94, 12, "F2", colorRGB{167, 243, 208}, fallbackPDFText(input.MonthLabel, "Periode laporan"))

	canvas.roundedCard(42, 150, 511, 86, colorRGB{248, 250, 252})
	canvas.text(62, 178, 10, "F2", colorRGB{100, 116, 139}, "IDENTITAS SISWA")
	canvas.text(62, 205, 20, "F2", colorRGB{15, 23, 42}, fallbackPDFText(input.StudentName, "Siswa"))
	canvas.text(382, 184, 10, "F2", colorRGB{100, 116, 139}, "KELAS")
	canvas.text(382, 207, 18, "F2", colorRGB{15, 23, 42}, fallbackPDFText(input.ClassName, "-"))

	canvas.text(42, 274, 15, "F2", colorRGB{15, 23, 42}, "Ringkasan Kehadiran")
	canvas.statCard(42, 292, "Hadir", strconv.Itoa(input.PresentCount), colorRGB{16, 185, 129})
	canvas.statCard(172, 292, "Terlambat", strconv.Itoa(input.LateCount), colorRGB{245, 158, 11})
	canvas.statCard(302, 292, "Catatan Lain", strconv.Itoa(input.AbsentCount), colorRGB{244, 63, 94})
	canvas.statCard(432, 292, "Total Catatan", strconv.Itoa(input.RecordedCount), colorRGB{14, 165, 233})

	canvas.text(42, 402, 15, "F2", colorRGB{15, 23, 42}, "Ringkasan Nilai")
	canvas.statCard(42, 420, "Tugas/Ujian", strconv.Itoa(input.TotalAssignments), colorRGB{99, 102, 241})
	canvas.statCard(172, 420, "Dikumpulkan", strconv.Itoa(input.SubmittedCount), colorRGB{20, 184, 166})
	canvas.statCard(302, 420, "Dinilai", strconv.Itoa(input.GradedCount), colorRGB{59, 130, 246})
	canvas.statCard(432, 420, "Rata-rata", fallbackPDFText(input.AverageScore, "-"), colorRGB{15, 23, 42})

	canvas.text(42, 530, 15, "F2", colorRGB{15, 23, 42}, "Nilai Terbaru Bulan Ini")
	canvas.rect(42, 548, 511, 28, colorRGB{226, 232, 240})
	canvas.text(58, 567, 9, "F2", colorRGB{51, 65, 85}, "Mapel")
	canvas.text(176, 567, 9, "F2", colorRGB{51, 65, 85}, "Tugas/Ujian")
	canvas.text(395, 567, 9, "F2", colorRGB{51, 65, 85}, "Nilai")
	canvas.text(456, 567, 9, "F2", colorRGB{51, 65, 85}, "Status")

	rows := input.GradeRows
	if len(rows) == 0 {
		canvas.rect(42, 576, 511, 38, colorRGB{248, 250, 252})
		canvas.text(58, 600, 10, "F1", colorRGB{100, 116, 139}, "Belum ada data nilai pada periode ini.")
	} else {
		if len(rows) > 9 {
			rows = rows[:9]
		}
		y := 576
		for index, row := range rows {
			bg := colorRGB{255, 255, 255}
			if index%2 == 1 {
				bg = colorRGB{248, 250, 252}
			}
			canvas.rect(42, float64(y), 511, 42, bg)
			canvas.text(58, float64(y+18), 9, "F2", colorRGB{15, 23, 42}, truncatePDFText(row.Subject, 18))
			canvas.text(176, float64(y+18), 9, "F1", colorRGB{51, 65, 85}, truncatePDFText(row.Title, 42))
			canvas.text(395, float64(y+18), 9, "F2", colorRGB{15, 23, 42}, truncatePDFText(row.Score, 10))
			canvas.text(456, float64(y+18), 9, "F1", colorRGB{51, 65, 85}, truncatePDFText(row.Status, 18))
			y += 42
		}
	}

	canvas.text(42, 804, 8, "F1", colorRGB{100, 116, 139}, "Dokumen ini dibuat otomatis oleh sistem sekolah. Silakan hubungi pihak sekolah apabila terdapat data yang perlu dikonfirmasi.")
	return canvas.build()
}

func wrapPDFLine(line string, limit int) []string {
	clean := strings.TrimSpace(line)
	if clean == "" || len(clean) <= limit {
		return []string{clean}
	}

	words := strings.Fields(clean)
	lines := make([]string, 0, 2)
	current := ""
	for _, word := range words {
		if current == "" {
			current = word
			continue
		}
		if len(current)+1+len(word) <= limit {
			current += " " + word
			continue
		}
		lines = append(lines, current)
		current = word
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func escapePDFText(value string) string {
	var builder strings.Builder
	for _, r := range value {
		switch r {
		case '\\', '(', ')':
			builder.WriteByte('\\')
			builder.WriteRune(r)
		case '\r', '\n', '\t':
			builder.WriteByte(' ')
		default:
			if r < 32 || r > 126 {
				builder.WriteByte(' ')
				continue
			}
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

type colorRGB struct {
	R int
	G int
	B int
}

type pdfCanvas struct {
	content bytes.Buffer
}

func newPDFCanvas() *pdfCanvas {
	return &pdfCanvas{}
}

func (p *pdfCanvas) rect(x, yTop, w, h float64, color colorRGB) {
	p.content.WriteString(fmt.Sprintf("%.3f %.3f %.3f rg\n", float64(color.R)/255, float64(color.G)/255, float64(color.B)/255))
	p.content.WriteString(fmt.Sprintf("%.1f %.1f %.1f %.1f re f\n", x, 842-yTop-h, w, h))
}

func (p *pdfCanvas) roundedCard(x, yTop, w, h float64, color colorRGB) {
	p.rect(x, yTop, w, h, color)
	p.content.WriteString("0.886 0.909 0.941 RG\n")
	p.content.WriteString(fmt.Sprintf("%.1f %.1f %.1f %.1f re S\n", x, 842-yTop-h, w, h))
}

func (p *pdfCanvas) statCard(x, yTop float64, label, value string, accent colorRGB) {
	p.roundedCard(x, yTop, 112, 70, colorRGB{248, 250, 252})
	p.rect(x, yTop, 112, 5, accent)
	p.text(x+14, yTop+29, 9, "F2", colorRGB{100, 116, 139}, label)
	p.text(x+14, yTop+55, 18, "F2", colorRGB{15, 23, 42}, value)
}

func (p *pdfCanvas) text(x, yTop, size float64, font string, color colorRGB, value string) {
	p.content.WriteString(fmt.Sprintf("%.3f %.3f %.3f rg\n", float64(color.R)/255, float64(color.G)/255, float64(color.B)/255))
	p.content.WriteString("BT\n")
	p.content.WriteString(fmt.Sprintf("/%s %.1f Tf\n", font, size))
	p.content.WriteString(fmt.Sprintf("%.1f %.1f Td\n", x, 842-yTop))
	p.content.WriteString("(")
	p.content.WriteString(escapePDFText(value))
	p.content.WriteString(") Tj\n")
	p.content.WriteString("ET\n")
}

func (p *pdfCanvas) build() []byte {
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 5 0 R /F2 6 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", p.content.Len(), p.content.String()),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold >>",
	}

	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.4\n")
	offsets := make([]int, 0, len(objects)+1)
	offsets = append(offsets, 0)
	for index, object := range objects {
		offsets = append(offsets, pdf.Len())
		pdf.WriteString(strconv.Itoa(index + 1))
		pdf.WriteString(" 0 obj\n")
		pdf.WriteString(object)
		pdf.WriteString("\nendobj\n")
	}

	xrefOffset := pdf.Len()
	pdf.WriteString("xref\n")
	pdf.WriteString(fmt.Sprintf("0 %d\n", len(objects)+1))
	pdf.WriteString("0000000000 65535 f \n")
	for _, offset := range offsets[1:] {
		pdf.WriteString(fmt.Sprintf("%010d 00000 n \n", offset))
	}
	pdf.WriteString("trailer\n")
	pdf.WriteString(fmt.Sprintf("<< /Size %d /Root 1 0 R >>\n", len(objects)+1))
	pdf.WriteString("startxref\n")
	pdf.WriteString(strconv.Itoa(xrefOffset))
	pdf.WriteString("\n%%EOF\n")
	return pdf.Bytes()
}

func fallbackPDFText(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func truncatePDFText(value string, limit int) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= limit {
		return trimmed
	}
	if limit <= 3 {
		return trimmed[:limit]
	}
	return strings.TrimSpace(trimmed[:limit-3]) + "..."
}
