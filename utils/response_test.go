package utils

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestSuccessAndErrorResponse(t *testing.T) {
	app := fiber.New()
	app.Get("/success", func(c *fiber.Ctx) error {
		return Success(c, 200, "ok", fiber.Map{"x": 1})
	})
	app.Get("/error", func(c *fiber.Ctx) error {
		return Error(c, 400, "bad", "detail")
	})

	res, err := app.Test(httptest.NewRequest("GET", "/success", nil))
	if err != nil {
		t.Fatalf("success request err: %v", err)
	}
	var s map[string]any
	if err := json.NewDecoder(res.Body).Decode(&s); err != nil {
		t.Fatalf("decode success body: %v", err)
	}
	if s["success"] != true || s["message"] != "ok" {
		t.Fatalf("unexpected success response: %#v", s)
	}

	res, err = app.Test(httptest.NewRequest("GET", "/error", nil))
	if err != nil {
		t.Fatalf("error request err: %v", err)
	}
	var e map[string]any
	if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if e["success"] != false || e["message"] != "bad" || e["error"] != "detail" {
		t.Fatalf("unexpected error response: %#v", e)
	}
}

