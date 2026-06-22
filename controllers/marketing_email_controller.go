package controllers

import (
	"html"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"lms/services"
	"lms/utils"
)

const (
	marketingPricePerStudent = 1700 // Rp per siswa per bulan
	marketingDiscountMonths  = 1    // potongan untuk langganan tahunan (1 bulan gratis)
)

// buildMarketingProposalAttachment generates a personalized proposal PDF for a
// school and wraps it as an email attachment. A nil attachment slice is
// returned when generation fails, so email delivery is never blocked by it.
func buildMarketingProposalAttachment(schoolName string) ([]services.EmailAttachment, error) {
	pdfBytes, err := services.GenerateMarketingProposalPDF(services.ProposalPDFParams{
		SchoolName:      schoolName,
		PricePerStudent: marketingPricePerStudent,
		DiscountMonths:  marketingDiscountMonths,
		GeneratedAt:     time.Now(),
	})
	if err != nil {
		return nil, err
	}
	return []services.EmailAttachment{{
		Filename: services.ProposalPDFFilename(schoolName),
		Content:  pdfBytes,
		MimeType: "application/pdf",
	}}, nil
}

type marketingEmailRecipient struct {
	Email       string `json:"email"`
	SchoolName  string `json:"school_name"`
	ContactName string `json:"contact_name"`
}

type marketingEmailResult struct {
	Email       string `json:"email"`
	SchoolName  string `json:"school_name,omitempty"`
	ContactName string `json:"contact_name,omitempty"`
	Success     bool   `json:"success"`
	Skipped     bool   `json:"skipped,omitempty"`
	Error       string `json:"error,omitempty"`
}

type marketingEmailStatus struct {
	Email       string `json:"email"`
	Sent        bool   `json:"sent"`
	Status      string `json:"status"`
	LastSentAt  string `json:"last_sent_at,omitempty"`
	Source      string `json:"source,omitempty"`
	SchoolName  string `json:"school_name,omitempty"`
	ContactName string `json:"contact_name,omitempty"`
}

func (a *AppContext) GetSuperAdminMarketingEmailStatuses(c *fiber.Ctx) error {
	rawEmails := strings.Split(c.Query("emails"), ",")
	emails := make([]string, 0, len(rawEmails))
	seen := map[string]bool{}
	for _, raw := range rawEmails {
		email, err := normalizeMarketingEmail(raw)
		if err != nil || email == "" || seen[email] {
			continue
		}
		seen[email] = true
		emails = append(emails, email)
	}
	if len(emails) == 0 {
		return utils.Success(c, 200, "Success Get Email Penawaran Status", fiber.Map{"items": []marketingEmailStatus{}})
	}
	if len(emails) > 200 {
		emails = emails[:200]
	}

	statuses := make(map[string]marketingEmailStatus, len(emails))
	for _, email := range emails {
		statuses[email] = marketingEmailStatus{
			Email:  email,
			Sent:   false,
			Status: "Belum dikirim",
		}
	}

	var rows []struct {
		Email       string
		SchoolName  *string
		ContactName *string
		SentAt      time.Time
	}
	if err := a.DB.Raw(`
		SELECT DISTINCT ON (LOWER(email))
			LOWER(email) AS email,
			school_name,
			contact_name,
			sent_at
		FROM marketing_email_offer_logs
		WHERE success = TRUE AND LOWER(email) IN ?
		ORDER BY LOWER(email), sent_at DESC
	`, emails).Scan(&rows).Error; err != nil {
		return utils.Error(c, 500, "Gagal memuat status email penawaran", err.Error())
	}
	for _, row := range rows {
		item := marketingEmailStatus{
			Email:      row.Email,
			Sent:       true,
			Status:     "Penawaran terkirim",
			LastSentAt: row.SentAt.Format(time.RFC3339),
			Source:     "local",
		}
		if row.SchoolName != nil {
			item.SchoolName = *row.SchoolName
		}
		if row.ContactName != nil {
			item.ContactName = *row.ContactName
		}
		statuses[row.Email] = item
	}

	items := make([]marketingEmailStatus, 0, len(emails))
	for _, email := range emails {
		items = append(items, statuses[email])
	}
	return utils.Success(c, 200, "Success Get Email Penawaran Status", fiber.Map{"items": items})
}

func (a *AppContext) SendSuperAdminMarketingEmail(c *fiber.Ctx) error {
	var body struct {
		Recipients []marketingEmailRecipient `json:"recipients"`
		Subject    string                    `json:"subject"`
		Body       string                    `json:"body"`
		ReplyTo    string                    `json:"reply_to"`
		SendAsHTML bool                      `json:"send_as_html"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request")
	}

	subjectTemplate := strings.TrimSpace(body.Subject)
	bodyTemplate := strings.TrimSpace(body.Body)
	if subjectTemplate == "" {
		return utils.Error(c, 400, "Subject email wajib diisi")
	}
	if bodyTemplate == "" {
		return utils.Error(c, 400, "Isi email wajib diisi")
	}

	recipients, validationResults := normalizeMarketingRecipients(body.Recipients)
	if len(recipients) == 0 {
		return utils.ErrorData(c, 400, "Minimal satu email penerima valid wajib diisi", fiber.Map{
			"results": validationResults,
		})
	}
	if len(recipients) > 100 {
		return utils.Error(c, 400, "Maksimal 100 penerima per pengiriman")
	}

	cfg, err := services.LoadBrevoEmailConfigFromEnv()
	if err != nil {
		return utils.Error(c, 500, "Konfigurasi Brevo belum lengkap", err.Error())
	}

	results := make([]marketingEmailResult, 0, len(validationResults)+len(recipients))
	results = append(results, validationResults...)
	successCount := 0
	failedCount := len(validationResults)
	skippedCount := 0

	for _, recipient := range recipients {
		if sent, _ := a.marketingEmailAlreadySent(recipient.Email); sent {
			results = append(results, marketingEmailResult{
				Email:      recipient.Email,
				SchoolName: recipient.SchoolName,
				Skipped:    true,
				Error:      "Penawaran sudah pernah terkirim",
			})
			skippedCount++
			continue
		}

		values := map[string]string{
			"school_name": recipient.SchoolName,
		}

		subject := renderMarketingTemplate(subjectTemplate, values)
		textBody := renderMarketingTemplate(bodyTemplate, values)
		htmlBody := marketingPlainTextToHTML(textBody)
		if body.SendAsHTML {
			htmlBody = renderMarketingTemplate(bodyTemplate, values)
		}

		result := marketingEmailResult{
			Email:       recipient.Email,
			SchoolName:  recipient.SchoolName,
			ContactName: recipient.ContactName,
		}
		attachments, attErr := buildMarketingProposalAttachment(recipient.SchoolName)
		if attErr != nil {
			attachments = nil
		}
		err := services.SendBrevoEmail(cfg, services.OutboundEmail{
			To:          recipient.Email,
			Subject:     subject,
			TextBody:    textBody,
			HTMLBody:    htmlBody,
			ReplyTo:     cfg.NoReplyEmail,
			FromEmail:   cfg.NoReplyEmail,
			Headers:     marketingNoReplyHeaders(cfg.NoReplyEmail),
			Attachments: attachments,
		})
		if err != nil {
			result.Success = false
			result.Error = err.Error()
			failedCount++
		} else {
			result.Success = true
			successCount++
			_ = a.recordMarketingEmailOffer(recipient, true, "")
		}
		results = append(results, result)
	}

	return utils.Success(c, 200, "Pengiriman email penawaran selesai diproses", fiber.Map{
		"total_recipients": len(recipients),
		"success_count":    successCount,
		"failed_count":     failedCount,
		"skipped_count":    skippedCount,
		"results":          results,
		"generated_at":     time.Now().Format(time.RFC3339),
	})
}

func (a *AppContext) SendSuperAdminMarketingEmailTest(c *fiber.Ctx) error {
	var body struct {
		To         string `json:"to"`
		SchoolName string `json:"school_name"`
		Subject    string `json:"subject"`
		Body       string `json:"body"`
		SendAsHTML bool   `json:"send_as_html"`
	}
	if err := c.BodyParser(&body); err != nil {
		return utils.Error(c, 400, "Invalid request")
	}

	to, err := normalizeMarketingEmail(body.To)
	if err != nil || to == "" {
		return utils.Error(c, 400, "Email tujuan test tidak valid")
	}
	subjectTemplate := strings.TrimSpace(body.Subject)
	bodyTemplate := strings.TrimSpace(body.Body)
	if subjectTemplate == "" {
		return utils.Error(c, 400, "Subject email wajib diisi")
	}
	if bodyTemplate == "" {
		return utils.Error(c, 400, "Isi email wajib diisi")
	}

	cfg, err := services.LoadBrevoEmailConfigFromEnv()
	if err != nil {
		return utils.Error(c, 500, "Konfigurasi Brevo belum lengkap", err.Error())
	}

	schoolName := strings.TrimSpace(body.SchoolName)
	if schoolName == "" {
		schoolName = "Nama Sekolah"
	}
	values := map[string]string{"school_name": schoolName}
	subject := "[TEST] " + renderMarketingTemplate(subjectTemplate, values)
	textBody := renderMarketingTemplate(bodyTemplate, values)
	htmlBody := marketingPlainTextToHTML(textBody)
	if body.SendAsHTML {
		htmlBody = renderMarketingTemplate(bodyTemplate, values)
	}

	attachments, attErr := buildMarketingProposalAttachment(schoolName)
	if attErr != nil {
		attachments = nil
	}
	if err := services.SendBrevoEmail(cfg, services.OutboundEmail{
		To:          to,
		Subject:     subject,
		TextBody:    textBody,
		HTMLBody:    htmlBody,
		ReplyTo:     cfg.NoReplyEmail,
		FromEmail:   cfg.NoReplyEmail,
		Headers:     marketingNoReplyHeaders(cfg.NoReplyEmail),
		Attachments: attachments,
	}); err != nil {
		return utils.Error(c, 500, "Gagal mengirim test email", err.Error())
	}

	return utils.Success(c, 200, "Test email berhasil dikirim", fiber.Map{
		"email":          to,
		"school_name":    schoolName,
		"no_reply_email": cfg.NoReplyEmail,
		"generated_at":   time.Now().Format(time.RFC3339),
	})
}

// GetSuperAdminMarketingProposalPDF streams the personalized proposal PDF so
// the operator can preview/download exactly what gets attached to the email.
func (a *AppContext) GetSuperAdminMarketingProposalPDF(c *fiber.Ctx) error {
	schoolName := strings.TrimSpace(c.Query("school_name"))
	if schoolName == "" {
		schoolName = "Nama Sekolah"
	}

	pdfBytes, err := services.GenerateMarketingProposalPDF(services.ProposalPDFParams{
		SchoolName:      schoolName,
		PricePerStudent: marketingPricePerStudent,
		DiscountMonths:  marketingDiscountMonths,
		GeneratedAt:     time.Now(),
	})
	if err != nil {
		return utils.Error(c, 500, "Gagal membuat PDF penawaran", err.Error())
	}

	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", `inline; filename="`+services.ProposalPDFFilename(schoolName)+`"`)
	return c.Send(pdfBytes)
}

func marketingNoReplyHeaders(noReplyEmail string) map[string]string {
	return map[string]string{
		"Auto-Submitted":           "auto-generated",
		"Precedence":               "bulk",
		"X-Auto-Response-Suppress": "All",
		"List-Unsubscribe":         "<mailto:" + noReplyEmail + "?subject=unsubscribe>",
		"List-Unsubscribe-Post":    "List-Unsubscribe=One-Click",
	}
}

func normalizeMarketingEmail(value string) (string, error) {
	parsed, err := mail.ParseAddress(strings.TrimSpace(value))
	if err != nil || parsed.Address == "" {
		return "", err
	}
	return strings.ToLower(parsed.Address), nil
}

func (a *AppContext) marketingEmailAlreadySent(email string) (bool, error) {
	normalized, err := normalizeMarketingEmail(email)
	if err != nil {
		return false, err
	}

	var count int64
	if err := a.DB.Table("marketing_email_offer_logs").Where("success = TRUE AND LOWER(email) = ?", normalized).Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	return false, nil
}

func (a *AppContext) recordMarketingEmailOffer(recipient marketingEmailRecipient, success bool, source string) error {
	email, err := normalizeMarketingEmail(recipient.Email)
	if err != nil || email == "" {
		return err
	}
	if strings.TrimSpace(source) == "" {
		source = "brevo"
	}
	return a.DB.Exec(`
		INSERT INTO marketing_email_offer_logs (email, school_name, contact_name, success, source, sent_at, created_at, updated_at)
		VALUES (?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, NOW(), NOW(), NOW())
	`, email, strings.TrimSpace(recipient.SchoolName), strings.TrimSpace(recipient.ContactName), success, source).Error
}

func normalizeMarketingRecipients(input []marketingEmailRecipient) ([]marketingEmailRecipient, []marketingEmailResult) {
	seen := map[string]bool{}
	valid := []marketingEmailRecipient{}
	invalid := []marketingEmailResult{}

	for _, item := range input {
		email := strings.TrimSpace(item.Email)
		schoolName := strings.TrimSpace(item.SchoolName)
		contactName := strings.TrimSpace(item.ContactName)
		if email == "" {
			continue
		}
		parsed, err := mail.ParseAddress(email)
		if err != nil || parsed.Address == "" {
			invalid = append(invalid, marketingEmailResult{
				Email:       email,
				SchoolName:  schoolName,
				ContactName: contactName,
				Success:     false,
				Error:       "Format email tidak valid",
			})
			continue
		}
		normalizedEmail := strings.ToLower(parsed.Address)
		if seen[normalizedEmail] {
			continue
		}
		seen[normalizedEmail] = true
		valid = append(valid, marketingEmailRecipient{
			Email:       parsed.Address,
			SchoolName:  schoolName,
			ContactName: contactName,
		})
	}

	return valid, invalid
}

func renderMarketingTemplate(template string, values map[string]string) string {
	result := template
	for key, value := range values {
		pattern := regexp.MustCompile(`\{\{\s*` + regexp.QuoteMeta(key) + `\s*\}\}`)
		result = pattern.ReplaceAllString(result, value)
	}
	return result
}

func marketingPlainTextToHTML(value string) string {
	escaped := html.EscapeString(value)
	escaped = strings.ReplaceAll(escaped, "\r\n", "\n")
	escaped = strings.ReplaceAll(escaped, "\r", "\n")
	paragraphs := strings.Split(escaped, "\n\n")
	for index, paragraph := range paragraphs {
		paragraphs[index] = "<p>" + strings.ReplaceAll(paragraph, "\n", "<br>") + "</p>"
	}
	return strings.Join(paragraphs, "\n")
}
