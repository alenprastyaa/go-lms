package controllers

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"lms/models"
	"lms/utils"
)

func (a *AppContext) CreateSchool(c *fiber.Ctx) error {
	name := strings.TrimSpace(c.FormValue("name"))
	if name == "" {
		var body struct {
			Name string `json:"name"`
		}
		if err := c.BodyParser(&body); err != nil {
			return utils.Error(c, 400, "Invalid request body")
		}
		name = strings.TrimSpace(body.Name)
	}
	if name == "" {
		return utils.Error(c, 400, "Nama sekolah wajib diisi")
	}

	school := models.School{Name: name}
	if file, err := c.FormFile("logo"); err == nil && file != nil {
		logoURL, upErr := utils.SaveUploadedFile(c, file)
		if upErr != nil {
			return utils.Error(c, 500, "Gagal upload avatar sekolah", upErr.Error())
		}
		school.LogoURL = &logoURL
	}

	if err := a.DB.Create(&school).Error; err != nil {
		return utils.Error(c, 500, "Gagal membuat sekolah", err.Error())
	}

	var row map[string]interface{}
	a.DB.Raw(schoolListQuery(`WHERE s.id = ?`), school.ID).Scan(&row)
	return utils.Success(c, 201, "Sekolah berhasil dibuat", row)
}

func (a *AppContext) GetSchools(c *fiber.Ctx) error {
	var rows []map[string]interface{}
	a.DB.Raw(schoolListQuery(``) + ` ORDER BY total_students DESC, s.name ASC`).Scan(&rows)
	return utils.Success(c, 200, "Success Get Schools", fiber.Map{
		"items": recentOrEmpty(rows),
	})
}

func (a *AppContext) UpdateSchool(c *fiber.Ctx) error {
	id := c.Params("id")
	var school models.School
	if err := a.DB.Where("id = ?", id).First(&school).Error; err != nil {
		return utils.Error(c, 404, "Sekolah tidak ditemukan")
	}

	name := strings.TrimSpace(c.FormValue("name"))
	if name == "" {
		var body struct {
			Name string `json:"name"`
		}
		if err := c.BodyParser(&body); err != nil {
			return utils.Error(c, 400, "Invalid request body")
		}
		name = strings.TrimSpace(body.Name)
	}
	if name == "" {
		return utils.Error(c, 400, "Nama sekolah wajib diisi")
	}

	updates := map[string]interface{}{
		"name": name,
	}
	if strings.EqualFold(strings.TrimSpace(c.FormValue("remove_logo")), "true") {
		updates["logo_url"] = nil
	}
	if file, err := c.FormFile("logo"); err == nil && file != nil {
		logoURL, upErr := utils.SaveUploadedFile(c, file)
		if upErr != nil {
			return utils.Error(c, 500, "Gagal upload avatar sekolah", upErr.Error())
		}
		updates["logo_url"] = logoURL
	}

	if err := a.DB.Model(&school).Updates(updates).Error; err != nil {
		return utils.Error(c, 500, "Gagal memperbarui sekolah", err.Error())
	}

	var row map[string]interface{}
	a.DB.Raw(schoolListQuery(`WHERE s.id = ?`), school.ID).Scan(&row)
	return utils.Success(c, 200, "Sekolah berhasil diperbarui", row)
}

func (a *AppContext) UpdateCurrentSchoolBranding(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)

	var school models.School
	if err := a.DB.Where("id = ?", schoolID).First(&school).Error; err != nil {
		return utils.Error(c, 404, "Sekolah tidak ditemukan")
	}

	updates := map[string]interface{}{}
	if strings.EqualFold(strings.TrimSpace(c.FormValue("remove_logo")), "true") {
		updates["logo_url"] = nil
	}

	if file, err := c.FormFile("logo"); err == nil && file != nil {
		logoURL, upErr := utils.SaveUploadedFile(c, file)
		if upErr != nil {
			return utils.Error(c, 500, "Gagal upload logo sekolah", upErr.Error())
		}
		updates["logo_url"] = logoURL
	}

	if len(updates) == 0 {
		return utils.Error(c, 400, "Tidak ada perubahan logo sekolah")
	}

	if err := a.DB.Model(&school).Updates(updates).Error; err != nil {
		return utils.Error(c, 500, "Gagal memperbarui logo sekolah", err.Error())
	}

	var row map[string]interface{}
	a.DB.Raw(`SELECT id, name, logo_url FROM schools WHERE id = ?`, schoolID).Scan(&row)
	return utils.Success(c, 200, "Logo sekolah berhasil diperbarui", row)
}

func (a *AppContext) UpdateCurrentSchool(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)

	var school models.School
	if err := a.DB.Where("id = ?", schoolID).First(&school).Error; err != nil {
		return utils.Error(c, 404, "Sekolah tidak ditemukan")
	}

	name := strings.TrimSpace(c.FormValue("name"))
	if name == "" {
		var body struct {
			Name string `json:"name"`
		}
		if err := c.BodyParser(&body); err != nil {
			return utils.Error(c, 400, "Invalid request body")
		}
		name = strings.TrimSpace(body.Name)
	}
	if name == "" {
		return utils.Error(c, 400, "Nama sekolah wajib diisi")
	}

	updates := map[string]interface{}{
		"name": name,
	}
	if strings.EqualFold(strings.TrimSpace(c.FormValue("remove_logo")), "true") {
		updates["logo_url"] = nil
	}
	if file, err := c.FormFile("logo"); err == nil && file != nil {
		logoURL, upErr := utils.SaveUploadedFile(c, file)
		if upErr != nil {
			return utils.Error(c, 500, "Gagal upload logo sekolah", upErr.Error())
		}
		updates["logo_url"] = logoURL
	}

	if err := a.DB.Model(&school).Updates(updates).Error; err != nil {
		return utils.Error(c, 500, "Gagal memperbarui sekolah", err.Error())
	}

	var row map[string]interface{}
	a.DB.Raw(`SELECT id, name, logo_url FROM schools WHERE id = ?`, schoolID).Scan(&row)
	return utils.Success(c, 200, "Data sekolah berhasil diperbarui", row)
}

func (a *AppContext) DeleteSchool(c *fiber.Ctx) error {
	id := c.Params("id")

	var school models.School
	if err := a.DB.Where("id = ?", id).First(&school).Error; err != nil {
		return utils.Error(c, 404, "Sekolah tidak ditemukan")
	}

	err := a.DB.Transaction(func(tx *gorm.DB) error {
		tx.Exec(`DELETE FROM learning_chat_reads WHERE subject_id IN (SELECT id FROM learning_subjects WHERE school_id = ?)`, school.ID)
		tx.Exec(`DELETE FROM learning_chat_messages WHERE subject_id IN (SELECT id FROM learning_subjects WHERE school_id = ?)`, school.ID)
		tx.Exec(`DELETE FROM learning_question_bank WHERE subject_id IN (SELECT id FROM learning_subjects WHERE school_id = ?)`, school.ID)
		tx.Exec(`DELETE FROM learning_submissions WHERE assignment_id IN (SELECT id FROM learning_assignments WHERE subject_id IN (SELECT id FROM learning_subjects WHERE school_id = ?))`, school.ID)
		tx.Exec(`DELETE FROM learning_assignments WHERE subject_id IN (SELECT id FROM learning_subjects WHERE school_id = ?)`, school.ID)
		tx.Exec(`DELETE FROM learning_materials WHERE subject_id IN (SELECT id FROM learning_subjects WHERE school_id = ?)`, school.ID)
		tx.Exec(`DELETE FROM curriculum_schedule_entries WHERE school_id = ?`, school.ID)
		tx.Exec(`DELETE FROM curriculum_schedule_slots WHERE school_id = ?`, school.ID)
		tx.Exec(`DELETE FROM curriculum_class_distributions WHERE school_id = ?`, school.ID)
		tx.Exec(`DELETE FROM curriculum_teacher_loads WHERE school_id = ?`, school.ID)
		tx.Exec(`DELETE FROM curriculum_subjects WHERE school_id = ?`, school.ID)
		tx.Exec(`DELETE FROM learning_subjects WHERE school_id = ?`, school.ID)
		tx.Exec(`DELETE FROM attendance WHERE user_id IN (SELECT id FROM users WHERE school_id = ?)`, school.ID)
		tx.Exec(`DELETE FROM payment_receipt WHERE user_id IN (SELECT id FROM users WHERE school_id = ?)`, school.ID)
		tx.Exec(`DELETE FROM academic_semesters WHERE academic_year_id IN (SELECT id FROM academic_years WHERE school_id = ?)`, school.ID)
		tx.Exec(`DELETE FROM academic_years WHERE school_id = ?`, school.ID)
		tx.Exec(`UPDATE class SET wali_guru_id = NULL WHERE school_id = ?`, school.ID)
		tx.Exec(`UPDATE users SET class_id = NULL WHERE school_id = ?`, school.ID)
		tx.Exec(`DELETE FROM class WHERE school_id = ?`, school.ID)
		tx.Exec(`DELETE FROM users WHERE school_id = ?`, school.ID)
		if err := tx.Delete(&school).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return utils.Error(c, 500, "Gagal menghapus sekolah", err.Error())
	}

	return utils.Success(c, 200, "Sekolah berhasil dihapus", fiber.Map{
		"id":   school.ID,
		"name": school.Name,
	})
}

func schoolListQuery(whereClause string) string {
	return fmt.Sprintf(`
		SELECT
			s.id,
			s.name,
			s.logo_url,
			COUNT(DISTINCT CASE WHEN u.role = 'ADMIN' THEN u.id END)::int AS total_admins,
			COUNT(DISTINCT CASE WHEN u.role = 'GURU' THEN u.id END)::int AS total_teachers,
			COUNT(DISTINCT CASE WHEN u.role = 'SISWA' THEN u.id END)::int AS total_students,
			COUNT(DISTINCT c.id)::int AS total_classes,
			COUNT(DISTINCT cs.id)::int AS total_curriculum_subjects,
			COUNT(DISTINCT ls.id)::int AS total_learning_subjects,
			COUNT(DISTINCT CASE WHEN ay.is_active = true THEN ay.id END)::int AS active_academic_years
		FROM schools s
		LEFT JOIN users u ON u.school_id = s.id
		LEFT JOIN class c ON c.school_id = s.id
		LEFT JOIN curriculum_subjects cs ON cs.school_id = s.id
		LEFT JOIN learning_subjects ls ON ls.school_id = s.id
		LEFT JOIN academic_years ay ON ay.school_id = s.id
		%s
		GROUP BY s.id, s.name, s.logo_url
	`, whereClause)
}
