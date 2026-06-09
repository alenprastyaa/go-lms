package controllers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

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

	school := models.School{
		Name:                           name,
		InventoryModuleEnabled:         true,
		AttendanceModuleEnabled:        true,
		AttendanceTeacherModuleEnabled: true,
		OfficialExamModuleEnabled:      true,
		KoperasiModuleEnabled:          true,
		PrivateChatModuleEnabled:       true,
		TeachingModuleAIEnabled:        true,
		PayrollModuleEnabled:           true,
		SPMBModuleEnabled:              false,
	}
	if v, ok := parseBoolFormValue(c.FormValue("personal_teacher_mode_enabled")); ok {
		school.PersonalTeacherModeEnabled = v
	}
	if v, ok := parseBoolFormValue(c.FormValue("inventory_module_enabled")); ok {
		school.InventoryModuleEnabled = v
	}
	if v, ok := parseBoolFormValue(c.FormValue("attendance_module_enabled")); ok {
		school.AttendanceModuleEnabled = v
	}
	if v, ok := parseBoolFormValue(c.FormValue("attendance_teacher_module_enabled")); ok {
		school.AttendanceTeacherModuleEnabled = v
	}
	if v, ok := parseBoolFormValue(c.FormValue("official_exam_module_enabled")); ok {
		school.OfficialExamModuleEnabled = v
	}
	if v, ok := parseBoolFormValue(c.FormValue("koperasi_module_enabled")); ok {
		school.KoperasiModuleEnabled = v
	}
	if v, ok := parseBoolFormValue(c.FormValue("private_chat_module_enabled")); ok {
		school.PrivateChatModuleEnabled = v
	}
	if v, ok := parseBoolFormValue(c.FormValue("teaching_module_ai_enabled")); ok {
		school.TeachingModuleAIEnabled = v
	}
	if v, ok := parseBoolFormValue(c.FormValue("payroll_module_enabled")); ok {
		school.PayrollModuleEnabled = v
	}
	if v, ok := parseBoolFormValue(c.FormValue("spmb_module_enabled")); ok {
		school.SPMBModuleEnabled = v
	}
	if v := strings.TrimSpace(c.FormValue("attendance_latitude")); v != "" {
		if lat, err := strconv.ParseFloat(v, 64); err == nil {
			school.AttendanceLatitude = &lat
		} else {
			return utils.Error(c, 400, "Latitude tidak valid")
		}
	}
	if v := strings.TrimSpace(c.FormValue("attendance_longitude")); v != "" {
		if lng, err := strconv.ParseFloat(v, 64); err == nil {
			school.AttendanceLongitude = &lng
		} else {
			return utils.Error(c, 400, "Longitude tidak valid")
		}
	}
	if v := strings.TrimSpace(c.FormValue("attendance_radius_meters")); v != "" {
		radius, err := strconv.Atoi(v)
		if err != nil || radius <= 0 {
			return utils.Error(c, 400, "Radius lokasi absensi tidak valid")
		}
		school.AttendanceRadiusMeters = &radius
	}
	if value, ok, err := parseAttendanceTimeFormValue(c.FormValue("attendance_late_after_time")); err != nil {
		return utils.Error(c, 400, "Batas terlambat harus format HH:MM")
	} else if ok {
		school.AttendanceLateAfterTime = &value
	}
	if value, ok, err := parseAttendanceTimeFormValue(c.FormValue("attendance_checkout_deadline")); err != nil {
		return utils.Error(c, 400, "Jam pulang minimal harus format HH:MM")
	} else if ok {
		school.AttendanceCheckoutDeadline = &value
	}
	if columns, ok, err := parseSeatMapColumnsFormValue(c.FormValue("attendance_seat_map_columns")); err != nil {
		return utils.Error(c, 400, "Jumlah kolom denah bangku harus 1 sampai 24")
	} else if ok {
		school.AttendanceSeatMapColumns = columns
	} else {
		school.AttendanceSeatMapColumns = 4
	}
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
	if v := strings.TrimSpace(c.FormValue("inventory_module_enabled")); v != "" {
		updates["inventory_module_enabled"] = strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "on")
	}
	if v := strings.TrimSpace(c.FormValue("attendance_module_enabled")); v != "" {
		updates["attendance_module_enabled"] = strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "on")
	}
	if v := strings.TrimSpace(c.FormValue("attendance_teacher_module_enabled")); v != "" {
		updates["attendance_teacher_module_enabled"] = strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "on")
	}
	if v := strings.TrimSpace(c.FormValue("attendance_latitude")); v != "" {
		lat, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return utils.Error(c, 400, "Latitude tidak valid")
		}
		updates["attendance_latitude"] = lat
	} else if strings.EqualFold(strings.TrimSpace(c.FormValue("clear_attendance_latitude")), "true") {
		updates["attendance_latitude"] = nil
	}
	if v := strings.TrimSpace(c.FormValue("attendance_longitude")); v != "" {
		lng, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return utils.Error(c, 400, "Longitude tidak valid")
		}
		updates["attendance_longitude"] = lng
	} else if strings.EqualFold(strings.TrimSpace(c.FormValue("clear_attendance_longitude")), "true") {
		updates["attendance_longitude"] = nil
	}
	if v := strings.TrimSpace(c.FormValue("attendance_radius_meters")); v != "" {
		radius, err := strconv.Atoi(v)
		if err != nil || radius <= 0 {
			return utils.Error(c, 400, "Radius lokasi absensi tidak valid")
		}
		updates["attendance_radius_meters"] = radius
	} else if strings.EqualFold(strings.TrimSpace(c.FormValue("clear_attendance_radius_meters")), "true") {
		updates["attendance_radius_meters"] = nil
	}
	if value, ok, err := parseAttendanceTimeFormValue(c.FormValue("attendance_late_after_time")); err != nil {
		return utils.Error(c, 400, "Batas terlambat harus format HH:MM")
	} else if ok {
		updates["attendance_late_after_time"] = value
	} else if strings.EqualFold(strings.TrimSpace(c.FormValue("clear_attendance_late_after_time")), "true") {
		updates["attendance_late_after_time"] = nil
	}
	if value, ok, err := parseAttendanceTimeFormValue(c.FormValue("attendance_checkout_deadline")); err != nil {
		return utils.Error(c, 400, "Jam pulang minimal harus format HH:MM")
	} else if ok {
		updates["attendance_checkout_deadline"] = value
	} else if strings.EqualFold(strings.TrimSpace(c.FormValue("clear_attendance_checkout_deadline")), "true") {
		updates["attendance_checkout_deadline"] = nil
	}
	if columns, ok, err := parseSeatMapColumnsFormValue(c.FormValue("attendance_seat_map_columns")); err != nil {
		return utils.Error(c, 400, "Jumlah kolom denah bangku harus 1 sampai 24")
	} else if ok {
		updates["attendance_seat_map_columns"] = columns
	}
	if v := strings.TrimSpace(c.FormValue("official_exam_module_enabled")); v != "" {
		updates["official_exam_module_enabled"] = strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "on")
	}
	if v := strings.TrimSpace(c.FormValue("koperasi_module_enabled")); v != "" {
		updates["koperasi_module_enabled"] = strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "on")
	}
	if v := strings.TrimSpace(c.FormValue("private_chat_module_enabled")); v != "" {
		updates["private_chat_module_enabled"] = strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "on")
	}
	if v := strings.TrimSpace(c.FormValue("teaching_module_ai_enabled")); v != "" {
		updates["teaching_module_ai_enabled"] = strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "on")
	}
	if v := strings.TrimSpace(c.FormValue("payroll_module_enabled")); v != "" {
		updates["payroll_module_enabled"] = strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "on")
	}
	if v := strings.TrimSpace(c.FormValue("spmb_module_enabled")); v != "" {
		updates["spmb_module_enabled"] = strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "on")
	}
	if v := strings.TrimSpace(c.FormValue("personal_teacher_mode_enabled")); v != "" {
		updates["personal_teacher_mode_enabled"] = strings.EqualFold(v, "true") || v == "1" || strings.EqualFold(v, "on")
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

func (a *AppContext) UpdateSchoolModules(c *fiber.Ctx) error {
	id := c.Params("id")
	var school models.School
	if err := a.DB.Where("id = ?", id).First(&school).Error; err != nil {
		return utils.Error(c, 404, "Sekolah tidak ditemukan")
	}

	updates := map[string]interface{}{}
	if v, ok := parseBoolFormValue(c.FormValue("inventory_module_enabled")); ok {
		updates["inventory_module_enabled"] = v
	}
	if v, ok := parseBoolFormValue(c.FormValue("attendance_module_enabled")); ok {
		updates["attendance_module_enabled"] = v
	}
	if v, ok := parseBoolFormValue(c.FormValue("attendance_teacher_module_enabled")); ok {
		updates["attendance_teacher_module_enabled"] = v
	}
	if v, ok := parseBoolFormValue(c.FormValue("official_exam_module_enabled")); ok {
		updates["official_exam_module_enabled"] = v
	}
	if v, ok := parseBoolFormValue(c.FormValue("koperasi_module_enabled")); ok {
		updates["koperasi_module_enabled"] = v
	}
	if v, ok := parseBoolFormValue(c.FormValue("private_chat_module_enabled")); ok {
		updates["private_chat_module_enabled"] = v
	}
	if v, ok := parseBoolFormValue(c.FormValue("teaching_module_ai_enabled")); ok {
		updates["teaching_module_ai_enabled"] = v
	}
	if v, ok := parseBoolFormValue(c.FormValue("payroll_module_enabled")); ok {
		updates["payroll_module_enabled"] = v
	}
	if v, ok := parseBoolFormValue(c.FormValue("spmb_module_enabled")); ok {
		updates["spmb_module_enabled"] = v
	}
	if v, ok := parseBoolFormValue(c.FormValue("personal_teacher_mode_enabled")); ok {
		updates["personal_teacher_mode_enabled"] = v
	}
	if len(updates) == 0 {
		return utils.Error(c, 400, "Tidak ada perubahan modul")
	}

	if err := a.DB.Model(&school).Updates(updates).Error; err != nil {
		return utils.Error(c, 500, "Gagal memperbarui modul sekolah", err.Error())
	}

	var row map[string]interface{}
	a.DB.Raw(schoolListQuery(`WHERE s.id = ?`), school.ID).Scan(&row)
	return utils.Success(c, 200, "Modul sekolah berhasil diperbarui", row)
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
	if v := strings.TrimSpace(c.FormValue("attendance_latitude")); v != "" {
		lat, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return utils.Error(c, 400, "Latitude tidak valid")
		}
		updates["attendance_latitude"] = lat
	} else if strings.EqualFold(strings.TrimSpace(c.FormValue("clear_attendance_latitude")), "true") {
		updates["attendance_latitude"] = nil
	}
	if v := strings.TrimSpace(c.FormValue("attendance_longitude")); v != "" {
		lng, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return utils.Error(c, 400, "Longitude tidak valid")
		}
		updates["attendance_longitude"] = lng
	} else if strings.EqualFold(strings.TrimSpace(c.FormValue("clear_attendance_longitude")), "true") {
		updates["attendance_longitude"] = nil
	}
	if v := strings.TrimSpace(c.FormValue("attendance_radius_meters")); v != "" {
		radius, err := strconv.Atoi(v)
		if err != nil || radius <= 0 {
			return utils.Error(c, 400, "Radius lokasi absensi tidak valid")
		}
		updates["attendance_radius_meters"] = radius
	} else if strings.EqualFold(strings.TrimSpace(c.FormValue("clear_attendance_radius_meters")), "true") {
		updates["attendance_radius_meters"] = nil
	}
	if value, ok, err := parseAttendanceTimeFormValue(c.FormValue("attendance_late_after_time")); err != nil {
		return utils.Error(c, 400, "Batas terlambat harus format HH:MM")
	} else if ok {
		updates["attendance_late_after_time"] = value
	} else if strings.EqualFold(strings.TrimSpace(c.FormValue("clear_attendance_late_after_time")), "true") {
		updates["attendance_late_after_time"] = nil
	}
	if value, ok, err := parseAttendanceTimeFormValue(c.FormValue("attendance_checkout_deadline")); err != nil {
		return utils.Error(c, 400, "Jam pulang minimal harus format HH:MM")
	} else if ok {
		updates["attendance_checkout_deadline"] = value
	} else if strings.EqualFold(strings.TrimSpace(c.FormValue("clear_attendance_checkout_deadline")), "true") {
		updates["attendance_checkout_deadline"] = nil
	}
	if columns, ok, err := parseSeatMapColumnsFormValue(c.FormValue("attendance_seat_map_columns")); err != nil {
		return utils.Error(c, 400, "Jumlah kolom denah bangku harus 1 sampai 24")
	} else if ok {
		updates["attendance_seat_map_columns"] = columns
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
	a.DB.Raw(`SELECT id, name, logo_url, attendance_latitude, attendance_longitude, attendance_radius_meters, attendance_late_after_time, attendance_checkout_deadline, COALESCE(attendance_seat_map_columns, 4) AS attendance_seat_map_columns FROM schools WHERE id = ?`, schoolID).Scan(&row)
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
			COALESCE(s.inventory_module_enabled, true) AS inventory_module_enabled,
			COALESCE(s.attendance_module_enabled, true) AS attendance_module_enabled,
			COALESCE(s.attendance_teacher_module_enabled, true) AS attendance_teacher_module_enabled,
			s.attendance_latitude,
			s.attendance_longitude,
			s.attendance_radius_meters,
			s.attendance_late_after_time,
			s.attendance_checkout_deadline,
			COALESCE(s.attendance_seat_map_columns, 4) AS attendance_seat_map_columns,
			COALESCE(s.official_exam_module_enabled, true) AS official_exam_module_enabled,
			COALESCE(s.koperasi_module_enabled, true) AS koperasi_module_enabled,
			COALESCE(s.private_chat_module_enabled, true) AS private_chat_module_enabled,
			COALESCE(s.teaching_module_ai_enabled, true) AS teaching_module_ai_enabled,
			COALESCE(s.payroll_module_enabled, true) AS payroll_module_enabled,
			COALESCE(s.spmb_module_enabled, false) AS spmb_module_enabled,
			COALESCE(s.personal_teacher_mode_enabled, false) AS personal_teacher_mode_enabled,
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
		GROUP BY s.id, s.name, s.logo_url, s.inventory_module_enabled, s.attendance_module_enabled, s.attendance_teacher_module_enabled, s.attendance_latitude, s.attendance_longitude, s.attendance_radius_meters, s.attendance_late_after_time, s.attendance_checkout_deadline, s.attendance_seat_map_columns, s.official_exam_module_enabled, s.koperasi_module_enabled, s.private_chat_module_enabled, s.teaching_module_ai_enabled, s.payroll_module_enabled, s.spmb_module_enabled, s.personal_teacher_mode_enabled
	`, whereClause)
}

func parseSeatMapColumnsFormValue(value string) (int, bool, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return 0, false, nil
	}
	columns, err := strconv.Atoi(raw)
	if err != nil || columns < 1 || columns > 24 {
		return 0, false, fmt.Errorf("invalid seat map columns")
	}
	return columns, true, nil
}

func parseAttendanceTimeFormValue(value string) (string, bool, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return "", false, nil
	}
	parsed, err := time.Parse("15:04", raw)
	if err != nil {
		return "", false, err
	}
	return parsed.Format("15:04"), true, nil
}

func parseBoolFormValue(value string) (bool, bool) {
	normalized := strings.TrimSpace(strings.ToLower(value))
	switch normalized {
	case "true", "1", "yes", "y", "on":
		return true, true
	case "false", "0", "no", "n", "off":
		return false, true
	default:
		return false, false
	}
}
