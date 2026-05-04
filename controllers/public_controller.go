package controllers

import (
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"lms/models"
	"lms/utils"
)

func (a *AppContext) GetPublicRegistrationOptions(c *fiber.Ctx) error {
	token := c.Query("token")
	schoolID, err := utils.ParseSchoolRegistrationToken(token)
	if err != nil {
		return utils.Error(c, 401, "Link pendaftaran tidak valid atau sudah kadaluarsa")
	}

	var school models.School
	if err := a.DB.Where("id = ?", schoolID).First(&school).Error; err != nil {
		return utils.Error(c, 404, "Sekolah tidak ditemukan")
	}

	var classes []models.Class
	a.DB.Where("school_id = ?", schoolID).Order("class_name asc").Find(&classes)
	return utils.Success(c, 200, "Success Get Registration Options", fiber.Map{
		"school": fiber.Map{
			"id":   school.ID,
			"name": school.Name,
		},
		"classes": classes,
	})
}

func (a *AppContext) RegisterStudentPublic(c *fiber.Ctx) error {
	var body struct {
		Username    string  `json:"username"`
		Password    string  `json:"password"`
		Token       string  `json:"token"`
		ClassID     uint    `json:"class_id"`
		ParentEmail *string `json:"parent_email"`
		PhoneNumber *string `json:"phone_number"`
	}
	_ = c.BodyParser(&body)

	schoolID, err := utils.ParseSchoolRegistrationToken(body.Token)
	if err != nil {
		return utils.Error(c, 401, "Link pendaftaran tidak valid atau sudah kadaluarsa")
	}

	var classItem models.Class
	if err := a.DB.Where("id = ? AND school_id = ?", body.ClassID, schoolID).First(&classItem).Error; err != nil {
		return utils.Error(c, 404, "Class not found")
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(body.Password), 8)
	user := models.User{
		Username:    body.Username,
		Password:    string(hash),
		Role:        "SISWA",
		SchoolID:    &schoolID,
		ClassID:     &body.ClassID,
		ParentEmail: body.ParentEmail,
		PhoneNumber: body.PhoneNumber,
	}
	if err := a.DB.Create(&user).Error; err != nil {
		return utils.Error(c, 500, "Failed Student Registration", err.Error())
	}
	return utils.Success(c, 201, "Registrasi siswa berhasil", user)
}
