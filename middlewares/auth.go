package middlewares

import (
	"fmt"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	jwtware "github.com/gofiber/jwt/v3"
	"github.com/golang-jwt/jwt/v4"
	"gorm.io/gorm"
	"lms/utils"
)

func Auth(db *gorm.DB) fiber.Handler {
	return jwtware.New(jwtware.Config{
		SigningKey: []byte(os.Getenv("JWT_SECRET")),
		SuccessHandler: func(c *fiber.Ctx) error {
			tok, ok := c.Locals("user").(*jwt.Token)
			if !ok {
				return utils.Error(c, 401, "Unauthorized")
			}

			claims, ok := tok.Claims.(jwt.MapClaims)
			if !ok {
				return utils.Error(c, 401, "Unauthorized")
			}

			userID, _ := claims["id"].(float64)
			sessionVersion, _ := claims["sessionVersion"].(float64)
			if userID <= 0 {
				return utils.Error(c, 401, "Unauthorized")
			}

			var current struct {
				SessionVersion        int64      `gorm:"column:session_version"`
				CurrentSessionDevice  *string    `gorm:"column:current_session_device"`
				CurrentSessionIP      *string    `gorm:"column:current_session_ip"`
				CurrentSessionLoginAt *time.Time `gorm:"column:current_session_login_at"`
			}
			if err := db.Table("users").
				Select("session_version, current_session_device, current_session_ip, current_session_login_at").
				Where("id = ?", uint(userID)).
				Take(&current).Error; err != nil {
				return utils.Error(c, 401, "Unauthorized")
			}

			if current.SessionVersion != int64(sessionVersion) {
				return c.Status(401).JSON(fiber.Map{
					"success": false,
					"message": "Sesi login Anda telah digantikan oleh login dari perangkat lain",
					"code":    "SESSION_REPLACED",
					"data": fiber.Map{
						"reason":            "SESSION_REPLACED",
						"active_device":     current.CurrentSessionDevice,
						"active_ip":         current.CurrentSessionIP,
						"active_login_at":   current.CurrentSessionLoginAt,
						"forced_logout":     true,
						"should_show_modal": true,
					},
				})
			}

			return c.Next()
		},
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
		sessionVersion, _ := claims["sessionVersion"].(float64)

		c.Locals("userID", uint(userID))
		c.Locals("schoolID", uint(schoolID))
		c.Locals("userRole", fmt.Sprint(claims["role"]))
		c.Locals("sessionVersion", int64(sessionVersion))
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
