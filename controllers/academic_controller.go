package controllers

import (
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
	"lms/utils"
)

func (a *AppContext) GetAcademicPeriods(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	var years []map[string]interface{}
	a.DB.Raw(`
		SELECT ay.*, COALESCE(
			(SELECT json_agg(sem ORDER BY sem.start_date ASC, sem.id ASC) FROM academic_semesters sem WHERE sem.academic_year_id = ay.id),
			'[]'::json
		) AS semesters
		FROM academic_years ay
		WHERE ay.school_id = ?
		ORDER BY ay.start_date DESC, ay.id DESC
	`, schoolID).Scan(&years)
	for _, year := range years {
		rawSemesters, exists := year["semesters"]
		if !exists || rawSemesters == nil {
			year["semesters"] = []map[string]interface{}{}
			continue
		}

		switch value := rawSemesters.(type) {
		case []byte:
			var parsed []map[string]interface{}
			if err := json.Unmarshal(value, &parsed); err == nil {
				year["semesters"] = parsed
				continue
			}
			year["semesters"] = []map[string]interface{}{}
		case string:
			var parsed []map[string]interface{}
			if err := json.Unmarshal([]byte(value), &parsed); err == nil {
				year["semesters"] = parsed
				continue
			}
			year["semesters"] = []map[string]interface{}{}
		}
	}

	var active map[string]interface{}
	a.DB.Raw(`
		SELECT ay.id AS academic_year_id, ay.name AS academic_year_name, ay.start_date AS academic_year_start_date, ay.end_date AS academic_year_end_date,
		       sem.id AS semester_id, sem.name AS semester_name, sem.code AS semester_code, sem.start_date AS semester_start_date, sem.end_date AS semester_end_date
		FROM academic_years ay
		LEFT JOIN academic_semesters sem ON sem.academic_year_id = ay.id AND sem.is_active = true
		WHERE ay.school_id = ? AND ay.is_active = true
		LIMIT 1
	`, schoolID).Scan(&active)

	return utils.Success(c, 200, "Success Get Academic Periods", fiber.Map{
		"years":  years,
		"active": active,
	})
}

func (a *AppContext) CreateAcademicYear(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	var body struct {
		Name      string `json:"name"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	}
	_ = c.BodyParser(&body)

	var row map[string]interface{}
	a.DB.Raw(`
		INSERT INTO academic_years (school_id, name, start_date, end_date)
		VALUES (?, ?, ?, ?) RETURNING *
	`, schoolID, body.Name, body.StartDate, body.EndDate).Scan(&row)
	return utils.Success(c, 201, "Success Create Academic Year", row)
}

func (a *AppContext) UpdateAcademicYear(c *fiber.Ctx) error {
	id := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)
	var body struct {
		Name      string `json:"name"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	}
	_ = c.BodyParser(&body)

	var row map[string]interface{}
	a.DB.Raw(`
		UPDATE academic_years SET name = ?, start_date = ?, end_date = ?, updated_at = NOW()
		WHERE id = ? AND school_id = ? RETURNING *
	`, body.Name, body.StartDate, body.EndDate, id, schoolID).Scan(&row)
	return utils.Success(c, 200, "Success Update Academic Year", row)
}

func (a *AppContext) ActivateAcademicYear(c *fiber.Ctx) error {
	id := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)
	a.DB.Exec(`UPDATE academic_years SET is_active = CASE WHEN id::text = ? THEN true ELSE false END, updated_at = NOW() WHERE school_id = ?`, id, schoolID)
	var row map[string]interface{}
	a.DB.Raw(`SELECT * FROM academic_years WHERE id = ?`, id).Scan(&row)
	return utils.Success(c, 200, "Success Activate Academic Year", row)
}

func (a *AppContext) CreateSemester(c *fiber.Ctx) error {
	var body struct {
		AcademicYearID int    `json:"academic_year_id"`
		Name           string `json:"name"`
		Code           string `json:"code"`
		StartDate      string `json:"start_date"`
		EndDate        string `json:"end_date"`
	}
	_ = c.BodyParser(&body)
	var row map[string]interface{}
	a.DB.Raw(`
		INSERT INTO academic_semesters (academic_year_id, name, code, start_date, end_date, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW()) RETURNING *
	`, body.AcademicYearID, body.Name, body.Code, body.StartDate, body.EndDate).Scan(&row)
	return utils.Success(c, 201, "Success Create Semester", row)
}

func (a *AppContext) UpdateSemester(c *fiber.Ctx) error {
	id := c.Params("id")
	var body struct {
		Name      string `json:"name"`
		Code      string `json:"code"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	}
	_ = c.BodyParser(&body)
	var row map[string]interface{}
	a.DB.Raw(`
		UPDATE academic_semesters SET name = ?, code = ?, start_date = ?, end_date = ?, updated_at = NOW()
		WHERE id = ? RETURNING *
	`, body.Name, body.Code, body.StartDate, body.EndDate, id).Scan(&row)
	return utils.Success(c, 200, "Success Update Semester", row)
}

func (a *AppContext) ActivateSemester(c *fiber.Ctx) error {
	id := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)

	var sem struct {
		ID             int `json:"id"`
		AcademicYearID int `json:"academic_year_id"`
	}
	a.DB.Raw(`SELECT sem.id, sem.academic_year_id FROM academic_semesters sem INNER JOIN academic_years ay ON ay.id = sem.academic_year_id WHERE sem.id = ? AND ay.school_id = ?`, id, schoolID).Scan(&sem)
	if sem.ID == 0 {
		return utils.Error(c, 404, "Semester not found")
	}
	a.DB.Exec(`
		UPDATE academic_semesters
		SET is_active = CASE WHEN id::text = ? THEN true ELSE false END, updated_at = NOW()
		WHERE academic_year_id IN (SELECT id FROM academic_years WHERE school_id = ?)
	`, id, schoolID)
	a.DB.Exec(`UPDATE academic_years SET is_active = CASE WHEN id = ? THEN true ELSE false END, updated_at = NOW() WHERE school_id = ?`, sem.AcademicYearID, schoolID)
	var row map[string]interface{}
	a.DB.Raw(`SELECT * FROM academic_semesters WHERE id = ?`, id).Scan(&row)
	return utils.Success(c, 200, "Success Activate Semester", row)
}

func parseDate(value string) *time.Time {
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil
	}
	return &t
}
