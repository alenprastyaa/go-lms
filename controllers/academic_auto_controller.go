package controllers

import (
	"fmt"
	"strings"
	"time"

	"lms/utils"

	"github.com/gofiber/fiber/v2"
)

// Penyusunan tahun ajaran secara otomatis.
//
// Catatan penting mengenai sumber data: pemerintah tidak menyediakan API kalender
// pendidikan. Kalender diterbitkan tiap Dinas Pendidikan provinsi dalam bentuk PDF
// hasil pindaian, dan tanggalnya berbeda antar provinsi. Karena itu tanggal di sini
// dihitung dari pola nasional yang berlaku setiap tahun, lalu wajib dapat disunting
// oleh admin kurikulum agar cocok dengan kalender daerah masing-masing.
//
// Pola nasional yang dipakai:
//   - Tahun ajaran dimulai hari Senin pertama pada atau setelah 13 Juli.
//     Pola ini cocok dengan hari pertama masuk sekolah 2024 (15 Juli),
//     2025 (14 Juli), dan 2026 (13 Juli).
//   - Semester Ganjil berjalan sampai 31 Desember, Genap mulai 1 Januari.
//   - Tahun ajaran berakhir 30 Juni tahun berikutnya.

type academicSemesterSuggestion struct {
	Name      string `json:"name"`
	Code      string `json:"code"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	IsCurrent bool   `json:"is_current"`
}

type academicYearSuggestion struct {
	Name      string                       `json:"name"`
	StartDate string                       `json:"start_date"`
	EndDate   string                       `json:"end_date"`
	Semesters []academicSemesterSuggestion `json:"semesters"`
	Source    string                       `json:"source"`
}

const academicDateLayout = "2006-01-02"

// hariPertamaMasuk mengembalikan Senin pertama pada atau setelah 13 Juli.
func hariPertamaMasuk(tahun int) time.Time {
	tanggal := time.Date(tahun, time.July, 13, 0, 0, 0, 0, time.UTC)
	for tanggal.Weekday() != time.Monday {
		tanggal = tanggal.AddDate(0, 0, 1)
	}
	return tanggal
}

// tahunAjaranBerjalan menentukan tahun ajaran yang sedang berlangsung pada
// tanggal acuan. Sebelum hari pertama masuk, yang berlaku masih tahun sebelumnya.
func tahunAjaranBerjalan(acuan time.Time) int {
	tahun := acuan.Year()
	if acuan.Before(hariPertamaMasuk(tahun)) {
		return tahun - 1
	}
	return tahun
}

func susunSaranTahunAjaran(acuan time.Time) academicYearSuggestion {
	tahun := tahunAjaranBerjalan(acuan)
	mulai := hariPertamaMasuk(tahun)
	selesai := time.Date(tahun+1, time.June, 30, 0, 0, 0, 0, time.UTC)

	ganjilSelesai := time.Date(tahun, time.December, 31, 0, 0, 0, 0, time.UTC)
	genapMulai := time.Date(tahun+1, time.January, 1, 0, 0, 0, 0, time.UTC)

	hariIni := time.Date(acuan.Year(), acuan.Month(), acuan.Day(), 0, 0, 0, 0, time.UTC)
	ganjilBerjalan := !hariIni.Before(mulai) && !hariIni.After(ganjilSelesai)

	return academicYearSuggestion{
		Name:      fmt.Sprintf("%d/%d", tahun, tahun+1),
		StartDate: mulai.Format(academicDateLayout),
		EndDate:   selesai.Format(academicDateLayout),
		Semesters: []academicSemesterSuggestion{
			{
				Name:      "Semester Ganjil",
				Code:      "GANJIL",
				StartDate: mulai.Format(academicDateLayout),
				EndDate:   ganjilSelesai.Format(academicDateLayout),
				IsCurrent: ganjilBerjalan,
			},
			{
				Name:      "Semester Genap",
				Code:      "GENAP",
				StartDate: genapMulai.Format(academicDateLayout),
				EndDate:   selesai.Format(academicDateLayout),
				IsCurrent: !ganjilBerjalan,
			},
		},
		Source: "Dihitung dari pola kalender pendidikan nasional. Sesuaikan dengan kalender Dinas Pendidikan provinsi bila tanggalnya berbeda.",
	}
}

// GetAcademicYearSuggestion menampilkan usulan tahun ajaran tanpa menyimpannya.
func (a *AppContext) GetAcademicYearSuggestion(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	saran := susunSaranTahunAjaran(jakartaNow())

	var jumlah int64
	a.DB.Raw(`SELECT COUNT(*) FROM academic_years WHERE school_id = ? AND name = ?`, schoolID, saran.Name).Scan(&jumlah)

	return utils.Success(c, 200, "Usulan tahun ajaran", fiber.Map{
		"suggestion":     saran,
		"already_exists": jumlah > 0,
	})
}

// AutoCreateAcademicYear membuat tahun ajaran berjalan beserta dua semesternya.
// Seluruh tanggal tetap dapat disunting setelahnya lewat menu Tahun Ajaran.
func (a *AppContext) AutoCreateAcademicYear(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	saran := susunSaranTahunAjaran(jakartaNow())

	// Tanggal boleh ditimpa pengguna sebelum disimpan.
	var body struct {
		Name      string `json:"name"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	}
	_ = c.BodyParser(&body)
	if value := strings.TrimSpace(body.Name); value != "" {
		saran.Name = value
	}
	if value := strings.TrimSpace(body.StartDate); value != "" {
		saran.StartDate = value
	}
	if value := strings.TrimSpace(body.EndDate); value != "" {
		saran.EndDate = value
	}

	var existing struct {
		ID int `gorm:"column:id"`
	}
	a.DB.Raw(`SELECT id FROM academic_years WHERE school_id = ? AND name = ?`, schoolID, saran.Name).Scan(&existing)
	if existing.ID != 0 {
		return utils.Error(c, 409, fmt.Sprintf("Tahun ajaran %s sudah ada", saran.Name))
	}

	tx := a.DB.Begin()
	if tx.Error != nil {
		return utils.Error(c, 500, "Gagal memulai transaksi")
	}

	// Tahun ajaran baru langsung diaktifkan, dan yang lama dinonaktifkan.
	if err := tx.Exec(`UPDATE academic_years SET is_active = false, updated_at = NOW() WHERE school_id = ?`, schoolID).Error; err != nil {
		tx.Rollback()
		return utils.Error(c, 500, "Gagal menonaktifkan tahun ajaran lama", err.Error())
	}

	var yearID int
	if err := tx.Raw(`
		INSERT INTO academic_years (school_id, name, start_date, end_date, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, true, NOW(), NOW()) RETURNING id
	`, schoolID, saran.Name, saran.StartDate, saran.EndDate).Scan(&yearID).Error; err != nil {
		tx.Rollback()
		return utils.Error(c, 500, "Gagal membuat tahun ajaran", err.Error())
	}

	for _, semester := range saran.Semesters {
		if err := tx.Exec(`
			INSERT INTO academic_semesters (academic_year_id, name, code, start_date, end_date, is_active, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, NOW(), NOW())
		`, yearID, semester.Name, semester.Code, semester.StartDate, semester.EndDate, semester.IsCurrent).Error; err != nil {
			tx.Rollback()
			return utils.Error(c, 500, "Gagal membuat semester", err.Error())
		}
	}

	if err := tx.Commit().Error; err != nil {
		return utils.Error(c, 500, "Gagal menyimpan tahun ajaran", err.Error())
	}

	var row map[string]interface{}
	a.DB.Raw(`SELECT * FROM academic_years WHERE id = ?`, yearID).Scan(&row)
	return utils.Success(c, 201, fmt.Sprintf("Tahun ajaran %s beserta dua semesternya berhasil dibuat", saran.Name), fiber.Map{
		"academic_year": row,
		"suggestion":    saran,
	})
}
