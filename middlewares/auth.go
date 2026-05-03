package middlewares

import (
	"fmt"
	"os"

	"github.com/gofiber/fiber/v2"
	jwtware "github.com/gofiber/jwt/v3"
	"github.com/golang-jwt/jwt/v4"
	"lms/utils"
)

func Auth() fiber.Handler {
	return jwtware.New(jwtware.Config{
		SigningKey: []byte(os.Getenv("JWT_SECRET")),
		ErrorHandler: func(c *fiber.Ctx, _ error) error {
			return utils.Error(c, 401, "Unauthorized")
		},
	})
}

func ExtractClaims() fiber.Handler {
	return func(c *fiber.Ctx) error {
		tok, ok := c.Locals("user").(*jwt.Token)
		if !ok {
			return utils.Error(c, 401, "Unauthorized")
		}
		claims, ok := tok.Claims.(jwt.MapClaims)
		if !ok {
			return utils.Error(c, 401, "Unauthorized")
		}

		userID, _ := claims["id"].(float64)
		schoolID, _ := claims["schoolId"].(float64)

		c.Locals("userID", uint(userID))
		c.Locals("schoolID", uint(schoolID))
		c.Locals("userRole", fmt.Sprint(claims["role"]))
		return c.Next()
	}
}

func RoleAllowed(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		currentRole, _ := c.Locals("userRole").(string)
		for _, role := range roles {
			if role == currentRole {
				return c.Next()
			}
		}
		return utils.Error(c, 403, "Forbidden: Insufficient privileges")
	}
}
