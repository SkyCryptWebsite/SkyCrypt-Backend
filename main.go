package main

import (
	"fmt"
	"runtime/debug"
	"skycrypt/src"
	"skycrypt/src/utility"

	_ "skycrypt/docs"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

// @title SkyCrypt API
// @version 1.0
// @description API for SkyCrypt - A Hypixel SkyBlock Stats Viewer
// @host localhost:8080
// @BasePath /
func main() {
	app := fiber.New(fiber.Config{
		Prefork:                   true,  // Enable prefork (requires --pid=host in Docker)
		ServerHeader:              "",    // Remove server header for slight perf gain
		DisableKeepalive:          false, // Keep connections alive
		DisableDefaultDate:        true,  // Disable date header
		DisableDefaultContentType: false,
		BodyLimit:                 10 << 20, // 10MB
		ReadBufferSize:            4096,
		WriteBufferSize:           4096,
		ReadTimeout:               0, // No timeout for max throughput
		WriteTimeout:              0,
		IdleTimeout:               0,
	})

	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c *fiber.Ctx, err interface{}) {
			stack := debug.Stack()
			fmt.Printf("\033[31m\n========== FATAL PANIC ==========\nPANIC: %v\n\nSTACK TRACE:\n%s\n==================================\033[0m\n", err, stack)

			utility.SendWebhook(c.OriginalURL(), err, stack)

			// TODO: Figure out why this doesn't work
			// Return JSON error to the client
			_ = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":  "Internal Server Error",
				"type":   fmt.Sprintf("%T", err),
				"detail": fmt.Sprintf("%v", err),
			})
		},
	}))
	app.Use(cors.New())

	err := src.SetupApplication()
	if err != nil {
		panic(err)
	}

	src.SetupRoutes(app)

	if err := app.Listen(":8080"); err != nil {
		panic(err)
	}
}
