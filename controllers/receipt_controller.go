package controllers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"lms/models"
	"lms/utils"
)

func (a *AppContext) CreateReceipt(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	paymentDate := c.FormValue("payment_date")
	description := c.FormValue("description")

	file, err := c.FormFile("image")
	if err != nil {
		return utils.Error(c, 400, "Image is required")
	}

	imageURL, err := utils.SaveUploadedFile(c, file)
	if err != nil {
		return utils.Error(c, 500, "Faild Create Receipt", err.Error())
	}

	parsedDate, _ := time.Parse("2006-01-02", paymentDate)
	receipt := models.Receipt{
		ImagePath:   imageURL,
		UserID:      userID,
		Description: utils.StringPtr(description),
		PaymentDate: &parsedDate,
	}
	if err := a.DB.Create(&receipt).Error; err != nil {
		return utils.Error(c, 500, "Faild Create Receipt", err.Error())
	}
	return utils.Success(c, 201, "Success Create Receipt", receipt)
}

func (a *AppContext) GetReceipt(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	var receipts []models.Receipt
	a.DB.Where("user_id = ?", userID).Order("payment_date desc nulls last, created_at desc").Find(&receipts)
	return utils.Success(c, 200, "Success Get Data Receipt", receipts)
}
