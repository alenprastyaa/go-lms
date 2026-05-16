package utils

import "github.com/gofiber/fiber/v2"

func Success(c *fiber.Ctx, code int, msg string, data interface{}) error {
	resp := fiber.Map{"success": true, "message": msg}
	if data != nil {
		resp["data"] = data
	}
	return c.Status(code).JSON(resp)
}

func Error(c *fiber.Ctx, code int, msg string, errMsg ...string) error {
	resp := fiber.Map{"success": false, "message": msg}
	if len(errMsg) > 0 && errMsg[0] != "" {
		resp["error"] = errMsg[0]
	}
	return c.Status(code).JSON(resp)
}

func ErrorData(c *fiber.Ctx, code int, msg string, data interface{}, errMsg ...string) error {
	resp := fiber.Map{"success": false, "message": msg}
	if data != nil {
		resp["data"] = data
	}
	if len(errMsg) > 0 && errMsg[0] != "" {
		resp["error"] = errMsg[0]
	}
	return c.Status(code).JSON(resp)
}
