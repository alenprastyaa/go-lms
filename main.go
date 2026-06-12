package main

import (
	"log"
	"os"
	"strings"

	"lms/config"
	"lms/controllers"
	"lms/realtime"
	"lms/routes"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func main() {
	config.LoadEnv()

	db, err := config.NewDatabase()
	if err != nil {
		log.Fatal(err)
	}

	app := fiber.New(fiber.Config{
		BodyLimit: 25 * 1024 * 1024,
	})

	allowedOrigins := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if allowedOrigins == "" {
		allowedOrigins = strings.Join([]string{
			"https://school-system.my.id",
			"https://lms.school-system.my.id",
			"https://lms.idschoolsystem.com",
			"https://demo.school-system.my.id",
			"https://demo.idschoolsystem.com",
			"https://alentest.my.id",
			"http://localhost:8080",
			"http://localhost:8081",
			"http://localhost:8082",
			"http://localhost:8083",
			"http://localhost:8084",
			"http://localhost:5173",
			"http://127.0.0.1:8080",
			"http://127.0.0.1:8081",
			"http://127.0.0.1:8082",
			"http://127.0.0.1:8083",
			"http://127.0.0.1:8084",
			"http://127.0.0.1:5173",
		}, ",")
	}

	app.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization,X-Requested-With",
		AllowCredentials: true,
	}))
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed,
		Next: func(c *fiber.Ctx) bool {
			return c.Path() == "/api/realtime/events"
		},
	}))
	app.Static("/uploads", "./uploads")

	realtimeHub := realtime.NewHub(db)
	app.Get("/api/realtime/events", realtimeHub.FiberHandler)

	appContext := &controllers.AppContext{DB: db, Realtime: realtimeHub}
	controllers.StartParentWhatsAppReportScheduler(appContext)

	routes.Register(app, db, realtimeHub)

	port := os.Getenv("PORT")
	if port == "" {
		port = "7777"
	}
	log.Fatal(app.Listen(":" + port))
}
