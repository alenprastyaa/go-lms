package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"
)

type BrevoEmailConfig struct {
	APIKey      string
	APIURL      string
	SenderName  string
	SenderEmail string
}

type brevoEmailAddress struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email"`
}

type brevoSendEmailRequest struct {
	Sender      brevoEmailAddress   `json:"sender"`
	To          []brevoEmailAddress `json:"to"`
	Subject     string              `json:"subject"`
	HTMLContent string              `json:"htmlContent,omitempty"`
	TextContent string              `json:"textContent,omitempty"`
	ReplyTo     *brevoEmailAddress  `json:"replyTo,omitempty"`
}

func LoadBrevoEmailConfigFromEnv() (BrevoEmailConfig, error) {
	cfg := BrevoEmailConfig{
		APIKey:      strings.TrimSpace(os.Getenv("BREVO_API_KEY")),
		APIURL:      strings.TrimSpace(os.Getenv("BREVO_API_URL")),
		SenderName:  strings.TrimSpace(os.Getenv("BREVO_SENDER_NAME")),
		SenderEmail: strings.TrimSpace(os.Getenv("BREVO_SENDER_EMAIL")),
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

	payload := brevoSendEmailRequest{
		Sender: brevoEmailAddress{
			Name:  cfg.SenderName,
			Email: cfg.SenderEmail,
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
