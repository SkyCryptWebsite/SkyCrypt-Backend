package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"skycrypt/src"
	"skycrypt/src/forensics"
	"skycrypt/src/utility"
	"strconv"
	"strings"
	"time"

	_ "skycrypt/docs"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

// @title			SkyCrypt API
// @version		3.0
// @description	SkyCrypt API provides Hypixel SkyBlock profile data, player statistics, networth calculations, inventory views, rendered Minecraft item assets, resource pack metadata, emoji data, and source/license information used by SkyCrypt clients.
// @servers.url		https://sky.shiiyu.moe/
// @servers.description	SkyCrypt production API

// @securityDefinitions.apikey ApiTokenHeader
// @in header
// @name X-API-Token
func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("[SKYCRYPT] No .env file found, relying on environment variables")
	}

	if utility.IsForensicsEnabled() {
		// ========== FORENSIC LOGGING INIT (MUST BE FIRST) ==========
		forensics.InitLogger()
		defer forensics.Sync()

		forensics.InitErrorTracker()
		forensics.InitCriticalPathAnalyzer()
		forensics.InitNPlus1Detector()

		forensics.Logger.Info("application_starting",
			zap.String("phase", "init"),
		)

		runtimeMonitor := forensics.NewRuntimeMonitor()
		go runtimeMonitor.Start()
		go forensics.GlobalErrorTracker.StartPeriodicSummary()
		go forensics.GlobalCPAnalyzer.StartPeriodicReport()
		go forensics.GlobalNPlus1Detector.CleanupOldPatterns()
		go forensics.LogCacheStatsPeriodically(nil)
		// ========== END FORENSIC LOGGING INIT ==========
	}

	err = src.SetupApplication()
	if err != nil {
		if utility.IsForensicsEnabled() {
			forensics.PrintFatal(fmt.Sprintf("Application setup failed: %v", err), zap.Error(err))
		}

		panic(err)
	}

	app := fiber.New(fiber.Config{
		Prefork:                   preforkEnabled(), // Requires --pid=host in Docker when enabled
		ServerHeader:              "",               // Remove server header for slight perf gain
		DisableKeepalive:          false,            // Keep connections alive
		DisableDefaultDate:        true,             // Disable date header
		DisableDefaultContentType: false,
		BodyLimit:                 10 << 20, // 10MB
		ReadBufferSize:            4096,
		WriteBufferSize:           4096,
		ReadTimeout:               15 * time.Second,
		WriteTimeout:              30 * time.Second,
		IdleTimeout:               120 * time.Second,
	})

	// ========== FORENSIC REQUEST TRACING (MUST BE BEFORE RECOVER) ==========
	if utility.IsForensicsEnabled() {
		app.Use(forensics.RequestTracingMiddleware())
	}

	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c *fiber.Ctx, err any) {
			stack := debug.Stack()
			fmt.Printf("\033[31m\n========== FATAL PANIC ==========\nPANIC: %v\n\nSTACK TRACE:\n%s\n==================================\033[0m\n", err, stack)

			if utility.IsForensicsEnabled() {
				forensics.Logger.Error("panic_recovered",
					zap.Any("error", err),
					zap.String("url", c.OriginalURL()),
					zap.String("method", c.Method()),
					zap.String("stack_trace", string(stack)),
				)
			}

			utility.SendWebhook(c.OriginalURL(), err, stack)

			_ = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":  "Internal Server Error",
				"type":   fmt.Sprintf("%T", err),
				"detail": fmt.Sprintf("%v", err),
			})
		},
	}))
	app.Use(cors.New())

	src.SetupRoutes(app)

	if err := app.Listen(":8080"); err != nil {
		panic(err)
	}
}

func preforkEnabled() bool {
	if value, ok := os.LookupEnv("SKYCRYPT_PREFORK"); ok {
		enabled, err := strconv.ParseBool(strings.TrimSpace(value))
		if err == nil {
			return enabled
		}
		fmt.Printf("[SKYCRYPT] invalid SKYCRYPT_PREFORK value %q; falling back to DEV-based default\n", value)
	}

	return os.Getenv("DEV") != "true"
}
