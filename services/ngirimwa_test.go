package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendWhatsAppMessageSendsNgirimWAPayload(t *testing.T) {
	var captured ngirimWASendMessageRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/messages/send" {
			t.Fatalf("path = %s, want /messages/send", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "test-ngirimwa-key" {
			t.Fatalf("x-api-key = %q, want test-ngirimwa-key", got)
		}
		if got := r.Header.Get("content-type"); got != "application/json" {
			t.Fatalf("content-type = %q, want application/json", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"message":"sent","data":{"id":"wa-test"}}`))
	}))
	defer server.Close()

	t.Setenv("NGIRIMWA_API_KEY", "test-ngirimwa-key")
	t.Setenv("NGIRIMWA_BASE_URL", server.URL)

	resp, err := SendWhatsAppMessage(" 6281234567890 ", " Laporan PDF: https://cdn.example.test/report.pdf ")
	if err != nil {
		t.Fatalf("SendWhatsAppMessage returned error: %v", err)
	}
	if resp == nil || !resp.Success {
		t.Fatalf("response = %#v, want success", resp)
	}
	if captured.To != "6281234567890" {
		t.Fatalf("to = %q, want trimmed phone", captured.To)
	}
	if captured.Message != "Laporan PDF: https://cdn.example.test/report.pdf" {
		t.Fatalf("message = %q, want trimmed message", captured.Message)
	}
}

func TestSendWhatsAppMessageReturnsNgirimWAErrorMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"success":false,"message":"nomor tujuan tidak valid"}`))
	}))
	defer server.Close()

	t.Setenv("NGIRIMWA_API_KEY", "test-ngirimwa-key")
	t.Setenv("NGIRIMWA_BASE_URL", server.URL)

	resp, err := SendWhatsAppMessage("6281234567890", "Laporan")
	if err == nil {
		t.Fatal("expected error")
	}
	if resp == nil {
		t.Fatal("expected parsed response")
	}
	if !strings.Contains(err.Error(), "nomor tujuan tidak valid") {
		t.Fatalf("error = %q, want NgirimWA message", err.Error())
	}
}

func TestSendWhatsAppMessageRequiresAPIKey(t *testing.T) {
	t.Setenv("NGIRIMWA_API_KEY", "")

	resp, err := SendWhatsAppMessage("6281234567890", "Laporan")
	if err == nil {
		t.Fatal("expected error")
	}
	if resp != nil {
		t.Fatalf("response = %#v, want nil", resp)
	}
	if !strings.Contains(err.Error(), "NGIRIMWA_API_KEY belum diatur") {
		t.Fatalf("error = %q, want missing API key message", err.Error())
	}
}
