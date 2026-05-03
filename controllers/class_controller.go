package controllers

import (
	"github.com/gofiber/fiber/v2"
	"lms/utils"
)

func (a *AppContext) CreateClass(c *fiber.Ctx) error {
	var body struct {
		ClassName  string `json:"class_name"`
		WaliGuruID *uint  `json:"wali_guru_id"`
	}
	_ = c.BodyParser(&body)
	schoolID := c.Locals("schoolID").(uint)
	var row map[string]interface{}
	a.DB.Raw(`
		INSERT INTO class (class_name, school_id, wali_guru_id)
		VALUES (?, ?, ?) RETURNING id, class_name, school_id, wali_guru_id
	`, body.ClassName, schoolID, body.WaliGuruID).Scan(&row)
	return utils.Success(c, 201, "class registered successfully", row)
}

func (a *AppContext) GetClasses(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	var rows []map[string]interface{}
	a.DB.Raw(`
		SELECT c.id, c.class_name, c.school_id, c.wali_guru_id,
		       u.username AS wali_guru_name, u.parent_email AS wali_guru_email, u.phone_number AS wali_guru_phone_number
		FROM class c
		LEFT JOIN users u ON c.wali_guru_id = u.id
		WHERE c.school_id = ?
		ORDER BY c.class_name ASC
	`, schoolID).Scan(&rows)
	return utils.Success(c, 201, "Succes Get Data class", rows)
}

func (a *AppContext) UpdateClass(c *fiber.Ctx) error {
	id := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)
	var body struct {
		ClassName  *string `json:"class_name"`
		WaliGuruID *uint   `json:"wali_guru_id"`
	}
	_ = c.BodyParser(&body)
	var current struct {
		ClassName  string `json:"class_name"`
		WaliGuruID *uint  `json:"wali_guru_id"`
	}
	if err := a.DB.Raw(`SELECT class_name, wali_guru_id FROM class WHERE id = ? AND school_id = ?`, id, schoolID).Scan(&current).Error; err != nil {
		return utils.Error(c, 404, "Class not found")
	}
	className := current.ClassName
	if body.ClassName != nil {
		className = *body.ClassName
	}
	waliGuruID := current.WaliGuruID
	if body.WaliGuruID != nil {
		waliGuruID = body.WaliGuruID
	}
	var row map[string]interface{}
	a.DB.Raw(`
		UPDATE class SET class_name = ?, school_id = ?, wali_guru_id = ?
		WHERE id = ? RETURNING id, class_name, school_id, wali_guru_id
	`, className, schoolID, waliGuruID, id).Scan(&row)
	return utils.Success(c, 200, "Success Update Class", row)
}
func (a *AppContext) DeleteClass(c *fiber.Ctx) error {
	id := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)
	var row struct {
		ID        int    `json:"id"`
		ClassName string `json:"class_name"`
	}
	a.DB.Raw(`DELETE FROM class WHERE id = ? AND school_id = ? RETURNING id, class_name`, id, schoolID).Scan(&row)
	if row.ID == 0 {
		return utils.Error(c, 404, "Class not found")
	}
	return utils.Success(c, 200, `Kelas "`+row.ClassName+`" berhasil dihapus`, nil)
}

func (a *AppContext) GetMyClass(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	userID := c.Locals("userID").(uint)
	var row map[string]interface{}
	a.DB.Raw(`
		SELECT c.id, c.class_name, c.school_id, c.wali_guru_id,
		       u.username AS wali_guru_name, u.parent_email AS wali_guru_email, u.phone_number AS wali_guru_phone_number
		FROM class c
		LEFT JOIN users u ON c.wali_guru_id = u.id
		WHERE c.wali_guru_id = ? AND c.school_id = ?
	`, userID, schoolID).Scan(&row)
	if len(row) == 0 {
		return utils.Error(c, 404, "Homeroom class not found")
	}
	return utils.Success(c, 200, "Success Get Homeroom Class", row)
}
