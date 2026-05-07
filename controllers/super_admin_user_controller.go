package controllers

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"lms/models"
	"lms/utils"
)

func (a *AppContext) GetSuperAdminAdminUsers(c *fiber.Ctx) error {
	schoolID := utils.ToInt(c.Query("school_id"), 0)
	search := strings.TrimSpace(c.Query("search"))

	query := a.DB.Table("users u").
		Select(`
			u.id,
			u.full_name,
			u.username,
			u.role,
			u.school_id,
			u.parent_email,
			u.phone_number,
			u.profile_image,
			s.name AS school_name
		`).
		Joins("LEFT JOIN schools s ON s.id = u.school_id").
		Where("u.role = ?", "ADMIN")

	if schoolID > 0 {
		query = query.Where("u.school_id = ?", schoolID)
	}
	if search != "" {
		like := "%" + search + "%"
		query = query.Where(`
			COALESCE(u.full_name, '') ILIKE ?
			OR u.username ILIKE ?
			OR COALESCE(u.parent_email, '') ILIKE ?
			OR COALESCE(s.name, '') ILIKE ?
		`, like, like, like, like)
	}

	var items []map[string]interface{}
	if err := query.Order("s.name ASC, u.username ASC").Scan(&items).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat admin sekolah", err.Error())
	}

	return utils.Success(c, 200, "Success Get Super Admin Admin Users", fiber.Map{
		"items": recentOrEmpty(items),
	})
}

func (a *AppContext) CreateSuperAdminAdminUser(c *fiber.Ctx) error {
	var body struct {
		FullName    string `json:"full_name"`
		Username    string `json:"username"`
		Password    string `json:"password"`
		SchoolID    uint   `json:"school_id"`
		ParentEmail string `json:"parent_email"`
		PhoneNumber string `json:"phone_number"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request")
	}

	username := strings.TrimSpace(body.Username)
	if username == "" {
		return utils.Error(c, 400, "Username wajib diisi")
	}
	if len(strings.TrimSpace(body.Password)) < 6 {
		return utils.Error(c, 400, "Password minimal 6 karakter")
	}
	if body.SchoolID == 0 {
		return utils.Error(c, 400, "Sekolah wajib dipilih")
	}

	if err := ensureSchoolExists(a.DB, body.SchoolID); err != nil {
		return respondValidationError(c, err, "Gagal memvalidasi sekolah")
	}
	if err := ensureUsernameAvailable(a.DB, username, 0); err != nil {
		return respondValidationError(c, err, "Gagal memvalidasi username")
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(body.Password), 8)
	user := models.User{
		FullName:    nullIfBlankPtr(body.FullName),
		Username:    username,
		Password:    string(hash),
		Role:        "ADMIN",
		SchoolID:    &body.SchoolID,
		ParentEmail: nullIfBlankPtr(body.ParentEmail),
		PhoneNumber: nullIfBlankPtr(body.PhoneNumber),
	}

	if err := a.DB.Create(&user).Error; err != nil {
		return utils.Error(c, 500, "Gagal membuat admin sekolah", err.Error())
	}

	return a.respondSuperAdminAdminUser(c, user.ID, 201, "Admin sekolah berhasil dibuat")
}

func (a *AppContext) UpdateSuperAdminAdminUser(c *fiber.Ctx) error {
	id := c.Params("id")
	var body struct {
		FullName    *string `json:"full_name"`
		Username    *string `json:"username"`
		Password    *string `json:"password"`
		SchoolID    *uint   `json:"school_id"`
		ParentEmail *string `json:"parent_email"`
		PhoneNumber *string `json:"phone_number"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request")
	}

	var current models.User
	if err := a.DB.Where("id = ? AND role = ?", id, "ADMIN").First(&current).Error; err != nil {
		return utils.Error(c, 404, "Admin sekolah tidak ditemukan")
	}

	updates := map[string]interface{}{}
	if body.FullName != nil {
		updates["full_name"] = nilIfBlank(*body.FullName)
	}
	if body.ParentEmail != nil {
		updates["parent_email"] = nilIfBlank(*body.ParentEmail)
	}
	if body.PhoneNumber != nil {
		updates["phone_number"] = nilIfBlank(*body.PhoneNumber)
	}
	if body.Username != nil {
		username := strings.TrimSpace(*body.Username)
		if username == "" {
			return utils.Error(c, 400, "Username wajib diisi")
		}
		if err := ensureUsernameAvailable(a.DB, username, current.ID); err != nil {
			return respondValidationError(c, err, "Gagal memvalidasi username")
		}
		updates["username"] = username
	}
	if body.SchoolID != nil {
		if *body.SchoolID == 0 {
			return utils.Error(c, 400, "Sekolah wajib dipilih")
		}
		if err := ensureSchoolExists(a.DB, *body.SchoolID); err != nil {
			return respondValidationError(c, err, "Gagal memvalidasi sekolah")
		}
		updates["school_id"] = *body.SchoolID
	}
	if body.Password != nil && strings.TrimSpace(*body.Password) != "" {
		if len(strings.TrimSpace(*body.Password)) < 6 {
			return utils.Error(c, 400, "Password minimal 6 karakter")
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(strings.TrimSpace(*body.Password)), 8)
		updates["password"] = string(hash)
		updates["session_version"] = gorm.Expr("COALESCE(session_version, 0) + 1")
	}

	if len(updates) == 0 {
		return utils.Error(c, 400, "Tidak ada perubahan data admin")
	}

	if err := a.DB.Model(&models.User{}).Where("id = ? AND role = ?", current.ID, "ADMIN").Updates(updates).Error; err != nil {
		return utils.Error(c, 500, "Gagal memperbarui admin sekolah", err.Error())
	}

	return a.respondSuperAdminAdminUser(c, current.ID, 200, "Admin sekolah berhasil diperbarui")
}

func (a *AppContext) DeleteSuperAdminAdminUser(c *fiber.Ctx) error {
	id := c.Params("id")

	var current models.User
	if err := a.DB.Where("id = ? AND role = ?", id, "ADMIN").First(&current).Error; err != nil {
		return utils.Error(c, 404, "Admin sekolah tidak ditemukan")
	}

	if err := a.DB.Delete(&current).Error; err != nil {
		return utils.Error(c, 500, "Gagal menghapus admin sekolah", err.Error())
	}

	return utils.Success(c, 200, `Admin sekolah berhasil dihapus`, fiber.Map{
		"id":       current.ID,
		"username": current.Username,
	})
}

func (a *AppContext) respondSuperAdminAdminUser(c *fiber.Ctx, userID uint, code int, message string) error {
	var item map[string]interface{}
	if err := a.DB.Table("users u").
		Select(`
			u.id,
			u.full_name,
			u.username,
			u.role,
			u.school_id,
			u.parent_email,
			u.phone_number,
			u.profile_image,
			s.name AS school_name
		`).
		Joins("LEFT JOIN schools s ON s.id = u.school_id").
		Where("u.id = ?", userID).
		Scan(&item).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat data admin sekolah", err.Error())
	}

	return utils.Success(c, code, message, item)
}

func ensureSchoolExists(db *gorm.DB, schoolID uint) error {
	var school models.School
	if err := db.Select("id").Where("id = ?", schoolID).First(&school).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fiber.NewError(404, "Sekolah tidak ditemukan")
		}
		return err
	}
	return nil
}

func ensureUsernameAvailable(db *gorm.DB, username string, excludedUserID uint) error {
	query := db.Table("users").Where("LOWER(username) = LOWER(?)", username)
	if excludedUserID > 0 {
		query = query.Where("id <> ?", excludedUserID)
	}

	var existing models.User
	if err := query.Select("id").Take(&existing).Error; err == nil {
		return fiber.NewError(400, "Username sudah digunakan")
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return nil
}

func nullIfBlankPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func nilIfBlank(value string) interface{} {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func respondValidationError(c *fiber.Ctx, err error, fallback string) error {
	if err == nil {
		return nil
	}
	if fiberErr, ok := err.(*fiber.Error); ok {
		return utils.Error(c, fiberErr.Code, fiberErr.Message)
	}
	return utils.Error(c, 500, fallback, err.Error())
}
