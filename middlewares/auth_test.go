package middlewares

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

func TestRoleAllowed(t *testing.T) {
	app := fiber.New()
	app.Get("/ok", func(c *fiber.Ctx) error {
		c.Locals("userRole", "ADMIN")
		return c.Next()
	}, RoleAllowed("ADMIN"), func(c *fiber.Ctx) error {
		return c.SendStatus(204)
	})
	app.Get("/forbidden", func(c *fiber.Ctx) error {
		c.Locals("userRole", "SISWA")
		return c.Next()
	}, RoleAllowed("ADMIN"), func(c *fiber.Ctx) error {
		return c.SendStatus(204)
	})

	req := httptest.NewRequest("GET", "/ok", nil)
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test /ok error: %v", err)
	}
	if res.StatusCode != 204 {
		t.Fatalf("/ok status = %d, want 204", res.StatusCode)
	}

	req = httptest.NewRequest("GET", "/forbidden", nil)
	res, err = app.Test(req)
	if err != nil {
		t.Fatalf("app.Test /forbidden error: %v", err)
	}
	if res.StatusCode != 403 {
		t.Fatalf("/forbidden status = %d, want 403", res.StatusCode)
	}
}

func TestExtractClaims(t *testing.T) {
	app := fiber.New()
	app.Get("/claims", func(c *fiber.Ctx) error {
		c.Locals("user", &jwt.Token{Claims: jwt.MapClaims{
			"id":       float64(10),
			"schoolId": float64(5),
			"role":     "GURU",
		}})
		return c.Next()
	}, ExtractClaims(), func(c *fiber.Ctx) error {
		if c.Locals("userID").(uint) != 10 {
			t.Fatalf("userID mismatch")
		}
		if c.Locals("schoolID").(uint) != 5 {
			t.Fatalf("schoolID mismatch")
		}
		if c.Locals("userRole").(string) != "GURU" {
			t.Fatalf("userRole mismatch")
		}
		return c.SendStatus(204)
	})

	req := httptest.NewRequest("GET", "/claims", nil)
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test /claims error: %v", err)
	}
	if res.StatusCode != 204 {
		t.Fatalf("/claims status = %d, want 204", res.StatusCode)
	}
}
