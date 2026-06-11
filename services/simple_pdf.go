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

var (
	pdfNavy     = colorRGB{15, 23, 42}
	pdfSlate700 = colorRGB{51, 65, 85}
	pdfSlate500 = colorRGB{100, 116, 139}
	pdfSlate400 = colorRGB{148, 163, 184}
	pdfSlate200 = colorRGB{226, 232, 240}
	pdfSlate100 = colorRGB{241, 245, 249}
	pdfSlate50  = colorRGB{248, 250, 252}
	pdfWhite    = colorRGB{255, 255, 255}
	pdfEmerald  = colorRGB{16, 185, 129}
	pdfTeal     = colorRGB{20, 184, 166}
	pdfSky      = colorRGB{14, 165, 233}
	pdfAmber    = colorRGB{245, 158, 11}
	pdfRose     = colorRGB{244, 63, 94}
)

func BuildMonthlyStudentReportPDF(input MonthlyStudentReportPDFInput) []byte {
	canvas := newPDFCanvas()

	// ===== Header =====
	canvas.rect(0, 0, 595, 116, pdfNavy)
	canvas.rect(0, 112, 199, 4, pdfEmerald)
	canvas.rect(199, 112, 198, 4, pdfSky)
	canvas.rect(397, 112, 198, 4, pdfAmber)
	canvas.text(42, 40, 8.5, "F2", pdfSlate400, "L A P O R A N   B U L A N A N   S I S W A")
	canvas.text(42, 68, 19, "F2", pdfWhite, truncatePDFText(fallbackPDFText(input.SchoolName, "Sekolah"), 42))

	monthText := strings.ToUpper(fallbackPDFText(input.MonthLabel, "Periode laporan"))
	pillW := approxPDFTextWidth(monthText, 10, true) + 26
	canvas.rect(553-pillW, 82, pillW, 22, pdfEmerald)
	canvas.text(553-pillW+13, 97, 10, "F2", pdfWhite, monthText)
	canvas.text(42, 97, 9, "F1", pdfSlate400, "Ringkasan kehadiran dan hasil belajar untuk orang tua/wali")

	// ===== Kartu identitas =====
	canvas.roundedCard(42, 134, 511, 64, pdfWhite)
	canvas.rect(42, 134, 4, 64, pdfEmerald)
	canvas.text(66, 158, 8, "F2", pdfSlate500, "NAMA SISWA")
	canvas.text(66, 184, 16, "F2", pdfNavy, truncatePDFText(fallbackPDFText(input.StudentName, "Siswa"), 34))
	canvas.text(372, 158, 8, "F2", pdfSlate500, "KELAS")
	canvas.text(372, 184, 14, "F2", pdfNavy, truncatePDFText(fallbackPDFText(input.ClassName, "-"), 18))

	// ===== Ringkasan kehadiran =====
	canvas.sectionTitle(230, "Ringkasan Kehadiran", pdfEmerald)

	attended := input.PresentCount + input.LateCount
	attendanceRate := pdfFraction(attended, input.RecordedCount)
	rateColor := pdfEmerald
	switch {
	case input.RecordedCount == 0:
		rateColor = pdfSlate400
	case attendanceRate < 0.70:
		rateColor = pdfRose
	case attendanceRate < 0.85:
		rateColor = pdfAmber
	}

	canvas.roundedCard(42, 246, 158, 140, pdfSlate50)
	canvas.text(58, 272, 7.5, "F2", pdfSlate500, "TINGKAT KEHADIRAN")
	rateText := "-"
	if input.RecordedCount > 0 {
		rateText = fmt.Sprintf("%d%%", int(attendanceRate*100+0.5))
	}
	canvas.text(58, 308, 30, "F2", rateColor, rateText)
	canvas.progressBar(58, 322, 126, 8, attendanceRate, rateColor)
	if input.RecordedCount > 0 {
		canvas.text(58, 348, 7.5, "F1", pdfSlate500, fmt.Sprintf("%d dari %d hari tercatat", attended, input.RecordedCount))
		canvas.text(58, 360, 7.5, "F1", pdfSlate500, "hadir atau terlambat")
	} else {
		canvas.text(58, 348, 7.5, "F1", pdfSlate500, "Belum ada catatan")
		canvas.text(58, 360, 7.5, "F1", pdfSlate500, "kehadiran bulan ini")
	}

	canvas.barRow(222, 252, 331, "Hadir", pdfCountLabel(input.PresentCount, input.RecordedCount, "hari"), pdfFraction(input.PresentCount, input.RecordedCount), pdfEmerald)
	canvas.barRow(222, 297, 331, "Terlambat", pdfCountLabel(input.LateCount, input.RecordedCount, "hari"), pdfFraction(input.LateCount, input.RecordedCount), pdfAmber)
	canvas.barRow(222, 342, 331, "Tidak Hadir / Lainnya", pdfCountLabel(input.AbsentCount, input.RecordedCount, "hari"), pdfFraction(input.AbsentCount, input.RecordedCount), pdfRose)

	// ===== Ringkasan nilai =====
	canvas.sectionTitle(420, "Ringkasan Nilai", pdfSky)

	averageValue, hasAverage := parsePDFScore(input.AverageScore)
	canvas.roundedCard(42, 436, 158, 140, pdfSlate50)
	canvas.text(58, 462, 7.5, "F2", pdfSlate500, "RATA-RATA NILAI")
	if hasAverage {
		scoreColor, predicate := pdfScorePalette(averageValue)
		canvas.text(58, 498, 30, "F2", scoreColor, formatPDFScore(averageValue))
		predicateW := approxPDFTextWidth(predicate, 8, true) + 20
		canvas.rect(58, 510, predicateW, 17, scoreColor)
		canvas.text(68, 522, 8, "F2", pdfWhite, predicate)
		canvas.text(58, 552, 7.5, "F1", pdfSlate500, "Skala penilaian 0 - 100")
	} else {
		canvas.text(58, 498, 22, "F2", pdfSlate400, "-")
		canvas.text(58, 524, 8, "F1", pdfSlate500, "Belum ada nilai yang")
		canvas.text(58, 536, 8, "F1", pdfSlate500, "tersedia bulan ini")
	}

	canvas.barRow(222, 442, 331, "Tugas Dikumpulkan", pdfCountLabel(input.SubmittedCount, input.TotalAssignments, "tugas"), pdfFraction(input.SubmittedCount, input.TotalAssignments), pdfTeal)
	canvas.barRow(222, 487, 331, "Tugas Sudah Dinilai", pdfCountLabel(input.GradedCount, input.TotalAssignments, "tugas"), pdfFraction(input.GradedCount, input.TotalAssignments), pdfSky)
	canvas.barRow(222, 532, 331, "Belum Dikumpulkan", pdfCountLabel(input.PendingCount, input.TotalAssignments, "tugas"), pdfFraction(input.PendingCount, input.TotalAssignments), pdfAmber)

	// ===== Tabel detail nilai =====
	canvas.sectionTitle(606, "Detail Nilai Bulan Ini", pdfAmber)
	canvas.rect(42, 620, 511, 22, colorRGB{30, 41, 59})
	canvas.text(58, 635, 8, "F2", pdfWhite, "MATA PELAJARAN")
	canvas.text(170, 635, 8, "F2", pdfWhite, "TUGAS / UJIAN")
	canvas.text(396, 635, 8, "F2", pdfWhite, "NILAI")
	canvas.text(462, 635, 8, "F2", pdfWhite, "STATUS")

	rows := input.GradeRows
	totalRows := len(rows)
	if totalRows == 0 {
		canvas.rect(42, 642, 511, 38, pdfSlate50)
		canvas.text(58, 666, 9, "F1", pdfSlate500, "Belum ada data nilai pada periode ini.")
	} else {
		if len(rows) > 6 {
			rows = rows[:6]
		}
		y := 642.0
		for index, row := range rows {
			bg := pdfWhite
			if index%2 == 1 {
				bg = pdfSlate50
			}
			canvas.rect(42, y, 511, 25, bg)
			canvas.text(58, y+16, 9, "F2", pdfNavy, truncatePDFText(row.Subject, 16))
			canvas.text(170, y+16, 9, "F1", pdfSlate700, truncatePDFText(row.Title, 40))

			if score, ok := parsePDFScore(row.Score); ok {
				scoreColor, _ := pdfScorePalette(score)
				canvas.rect(392, y+4.5, 46, 16, scoreColor)
				scoreText := formatPDFScore(score)
				canvas.text(392+(46-approxPDFTextWidth(scoreText, 8.5, true))/2, y+16, 8.5, "F2", pdfWhite, scoreText)
			} else {
				canvas.text(392, y+16, 8, "F1", pdfSlate400, truncatePDFText(row.Score, 15))
			}

			statusColor := pdfRose
			if strings.Contains(strings.ToLower(row.Status), "sudah") {
				statusColor = pdfEmerald
			}
			canvas.circle(465, y+12.5, 3, statusColor)
			canvas.text(473, y+16, 8, "F1", pdfSlate700, truncatePDFText(row.Status, 19))
			y += 25
		}
		if totalRows > 6 {
			canvas.text(42, 800, 7.5, "F1", pdfSlate500, fmt.Sprintf("+ %d tugas/ujian lainnya tidak ditampilkan. Detail lengkap dapat dilihat di aplikasi.", totalRows-6))
		}
	}

	// ===== Footer =====
	canvas.rect(42, 808, 511, 1, pdfSlate200)
	canvas.text(42, 822, 8, "F1", pdfSlate500, "Dokumen ini dibuat otomatis oleh sistem sekolah.")
	canvas.text(42, 833, 8, "F1", pdfSlate500, "Silakan hubungi pihak sekolah apabila terdapat data yang perlu dikonfirmasi.")
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

func (p *pdfCanvas) sectionTitle(yTop float64, title string, accent colorRGB) {
	p.rect(42, yTop-11, 11, 11, accent)
	p.text(60, yTop, 13, "F2", pdfNavy, title)
}

func (p *pdfCanvas) progressBar(x, yTop, w, h, fraction float64, color colorRGB) {
	p.rect(x, yTop, w, h, pdfSlate200)
	fill := fraction * w
	if fraction > 0 && fill < 4 {
		fill = 4
	}
	if fill > w {
		fill = w
	}
	if fill > 0 {
		p.rect(x, yTop, fill, h, color)
	}
}

func (p *pdfCanvas) barRow(x, yTop, w float64, label, value string, fraction float64, color colorRGB) {
	p.text(x, yTop+10, 10, "F2", pdfNavy, label)
	p.text(x+w-approxPDFTextWidth(value, 9, false), yTop+10, 9, "F1", pdfSlate700, value)
	p.progressBar(x, yTop+18, w, 9, fraction, color)
}

func (p *pdfCanvas) circle(cx, cyTop, r float64, color colorRGB) {
	k := 0.5523 * r
	cy := 842 - cyTop
	p.content.WriteString(fmt.Sprintf("%.3f %.3f %.3f rg\n", float64(color.R)/255, float64(color.G)/255, float64(color.B)/255))
	p.content.WriteString(fmt.Sprintf("%.2f %.2f m\n", cx+r, cy))
	p.content.WriteString(fmt.Sprintf("%.2f %.2f %.2f %.2f %.2f %.2f c\n", cx+r, cy+k, cx+k, cy+r, cx, cy+r))
	p.content.WriteString(fmt.Sprintf("%.2f %.2f %.2f %.2f %.2f %.2f c\n", cx-k, cy+r, cx-r, cy+k, cx-r, cy))
	p.content.WriteString(fmt.Sprintf("%.2f %.2f %.2f %.2f %.2f %.2f c\n", cx-r, cy-k, cx-k, cy-r, cx, cy-r))
	p.content.WriteString(fmt.Sprintf("%.2f %.2f %.2f %.2f %.2f %.2f c\n", cx+k, cy-r, cx+r, cy-k, cx+r, cy))
	p.content.WriteString("f\n")
}

// approxPDFTextWidth estimates Helvetica text width for alignment; the built-in
// Type1 fonts carry no embedded metrics here, so an average glyph factor is used.
func approxPDFTextWidth(value string, size float64, bold bool) float64 {
	factor := 0.50
	if bold {
		factor = 0.55
	}
	return float64(len(value)) * size * factor
}

func pdfFraction(part, total int) float64 {
	if total <= 0 || part <= 0 {
		return 0
	}
	fraction := float64(part) / float64(total)
	if fraction > 1 {
		return 1
	}
	return fraction
}

func pdfCountLabel(part, total int, unit string) string {
	if total <= 0 {
		return "Belum ada data"
	}
	return fmt.Sprintf("%d dari %d %s (%d%%)", part, total, unit, int(pdfFraction(part, total)*100+0.5))
}

func parsePDFScore(value string) (float64, bool) {
	score, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || score < 0 {
		return 0, false
	}
	return score, true
}

func formatPDFScore(score float64) string {
	if score == float64(int(score)) {
		return strconv.Itoa(int(score))
	}
	return strconv.FormatFloat(score, 'f', 1, 64)
}

func pdfScorePalette(score float64) (colorRGB, string) {
	switch {
	case score >= 85:
		return pdfEmerald, "SANGAT BAIK"
	case score >= 75:
		return pdfSky, "BAIK"
	case score >= 60:
		return pdfAmber, "CUKUP"
	default:
		return pdfRose, "PERLU BIMBINGAN"
	}
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
