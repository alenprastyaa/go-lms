package controllers

import (
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"lms/utils"
)

type studentPromotionTarget struct {
	ID      uint  `gorm:"column:id"`
	ClassID *uint `gorm:"column:class_id"`
}

func (a *AppContext) PromoteStudents(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	createdBy := uintPointerFromLocal(c, "userID")
	var body struct {
		FromClassID    *uint  `json:"from_class_id"`
		ToClassID      uint   `json:"to_class_id"`
		StudentIDs     []uint `json:"student_ids"`
		EffectiveDate  string `json:"effective_date"`
		AcademicYearID *uint  `json:"academic_year_id"`
		SemesterID     *uint  `json:"semester_id"`
		Note           string `json:"note"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request")
	}
	if body.ToClassID == 0 {
		return utils.Error(c, 400, "Kelas tujuan wajib dipilih")
	}
	if body.FromClassID == nil && len(body.StudentIDs) == 0 {
		return utils.Error(c, 400, "Pilih kelas asal atau siswa yang akan dinaikkan")
	}
	if body.FromClassID != nil && *body.FromClassID == body.ToClassID {
		return utils.Error(c, 400, "Kelas asal dan kelas tujuan tidak boleh sama")
	}

	effectiveDate, err := parsePromotionDate(body.EffectiveDate)
	if err != nil {
		return utils.Error(c, 400, "Tanggal efektif tidak valid")
	}

	var toClass struct {
		ID        uint   `gorm:"column:id"`
		ClassName string `gorm:"column:class_name"`
	}
	a.DB.Raw(`SELECT id, class_name FROM class WHERE id = ? AND school_id = ?`, body.ToClassID, schoolID).Scan(&toClass)
	if toClass.ID == 0 {
		return utils.Error(c, 404, "Kelas tujuan tidak ditemukan")
	}

	if body.FromClassID != nil {
		var fromClassID uint
		a.DB.Raw(`SELECT id FROM class WHERE id = ? AND school_id = ?`, *body.FromClassID, schoolID).Scan(&fromClassID)
		if fromClassID == 0 {
			return utils.Error(c, 404, "Kelas asal tidak ditemukan")
		}
	}

	studentIDs := uniqueUintValues(body.StudentIDs)
	var targets []studentPromotionTarget
	query := a.DB.Table("users").
		Select("id, class_id").
		Where("school_id = ? AND role = 'SISWA'", schoolID)
	if body.FromClassID != nil {
		query = query.Where("class_id = ?", *body.FromClassID)
	}
	if len(studentIDs) > 0 {
		query = query.Where("id IN ?", studentIDs)
	}
	if err := query.Order("username ASC").Scan(&targets).Error; err != nil {
		return utils.Error(c, 500, "Gagal membaca data siswa", err.Error())
	}

	promotedIDs := make([]uint, 0, len(targets))
	skippedAlreadyInTarget := 0
	for _, target := range targets {
		if target.ClassID != nil && *target.ClassID == body.ToClassID {
			skippedAlreadyInTarget++
			continue
		}
		promotedIDs = append(promotedIDs, target.ID)
	}
	if len(promotedIDs) == 0 {
		if skippedAlreadyInTarget > 0 {
			return utils.Error(c, 400, "Semua siswa terpilih sudah berada di kelas tujuan")
		}
		return utils.Error(c, 404, "Tidak ada siswa yang sesuai untuk dinaikkan")
	}

	academicYearID, semesterID := body.AcademicYearID, body.SemesterID
	if academicYearID == nil || semesterID == nil {
		activeAcademicYearID, activeSemesterID := a.resolveActiveAcademicPeriod(int(schoolID))
		if academicYearID == nil && activeAcademicYearID > 0 {
			v := uint(activeAcademicYearID)
			academicYearID = &v
		}
		if semesterID == nil && activeSemesterID > 0 {
			v := uint(activeSemesterID)
			semesterID = &v
		}
	}

	note := strings.TrimSpace(body.Note)
	if note == "" {
		note = "Naik kelas ke " + toClass.ClassName
	}

	err = a.DB.Transaction(func(tx *gorm.DB) error {
		for _, studentID := range promotedIDs {
			if err := recordStudentClassPlacementTx(tx, schoolID, studentID, body.ToClassID, academicYearID, semesterID, effectiveDate, note, createdBy); err != nil {
				return err
			}
		}
		if err := tx.Exec(`
			UPDATE users
			SET class_id = ?
			WHERE school_id = ?
			  AND role = 'SISWA'
			  AND id IN ?
		`, body.ToClassID, schoolID, promotedIDs).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return utils.Error(c, 500, "Gagal memproses kenaikan kelas", err.Error())
	}

	return utils.Success(c, 200, "Kenaikan kelas berhasil diproses", fiber.Map{
		"promoted_count":            len(promotedIDs),
		"skipped_already_in_target": skippedAlreadyInTarget,
		"to_class_id":               body.ToClassID,
		"to_class_name":             toClass.ClassName,
		"effective_date":            effectiveDate.Format("2006-01-02"),
		"academic_year_id":          academicYearID,
		"semester_id":               semesterID,
		"promoted_student_ids":      promotedIDs,
	})
}

func (a *AppContext) GetStudentClassHistory(c *fiber.Ctx) error {
	studentID := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)

	var student struct {
		ID uint `gorm:"column:id"`
	}
	a.DB.Raw(`SELECT id FROM users WHERE id = ? AND school_id = ? AND role = 'SISWA'`, studentID, schoolID).Scan(&student)
	if student.ID == 0 {
		return utils.Error(c, 404, "Student not found")
	}

	var rows []map[string]interface{}
	a.DB.Raw(`
		SELECT
			sce.id,
			sce.student_id,
			sce.class_id,
			c.class_name,
			sce.academic_year_id,
			ay.name AS academic_year_name,
			sce.semester_id,
			sem.name AS semester_name,
			sem.code AS semester_code,
			sce.start_date,
			sce.end_date,
			sce.is_active,
			sce.promotion_note,
			sce.created_at,
			creator.username AS created_by_name
		FROM student_class_enrollments sce
		LEFT JOIN class c ON c.id = sce.class_id
		LEFT JOIN academic_years ay ON ay.id = sce.academic_year_id
		LEFT JOIN academic_semesters sem ON sem.id = sce.semester_id
		LEFT JOIN users creator ON creator.id = sce.created_by
		WHERE sce.school_id = ?
		  AND sce.student_id = ?
		ORDER BY sce.start_date DESC, sce.id DESC
	`, schoolID, student.ID).Scan(&rows)

	return utils.Success(c, 200, "Success Get Student Class History", rows)
}

func recordStudentClassPlacementTx(tx *gorm.DB, schoolID, studentID, classID uint, academicYearID, semesterID *uint, startDate time.Time, note string, createdBy *uint) error {
	if classID == 0 {
		return nil
	}
	dateText := startDate.Format("2006-01-02")
	if err := tx.Exec(`
		UPDATE student_class_enrollments
		SET is_active = false,
		    end_date = CASE
		        WHEN start_date >= ?::date THEN ?::date
		        ELSE (?::date - INTERVAL '1 day')::date
		    END,
		    updated_at = NOW()
		WHERE school_id = ?
		  AND student_id = ?
		  AND is_active = true
	`, dateText, dateText, dateText, schoolID, studentID).Error; err != nil {
		return err
	}

	return tx.Exec(`
		INSERT INTO student_class_enrollments (
			school_id, student_id, class_id, academic_year_id, semester_id,
			start_date, end_date, is_active, promotion_note, created_by, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?::date, NULL, true, ?, ?, NOW(), NOW())
	`, schoolID, studentID, classID, academicYearID, semesterID, dateText, nullIfEmpty(note), createdBy).Error
}

func ensureInitialStudentClassEnrollmentTx(tx *gorm.DB, schoolID, studentID, classID uint, createdBy *uint) error {
	if classID == 0 {
		return nil
	}
	return tx.Exec(`
		INSERT INTO student_class_enrollments (
			school_id, student_id, class_id, start_date, is_active, promotion_note, created_by, created_at, updated_at
		)
		SELECT ?, ?, ?, CURRENT_DATE, true, 'Penempatan kelas awal', ?, NOW(), NOW()
		WHERE NOT EXISTS (
			SELECT 1
			FROM student_class_enrollments
			WHERE student_id = ?
			  AND is_active = true
		)
	`, schoolID, studentID, classID, createdBy, studentID).Error
}

func parsePromotionDate(raw string) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Now(), nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed, nil
}

func uniqueUintValues(values []uint) []uint {
	seen := make(map[uint]struct{}, len(values))
	result := make([]uint, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func uintPointerFromLocal(c *fiber.Ctx, key string) *uint {
	value := c.Locals(key)
	switch typed := value.(type) {
	case uint:
		return &typed
	case int:
		if typed > 0 {
			v := uint(typed)
			return &v
		}
	case float64:
		if typed > 0 {
			v := uint(typed)
			return &v
		}
	case string:
		intValue := utils.ToInt(typed, 0)
		if intValue > 0 {
			v := uint(intValue)
			return &v
		}
	}
	return nil
}
