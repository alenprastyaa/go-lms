package controllers

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"lms/utils"
)

func (a *AppContext) GetStudents(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	classID := c.Query("class_id")
	page := utils.ToInt(c.Query("page", "1"), 1)
	limit := utils.ToInt(c.Query("limit", "10"), 10)
	offset := (page - 1) * limit

	var rows []map[string]interface{}
	q := a.DB.Table("users u").
		Select("u.id, u.username, u.class_id, u.parent_email, u.phone_number, cn.class_name").
		Joins("LEFT JOIN class cn ON u.class_id = cn.id").
		Where("u.role = 'SISWA' AND u.school_id = ?", schoolID)
	if classID != "" {
		q = q.Where("u.class_id = ?", classID)
	}
	q.Limit(limit).Offset(offset).Scan(&rows)

	return utils.Success(c, 200, "Success Get Data Student", fiber.Map{"page": page, "limit": limit, "data": rows})
}

func (a *AppContext) EditStudent(c *fiber.Ctx) error {
	id := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)
	var body struct {
		Username    *string `json:"username"`
		Role        *string `json:"role"`
		ClassID     *uint   `json:"class_id"`
		ParentEmail *string `json:"parent_email"`
		PhoneNumber *string `json:"phone_number"`
	}
	_ = c.BodyParser(&body)
	var current struct {
		ID          uint    `json:"id"`
		Username    string  `json:"username"`
		Role        string  `json:"role"`
		ClassID     *uint   `json:"class_id"`
		ParentEmail *string `json:"parent_email"`
		PhoneNumber *string `json:"phone_number"`
	}
	a.DB.Raw(`SELECT id, username, role, class_id, parent_email, phone_number FROM users WHERE id = ?`, id).Scan(&current)
	if current.ID == 0 {
		return utils.Error(c, 404, "Student not found")
	}
	username := current.Username
	if body.Username != nil {
		username = *body.Username
	}
	role := current.Role
	if body.Role != nil {
		role = *body.Role
	}
	classID := current.ClassID
	if body.ClassID != nil {
		classID = body.ClassID
	}
	var row map[string]interface{}
	a.DB.Raw(`
		UPDATE users SET username = ?, role = ?, class_id = ?, parent_email = ?, phone_number = ?, school_id = ?
		WHERE id = ?
		RETURNING id, username, role, class_id, parent_email, phone_number, profile_image
	`, username, role, classID, coalesceStrPtr(body.ParentEmail, current.ParentEmail), coalesceStrPtr(body.PhoneNumber, current.PhoneNumber), schoolID, id).Scan(&row)
	return utils.Success(c, 200, "Success Edit Student", row)
}

func (a *AppContext) DeleteStudent(c *fiber.Ctx) error {
	id := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)
	var row struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
	}
	a.DB.Raw(`DELETE FROM users WHERE id = ? AND school_id = ? AND role = 'SISWA' RETURNING id, username`, id, schoolID).Scan(&row)
	if row.ID == 0 {
		return utils.Error(c, 404, "Student not found")
	}
	return utils.Success(c, 200, fmt.Sprintf(`Siswa "%s" berhasil dihapus`, row.Username), nil)
}

func (a *AppContext) RegisterStudentByAdmin(c *fiber.Ctx) error {
	var body struct {
		Username    string  `json:"username"`
		Password    string  `json:"password"`
		Role        string  `json:"role"`
		ClassID     uint    `json:"class_id"`
		ParentEmail *string `json:"parent_email"`
		PhoneNumber *string `json:"phone_number"`
	}
	_ = c.BodyParser(&body)
	schoolID := c.Locals("schoolID").(uint)
	if body.Role == "" {
		body.Role = "SISWA"
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(body.Password), 8)
	var row map[string]interface{}
	a.DB.Raw(`
		INSERT INTO users (username, password, role, school_id, class_id, parent_email, phone_number)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		RETURNING id, username, role, school_id, class_id, parent_email, phone_number, profile_image
	`, body.Username, string(hash), body.Role, schoolID, body.ClassID, body.ParentEmail, body.PhoneNumber).Scan(&row)
	return utils.Success(c, 201, "User registered successfully", row)
}
