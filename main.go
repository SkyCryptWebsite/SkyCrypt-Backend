package main

import (
	"fmt"
	"runtime/debug"
	"skycrypt/src"
	"skycrypt/src/forensics"
	"skycrypt/src/utility"
	"time"

	_ "skycrypt/docs"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"go.uber.org/zap"
)

// @title			SkyCrypt API
// @version		1.0
// @description	API for SkyCrypt - A Hypixel SkyBlock Stats Viewer
// @host			localhost:8080
// @BasePath		/
func main() {
	// ========== FORENSIC LOGGING INIT (MUST BE FIRST) ==========
	forensics.InitLogger()
	defer forensics.Sync()

	forensics.InitErrorTracker()
	forensics.InitCriticalPathAnalyzer()
	forensics.InitNPlus1Detector()

	forensics.Logger.Info("application_starting",
		zap.String("phase", "init"),
	)

	// Start background monitors
	runtimeMonitor := forensics.NewRuntimeMonitor()
	go runtimeMonitor.Start()
	go forensics.GlobalErrorTracker.StartPeriodicSummary()
	go forensics.GlobalCPAnalyzer.StartPeriodicReport()
	go forensics.GlobalNPlus1Detector.CleanupOldPatterns()
	go forensics.LogCacheStatsPeriodically(nil)
	// ========== END FORENSIC LOGGING INIT ==========

	app := fiber.New(fiber.Config{
		Prefork:                   true,  // Enable prefork (requires --pid=host in Docker)
		ServerHeader:              "",    // Remove server header for slight perf gain
		DisableKeepalive:          false, // Keep connections alive
		DisableDefaultDate:        true,  // Disable date header
		DisableDefaultContentType: false,
		BodyLimit:                 10 << 20, // 10MB
		ReadBufferSize:            4096,
		WriteBufferSize:           4096,
		ReadTimeout:               15 * time.Second,
		WriteTimeout:              30 * time.Second,
		IdleTimeout:               120 * time.Second,
	})

	// ========== FORENSIC REQUEST TRACING (BEFORE RECOVER) ==========
	app.Use(forensics.RequestTracingMiddleware())

	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c *fiber.Ctx, err interface{}) {
			stack := debug.Stack()
			fmt.Printf("\033[31m\n========== FATAL PANIC ==========\nPANIC: %v\n\nSTACK TRACE:\n%s\n==================================\033[0m\n", err, stack)

			forensics.Logger.Error("panic_recovered",
				zap.Any("error", err),
				zap.String("url", c.OriginalURL()),
				zap.String("method", c.Method()),
				zap.String("stack_trace", string(stack)),
			)

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
		forensics.PrintFatal(fmt.Sprintf("Application setup failed: %v", err), zap.Error(err))
		panic(err)
	}

	src.SetupRoutes(app)

	forensics.Logger.Info("application_started",
		zap.String("address", ":8080"),
	)

	if err := app.Listen(":8080"); err != nil {
		forensics.PrintFatal(fmt.Sprintf("Listen failed: %v", err), zap.Error(err))
		panic(err)
	}
}
