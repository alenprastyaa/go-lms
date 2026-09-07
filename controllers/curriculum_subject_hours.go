package controllers

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// JP per minggu tiap tingkat kelas.
//
// curriculum_subjects.weekly_hours tetap dipertahankan sebagai nilai bawaan untuk
// tingkat yang belum diisi, sehingga data lama tidak perlu diubah dan layar yang
// sudah memakainya tetap berjalan.

type subjectLevelHour struct {
	ClassLevelID   uint   `gorm:"column:class_level_id" json:"class_level_id"`
	ClassLevelName string `gorm:"column:class_level_name" json:"class_level_name"`
	SortOrder      int    `gorm:"column:sort_order" json:"sort_order"`
	WeeklyHours    int    `gorm:"column:weekly_hours" json:"weekly_hours"`
}

type subjectLevelHourInput struct {
	ClassLevelID uint `json:"class_level_id"`
	WeeklyHours  int  `json:"weekly_hours"`
}

// levelHoursForSubjects mengambil JP tiap tingkat untuk sekumpulan mapel sekaligus,
// supaya tidak ada kueri berulang per baris.
func levelHoursForSubjects(db *gorm.DB, schoolID uint, subjectIDs []uint) map[uint][]subjectLevelHour {
	hasil := map[uint][]subjectLevelHour{}
	if len(subjectIDs) == 0 {
		return hasil
	}

	var rows []struct {
		SubjectID      uint   `gorm:"column:curriculum_subject_id"`
		ClassLevelID   uint   `gorm:"column:class_level_id"`
		ClassLevelName string `gorm:"column:class_level_name"`
		SortOrder      int    `gorm:"column:sort_order"`
		WeeklyHours    int    `gorm:"column:weekly_hours"`
	}
	db.Raw(`
		SELECT h.curriculum_subject_id, h.class_level_id, cl.name AS class_level_name,
		       cl.sort_order, h.weekly_hours
		FROM curriculum_subject_level_hours h
		JOIN class_levels cl ON cl.id = h.class_level_id
		WHERE h.school_id = ? AND h.curriculum_subject_id IN ?
		ORDER BY cl.sort_order, cl.name
	`, schoolID, subjectIDs).Scan(&rows)

	for _, row := range rows {
		hasil[row.SubjectID] = append(hasil[row.SubjectID], subjectLevelHour{
			ClassLevelID:   row.ClassLevelID,
			ClassLevelName: row.ClassLevelName,
			SortOrder:      row.SortOrder,
			WeeklyHours:    row.WeeklyHours,
		})
	}
	return hasil
}

// simpanLevelHours menyimpan ulang JP per tingkat untuk satu mapel.
// Tingkat yang diberi nilai 0 atau tidak dikirim berarti memakai nilai bawaan mapel.
func simpanLevelHours(db *gorm.DB, schoolID, subjectID uint, input []subjectLevelHourInput) error {
	if err := db.Exec(`DELETE FROM curriculum_subject_level_hours WHERE school_id = ? AND curriculum_subject_id = ?`,
		schoolID, subjectID).Error; err != nil {
		return err
	}

	for _, item := range input {
		if item.ClassLevelID == 0 || item.WeeklyHours <= 0 {
			continue
		}
		// Tingkat harus benar-benar milik sekolah ini.
		var jumlah int64
		db.Raw(`SELECT COUNT(*) FROM class_levels WHERE id = ? AND school_id = ?`, item.ClassLevelID, schoolID).Scan(&jumlah)
		if jumlah == 0 {
			return fmt.Errorf("tingkat kelas tidak dikenali")
		}
		if err := db.Exec(`
			INSERT INTO curriculum_subject_level_hours (school_id, curriculum_subject_id, class_level_id, weekly_hours, created_at, updated_at)
			VALUES (?, ?, ?, ?, NOW(), NOW())
			ON CONFLICT (curriculum_subject_id, class_level_id)
			DO UPDATE SET weekly_hours = EXCLUDED.weekly_hours, updated_at = NOW()
		`, schoolID, subjectID, item.ClassLevelID, item.WeeklyHours).Error; err != nil {
			return err
		}
	}
	return nil
}

// jpMapelUntukKelas mengembalikan JP mapel untuk satu rombel, mengikuti tingkat
// kelasnya. Bila tingkat itu belum diatur, dipakai nilai bawaan mapel.
func jpMapelUntukKelas(db *gorm.DB, schoolID, subjectID, classID uint) int {
	var hasil struct {
		LevelHours   *int `gorm:"column:level_hours"`
		DefaultHours int  `gorm:"column:default_hours"`
	}
	db.Raw(`
		SELECT h.weekly_hours AS level_hours, cs.weekly_hours AS default_hours
		FROM curriculum_subjects cs
		LEFT JOIN class c ON c.id = ? AND c.school_id = ?
		LEFT JOIN curriculum_subject_level_hours h
		       ON h.curriculum_subject_id = cs.id AND h.class_level_id = c.class_level_id
		WHERE cs.id = ? AND cs.school_id = ?
	`, classID, schoolID, subjectID, schoolID).Scan(&hasil)

	if hasil.LevelHours != nil && *hasil.LevelHours > 0 {
		return *hasil.LevelHours
	}
	return hasil.DefaultHours
}

// ringkasLevelHours merangkum JP per tingkat menjadi teks pendek untuk tampilan,
// contoh: "10: 4 JP · 11: 5 JP".
func ringkasLevelHours(items []subjectLevelHour) string {
	if len(items) == 0 {
		return ""
	}
	bagian := make([]string, 0, len(items))
	for _, item := range items {
		bagian = append(bagian, fmt.Sprintf("%s: %d JP", item.ClassLevelName, item.WeeklyHours))
	}
	return strings.Join(bagian, " · ")
}
