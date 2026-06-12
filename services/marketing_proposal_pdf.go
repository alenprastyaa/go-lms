package services

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
)

// ProposalPDFParams holds the dynamic data used to render a marketing
// proposal (Surat Penawaran) document for a specific school.
type ProposalPDFParams struct {
	SchoolName      string
	PricePerStudent int // harga per siswa per bulan, mis. 1700
	DiscountMonths  int // potongan untuk langganan tahunan, mis. 1 bulan
	GeneratedAt     time.Time
	OfferNumber     string
	CompanyName     string
	FounderName     string
	FounderTitle    string
	DemoURL         string
	WhatsApp        string
	ContactEmail    string
}

func (p *ProposalPDFParams) applyDefaults() {
	if strings.TrimSpace(p.SchoolName) == "" {
		p.SchoolName = "Bapak/Ibu Pimpinan Sekolah"
	}
	if p.PricePerStudent <= 0 {
		p.PricePerStudent = 1700
	}
	if p.DiscountMonths < 0 {
		p.DiscountMonths = 0
	}
	if p.DiscountMonths == 0 {
		p.DiscountMonths = 1
	}
	if p.GeneratedAt.IsZero() {
		p.GeneratedAt = time.Now()
	}
	if strings.TrimSpace(p.CompanyName) == "" {
		p.CompanyName = "Bitwize Digital Platform"
	}
	if strings.TrimSpace(p.FounderName) == "" {
		p.FounderName = "Alen Prastya"
	}
	if strings.TrimSpace(p.FounderTitle) == "" {
		p.FounderTitle = "Founder"
	}
	if strings.TrimSpace(p.DemoURL) == "" {
		p.DemoURL = "demo.idschoolsystem.com"
	}
	if strings.TrimSpace(p.WhatsApp) == "" {
		p.WhatsApp = "0857-1957-8195"
	}
	if strings.TrimSpace(p.ContactEmail) == "" {
		p.ContactEmail = "no-reply@school-system.my.id"
	}
	if strings.TrimSpace(p.OfferNumber) == "" {
		p.OfferNumber = defaultOfferNumber(p.SchoolName, p.GeneratedAt)
	}
}

var indonesianMonths = []string{
	"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember",
}

var romanMonths = []string{
	"", "I", "II", "III", "IV", "V", "VI",
	"VII", "VIII", "IX", "X", "XI", "XII",
}

func formatIndonesianDate(t time.Time) string {
	return fmt.Sprintf("%d %s %d", t.Day(), indonesianMonths[int(t.Month())], t.Year())
}

func defaultOfferNumber(schoolName string, t time.Time) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.ToLower(strings.TrimSpace(schoolName))))
	seq := (h.Sum32() % 900) + 100
	return fmt.Sprintf("%03d/PNW-BWZ/%s/%d", seq, romanMonths[int(t.Month())], t.Year())
}

// formatRupiah formats an integer amount as "Rp 1.700".
func formatRupiah(amount int) string {
	negative := amount < 0
	if negative {
		amount = -amount
	}
	digits := fmt.Sprintf("%d", amount)
	var b strings.Builder
	for i, c := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	if negative {
		return "Rp -" + b.String()
	}
	return "Rp " + b.String()
}

// ProposalPDFFilename returns a safe attachment filename for a school.
func ProposalPDFFilename(schoolName string) string {
	slug := strings.TrimSpace(schoolName)
	if slug == "" {
		slug = "Sekolah"
	}
	var b strings.Builder
	prevDash := false
	for _, r := range slug {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	cleaned := strings.Trim(b.String(), "-")
	if cleaned == "" {
		cleaned = "Sekolah"
	}
	if len(cleaned) > 60 {
		cleaned = strings.Trim(cleaned[:60], "-")
	}
	return "Penawaran-LMS-" + cleaned + ".pdf"
}

// GenerateMarketingProposalPDF renders a professional Indonesian business
// proposal (Surat Penawaran Harga) tailored to the given school and returns
// the PDF as raw bytes, ready to be attached to an email.
func GenerateMarketingProposalPDF(params ProposalPDFParams) ([]byte, error) {
	p := params
	p.applyDefaults()

	const (
		marginL  = 18.0
		marginR  = 18.0
		marginT  = 16.0
		pageW    = 210.0
		contentW = pageW - marginL - marginR
	)

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(marginL, marginT, marginR)
	pdf.SetAutoPageBreak(true, 20)
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	// Palette (RGB)
	navy := []int{15, 23, 42}
	sky := []int{2, 132, 199}
	emerald := []int{22, 163, 74}
	textColor := []int{51, 65, 85}
	muted := []int{100, 116, 139}
	lightBg := []int{248, 250, 252}
	border := []int{226, 232, 240}

	setText := func(c []int) { pdf.SetTextColor(c[0], c[1], c[2]) }
	setFill := func(c []int) { pdf.SetFillColor(c[0], c[1], c[2]) }
	setDraw := func(c []int) { pdf.SetDrawColor(c[0], c[1], c[2]) }
	write := func(s string) string { return tr(s) }

	pdf.SetFooterFunc(func() {
		pdf.SetY(-15)
		setDraw(border)
		pdf.SetLineWidth(0.2)
		pdf.Line(marginL, pdf.GetY(), pageW-marginR, pdf.GetY())
		pdf.Ln(2)
		pdf.SetFont("Arial", "", 7.5)
		setText(muted)
		footer := fmt.Sprintf("%s  •  WhatsApp %s  •  %s", p.CompanyName, p.WhatsApp, p.DemoURL)
		pdf.CellFormat(contentW-20, 5, write(footer), "", 0, "L", false, 0, "")
		pdf.CellFormat(20, 5, write(fmt.Sprintf("Hal. %d", pdf.PageNo())), "", 0, "R", false, 0, "")
	})

	pdf.AddPage()

	// ---- HEADER BAND ----
	setFill(navy)
	pdf.Rect(0, 0, pageW, 40, "F")
	setFill(sky)
	pdf.Rect(0, 40, pageW, 1.6, "F")

	// Logo block
	setFill(sky)
	pdf.Rect(marginL, 12, 16, 16, "F")
	pdf.SetFont("Arial", "B", 14)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetXY(marginL, 12)
	pdf.CellFormat(16, 16, write("BW"), "", 0, "C", false, 0, "")

	pdf.SetXY(marginL+21, 11.5)
	pdf.SetFont("Arial", "B", 17)
	pdf.SetTextColor(255, 255, 255)
	pdf.CellFormat(120, 8, write(p.CompanyName), "", 2, "L", false, 0, "")
	pdf.SetFont("Arial", "", 9.5)
	pdf.SetTextColor(203, 213, 225)
	pdf.CellFormat(150, 6, write("Learning Management System untuk Sekolah Modern"), "", 2, "L", false, 0, "")

	// Right side label inside band
	pdf.SetXY(pageW-marginR-60, 14)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetTextColor(56, 189, 248)
	pdf.CellFormat(60, 6, write("SURAT PENAWARAN"), "", 2, "R", false, 0, "")
	pdf.SetFont("Arial", "", 8)
	pdf.SetTextColor(203, 213, 225)
	pdf.CellFormat(60, 5, write("Harga & Layanan"), "", 2, "R", false, 0, "")

	pdf.SetY(50)

	// ---- DOCUMENT TITLE ----
	pdf.SetFont("Arial", "B", 16)
	setText(navy)
	pdf.CellFormat(contentW, 9, write("SURAT PENAWARAN HARGA"), "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10.5)
	setText(muted)
	pdf.CellFormat(contentW, 6, write("Layanan Learning Management System (LMS) Berbasis Cloud"), "", 1, "L", false, 0, "")
	pdf.Ln(3)

	// ---- META + RECIPIENT ----
	metaY := pdf.GetY()
	pdf.SetFont("Arial", "", 9.5)
	setText(textColor)
	metaLine := func(label, value string) {
		pdf.SetFont("Arial", "", 9.5)
		setText(muted)
		pdf.CellFormat(20, 5.6, write(label), "", 0, "L", false, 0, "")
		pdf.CellFormat(4, 5.6, write(":"), "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "B", 9.5)
		setText(textColor)
		pdf.CellFormat(70, 5.6, write(value), "", 1, "L", false, 0, "")
	}
	metaLine("Nomor", p.OfferNumber)
	metaLine("Tanggal", formatIndonesianDate(p.GeneratedAt))
	metaLine("Perihal", "Penawaran Langganan LMS")

	// Recipient on the right column
	pdf.SetXY(pageW-marginR-72, metaY)
	pdf.SetFont("Arial", "", 9.5)
	setText(muted)
	pdf.CellFormat(72, 5.6, write("Kepada Yth,"), "", 2, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 10.5)
	setText(navy)
	pdf.MultiCell(72, 5.4, write("Bapak/Ibu Pimpinan\n"+p.SchoolName), "", "L", false)
	pdf.SetX(pageW - marginR - 72)
	pdf.SetFont("Arial", "I", 9)
	setText(muted)
	pdf.CellFormat(72, 5.2, write("di Tempat"), "", 1, "L", false, 0, "")

	if pdf.GetY() < metaY+24 {
		pdf.SetY(metaY + 24)
	}
	pdf.Ln(2)

	// Section heading helper
	section := func(title string) {
		if pdf.GetY() > 250 {
			pdf.AddPage()
		}
		pdf.Ln(2)
		y := pdf.GetY()
		setFill(sky)
		pdf.Rect(marginL, y+0.5, 3, 5, "F")
		pdf.SetX(marginL + 6)
		pdf.SetFont("Arial", "B", 11.5)
		setText(navy)
		pdf.CellFormat(contentW-6, 6, write(title), "", 1, "L", false, 0, "")
		pdf.Ln(1)
	}

	paragraph := func(text string) {
		pdf.SetFont("Arial", "", 10)
		setText(textColor)
		pdf.MultiCell(contentW, 5.4, write(text), "", "J", false)
	}

	// ---- OPENING ----
	paragraph(fmt.Sprintf("Dengan hormat, perkenalkan kami dari %s. Bersama surat ini, kami bermaksud menyampaikan penawaran kerja sama penyediaan Learning Management System (LMS) untuk mendukung digitalisasi kegiatan belajar mengajar di %s.", p.CompanyName, p.SchoolName))
	pdf.Ln(1)
	paragraph("LMS kami dirancang khusus untuk lingkungan sekolah: mudah digunakan oleh guru dan siswa, terintegrasi dalam satu platform, serta membantu sekolah meningkatkan efektivitas pembelajaran sekaligus mempercepat transformasi digital.")

	// ---- FEATURES ----
	section("Fitur Unggulan")
	features := []string{
		"Manajemen materi & modul ajar",
		"Tugas dan penilaian online",
		"CBT / ujian anti-curang",
		"Bank soal dengan bantuan AI",
		"Presensi Face Recognition",
		"Laporan otomatis ke orang tua",
		"Kalender & pengumuman sekolah",
		"Dashboard guru, siswa, admin",
	}
	pdf.SetFont("Arial", "", 9.8)
	setText(textColor)
	colW := contentW / 2
	for i := 0; i < len(features); i += 2 {
		x := pdf.GetX()
		y := pdf.GetY()
		setText(emerald)
		pdf.SetFont("Arial", "B", 9.8)
		pdf.CellFormat(5, 5.4, write("•"), "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "", 9.8)
		setText(textColor)
		pdf.CellFormat(colW-5, 5.4, write(features[i]), "", 0, "L", false, 0, "")
		if i+1 < len(features) {
			pdf.SetXY(x+colW, y)
			setText(emerald)
			pdf.SetFont("Arial", "B", 9.8)
			pdf.CellFormat(5, 5.4, write("•"), "", 0, "L", false, 0, "")
			pdf.SetFont("Arial", "", 9.8)
			setText(textColor)
			pdf.CellFormat(colW-5, 5.4, write(features[i+1]), "", 1, "L", false, 0, "")
		} else {
			pdf.Ln(5.4)
		}
	}

	// ---- PRICING SCHEME ----
	payMonths := 12 - p.DiscountMonths
	if payMonths < 1 {
		payMonths = 1
	}
	section("Skema Harga & Investasi")

	// Highlight box
	boxY := pdf.GetY()
	setFill(lightBg)
	setDraw(border)
	pdf.SetLineWidth(0.3)
	pdf.Rect(marginL, boxY, contentW, 22, "FD")
	setFill(sky)
	pdf.Rect(marginL, boxY, 2, 22, "F")
	pdf.SetXY(marginL+6, boxY+3.5)
	pdf.SetFont("Arial", "", 9.5)
	setText(muted)
	pdf.CellFormat(contentW-12, 5, write("Harga Langganan LMS"), "", 2, "L", false, 0, "")
	pdf.SetX(marginL + 6)
	pdf.SetFont("Arial", "B", 17)
	setText(navy)
	pdf.CellFormat(70, 8, write(formatRupiah(p.PricePerStudent)), "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	setText(muted)
	pdf.CellFormat(40, 8, write("/ siswa / bulan"), "", 2, "L", false, 0, "")
	pdf.SetXY(marginL+6, boxY+15)
	pdf.SetFont("Arial", "B", 9.5)
	setText(emerald)
	pdf.CellFormat(contentW-12, 5, write(fmt.Sprintf("Langganan Tahunan: GRATIS %d bulan — cukup bayar %d bulan untuk pemakaian 12 bulan.", p.DiscountMonths, payMonths)), "", 1, "L", false, 0, "")
	pdf.SetY(boxY + 22)
	pdf.Ln(3)

	paragraph("Sebagai ilustrasi, berikut simulasi investasi berdasarkan jumlah siswa di sekolah Anda:")
	pdf.Ln(1)

	// ---- PRICING TABLE ----
	if pdf.GetY() > 225 {
		pdf.AddPage()
	}
	headers := []string{"Jumlah Siswa", "Biaya / Bulan", "Per Tahun (12 bln)", "Langganan Tahunan*", "Hemat"}
	widths := []float64{26, 34, 38, 40, 36}
	aligns := []string{"C", "R", "R", "R", "R"}

	pdf.SetFont("Arial", "B", 8.5)
	setFill(navy)
	pdf.SetTextColor(255, 255, 255)
	setDraw(navy)
	pdf.SetLineWidth(0.2)
	for i, h := range headers {
		pdf.CellFormat(widths[i], 8, write(h), "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	studentTiers := []int{100, 250, 500, 1000}
	setDraw(border)
	for idx, students := range studentTiers {
		monthly := students * p.PricePerStudent
		yearlyFull := monthly * 12
		yearlyDiscount := monthly * payMonths
		saving := monthly * p.DiscountMonths

		fill := idx%2 == 1
		if fill {
			setFill(lightBg)
		}
		pdf.SetFont("Arial", "", 8.6)
		setText(textColor)
		row := []string{
			fmt.Sprintf("%d siswa", students),
			formatRupiah(monthly),
			formatRupiah(yearlyFull),
			formatRupiah(yearlyDiscount),
			formatRupiah(saving),
		}
		for i, val := range row {
			if i == 3 {
				pdf.SetFont("Arial", "B", 8.6)
				setText(navy)
			} else if i == 4 {
				pdf.SetFont("Arial", "B", 8.6)
				setText(emerald)
			} else {
				pdf.SetFont("Arial", "", 8.6)
				setText(textColor)
			}
			pdf.CellFormat(widths[i], 7.5, write(val), "1", 0, aligns[i], fill, 0, "")
		}
		pdf.Ln(-1)
	}
	pdf.SetFont("Arial", "I", 8)
	setText(muted)
	pdf.Ln(1)
	pdf.MultiCell(contentW, 4.6, write(fmt.Sprintf("*Harga langganan tahunan sudah termasuk potongan %d bulan (hemat hingga %s per tahun untuk 1.000 siswa). Harga belum termasuk PPN bila berlaku. Penawaran ini bersifat estimasi dan dapat disesuaikan dengan kebutuhan sekolah.", p.DiscountMonths, formatRupiah(1000*p.PricePerStudent*p.DiscountMonths))), "", "L", false)

	// ---- INCLUDED ----
	section("Sudah Termasuk dalam Layanan")
	included := []string{
		"Implementasi & konfigurasi sistem untuk sekolah",
		"Pelatihan penggunaan untuk admin, guru, dan siswa",
		"Hosting cloud, keamanan data, dan pencadangan berkala",
		"Pembaruan fitur (update) tanpa biaya tambahan",
		"Dukungan teknis (support) selama masa langganan",
	}
	pdf.SetFont("Arial", "", 9.8)
	for _, item := range included {
		setText(emerald)
		pdf.SetFont("Arial", "B", 9.8)
		pdf.CellFormat(5, 5.4, write("•"), "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "", 9.8)
		setText(textColor)
		pdf.MultiCell(contentW-5, 5.4, write(item), "", "L", false)
	}

	// ---- CLOSING ----
	section("Penutup")
	paragraph(fmt.Sprintf("Penawaran ini berlaku selama 30 (tiga puluh) hari sejak tanggal diterbitkan. Kami dengan senang hati mengundang %s untuk mencoba demo online maupun menjadwalkan presentasi singkat sesuai waktu yang tersedia.", p.SchoolName))
	pdf.Ln(1)
	paragraph(fmt.Sprintf("Untuk informasi lebih lanjut, Bapak/Ibu dapat menghubungi kami melalui WhatsApp %s atau mengakses demo di %s. Besar harapan kami dapat menjalin kerja sama dengan %s.", p.WhatsApp, p.DemoURL, p.SchoolName))
	pdf.Ln(4)

	// ---- SIGNATURE ----
	if pdf.GetY() > 250 {
		pdf.AddPage()
	}
	pdf.SetFont("Arial", "", 10)
	setText(textColor)
	pdf.CellFormat(contentW, 5.4, write("Hormat kami,"), "", 1, "L", false, 0, "")
	pdf.Ln(12)
	pdf.SetFont("Arial", "B", 11)
	setText(navy)
	pdf.CellFormat(contentW, 5.4, write(p.FounderName), "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 9.5)
	setText(muted)
	pdf.CellFormat(contentW, 5, write(p.FounderTitle+" — "+p.CompanyName), "", 1, "L", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
