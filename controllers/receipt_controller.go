package controllers

import (
	"strings"
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

func (a *AppContext) GetReceiptByID(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	id := c.Params("id")

	var receipt models.Receipt
	if err := a.DB.Where("id = ? AND user_id = ?", id, userID).First(&receipt).Error; err != nil {
		return utils.Error(c, 404, "Receipt not found")
	}
	return utils.Success(c, 200, "Success Get Receipt", receipt)
}

func (a *AppContext) UpdateReceipt(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	id := c.Params("id")

	var receipt models.Receipt
	if err := a.DB.Where("id = ? AND user_id = ?", id, userID).First(&receipt).Error; err != nil {
		return utils.Error(c, 404, "Receipt not found")
	}

	description := strings.TrimSpace(c.FormValue("description"))
	paymentDateRaw := strings.TrimSpace(c.FormValue("payment_date"))

	updates := map[string]interface{}{
		"description": nil,
	}
	if description != "" {
		updates["description"] = description
	}

	if paymentDateRaw != "" {
		parsedDate, err := time.Parse("2006-01-02", paymentDateRaw)
		if err != nil {
			return utils.Error(c, 400, "Invalid payment_date format, expected YYYY-MM-DD")
		}
		updates["payment_date"] = parsedDate
	}

	file, err := c.FormFile("image")
	if err == nil && file != nil {
		imageURL, upErr := utils.SaveUploadedFile(c, file)
		if upErr != nil {
			return utils.Error(c, 500, "Faild Update Receipt", upErr.Error())
		}
		updates["image_path"] = imageURL
	}

	if err := a.DB.Model(&models.Receipt{}).Where("id = ? AND user_id = ?", id, userID).Updates(updates).Error; err != nil {
		return utils.Error(c, 500, "Faild Update Receipt", err.Error())
	}

	var updated models.Receipt
	_ = a.DB.Where("id = ? AND user_id = ?", id, userID).First(&updated).Error
	return utils.Success(c, 200, "Success Update Receipt", updated)
}

func (a *AppContext) DeleteReceipt(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)
	id := c.Params("id")

	var receipt models.Receipt
	if err := a.DB.Where("id = ? AND user_id = ?", id, userID).First(&receipt).Error; err != nil {
		return utils.Error(c, 404, "Receipt not found")
	}

	if err := a.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&models.Receipt{}).Error; err != nil {
		return utils.Error(c, 500, "Faild Delete Receipt", err.Error())
	}
	return utils.Success(c, 200, "Success Delete Receipt", fiber.Map{"id": receipt.ID})
}
