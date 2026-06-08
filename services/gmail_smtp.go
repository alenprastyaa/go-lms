package services

import (
	"bytes"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"net/smtp"
	"os"
	"strconv"
	"strings"
)

type GmailSMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	FromName string
}

type OutboundEmail struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string
	ReplyTo  string
}

func LoadGmailSMTPConfigFromEnv() (GmailSMTPConfig, error) {
	host := strings.TrimSpace(os.Getenv("GMAIL_SMTP_HOST"))
	if host == "" {
		host = "smtp.gmail.com"
	}

	port := 587
	if rawPort := strings.TrimSpace(os.Getenv("GMAIL_SMTP_PORT")); rawPort != "" {
		parsedPort, err := strconv.Atoi(rawPort)
		if err != nil || parsedPort <= 0 {
			return GmailSMTPConfig{}, errors.New("GMAIL_SMTP_PORT tidak valid")
		}
		port = parsedPort
	}

	cfg := GmailSMTPConfig{
		Host:     host,
		Port:     port,
		Username: firstNonBlankEnv("GMAIL_SMTP_USER", "EMAIL_USER"),
		Password: normalizeGmailAppPassword(firstNonBlankEnv("GMAIL_SMTP_APP_PASSWORD", "EMAIL_PASS")),
		FromName: strings.TrimSpace(os.Getenv("GMAIL_SMTP_FROM_NAME")),
	}

	if cfg.Username == "" || cfg.Password == "" {
		return GmailSMTPConfig{}, errors.New("GMAIL_SMTP_USER dan GMAIL_SMTP_APP_PASSWORD wajib diisi")
	}
	if cfg.FromName == "" {
		cfg.FromName = "School System"
	}

	return cfg, nil
}

func firstNonBlankEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func normalizeGmailAppPassword(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), " ", "")
}

func SendGmailSMTPEmail(cfg GmailSMTPConfig, msg OutboundEmail) error {
	to := strings.TrimSpace(msg.To)
	if _, err := mail.ParseAddress(to); err != nil {
		return fmt.Errorf("email tujuan tidak valid: %w", err)
	}
	if strings.TrimSpace(msg.Subject) == "" {
		return errors.New("subject email wajib diisi")
	}
	if strings.TrimSpace(msg.TextBody) == "" && strings.TrimSpace(msg.HTMLBody) == "" {
		return errors.New("isi email wajib diisi")
	}

	from := mail.Address{Name: cfg.FromName, Address: cfg.Username}
	headers := map[string]string{
		"From":         from.String(),
		"To":           to,
		"Subject":      mime.QEncoding.Encode("utf-8", strings.TrimSpace(msg.Subject)),
		"MIME-Version": "1.0",
	}
	if replyTo := strings.TrimSpace(msg.ReplyTo); replyTo != "" {
		if _, err := mail.ParseAddress(replyTo); err != nil {
			return fmt.Errorf("reply-to tidak valid: %w", err)
		}
		headers["Reply-To"] = replyTo
	}

	var body bytes.Buffer
	for _, key := range []string{"From", "To", "Subject", "MIME-Version", "Reply-To"} {
		if value := headers[key]; value != "" {
			body.WriteString(key)
			body.WriteString(": ")
			body.WriteString(value)
			body.WriteString("\r\n")
		}
	}

	if strings.TrimSpace(msg.HTMLBody) != "" {
		body.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		body.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
		body.WriteString("\r\n")
		body.WriteString(toQuotedPrintable(msg.HTMLBody))
	} else {
		body.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		body.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
		body.WriteString("\r\n")
		body.WriteString(toQuotedPrintable(msg.TextBody))
	}

	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	return smtp.SendMail(addr, auth, cfg.Username, []string{to}, body.Bytes())
}

func toQuotedPrintable(value string) string {
	var buffer bytes.Buffer
	writer := quotedprintable.NewWriter(&buffer)
	_, _ = writer.Write([]byte(value))
	_ = writer.Close()
	return buffer.String()
}
