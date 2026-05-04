package realtime

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestExtractTokenFromRequest(t *testing.T) {
	req := &http.Request{URL: &url.URL{RawQuery: "token=Bearer%20abc123"}}
	if got := extractTokenFromRequest(req); got != "abc123" {
		t.Fatalf("token = %q", got)
	}
	req = &http.Request{URL: &url.URL{RawQuery: "authToken=bearer%20z9"}}
	if got := extractTokenFromRequest(req); got != "z9" {
		t.Fatalf("authToken = %q", got)
	}
}

func TestAllowedOrigin(t *testing.T) {
	if got := allowedOrigin("http://localhost:5173"); got == "" {
		t.Fatalf("expected allowed origin")
	}
	if got := allowedOrigin("https://evil.test"); got != "" {
		t.Fatalf("unexpected allowed origin: %q", got)
	}
}

func TestNormalizeSSEPayload(t *testing.T) {
	b, err := normalizeSSEPayload(nil)
	if err != nil || string(b) != "null" {
		t.Fatalf("nil payload = %q err=%v", string(b), err)
	}
	b, err = normalizeSSEPayload("hello")
	if err != nil || string(b) != "hello" {
		t.Fatalf("string payload = %q err=%v", string(b), err)
	}
	raw := json.RawMessage(`{"ok":true}`)
	b, err = normalizeSSEPayload(raw)
	if err != nil || string(b) != `{"ok":true}` {
		t.Fatalf("raw payload = %q err=%v", string(b), err)
	}
}

func TestWriteSSEToWriter(t *testing.T) {
	var out bytes.Buffer
	w := bufio.NewWriter(&out)
	err := writeSSEToWriter(w, "evt", map[string]any{"a": 1})
	if err != nil {
		t.Fatalf("writeSSEToWriter error: %v", err)
	}
	_ = w.Flush()
	got := out.String()
	if got == "" || !bytes.Contains([]byte(got), []byte("event: evt")) {
		t.Fatalf("unexpected sse output: %q", got)
	}
}

func TestFirstIntAndAsFloat64(t *testing.T) {
	if got := firstInt([]any{"42"}); got != 42 {
		t.Fatalf("firstInt string = %d", got)
	}
	if got := firstInt([]any{uint64(7)}); got != 7 {
		t.Fatalf("firstInt uint64 = %d", got)
	}
	if got := asFloat64("12.5"); got != 12.5 {
		t.Fatalf("asFloat64 string = %v", got)
	}
	if got := asFloat64(nil); got != 0 {
		t.Fatalf("asFloat64 nil = %v", got)
	}
}

func TestOnlineCountAndSubjectOnlineUsers(t *testing.T) {
	h := NewHub(nil)
	h.subscribers[1] = &subscriber{userID: 1, schoolID: 1, subjects: map[uint]struct{}{10: {}}}
	h.subscribers[2] = &subscriber{userID: 2, schoolID: 1, subjects: map[uint]struct{}{10: {}, 11: {}}}
	h.subscribers[3] = &subscriber{userID: 1, schoolID: 1, subjects: map[uint]struct{}{10: {}}} // same user, another tab
	h.subscribers[4] = &subscriber{userID: 3, schoolID: 2, subjects: map[uint]struct{}{10: {}}}

	if got := h.OnlineCountBySchool(1); got != 2 {
		t.Fatalf("OnlineCountBySchool(1) = %d, want 2", got)
	}
	users := h.SubjectOnlineUsers(1, 10)
	if len(users) != 2 {
		t.Fatalf("SubjectOnlineUsers len = %d, want 2", len(users))
	}
}

func TestSubjectsCache(t *testing.T) {
	h := NewHub(nil)
	h.subjectsCacheTTL = 20 * time.Millisecond
	h.setSubjectsCache("1:1:SISWA", map[uint]struct{}{3: {}})

	cached, ok := h.getSubjectsFromCache("1:1:SISWA")
	if !ok || len(cached) != 1 {
		t.Fatalf("expected cache hit")
	}
	cached[99] = struct{}{}
	cachedAgain, ok := h.getSubjectsFromCache("1:1:SISWA")
	if !ok {
		t.Fatalf("expected cache hit second time")
	}
	if _, exists := cachedAgain[99]; exists {
		t.Fatalf("cache should return copy, not mutable ref")
	}

	time.Sleep(30 * time.Millisecond)
	if _, ok := h.getSubjectsFromCache("1:1:SISWA"); ok {
		t.Fatalf("expected cache miss after ttl")
	}
}

