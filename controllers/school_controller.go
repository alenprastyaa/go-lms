package controllers

import (
	"github.com/gofiber/fiber/v2"
	"lms/models"
	"lms/utils"
)

func (a *AppContext) CreateSchool(c *fiber.Ctx) error {
	var body struct {
		Name string `json:"name"`
	}
	_ = c.BodyParser(&body)

	school := models.School{Name: body.Name}
	if err := a.DB.Create(&school).Error; err != nil {
		return utils.Error(c, 500, "Registration failed", err.Error())
	}
	return utils.Success(c, 201, "school registered successfully", school)
}
