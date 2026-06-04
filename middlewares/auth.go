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
			if userID <= 0 {
				return utils.Error(c, 401, "Unauthorized")
			}

			rawSessionVersion, hasSessionVersion := claims["sessionVersion"]
			if !hasSessionVersion {
				return c.Next()
			}

			sessionVersion, ok := rawSessionVersion.(float64)
			if !ok {
				return c.Next()
			}

			var current struct {
				SessionVersion int64 `gorm:"column:session_version"`
			}
			if err := db.Table("users").
				Select("session_version").
				Where("id = ?", uint(userID)).
				Take(&current).Error; err != nil {
				return utils.Error(c, 401, "Unauthorized")
			}

			if current.SessionVersion != int64(sessionVersion) {
				var sessionDetails struct {
					CurrentSessionDevice  *string    `gorm:"column:current_session_device"`
					CurrentSessionIP      *string    `gorm:"column:current_session_ip"`
					CurrentSessionLoginAt *time.Time `gorm:"column:current_session_login_at"`
				}
				_ = db.Table("users").
					Select("current_session_device, current_session_ip, current_session_login_at").
					Where("id = ?", uint(userID)).
					Take(&sessionDetails).Error

				return c.Status(401).JSON(fiber.Map{
					"success": false,
					"message": "Sesi login Anda telah digantikan oleh login dari perangkat lain",
					"code":    "SESSION_REPLACED",
					"data": fiber.Map{
						"reason":            "SESSION_REPLACED",
						"active_device":     sessionDetails.CurrentSessionDevice,
						"active_ip":         sessionDetails.CurrentSessionIP,
						"active_login_at":   sessionDetails.CurrentSessionLoginAt,
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

		c.Locals("userID", uint(userID))
		c.Locals("schoolID", uint(schoolID))
		c.Locals("userRole", utils.NormalizeRoleName(fmt.Sprint(claims["role"])))
		if username, ok := claims["username"].(string); ok {
			c.Locals("username", username)
		}
		if sessionVersion, ok := claims["sessionVersion"].(float64); ok {
			c.Locals("sessionVersion", int64(sessionVersion))
		}
		return c.Next()
	}
}

func RoleAllowed(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		currentRole, _ := c.Locals("userRole").(string)
		currentRole = utils.NormalizeRoleName(currentRole)
		for _, role := range roles {
			if utils.NormalizeRoleName(role) == currentRole {
				return c.Next()
			}
		}
		return utils.Error(c, 403, "Forbidden: Insufficient privileges")
	}
}

func ModuleAllowed(db *gorm.DB, feature string) fiber.Handler {
	column := ""
	switch feature {
	case "inventory":
		column = "inventory_module_enabled"
	case "attendance":
		column = "attendance_module_enabled"
	case "attendance_teacher":
		column = "attendance_teacher_module_enabled"
	case "official_exam":
		column = "official_exam_module_enabled"
	case "koperasi":
		column = "koperasi_module_enabled"
	case "private_chat":
		column = "private_chat_module_enabled"
	case "teaching_module_ai":
		column = "teaching_module_ai_enabled"
	case "payroll":
		column = "payroll_module_enabled"
	default:
		return func(c *fiber.Ctx) error {
			return utils.Error(c, 500, "Unknown module")
		}
	}

	return func(c *fiber.Ctx) error {
		schoolID, ok := c.Locals("schoolID").(uint)
		if !ok || schoolID == 0 {
			return utils.Error(c, 403, "Forbidden: School context unavailable")
		}

		selectedColumn := column
		if feature == "attendance" {
			currentRole, _ := c.Locals("userRole").(string)
			if utils.NormalizeRoleName(currentRole) == "GURU" {
				selectedColumn = "attendance_teacher_module_enabled"
			}
		}

		var enabled bool
		if err := db.Table("schools").Select(selectedColumn).Where("id = ?", schoolID).Scan(&enabled).Error; err != nil {
			return utils.Error(c, 500, "Gagal memeriksa status modul", err.Error())
		}
		if !enabled {
			return utils.Error(c, 403, "Modul sedang dinonaktifkan")
		}
		return c.Next()
	}
}
