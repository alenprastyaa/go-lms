package main

import (
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"lms/config"
	"lms/realtime"
	"lms/routes"
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
	app.Use(cors.New(cors.Config{
		AllowOrigins: "https://school-system.my.id,https://alentest.my.id,http://localhost:8080,http://localhost:5173",
		AllowMethods: "GET,POST,PUT,PATCH,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
	}))
	app.Static("/uploads", "./uploads")

	realtimeHub := realtime.NewHub(db)
	app.Get("/api/realtime/events", realtimeHub.FiberHandler)

	routes.Register(app, db, realtimeHub)

	port := os.Getenv("PORT")
	if port == "" {
		port = "7777"
	}
	log.Fatal(app.Listen(":" + port))
}
