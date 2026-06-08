package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendBrevoEmailSendsTransactionalPayload(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("api-key"); got != "test-key" {
			t.Fatalf("api-key header = %q", got)
		}
		if got := r.Header.Get("content-type"); got != "application/json" {
			t.Fatalf("content-type header = %q", got)
		}
		if got := r.Header.Get("accept"); got != "application/json" {
			t.Fatalf("accept header = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"messageId":"test-message-id"}`))
	}))
	defer server.Close()

	err := SendBrevoEmail(BrevoEmailConfig{
		APIKey:      "test-key",
		APIURL:      server.URL,
		SenderName:  "Bitwize Digital Platform",
		SenderEmail: "sender@example.com",
	}, OutboundEmail{
		To:       "school@example.com",
		Subject:  "Penawaran untuk SMK Test",
		TextBody: "Plain text fallback",
		HTMLBody: "<p>Halo SMK Test</p>",
		ReplyTo:  "reply@example.com",
	})
	if err != nil {
		t.Fatalf("SendBrevoEmail returned error: %v", err)
	}

	if captured["subject"] != "Penawaran untuk SMK Test" {
		t.Fatalf("subject = %#v", captured["subject"])
	}
	if captured["htmlContent"] != "<p>Halo SMK Test</p>" {
		t.Fatalf("htmlContent = %#v", captured["htmlContent"])
	}
	if _, ok := captured["textContent"]; ok {
		t.Fatal("textContent should not be sent when htmlContent is available")
	}
}

func TestSendBrevoEmailReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":"invalid_parameter","message":"sender is invalid"}`))
	}))
	defer server.Close()

	err := SendBrevoEmail(BrevoEmailConfig{
		APIKey:      "test-key",
		APIURL:      server.URL,
		SenderName:  "Bitwize Digital Platform",
		SenderEmail: "sender@example.com",
	}, OutboundEmail{
		To:       "school@example.com",
		Subject:  "Penawaran untuk SMK Test",
		HTMLBody: "<p>Halo SMK Test</p>",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Brevo mengembalikan status 400") {
		t.Fatalf("error = %q", err.Error())
	}
}
