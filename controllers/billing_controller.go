package controllers

import (
	"bytes"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"lms/models"
	"lms/utils"
)

type xenditPaymentRequest struct {
	ReferenceID      string `json:"reference_id"`
	SessionType      string `json:"session_type"`
	Mode             string `json:"mode"`
	Amount           int64  `json:"amount"`
	Currency         string `json:"currency"`
	Country          string `json:"country"`
	SuccessReturnURL string `json:"success_return_url,omitempty"`
	CancelReturnURL  string `json:"cancel_return_url,omitempty"`
	Locale           string `json:"locale,omitempty"`
	Customer         struct {
		ReferenceID string `json:"reference_id,omitempty"`
		Type        string `json:"type,omitempty"`
		Email       string `json:"email,omitempty"`
	} `json:"customer,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

func (a *AppContext) GetSchoolBillingSettings(c *fiber.Ctx) error {
	schoolID := uint(utils.ToInt(c.Params("schoolId"), 0))
	if schoolID == 0 {
		return utils.Error(c, 400, "Sekolah wajib dipilih")
	}

	var billing models.SchoolBilling
	if err := a.DB.Where("school_id = ?", schoolID).Order("id desc").First(&billing).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Success(c, 200, "Success Get Billing Settings", fiber.Map{"item": nil})
		}
		return utils.Error(c, 500, "Gagal memuat billing sekolah", err.Error())
	}
	return utils.Success(c, 200, "Success Get Billing Settings", billing)
}

func (a *AppContext) UpsertSchoolBillingSettings(c *fiber.Ctx) error {
	schoolID := uint(utils.ToInt(c.Params("schoolId"), 0))
	if schoolID == 0 {
		return utils.Error(c, 400, "Sekolah wajib dipilih")
	}
	var body struct {
		BillingName   string  `json:"billing_name"`
		Amount        int64   `json:"amount"`
		Currency      string  `json:"currency"`
		DueDayOfMonth int     `json:"due_day_of_month"`
		IsActive      bool    `json:"is_active"`
		Notes         *string `json:"notes"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request")
	}
	if strings.TrimSpace(body.BillingName) == "" {
		return utils.Error(c, 400, "Nama billing wajib diisi")
	}
	if body.Amount <= 0 {
		return utils.Error(c, 400, "Nominal billing wajib lebih dari 0")
	}
	if body.DueDayOfMonth < 1 || body.DueDayOfMonth > 28 {
		return utils.Error(c, 400, "Tanggal jatuh tempo harus 1-28")
	}

	var billing models.SchoolBilling
	err := a.DB.Where("school_id = ?", schoolID).First(&billing).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return utils.Error(c, 500, "Gagal memuat billing sekolah", err.Error())
	}
	now := time.Now()
	if err == gorm.ErrRecordNotFound {
		billing = models.SchoolBilling{
			SchoolID:      schoolID,
			BillingName:   strings.TrimSpace(body.BillingName),
			Amount:        body.Amount,
			Currency:      defaultBillingCurrency(body.Currency),
			DueDayOfMonth: body.DueDayOfMonth,
			IsActive:      body.IsActive,
			Notes:         body.Notes,
			CreatedAt:     &now,
			UpdatedAt:     &now,
		}
		if err := a.DB.Create(&billing).Error; err != nil {
			return utils.Error(c, 500, "Gagal membuat billing sekolah", err.Error())
		}
		return utils.Success(c, 201, "Billing sekolah berhasil dibuat", billing)
	}

	updates := map[string]interface{}{
		"billing_name":     strings.TrimSpace(body.BillingName),
		"amount":           body.Amount,
		"currency":         defaultBillingCurrency(body.Currency),
		"due_day_of_month": body.DueDayOfMonth,
		"is_active":        body.IsActive,
		"notes":            body.Notes,
		"updated_at":       now,
	}
	if err := a.DB.Model(&models.SchoolBilling{}).Where("id = ?", billing.ID).Updates(updates).Error; err != nil {
		return utils.Error(c, 500, "Gagal memperbarui billing sekolah", err.Error())
	}
	if err := a.DB.Where("id = ?", billing.ID).First(&billing).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat billing sekolah", err.Error())
	}
	return utils.Success(c, 200, "Billing sekolah berhasil diperbarui", billing)
}

func (a *AppContext) DeleteSchoolBillingSettings(c *fiber.Ctx) error {
	schoolID := uint(utils.ToInt(c.Params("schoolId"), 0))
	if schoolID == 0 {
		return utils.Error(c, 400, "Sekolah wajib dipilih")
	}
	result := a.DB.Where("school_id = ?", schoolID).Delete(&models.SchoolBilling{})
	if result.Error != nil {
		return utils.Error(c, 500, "Gagal menghapus billing sekolah", result.Error.Error())
	}
	if result.RowsAffected == 0 {
		return utils.Error(c, 404, "Billing sekolah tidak ditemukan")
	}
	return utils.Success(c, 200, "Billing sekolah berhasil dihapus", fiber.Map{"school_id": schoolID})
}

func (a *AppContext) GetCurrentSchoolBilling(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	var billing models.SchoolBilling
	if err := a.DB.Where("school_id = ?", schoolID).Order("id desc").First(&billing).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return utils.Success(c, 200, "Success Get Current School Billing", fiber.Map{"item": nil})
		}
		return utils.Error(c, 500, "Gagal memuat billing sekolah", err.Error())
	}
	return utils.Success(c, 200, "Success Get Current School Billing", billing)
}

func (a *AppContext) CreateSchoolInvoice(c *fiber.Ctx) error {
	schoolID := uint(utils.ToInt(c.Params("schoolId"), 0))
	if schoolID == 0 {
		return utils.Error(c, 400, "Sekolah wajib dipilih")
	}
	var billing models.SchoolBilling
	if err := a.DB.Where("school_id = ? AND is_active = true", schoolID).First(&billing).Error; err != nil {
		return utils.Error(c, 404, "Billing sekolah tidak ditemukan")
	}
	dueDate := nextBillingDueDate(billing.DueDayOfMonth)
	invoiceNumber := fmt.Sprintf("INV-%d-%d", schoolID, time.Now().Unix())
	invoice := models.SchoolInvoice{
		SchoolBillingID: billing.ID,
		SchoolID:        schoolID,
		InvoiceNumber:   invoiceNumber,
		Amount:          billing.Amount,
		DueDate:         dueDate,
		Status:          "PENDING",
	}
	if err := a.DB.Create(&invoice).Error; err != nil {
		return utils.Error(c, 500, "Gagal membuat invoice", err.Error())
	}
	return utils.Success(c, 201, "Invoice berhasil dibuat", invoice)
}

func (a *AppContext) GetSchoolInvoices(c *fiber.Ctx) error {
	schoolID := uint(utils.ToInt(c.Params("schoolId"), 0))
	var items []models.SchoolInvoice
	if err := a.DB.Where("school_id = ?", schoolID).Order("id desc").Find(&items).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat invoice", err.Error())
	}
	return utils.Success(c, 200, "Success Get Invoices", items)
}

func (a *AppContext) DeleteSchoolInvoice(c *fiber.Ctx) error {
	schoolID := uint(utils.ToInt(c.Params("schoolId"), 0))
	invoiceID := uint(utils.ToInt(c.Params("invoiceId"), 0))
	if schoolID == 0 || invoiceID == 0 {
		return utils.Error(c, 400, "Invoice wajib dipilih")
	}
	result := a.DB.Where("id = ? AND school_id = ?", invoiceID, schoolID).Delete(&models.SchoolInvoice{})
	if result.Error != nil {
		return utils.Error(c, 500, "Gagal menghapus invoice", result.Error.Error())
	}
	if result.RowsAffected == 0 {
		return utils.Error(c, 404, "Invoice tidak ditemukan")
	}
	return utils.Success(c, 200, "Invoice berhasil dihapus", fiber.Map{"id": invoiceID, "school_id": schoolID})
}

func (a *AppContext) GetCurrentSchoolInvoices(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	var items []models.SchoolInvoice
	if err := a.DB.Where("school_id = ?", schoolID).Order("id desc").Find(&items).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat invoice", err.Error())
	}
	return utils.Success(c, 200, "Success Get Current School Invoices", items)
}

func (a *AppContext) CreateMidtransPaymentForInvoice(c *fiber.Ctx) error {
	schoolID := c.Locals("schoolID").(uint)
	invoiceID := c.Params("invoiceId")

	var invoice models.SchoolInvoice
	if err := a.DB.Where("id = ? AND school_id = ?", invoiceID, schoolID).First(&invoice).Error; err != nil {
		return utils.Error(c, 404, "Invoice tidak ditemukan")
	}

	xenditResp, err := createXenditCheckoutSession(invoice.InvoiceNumber, invoice.Amount, schoolID)
	if err != nil {
		return utils.Error(c, 500, "Gagal membuat pembayaran Xendit", err.Error())
	}

	now := time.Now()
	updates := map[string]interface{}{
		"snap_token":        nil,
		"snap_redirect_url": xenditResp.PaymentURL,
		"updated_at":        now,
	}
	if err := a.DB.Model(&models.SchoolInvoice{}).Where("id = ?", invoice.ID).Updates(updates).Error; err != nil {
		return utils.Error(c, 500, "Gagal menyimpan token pembayaran", err.Error())
	}

	invoice.SnapRedirectURL = &xenditResp.PaymentURL
	return utils.Success(c, 200, "Success Create Xendit Payment", fiber.Map{
		"invoice":      invoice,
		"redirect_url": xenditResp.PaymentURL,
		"payment_type": "checkout",
	})
}

func (a *AppContext) MidtransNotification(c *fiber.Ctx) error {
	return utils.Success(c, 200, "Notification endpoint not used by Xendit", fiber.Map{"ok": true})
}

type xenditWebhookPayload struct {
	Event          string `json:"event"`
	ReferenceID    string `json:"reference_id"`
	PaymentRequest struct {
		ID            string `json:"id"`
		ReferenceID   string `json:"reference_id"`
		Status        string `json:"status"`
		ChannelCode   string `json:"channel_code"`
		InvoiceNumber string `json:"invoice_number"`
	} `json:"payment_request"`
	Data struct {
		ID            string `json:"id"`
		ReferenceID   string `json:"reference_id"`
		Status        string `json:"status"`
		ChannelCode   string `json:"channel_code"`
		InvoiceNumber string `json:"invoice_number"`
	} `json:"data"`
}

func (a *AppContext) XenditWebhook(c *fiber.Ctx) error {
	expectedToken := strings.TrimSpace(os.Getenv("XENDIT_CALLBACK_TOKEN"))
	if expectedToken == "" {
		return utils.Error(c, 500, "XENDIT_CALLBACK_TOKEN is not configured")
	}
	if strings.TrimSpace(c.Get("X-Callback-Token")) != expectedToken {
		return utils.Error(c, 401, "Invalid callback token")
	}

	var payload xenditWebhookPayload
	if err := c.BodyParser(&payload); err != nil {
		return utils.Error(c, 400, "Invalid webhook payload")
	}

	referenceID := strings.TrimSpace(payload.ReferenceID)
	if referenceID == "" {
		referenceID = strings.TrimSpace(payload.PaymentRequest.ReferenceID)
	}
	if referenceID == "" {
		referenceID = strings.TrimSpace(payload.Data.ReferenceID)
	}
	if referenceID == "" {
		referenceID = strings.TrimSpace(payload.PaymentRequest.InvoiceNumber)
	}
	if referenceID == "" {
		referenceID = strings.TrimSpace(payload.Data.InvoiceNumber)
	}
	if referenceID == "" {
		return utils.Error(c, 400, "reference_id is required")
	}

	status := strings.ToLower(strings.TrimSpace(payload.PaymentRequest.Status))
	if status == "" {
		status = strings.ToLower(strings.TrimSpace(payload.Data.Status))
	}
	if status == "" {
		status = strings.ToLower(strings.TrimSpace(payload.Event))
	}

	var invoice models.SchoolInvoice
	if err := a.DB.Where("invoice_number = ?", referenceID).First(&invoice).Error; err != nil {
		return utils.Error(c, 404, "Invoice tidak ditemukan")
	}

	updates := map[string]interface{}{
		"payment_method": "xendit",
		"updated_at":     time.Now(),
	}
	switch status {
	case "paid", "completed", "success", "settled", "settlement", "payment_session.completed":
		updates["status"] = "PAID"
		now := time.Now()
		updates["paid_at"] = &now
	case "expired", "expire", "failed", "cancel", "cancelled", "canceled", "rejected", "payment_session.expired":
		updates["status"] = strings.ToUpper(status)
	default:
		updates["status"] = "PENDING"
	}

	if err := a.DB.Model(&models.SchoolInvoice{}).Where("id = ?", invoice.ID).Updates(updates).Error; err != nil {
		return utils.Error(c, 500, "Gagal memperbarui invoice", err.Error())
	}

	return utils.Success(c, 200, "Webhook processed", fiber.Map{
		"invoice_number": referenceID,
		"status":         status,
	})
}

type xenditCreateResponse struct {
	PaymentURL string
}

func createXenditCheckoutSession(referenceID string, amount int64, schoolID uint) (*xenditCreateResponse, error) {
	apiKey := strings.TrimSpace(os.Getenv("XENDIT_API_KEY"))
	if apiKey == "" {
		return nil, fmt.Errorf("XENDIT_API_KEY is not configured")
	}

	baseURL := strings.TrimSpace(os.Getenv("XENDIT_BASE_URL"))
	if baseURL == "" {
		baseURL = "https://api.xendit.co"
	}

	payload := xenditPaymentRequest{
		ReferenceID:      referenceID,
		SessionType:      "PAY",
		Mode:             "PAYMENT_LINK",
		Amount:           amount,
		Currency:         envOrDefault("XENDIT_PAYMENT_CURRENCY", "IDR"),
		Country:          envOrDefault("XENDIT_PAYMENT_COUNTRY", "ID"),
		SuccessReturnURL: strings.TrimSpace(os.Getenv("XENDIT_SUCCESS_RETURN_URL")),
		CancelReturnURL:  strings.TrimSpace(os.Getenv("XENDIT_FAILURE_RETURN_URL")),
		Locale:           "id",
		Metadata: map[string]interface{}{
			"invoice_number": referenceID,
			"school_id":      schoolID,
			"description":    fmt.Sprintf("Pembayaran invoice %s", referenceID),
		},
	}
	if payload.SuccessReturnURL == "" {
		payload.SuccessReturnURL = strings.TrimSpace(os.Getenv("XENDIT_SUCCESS_RETURN_URL"))
	}
	if payload.CancelReturnURL == "" {
		payload.CancelReturnURL = strings.TrimSpace(os.Getenv("XENDIT_FAILURE_RETURN_URL"))
	}
	expireMinutes := envIntOrDefault("XENDIT_EXPIRE_AFTER_MINUTES", 30)
	if expireMinutes > 0 {
		// Xendit expects an expiry date on the session so use a bounded TTL.
		payload.Metadata["expires_in_minutes"] = expireMinutes
	}

	bodyBytes, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", baseURL+"/sessions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(apiKey, "")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var parsed struct {
		PaymentLinkURL string `json:"payment_link_url"`
		Message        string `json:"message"`
	}
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		return nil, fmt.Errorf("xendit response parse failed: %w; body=%s", err, string(rawBody))
	}
	if resp.StatusCode >= 300 {
		msg := strings.TrimSpace(parsed.Message)
		if msg == "" {
			msg = string(rawBody)
		}
		return nil, fmt.Errorf("xendit error: %s", msg)
	}

	out := &xenditCreateResponse{PaymentURL: parsed.PaymentLinkURL}
	if out.PaymentURL == "" {
		return nil, fmt.Errorf("xendit response missing payment_link_url: %s", string(rawBody))
	}
	return out, nil
}

func defaultBillingCurrency(value string) string {
	if strings.TrimSpace(value) == "" {
		return "IDR"
	}
	return strings.ToUpper(strings.TrimSpace(value))
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envIntOrDefault(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func nextBillingDueDate(dayOfMonth int) time.Time {
	now := time.Now()
	year, month, _ := now.Date()
	location := now.Location()
	due := time.Date(year, month, dayOfMonth, 23, 59, 59, 0, location)
	if !due.After(now) {
		due = due.AddDate(0, 1, 0)
	}
	return due
}

func verifyMidtransSignature(orderID, statusCode, grossAmount, signatureKey string) bool {
	serverKey := strings.TrimSpace(os.Getenv("MIDTRANS_SERVER_KEY"))
	if serverKey == "" {
		return false
	}
	sum := sha512.Sum512([]byte(orderID + statusCode + grossAmount + serverKey))
	expected := fmt.Sprintf("%x", sum)
	return strings.EqualFold(expected, signatureKey)
}
