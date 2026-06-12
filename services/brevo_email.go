package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"strings"
	"time"
)

type BrevoEmailConfig struct {
	APIKey       string
	APIURL       string
	SenderName   string
	SenderEmail  string
	NoReplyEmail string
}

type brevoEmailAddress struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email"`
}

type brevoAttachment struct {
	Content string `json:"content"`
	Name    string `json:"name"`
}

type brevoSendEmailRequest struct {
	Sender      brevoEmailAddress   `json:"sender"`
	To          []brevoEmailAddress `json:"to"`
	Subject     string              `json:"subject"`
	HTMLContent string              `json:"htmlContent,omitempty"`
	TextContent string              `json:"textContent,omitempty"`
	ReplyTo     *brevoEmailAddress  `json:"replyTo,omitempty"`
	Headers     map[string]string   `json:"headers,omitempty"`
	Attachment  []brevoAttachment   `json:"attachment,omitempty"`
}

func LoadBrevoEmailConfigFromEnv() (BrevoEmailConfig, error) {
	cfg := BrevoEmailConfig{
		APIKey:       strings.TrimSpace(os.Getenv("BREVO_API_KEY")),
		APIURL:       strings.TrimSpace(os.Getenv("BREVO_API_URL")),
		SenderName:   strings.TrimSpace(os.Getenv("BREVO_SENDER_NAME")),
		SenderEmail:  strings.TrimSpace(os.Getenv("BREVO_SENDER_EMAIL")),
		NoReplyEmail: strings.TrimSpace(os.Getenv("BREVO_NO_REPLY_EMAIL")),
	}
	if cfg.APIURL == "" {
		cfg.APIURL = "https://api.brevo.com/v3/smtp/email"
	}
	if cfg.SenderName == "" {
		cfg.SenderName = "Bitwize Digital Platform"
	}
	if cfg.APIKey == "" || cfg.SenderEmail == "" {
		return BrevoEmailConfig{}, errors.New("BREVO_API_KEY dan BREVO_SENDER_EMAIL wajib diisi")
	}
	if _, err := mail.ParseAddress(cfg.SenderEmail); err != nil {
		return BrevoEmailConfig{}, fmt.Errorf("BREVO_SENDER_EMAIL tidak valid: %w", err)
	}
	if cfg.NoReplyEmail == "" {
		if at := strings.LastIndex(cfg.SenderEmail, "@"); at >= 0 && at < len(cfg.SenderEmail)-1 {
			cfg.NoReplyEmail = "no-reply@" + cfg.SenderEmail[at+1:]
		} else {
			cfg.NoReplyEmail = cfg.SenderEmail
		}
	}
	if _, err := mail.ParseAddress(cfg.NoReplyEmail); err != nil {
		return BrevoEmailConfig{}, fmt.Errorf("BREVO_NO_REPLY_EMAIL tidak valid: %w", err)
	}

	return cfg, nil
}

func SendBrevoEmail(cfg BrevoEmailConfig, msg OutboundEmail) error {
	to := strings.TrimSpace(msg.To)
	if _, err := mail.ParseAddress(to); err != nil {
		return fmt.Errorf("email tujuan tidak valid: %w", err)
	}
	subject := strings.TrimSpace(msg.Subject)
	if subject == "" {
		return errors.New("subject email wajib diisi")
	}
	if strings.TrimSpace(msg.TextBody) == "" && strings.TrimSpace(msg.HTMLBody) == "" {
		return errors.New("isi email wajib diisi")
	}

	senderEmail := strings.TrimSpace(msg.FromEmail)
	if senderEmail == "" {
		senderEmail = cfg.SenderEmail
	}
	if _, err := mail.ParseAddress(senderEmail); err != nil {
		return fmt.Errorf("sender email tidak valid: %w", err)
	}

	payload := brevoSendEmailRequest{
		Sender: brevoEmailAddress{
			Name:  cfg.SenderName,
			Email: senderEmail,
		},
		To: []brevoEmailAddress{
			{Email: to},
		},
		Subject: subject,
	}
	if htmlBody := strings.TrimSpace(msg.HTMLBody); htmlBody != "" {
		payload.HTMLContent = msg.HTMLBody
	} else {
		payload.TextContent = msg.TextBody
	}
	if replyTo := strings.TrimSpace(msg.ReplyTo); replyTo != "" {
		if _, err := mail.ParseAddress(replyTo); err != nil {
			return fmt.Errorf("reply-to tidak valid: %w", err)
		}
		payload.ReplyTo = &brevoEmailAddress{Email: replyTo}
	}
	if len(msg.Headers) > 0 {
		payload.Headers = msg.Headers
	}
	for _, att := range msg.Attachments {
		if len(att.Content) == 0 || strings.TrimSpace(att.Filename) == "" {
			continue
		}
		payload.Attachment = append(payload.Attachment, brevoAttachment{
			Name:    att.Filename,
			Content: base64.StdEncoding.EncodeToString(att.Content),
		})
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("gagal membuat payload Brevo: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, cfg.APIURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("gagal membuat request Brevo: %w", err)
	}
	req.Header.Set("accept", "application/json")
	req.Header.Set("api-key", cfg.APIKey)
	req.Header.Set("content-type", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("gagal mengirim email via Brevo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	message := strings.TrimSpace(string(responseBody))
	if message == "" {
		message = resp.Status
	}
	return fmt.Errorf("Brevo mengembalikan status %d: %s", resp.StatusCode, message)
}

func HasBrevoTransactionalEvent(cfg BrevoEmailConfig, email string) (bool, error) {
	parsed, err := mail.ParseAddress(strings.TrimSpace(email))
	if err != nil || parsed.Address == "" {
		return false, fmt.Errorf("email tidak valid: %w", err)
	}

	eventsURL := strings.TrimSpace(os.Getenv("BREVO_EVENTS_API_URL"))
	if eventsURL == "" {
		eventsURL = "https://api.brevo.com/v3/smtp/statistics/events"
	}
	parsedURL, err := url.Parse(eventsURL)
	if err != nil {
		return false, fmt.Errorf("BREVO_EVENTS_API_URL tidak valid: %w", err)
	}
	query := parsedURL.Query()
	query.Set("email", parsed.Address)
	query.Set("limit", "1")
	query.Set("offset", "0")
	parsedURL.RawQuery = query.Encode()

	req, err := http.NewRequest(http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return false, fmt.Errorf("gagal membuat request Brevo events: %w", err)
	}
	req.Header.Set("accept", "application/json")
	req.Header.Set("api-key", cfg.APIKey)

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("gagal mengecek history Brevo: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = resp.Status
		}
		return false, fmt.Errorf("Brevo events mengembalikan status %d: %s", resp.StatusCode, message)
	}

	var payload struct {
		Events []json.RawMessage `json:"events"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		return len(payload.Events) > 0, nil
	}

	var list []json.RawMessage
	if err := json.Unmarshal(body, &list); err == nil {
		return len(list) > 0, nil
	}

	return strings.Contains(strings.ToLower(string(body)), strings.ToLower(parsed.Address)), nil
}
