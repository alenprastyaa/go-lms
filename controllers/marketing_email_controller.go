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
	Error       string `json:"error,omitempty"`
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

	for _, recipient := range recipients {
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
		err := services.SendBrevoEmail(cfg, services.OutboundEmail{
			To:       recipient.Email,
			Subject:  subject,
			TextBody: textBody,
			HTMLBody: htmlBody,
			ReplyTo:  strings.TrimSpace(body.ReplyTo),
		})
		if err != nil {
			result.Success = false
			result.Error = err.Error()
			failedCount++
		} else {
			result.Success = true
			successCount++
		}
		results = append(results, result)
	}

	return utils.Success(c, 200, "Pengiriman email penawaran selesai diproses", fiber.Map{
		"total_recipients": len(recipients),
		"success_count":    successCount,
		"failed_count":     failedCount,
		"results":          results,
		"generated_at":     time.Now().Format(time.RFC3339),
	})
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
