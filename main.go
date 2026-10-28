package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"skycrypt/src"
	"skycrypt/src/db"
	"skycrypt/src/forensics"
	"skycrypt/src/routes"
	"skycrypt/src/security"
	"skycrypt/src/utility"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("[SKYCRYPT] No .env file found, relying on environment variables")
	}
	configureMemoryLimit(preforkEnabled())

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
	app := fiber.New(newAppConfig())

	// ========== FORENSIC REQUEST TRACING (MUST BE BEFORE RECOVER) ==========
	if utility.IsForensicsEnabled() {
		app.Use(forensics.RequestTracingMiddleware())
	}

	app.Use(newRecoverMiddleware())
	app.Use(cors.New())

	src.SetupRoutes(app)

	shutdownDone := make(chan struct{})
	if !prefork || fiber.IsChild() {
		go shutdownOnSignal(app, shutdownDone)
	} else {
		close(shutdownDone)
	}

	if err := app.Listen(":"+listenPort(), fiber.ListenConfig{EnablePrefork: prefork}); err != nil {
		panic(err)
	}

	// Listen returns as soon as the listener closes, while requests may still be draining. Returning from main
	// would end the process and cut them off, so wait for the drain before closing the databases.
	<-shutdownDone

	db.ClosePostgres()
	db.CloseRedis()
}

// newAppConfig returns the Fiber settings for the API server.
func newAppConfig() fiber.Config {
	return fiber.Config{
		ServerHeader:              "",    // Remove server header for slight perf gain
		DisableKeepalive:          false, // Keep connections alive
		DisableDefaultDate:        true,  // Disable date header
		DisableDefaultContentType: false,
		BodyLimit:                 10 << 20, // 10MB
		ReadBufferSize:            32 << 10, // Caps request header size; the frontend forwards browser cookies, so match Cloudflare's 32KB
		WriteBufferSize:           4096,
		ReadTimeout:               15 * time.Second,
		WriteTimeout:              30 * time.Second,
		IdleTimeout:               120 * time.Second,
		// Fiber's default is 256K open connections per process. Keep-alive connections each hold their buffers, so
		// a child stops accepting long before that would exhaust its memory share.
		Concurrency: 16 << 10,
		// Decode route parameters, so a search for "giant sword" doesn't reach the handler as "giant%20sword".
		UnescapePath: true,
	}
}

// newRecoverMiddleware logs panics with their stack and answers clients with a generic 500.
func newRecoverMiddleware() fiber.Handler {
	return recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c fiber.Ctx, err any) {
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

		},
		// The panic detail goes to logs and the webhook only, never to clients. Returning nil keeps the default
		// error handler from replacing this body with the panic message.
		PanicHandler: func(c fiber.Ctx, _ any) error {
			return c.Status(fiber.StatusInternalServerError).JSON(routes.InternalError(), "application/problem+json")
		},
	})
}

// shutdownOnSignal drains in-flight requests on SIGINT/SIGTERM. The prefork master serves nothing and keeps the
// default signal behaviour, so under prefork in Docker it dies at once and the children are killed without draining.
func shutdownOnSignal(app *fiber.App, done chan<- struct{}) {
	defer close(done)

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

// listenPort is PORT, or 8080. A second backend can run next to the usual one in development.
func listenPort() string {
	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
		return port
	}
	return "8080"
}

// configureMemoryLimit splits the container's memory budget between the prefork processes. GOMEMLIMIT applies to each
// Go process on its own, so a container-sized limit lets every child believe it may use the whole container, and the
// container is killed before any of them collects. The master sets its share and passes the rest to the children
// through GOMEMLIMIT, which they read at start.
func configureMemoryLimit(prefork bool) {
	if fiber.IsChild() {
		return
	}
	total := memoryBudget()
	if total <= 0 {
		return
	}

	processes := int64(1)
	if prefork {
		// fasthttp starts one child per GOMAXPROCS; GOMAXPROCS (or the container CPU limit) sets how many.
		processes += int64(runtime.GOMAXPROCS(0))
	}
	share := total / processes
	debug.SetMemoryLimit(share)
	_ = os.Setenv("GOMEMLIMIT", strconv.FormatInt(share, 10))
	fmt.Printf("[SKYCRYPT] memory limit %d MiB per process (%d processes, %d MiB in total)\n",
		share>>20, processes, total>>20)
}

// memoryBudget is SKYCRYPT_MEMORY_LIMIT (such as "7GiB"), or else 90% of the container's cgroup limit, for all
// processes together. 0 means no limit is known.
func memoryBudget() int64 {
	if value := strings.TrimSpace(os.Getenv("SKYCRYPT_MEMORY_LIMIT")); value != "" {
		if limit, ok := parseBytes(value); ok {
			return limit
		}
		fmt.Printf("[SKYCRYPT] invalid SKYCRYPT_MEMORY_LIMIT %q; ignoring it\n", value)
	}
	for _, path := range []string{"/sys/fs/cgroup/memory.max", "/sys/fs/cgroup/memory/memory.limit_in_bytes"} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		limit, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		// "max", or cgroup v1's near-int64 value, means no limit.
		if err != nil || limit <= 0 || limit > 1<<50 {
			return 0
		}
		return limit / 10 * 9
	}
	return 0
}

// parseBytes reads a size such as "7GiB", "512MiB" or a plain number of bytes.
func parseBytes(value string) (int64, bool) {
	units := []struct {
		suffix string
		size   int64
	}{{"TiB", 1 << 40}, {"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10}, {"B", 1}}
	for _, unit := range units {
		if number, ok := strings.CutSuffix(value, unit.suffix); ok {
			n, err := strconv.ParseFloat(strings.TrimSpace(number), 64)
			if err != nil || n <= 0 {
				return 0, false
			}
			return int64(n * float64(unit.size)), true
		}
	}
	n, err := strconv.ParseInt(value, 10, 64)
	return n, err == nil && n > 0
}
