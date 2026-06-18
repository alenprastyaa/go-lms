package controllers

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"lms/models"
	"lms/services"
	"lms/utils"
)

type packageCheckoutPayload struct {
	PackageID  uint   `json:"package_id"`
	SchoolName string `json:"school_name"`
	Email      string `json:"email"`
}

type ipaymuPaymentResponse struct {
	Status   int    `json:"Status"`
	Status2  int    `json:"status"`
	Message  string `json:"Message"`
	Message2 string `json:"message"`
	Data     struct {
		SessionID      string `json:"SessionID"`
		SessionID2     string `json:"SessionId"`
		SessionID3     string `json:"sessionId"`
		SessionID4     string `json:"session_id"`
		TransactionID  string `json:"TransactionId"`
		TransactionID2 string `json:"TransactionID"`
		TransactionID3 string `json:"transactionId"`
		TransactionID4 string `json:"transaction_id"`
		URL            string `json:"Url"`
		URL2           string `json:"URL"`
		URL3           string `json:"url"`
	} `json:"Data"`
	Data2 struct {
		SessionID      string `json:"SessionID"`
		SessionID2     string `json:"SessionId"`
		SessionID3     string `json:"sessionId"`
		SessionID4     string `json:"session_id"`
		TransactionID  string `json:"TransactionId"`
		TransactionID2 string `json:"TransactionID"`
		TransactionID3 string `json:"transactionId"`
		TransactionID4 string `json:"transaction_id"`
		URL            string `json:"Url"`
		URL2           string `json:"URL"`
		URL3           string `json:"url"`
	} `json:"data"`
}

type ipaymuPaymentPayload struct {
	Product    []string `json:"product"`
	Qty        []string `json:"qty"`
	Price      []string `json:"price"`
	Reference  string   `json:"referenceId"`
	ReturnURL  string   `json:"returnUrl"`
	NotifyURL  string   `json:"notifyUrl"`
	CancelURL  string   `json:"cancelUrl"`
	BuyerName  string   `json:"buyerName,omitempty"`
	BuyerEmail string   `json:"buyerEmail,omitempty"`
	Expired    string   `json:"expired,omitempty"`
	Account    string   `json:"account,omitempty"`
	Lang       string   `json:"lang,omitempty"`
	LogoURL    string   `json:"logoUrl,omitempty"`
	ImageURL   string   `json:"imageUrl,omitempty"`
}

func (a *AppContext) CreatePackageCheckout(c *fiber.Ctx) error {
	var body packageCheckoutPayload
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request")
	}

	schoolName := strings.TrimSpace(body.SchoolName)
	email := strings.ToLower(strings.TrimSpace(body.Email))
	if body.PackageID == 0 {
		return utils.Error(c, 400, "Paket wajib dipilih")
	}
	if schoolName == "" {
		return utils.Error(c, 400, "Nama sekolah wajib diisi")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return utils.Error(c, 400, "Email sekolah tidak valid")
	}

	var pkg models.Package
	if err := a.DB.Where("id = ? AND is_active = true", body.PackageID).First(&pkg).Error; err != nil {
		return utils.Error(c, 404, "Paket tidak ditemukan")
	}
	if pkg.Price <= 0 {
		return utils.Error(c, 400, "Harga paket tidak valid")
	}

	amount := int64(math.Round(pkg.Price))
	order := models.PackageCheckoutOrder{
		ReferenceID: fmt.Sprintf("PKG-%d-%d", pkg.ID, time.Now().UnixNano()),
		PackageID:   pkg.ID,
		PackageName: pkg.Name,
		SchoolName:  schoolName,
		Email:       email,
		Amount:      amount,
		Status:      "PENDING",
	}
	if err := a.DB.Create(&order).Error; err != nil {
		return utils.Error(c, 500, "Gagal membuat invoice paket", err.Error())
	}

	paymentURL, transactionID, err := createIPaymuPayment(order, pkg)
	if err != nil {
		_ = a.DB.Model(&models.PackageCheckoutOrder{}).Where("id = ?", order.ID).Updates(map[string]interface{}{
			"status":     "PAYMENT_CREATE_FAILED",
			"updated_at": jakartaNow(),
		}).Error
		return utils.Error(c, 500, "Gagal membuat pembayaran iPaymu", friendlyIPaymuPaymentError(err))
	}

	now := jakartaNow()
	updates := map[string]interface{}{
		"payment_method": "ipaymu",
		"payment_url":    paymentURL,
		"updated_at":     now,
	}
	if transactionID != "" {
		updates["transaction_id"] = transactionID
	}
	if err := a.DB.Model(&models.PackageCheckoutOrder{}).Where("id = ?", order.ID).Updates(updates).Error; err != nil {
		return utils.Error(c, 500, "Gagal menyimpan pembayaran paket", err.Error())
	}
	order.PaymentMethod = utils.StringPtr("ipaymu")
	order.PaymentURL = utils.StringPtr(paymentURL)
	if transactionID != "" {
		order.TransactionID = &transactionID
	}

	if err := sendPackageInvoiceEmail(order); err == nil {
		sentAt := jakartaNow()
		_ = a.DB.Model(&models.PackageCheckoutOrder{}).Where("id = ?", order.ID).Updates(map[string]interface{}{
			"invoice_sent_at": &sentAt,
			"updated_at":      sentAt,
		}).Error
		order.InvoiceSentAt = &sentAt
	}

	return utils.Success(c, 201, "Invoice paket berhasil dibuat", fiber.Map{
		"order":        order,
		"redirect_url": paymentURL,
	})
}

func (a *AppContext) IPaymuPackageWebhook(c *fiber.Ctx) error {
	referenceID := firstNonBlank(
		c.Query("reference_id"),
		c.FormValue("reference_id"),
		c.FormValue("referenceId"),
		c.FormValue("reference"),
		c.FormValue("invoice"),
		c.FormValue("sid"),
	)
	status := firstNonBlank(c.FormValue("status"), c.FormValue("Status"), c.FormValue("payment_status"))
	transactionID := firstNonBlank(c.FormValue("trx_id"), c.FormValue("transaction_id"), c.FormValue("TransactionId"))

	var body map[string]interface{}
	if len(c.Body()) > 0 {
		if err := json.Unmarshal(c.Body(), &body); err == nil {
			if referenceID == "" {
				referenceID = firstNonBlank(
					utils.ToString(body["reference_id"]),
					utils.ToString(body["referenceId"]),
					utils.ToString(body["reference"]),
					utils.ToString(body["invoice"]),
					utils.ToString(body["sid"]),
				)
			}
			status = firstNonBlank(status, utils.ToString(body["status"]), utils.ToString(body["Status"]), utils.ToString(body["payment_status"]))
			transactionID = firstNonBlank(transactionID, utils.ToString(body["trx_id"]), utils.ToString(body["transaction_id"]), utils.ToString(body["TransactionId"]))
		}
	}
	if referenceID == "" {
		if len(body) > 0 {
			referenceID = firstNonBlank(
				utils.ToString(body["reference_id"]),
				utils.ToString(body["referenceId"]),
				utils.ToString(body["reference"]),
				utils.ToString(body["invoice"]),
				utils.ToString(body["sid"]),
			)
			status = firstNonBlank(status, utils.ToString(body["status"]), utils.ToString(body["Status"]), utils.ToString(body["payment_status"]))
			transactionID = firstNonBlank(transactionID, utils.ToString(body["trx_id"]), utils.ToString(body["transaction_id"]), utils.ToString(body["TransactionId"]))
		}
	}
	referenceID = strings.TrimSpace(referenceID)
	if referenceID == "" {
		return utils.Error(c, 400, "reference_id is required")
	}
	if !verifyCheckoutWebhookToken(referenceID, c.Query("token")) {
		return utils.Error(c, 401, "Invalid iPaymu callback token")
	}

	var order models.PackageCheckoutOrder
	if err := a.DB.Where("reference_id = ?", referenceID).First(&order).Error; err != nil {
		return utils.Error(c, 404, "Checkout order tidak ditemukan")
	}

	normalizedStatus := normalizeIPaymuStatus(status)
	updates := map[string]interface{}{
		"status":         normalizedStatus,
		"payment_method": "ipaymu",
		"updated_at":     jakartaNow(),
	}
	if transactionID != "" {
		updates["transaction_id"] = transactionID
	}
	if normalizedStatus == "PAID" && order.PaidAt == nil {
		now := jakartaNow()
		updates["paid_at"] = &now
	}
	if err := a.DB.Model(&models.PackageCheckoutOrder{}).Where("id = ?", order.ID).Updates(updates).Error; err != nil {
		return utils.Error(c, 500, "Gagal memperbarui order", err.Error())
	}

	if normalizedStatus == "PAID" {
		if err := a.provisionPackageCheckout(order.ReferenceID); err != nil {
			return utils.Error(c, 500, "Pembayaran tercatat, tetapi provisioning gagal", err.Error())
		}
	}

	return utils.Success(c, 200, "iPaymu webhook processed", fiber.Map{
		"reference_id": referenceID,
		"status":       normalizedStatus,
	})
}

func createIPaymuPayment(order models.PackageCheckoutOrder, pkg models.Package) (string, string, error) {
	va := strings.TrimSpace(os.Getenv("IPAYMU_VA"))
	apiKey := strings.TrimSpace(os.Getenv("IPAYMU_API_KEY"))
	if va == "" || apiKey == "" {
		return "", "", fmt.Errorf("IPAYMU_VA dan IPAYMU_API_KEY wajib dikonfigurasi di environment server")
	}

	baseURL := strings.TrimRight(envOrDefault("IPAYMU_BASE_URL", "https://my.ipaymu.com"), "/")
	appURL := strings.TrimRight(envOrDefault("PUBLIC_APP_URL", "https://lms.idschoolsystem.com"), "/")
	apiURL := baseURL + "/api/v2/payment"
	notifyURL := strings.TrimSpace(os.Getenv("IPAYMU_NOTIFY_URL"))
	if notifyURL == "" {
		notifyURL = strings.TrimRight(envOrDefault("PUBLIC_API_URL", appURL+"/api"), "/") + "/billing/ipaymu/webhook"
	}
	notifyURL = appendReturnQuery(notifyURL, "reference_id", order.ReferenceID)
	notifyURL = appendReturnQuery(notifyURL, "token", checkoutWebhookToken(order.ReferenceID, apiKey))

	payload := ipaymuPaymentPayload{
		Product:    []string{pkg.Name},
		Qty:        []string{"1"},
		Price:      []string{strconv.FormatInt(order.Amount, 10)},
		Reference:  order.ReferenceID,
		ReturnURL:  appendReturnQuery(appURL+"/payment/success", "source", "landing"),
		NotifyURL:  notifyURL,
		CancelURL:  appendReturnQuery(appURL+"/payment/failed", "source", "landing"),
		BuyerName:  order.SchoolName,
		BuyerEmail: order.Email,
		Expired:    envOrDefault("IPAYMU_EXPIRED_HOURS", "24"),
		Account:    va,
		Lang:       "id",
		LogoURL:    strings.TrimSpace(pkg.LogoURL),
		ImageURL:   strings.TrimSpace(pkg.LogoURL),
	}

	bodyBytes, err := marshalIPaymuPayload(payload)
	if err != nil {
		return "", "", err
	}
	timestamp := time.Now().Format("20060102150405")
	signature := createIPaymuSignature("POST", va, apiKey, bodyBytes)

	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", "", err
	}
	req.Header = http.Header{
		"Content-Type": {"application/json"},
		"Accept":       {"application/json"},
		"va":           {va},
		"signature":    {signature},
		"timestamp":    {timestamp},
		"User-Agent":   {"Bitwize-LMS/1.0"},
	}

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 65536))
	if err != nil {
		return "", "", err
	}

	var parsed ipaymuPaymentResponse
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		return "", "", fmt.Errorf("ipaymu response parse failed: %w; body=%s", err, string(rawBody))
	}
	parsedStatus := parsed.Status
	if parsedStatus == 0 {
		parsedStatus = parsed.Status2
	}
	if resp.StatusCode >= 300 || parsedStatus >= 300 {
		message := strings.TrimSpace(firstNonBlank(parsed.Message, parsed.Message2))
		if message == "" {
			message = string(rawBody)
		}
		return "", "", fmt.Errorf("ipaymu error: %s", message)
	}

	paymentURL := firstNonBlank(parsed.Data.URL, parsed.Data.URL2, parsed.Data.URL3, parsed.Data2.URL, parsed.Data2.URL2, parsed.Data2.URL3)
	transactionID := firstNonBlank(
		parsed.Data.TransactionID, parsed.Data.TransactionID2, parsed.Data.TransactionID3, parsed.Data.TransactionID4,
		parsed.Data.SessionID, parsed.Data.SessionID2, parsed.Data.SessionID3, parsed.Data.SessionID4,
		parsed.Data2.TransactionID, parsed.Data2.TransactionID2, parsed.Data2.TransactionID3, parsed.Data2.TransactionID4,
		parsed.Data2.SessionID, parsed.Data2.SessionID2, parsed.Data2.SessionID3, parsed.Data2.SessionID4,
	)
	if paymentURL == "" {
		return "", "", fmt.Errorf("ipaymu response missing payment url: %s", string(rawBody))
	}
	return paymentURL, transactionID, nil
}

func friendlyIPaymuPaymentError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if strings.Contains(strings.ToLower(message), "invalid ip") {
		return message + ". IP server belum diizinkan di dashboard iPaymu untuk API key ini, atau IPAYMU_BASE_URL tidak cocok dengan kredensial yang dipakai. Daftarkan public outbound IP server pada pengaturan API iPaymu lalu coba lagi."
	}
	if strings.Contains(strings.ToLower(message), "unauthorized credential") {
		return message + ". Credential iPaymu tidak cocok dengan endpoint yang dipakai. Jika IPAYMU_BASE_URL memakai https://sandbox.ipaymu.com, gunakan IPAYMU_VA dan IPAYMU_API_KEY khusus sandbox dari dashboard iPaymu sandbox."
	}
	return message
}

func createIPaymuSignature(method, va, apiKey string, body []byte) string {
	bodyHash := sha256.Sum256(body)
	stringToSign := strings.ToUpper(method) + ":" + strings.TrimSpace(va) + ":" + strings.ToLower(hex.EncodeToString(bodyHash[:])) + ":" + strings.TrimSpace(apiKey)
	mac := hmac.New(sha256.New, []byte(apiKey))
	mac.Write([]byte(stringToSign))
	return hex.EncodeToString(mac.Sum(nil))
}

func marshalIPaymuPayload(payload ipaymuPaymentPayload) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(payload); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(buffer.Bytes()), nil
}

func checkoutWebhookToken(referenceID, apiKey string) string {
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(apiKey)))
	mac.Write([]byte(strings.TrimSpace(referenceID)))
	return hex.EncodeToString(mac.Sum(nil))
}

func verifyCheckoutWebhookToken(referenceID, token string) bool {
	apiKey := strings.TrimSpace(os.Getenv("IPAYMU_API_KEY"))
	if apiKey == "" {
		return false
	}
	expected := checkoutWebhookToken(referenceID, apiKey)
	return hmac.Equal([]byte(expected), []byte(strings.TrimSpace(token)))
}

func (a *AppContext) provisionPackageCheckout(referenceID string) error {
	return a.DB.Transaction(func(tx *gorm.DB) error {
		var order models.PackageCheckoutOrder
		if err := tx.Where("reference_id = ?", referenceID).First(&order).Error; err != nil {
			return err
		}
		if order.SchoolID != nil && order.AdminUserID != nil {
			return nil
		}

		var pkg models.Package
		if err := tx.Where("id = ?", order.PackageID).First(&pkg).Error; err != nil {
			return err
		}

		school := schoolFromPackageModules(order.SchoolName, pkg.Modules)
		if err := tx.Create(&school).Error; err != nil {
			return err
		}

		password, err := generateCheckoutPassword()
		if err != nil {
			return err
		}
		username := uniqueAdminUsername(tx, order.SchoolName)
		hash, err := bcrypt.GenerateFromPassword([]byte(password), 8)
		if err != nil {
			return err
		}
		fullName := "Admin " + order.SchoolName
		admin := models.User{
			FullName:    &fullName,
			Username:    username,
			Password:    string(hash),
			Role:        "ADMIN",
			SchoolID:    &school.ID,
			ParentEmail: &order.Email,
		}
		if err := tx.Create(&admin).Error; err != nil {
			return err
		}

		now := jakartaNow()
		if err := tx.Model(&models.PackageCheckoutOrder{}).Where("id = ?", order.ID).Updates(map[string]interface{}{
			"school_id":     school.ID,
			"admin_user_id": admin.ID,
			"status":        "PAID",
			"paid_at":       coalesceTimePtr(order.PaidAt, &now),
			"updated_at":    now,
		}).Error; err != nil {
			return err
		}

		if err := sendPackageCredentialEmail(order, username, password); err == nil {
			sentAt := jakartaNow()
			_ = tx.Model(&models.PackageCheckoutOrder{}).Where("id = ?", order.ID).Updates(map[string]interface{}{
				"credential_sent_at": &sentAt,
				"updated_at":         sentAt,
			}).Error
		}

		return nil
	})
}

func schoolFromPackageModules(name string, modules models.PackageModules) models.School {
	flags := map[string]bool{}
	for _, module := range modules {
		if !module.Included {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(module.Label))
		flags[key] = true
	}
	included := func(keys ...string) bool {
		for _, key := range keys {
			if flags[strings.ToLower(key)] {
				return true
			}
		}
		return false
	}
	return models.School{
		Name:                           strings.TrimSpace(name),
		InventoryModuleEnabled:         included("sarpras"),
		AttendanceModuleEnabled:        included("absensi siswa", "absensi siswa & guru"),
		AttendanceTeacherModuleEnabled: included("absensi guru", "absensi siswa & guru"),
		OfficialExamModuleEnabled:      included("ujian resmi"),
		KoperasiModuleEnabled:          included("koperasi"),
		PrivateChatModuleEnabled:       included("chat pribadi"),
		TeachingModuleAIEnabled:        included("modul ajar ai"),
		PayrollModuleEnabled:           included("payroll"),
		SPMBModuleEnabled:              included("spmb"),
		AttendanceSeatMapColumns:       4,
	}
}

func sendPackageInvoiceEmail(order models.PackageCheckoutOrder) error {
	subject := fmt.Sprintf("Invoice Pembayaran Paket %s - %s", order.PackageName, order.ReferenceID)
	text := fmt.Sprintf("Halo %s,\n\nInvoice pembayaran paket %s telah dibuat.\nNomor Invoice: %s\nNominal: Rp %s\n\nSilakan lanjutkan pembayaran melalui link berikut:\n%s\n\nAkun admin akan dikirim setelah pembayaran berhasil dikonfirmasi.\n\nBitwize Digital Platform",
		order.SchoolName, order.PackageName, order.ReferenceID, formatRupiah(order.Amount), stringValue(order.PaymentURL))
	htmlBody := fmt.Sprintf(`<p>Halo <strong>%s</strong>,</p>
<p>Invoice pembayaran paket <strong>%s</strong> telah dibuat.</p>
<table cellpadding="6" cellspacing="0" style="font-family:Arial,sans-serif;font-size:14px">
<tr><td>Nomor Invoice</td><td><strong>%s</strong></td></tr>
<tr><td>Nominal</td><td><strong>Rp %s</strong></td></tr>
</table>
<p>Silakan lanjutkan pembayaran melalui link berikut:</p>
<p><a href="%s" style="display:inline-block;background:#f8bd24;color:#15324b;padding:12px 18px;border-radius:10px;font-weight:700;text-decoration:none">Bayar Invoice</a></p>
<p>Akun admin akan dikirim setelah pembayaran berhasil dikonfirmasi.</p>
<p>Bitwize Digital Platform</p>`,
		html.EscapeString(order.SchoolName), html.EscapeString(order.PackageName), html.EscapeString(order.ReferenceID), formatRupiah(order.Amount), html.EscapeString(stringValue(order.PaymentURL)))
	return sendTransactionalEmail(order.Email, subject, text, htmlBody)
}

func sendPackageCredentialEmail(order models.PackageCheckoutOrder, username, password string) error {
	loginURL := strings.TrimRight(envOrDefault("PUBLIC_APP_URL", "https://lms.idschoolsystem.com"), "/") + "/auth/login"
	subject := "Akun Admin Bitwize Digital Platform"
	text := fmt.Sprintf("Halo %s,\n\nPembayaran invoice %s sudah berhasil dikonfirmasi.\n\nAkun admin sekolah Anda:\nUsername: %s\nPassword: %s\nLogin: %s\n\nSegera login dan ubah password setelah masuk.\n\nBitwize Digital Platform",
		order.SchoolName, order.ReferenceID, username, password, loginURL)
	htmlBody := fmt.Sprintf(`<p>Halo <strong>%s</strong>,</p>
<p>Pembayaran invoice <strong>%s</strong> sudah berhasil dikonfirmasi.</p>
<p>Akun admin sekolah Anda:</p>
<table cellpadding="6" cellspacing="0" style="font-family:Arial,sans-serif;font-size:14px">
<tr><td>Username</td><td><strong>%s</strong></td></tr>
<tr><td>Password</td><td><strong>%s</strong></td></tr>
</table>
<p><a href="%s" style="display:inline-block;background:#123a62;color:#ffffff;padding:12px 18px;border-radius:10px;font-weight:700;text-decoration:none">Login Sistem</a></p>
<p>Segera login dan ubah password setelah masuk.</p>
<p>Bitwize Digital Platform</p>`,
		html.EscapeString(order.SchoolName), html.EscapeString(order.ReferenceID), html.EscapeString(username), html.EscapeString(password), html.EscapeString(loginURL))
	return sendTransactionalEmail(order.Email, subject, text, htmlBody)
}

func sendTransactionalEmail(to, subject, textBody, htmlBody string) error {
	msg := services.OutboundEmail{To: to, Subject: subject, TextBody: textBody, HTMLBody: htmlBody}
	if cfg, err := services.LoadBrevoEmailConfigFromEnv(); err == nil {
		return services.SendBrevoEmail(cfg, msg)
	}
	if cfg, err := services.LoadGmailSMTPConfigFromEnv(); err == nil {
		return services.SendGmailSMTPEmail(cfg, msg)
	}
	return fmt.Errorf("konfigurasi email belum tersedia")
}

func normalizeIPaymuStatus(status string) string {
	raw := strings.ToLower(strings.TrimSpace(status))
	switch raw {
	case "1", "paid", "berhasil", "success", "settlement", "settled", "completed", "sukses":
		return "PAID"
	case "0", "pending", "process", "processing":
		return "PENDING"
	case "2", "expired", "expire", "cancel", "cancelled", "canceled", "failed", "gagal":
		return strings.ToUpper(raw)
	default:
		if raw == "" {
			return "PENDING"
		}
		return strings.ToUpper(raw)
	}
}

func uniqueAdminUsername(db *gorm.DB, schoolName string) string {
	base := slugUsername(schoolName)
	if base == "" {
		base = "admin"
	}
	candidate := base
	for i := 0; i < 100; i++ {
		var count int64
		db.Table("users").Where("LOWER(username) = LOWER(?)", candidate).Count(&count)
		if count == 0 {
			return candidate
		}
		candidate = fmt.Sprintf("%s%d", base, i+1)
	}
	return fmt.Sprintf("%s%d", base, time.Now().Unix())
}

func slugUsername(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	re := regexp.MustCompile(`[^a-z0-9]+`)
	value = strings.Trim(re.ReplaceAllString(value, ""), "")
	if len(value) > 24 {
		value = value[:24]
	}
	if value == "" {
		return ""
	}
	return value
}

func generateCheckoutPassword() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, len(buf))
	for i, b := range buf {
		out[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(out), nil
}

func formatRupiah(value int64) string {
	raw := strconv.FormatInt(value, 10)
	n := len(raw)
	if n <= 3 {
		return raw
	}
	var parts []string
	for n > 3 {
		parts = append([]string{raw[n-3:]}, parts...)
		raw = raw[:n-3]
		n = len(raw)
	}
	parts = append([]string{raw}, parts...)
	return strings.Join(parts, ".")
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func coalesceTimePtr(values ...*time.Time) *time.Time {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func appendLandingQuery(rawURL, key, value string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	query := parsed.Query()
	query.Set(key, value)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
