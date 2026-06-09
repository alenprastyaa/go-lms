package controllers

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"lms/models"
	"lms/utils"
)

func (a *AppContext) GetMajors(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	includeInactive := c.Query("include_inactive") == "1"

	q := a.DB.Table("majors").
		Where("school_id = ?", schoolID).
		Order("is_active DESC, name ASC")
	if !includeInactive {
		q = q.Where("is_active = true")
	}

	var rows []map[string]interface{}
	if err := q.Find(&rows).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat jurusan", err.Error())
	}
	return utils.Success(c, 200, "Success Get Majors", rows)
}

func (a *AppContext) CreateMajor(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	var body struct {
		Name     string `json:"name"`
		Code     string `json:"code"`
		Quota    *int   `json:"quota"`
		IsActive *bool  `json:"is_active"`
	}
	_ = c.BodyParser(&body)

	name := strings.TrimSpace(body.Name)
	code := strings.ToUpper(strings.TrimSpace(body.Code))
	if name == "" {
		return utils.Error(c, 400, "Nama jurusan wajib diisi")
	}
	if code == "" {
		return utils.Error(c, 400, "Kode jurusan wajib diisi")
	}
	if body.Quota != nil && *body.Quota < 0 {
		return utils.Error(c, 400, "Kuota tidak valid")
	}

	isActive := true
	if body.IsActive != nil {
		isActive = *body.IsActive
	}

	var row map[string]interface{}
	if err := a.DB.Raw(`
		INSERT INTO majors (school_id, name, code, quota, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())
		RETURNING id, school_id, name, code, quota, is_active, created_at, updated_at
	`, schoolID, name, code, body.Quota, isActive).Scan(&row).Error; err != nil {
		return utils.Error(c, 500, "Gagal membuat jurusan", err.Error())
	}
	return utils.Success(c, 201, "Jurusan berhasil dibuat", row)
}

func (a *AppContext) UpdateMajor(c *fiber.Ctx) error {
	id := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)

	var current models.Major
	if err := a.DB.Where("id = ? AND school_id = ?", id, schoolID).First(&current).Error; err != nil {
		return utils.Error(c, 404, "Jurusan tidak ditemukan")
	}

	var body struct {
		Name     *string `json:"name"`
		Code     *string `json:"code"`
		Quota    *int    `json:"quota"`
		IsActive *bool   `json:"is_active"`
	}
	_ = c.BodyParser(&body)

	updates := map[string]interface{}{
		"updated_at": gorm.Expr("NOW()"),
	}
	if body.Name != nil {
		name := strings.TrimSpace(*body.Name)
		if name == "" {
			return utils.Error(c, 400, "Nama jurusan wajib diisi")
		}
		updates["name"] = name
	}
	if body.Code != nil {
		code := strings.ToUpper(strings.TrimSpace(*body.Code))
		if code == "" {
			return utils.Error(c, 400, "Kode jurusan wajib diisi")
		}
		updates["code"] = code
	}
	if body.Quota != nil {
		if *body.Quota < 0 {
			return utils.Error(c, 400, "Kuota tidak valid")
		}
		updates["quota"] = body.Quota
	}
	if body.IsActive != nil {
		updates["is_active"] = *body.IsActive
	}

	if err := a.DB.Model(&current).Updates(updates).Error; err != nil {
		return utils.Error(c, 500, "Gagal memperbarui jurusan", err.Error())
	}

	var row map[string]interface{}
	a.DB.Table("majors").Where("id = ?", current.ID).Take(&row)
	return utils.Success(c, 200, "Jurusan berhasil diperbarui", row)
}

func (a *AppContext) DeleteMajor(c *fiber.Ctx) error {
	id := c.Params("id")
	schoolID := c.Locals("schoolID").(uint)

	var current models.Major
	if err := a.DB.Where("id = ? AND school_id = ?", id, schoolID).First(&current).Error; err != nil {
		return utils.Error(c, 404, "Jurusan tidak ditemukan")
	}

	var used int64
	a.DB.Table("spmb_applicants").
		Where("school_id = ? AND (? IN (first_major_id, second_major_id, third_major_id, accepted_major_id))", schoolID, current.ID).
		Count(&used)
	if used > 0 {
		return utils.Error(c, 400, "Jurusan sudah dipakai di data SPMB. Nonaktifkan jurusan jika tidak ingin ditampilkan.")
	}

	if err := a.DB.Delete(&current).Error; err != nil {
		return utils.Error(c, 500, "Gagal menghapus jurusan", err.Error())
	}
	return utils.Success(c, 200, "Jurusan berhasil dihapus", fiber.Map{"id": current.ID})
}

func (a *AppContext) majorBelongsToSchool(majorID uint, schoolID uint) bool {
	if majorID == 0 {
		return false
	}
	var count int64
	a.DB.Table("majors").Where("id = ? AND school_id = ?", majorID, schoolID).Count(&count)
	return count > 0
}
