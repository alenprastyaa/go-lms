package controllers

import (
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"lms/models"
	"lms/utils"
)

func (a *AppContext) GetPublicRegistrationOptions(c *fiber.Ctx) error {
	var schools []models.School
	a.DB.Order("name asc").Find(&schools)

	result := make([]fiber.Map, 0, len(schools))
	for _, school := range schools {
		var classes []models.Class
		a.DB.Where("school_id = ?", school.ID).Order("class_name asc").Find(&classes)
		result = append(result, fiber.Map{"id": school.ID, "name": school.Name, "classes": classes})
	}
	return utils.Success(c, 200, "Success Get Registration Options", result)
}

func (a *AppContext) RegisterStudentPublic(c *fiber.Ctx) error {
	var body struct {
		Username    string  `json:"username"`
		Password    string  `json:"password"`
		ClassID     uint    `json:"class_id"`
		ParentEmail *string `json:"parent_email"`
		PhoneNumber *string `json:"phone_number"`
	}
	_ = c.BodyParser(&body)

	var classItem models.Class
	if err := a.DB.Where("id = ?", body.ClassID).First(&classItem).Error; err != nil {
		return utils.Error(c, 404, "Class not found")
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(body.Password), 8)
	user := models.User{
		Username:    body.Username,
		Password:    string(hash),
		Role:        "SISWA",
		SchoolID:    &classItem.SchoolID,
		ClassID:     &body.ClassID,
		ParentEmail: body.ParentEmail,
		PhoneNumber: body.PhoneNumber,
	}
	if err := a.DB.Create(&user).Error; err != nil {
		return utils.Error(c, 500, "Failed Student Registration", err.Error())
	}
	return utils.Success(c, 201, "Registrasi siswa berhasil", user)
}
