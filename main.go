package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"skycrypt/src"
	"skycrypt/src/db"
	"skycrypt/src/forensics"
	"skycrypt/src/security"
	"skycrypt/src/utility"
	"strconv"
	"strings"
	"syscall"
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
// @description	SkyCrypt API provides Hypixel SkyBlock profile data, player statistics, networth calculations, inventory views, rendered Minecraft item assets, resource pack metadata, and source/license information used by SkyCrypt clients.
// @description
// @description	Profile endpoints need a SkyCrypt API key in the `x-api-key` header. Request one from the developer page in your SkyCrypt dashboard.
// @description	Each key has its own rate limit; responses carry `X-RateLimit-Limit`, `X-RateLimit-Remaining` and `X-RateLimit-Reset` headers.
// @description	Keys only get players that SkyCrypt has already cached, as Hypixel's API policy doesn't allow proxying it. Other players return `404` with code `NOT_CACHED`.
// @servers.url		https://api.shiiyu.moe
// @servers.description	Production
// @servers.url		https://beta-api.shiiyu.moe
// @servers.description	Beta, built from the dev branch

// @securityDefinitions.apikey ApiKeyHeader
// @in header
// @name x-api-key
// @description SkyCrypt API key, starting with `skycrypt_`.
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
		go forensics.StartResourceMonitor()
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

	prefork := preforkEnabled()
	app := fiber.New(fiber.Config{
		Prefork:                   prefork, // Requires --pid=host in Docker when enabled
		ServerHeader:              "",      // Remove server header for slight perf gain
		DisableKeepalive:          false,   // Keep connections alive
		DisableDefaultDate:        true,    // Disable date header
		DisableDefaultContentType: false,
		BodyLimit:                 10 << 20, // 10MB
		ReadBufferSize:            32 << 10, // Caps request header size; the frontend forwards browser cookies, so match Cloudflare's 32KB
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
			panicDetail := security.RedactString(fmt.Sprintf("%v", err))
			redactedStack := security.RedactString(string(stack))
			redactedURL := forensics.RedactURL(c.OriginalURL())
			fmt.Printf("\033[31m\n========== FATAL PANIC ==========\nPANIC: %s\n\nSTACK TRACE:\n%s\n==================================\033[0m\n", panicDetail, redactedStack)

			if utility.IsForensicsEnabled() {
				forensics.Logger.Error("panic_recovered",
					zap.String("error", panicDetail),
					zap.String("url", redactedURL),
					zap.String("method", c.Method()),
					zap.String("stack_trace", redactedStack),
				)
			}

			// Reported in the background so the 500 doesn't wait on Discord.
			go func() {
				utility.SendWebhook(redactedURL, panicDetail, []byte(redactedStack))
			}()

			// The panic detail goes to logs and the webhook only, never to clients.
			_ = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Internal Server Error",
			})
		},
	}))
	app.Use(cors.New())

	src.SetupRoutes(app)

	if !prefork || fiber.IsChild() {
		go shutdownOnSignal(app)
	}

	if err := app.Listen(":8080"); err != nil {
		panic(err)
	}

	db.ClosePostgres()
	db.CloseRedis()
}

// shutdownOnSignal drains in-flight requests on SIGINT/SIGTERM. The prefork master serves nothing and keeps the
// default signal behaviour; Fiber stops the children when the master exits.
func shutdownOnSignal(app *fiber.App) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	if err := app.ShutdownWithTimeout(10 * time.Second); err != nil {
		fmt.Printf("[SKYCRYPT] shutdown failed: %v\n", err)
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
