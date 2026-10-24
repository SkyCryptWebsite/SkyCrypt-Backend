package src

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	notenoughupdates "skycrypt/src/NotEnoughUpdates"
	"skycrypt/src/api"
	"skycrypt/src/apikeys"
	"skycrypt/src/db"
	"skycrypt/src/forensics"
	"skycrypt/src/lib"
	"skycrypt/src/routes"
	"skycrypt/src/utility"
	"strings"
	"sync"
	"time"

	skyhelpernetworthgo "github.com/SkyCryptWebsite/SkyHelper-Networth-Go"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/etag"
	"github.com/gofiber/fiber/v3/middleware/static"
	"go.uber.org/zap"
)

func SetupApplication() error {
	timeNow := time.Now()

	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}

	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}

	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisAddr := fmt.Sprintf("%s:%s", redisHost, redisPort)

	err := db.InitRedis(redisAddr, redisPassword, 0)
	if err != nil {
		return fmt.Errorf("failed to connect to Redis: %v", err)
	}

	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		if err := db.InitPostgres(databaseURL); err != nil {
			log.Printf("Postgres unavailable, fallback cache disabled: %v", err)
		}
	} else {
		log.Println("DATABASE_URL not set, fallback cache disabled")
	}

	// Main process: fetch all remote data, init/update repos, then parse
	// Prefork children: only parse local data and read from Redis cache
	if !fiber.IsChild() {
		err := notenoughupdates.InitializeNEURepository()
		if err != nil {
			return fmt.Errorf("failed to initialize NEU repository: %v", err)
		}

		if err := notenoughupdates.ParseNEURepository(); err != nil {
			return fmt.Errorf("failed to parse NEU repository: %v", err)
		}
		notenoughupdates.PublishRepositoryVersion()
		notenoughupdates.StartInitialUpdate()

		notenoughupdates.StartUpdateLoop()
		apikeys.StartUsageSync()

		if err := api.LoadSkyBlockItems(false); err != nil {
			return fmt.Errorf("error loading SkyBlock items: %v", err)
		}
		startItemsRefresh(false)

		if err := lib.StartCustomResources(); err != nil {
			return fmt.Errorf("failed to start custom resources: %v", err)
		}

		go func() {
			_, err := skyhelpernetworthgo.GetPrices(true, 0, 0)
			if err != nil {
				log.Printf("error fetching SkyHelper prices: %v", err)
			}
		}()

		go func() {
			_, err := skyhelpernetworthgo.GetItems(true, 0, 0)
			if err != nil {
				log.Printf("error fetching SkyHelper items: %v", err)
			}
		}()

		if utility.IsForensicsEnabled() {
			forensics.Logger.Info("setup_application_completed",
				zap.Duration("startup_time", time.Since(timeNow)),
			)
		}

		fmt.Printf("[SKYCRYPT] SkyCrypt initialized successfully in %s\n", time.Since(timeNow))

		if os.Getenv("DEV") != "true" {
			go utility.SendWebhook("BACKEND", "SkyCrypt Backend has started successfully!", fmt.Appendf(nil, "Startup Time: %s", time.Since(timeNow).String()))
		}
	} else {
		// Prefork children: load items from Redis cache
		if err := api.LoadSkyBlockItems(true); err != nil {
			return fmt.Errorf("error loading SkyBlock items: %v", err)
		}
		startItemsRefresh(true)

		neuBaseline := notenoughupdates.PublishedRepositoryVersion()
		if err := notenoughupdates.ParseWithRetry(); err != nil {
			return fmt.Errorf("failed to parse NEU repository: %v", err)
		}
		notenoughupdates.StartChildReloadLoop(neuBaseline)

		if err := lib.PrepareCustomResourceCache(); err != nil {
			log.Printf("failed to load custom resource index: %v", err)
		}

		lib.WarmCustomResourceRendererAsync(1500 * time.Millisecond)
	}

	return nil
}

// startItemsRefresh reloads Hypixel's item and collection constants, which were otherwise only read at startup.
// The process that fetches reads them from Redis while they are cached there (12 hours) and from Hypixel after;
// prefork children only read Redis.
func startItemsRefresh(fromCache bool) {
	go func() {
		for range time.Tick(6 * time.Hour) {
			if err := api.LoadSkyBlockItems(fromCache); err != nil {
				log.Printf("error refreshing SkyBlock items: %v", err)
			}
		}
	}()
}

func SetupRoutes(app *fiber.App) {
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed,
	}))

	// Assets folder
	// Without MaxAge Fiber sends no Cache-Control, and every icon on a page went back to the origin. Rendered files
	// are content-hashed or named after what they show, and are written atomically.
	app.Get("/assets*", static.New("assets", static.Config{MaxAge: 24 * 60 * 60}))
	app.Get("/cache*", static.New("cache", static.Config{MaxAge: 7 * 24 * 60 * 60, ModifyResponse: cacheStaticResponse}))

	if os.Getenv("DEV") != "true" {
		if !fiber.IsChild() {
			fmt.Println("[ENVIROMENT] Running in production mode.")
		}

		// Weak tags survive the compress middleware, which replaces strong ones with a hash of the compressed body.
		app.Use(etag.New(etag.Config{Weak: true}))
		/*
			app.Use("/api", cache.New(cache.Config{
				Expiration:   5 * time.Minute,
				DisableCacheControl: false,
			})
		*/
	}

	// api.shiiyu.moe serves the API reference at its root and the endpoints under /api.
	app.Get("/", apiReferenceHandler)

	api := app.Group("/api")

	// Forensic dashboard (live in-memory report)
	if utility.IsForensicsEnabled() {
		api.Get("/forensics", forensics.DashboardHandler())
		api.Get("/forensics/dashboard", forensics.DashboardHTMLHandler())
	}

	humaAPI := routes.NewAPI(app, string(apiReferenceIntro))
	// Marshaled once, on the first request.
	spec := sync.OnceValues(humaAPI.OpenAPI().MarshalJSON)
	app.Get("/openapi.json", func(c fiber.Ctx) error {
		body, err := spec()
		if err != nil {
			return err
		}
		// The spec changes on every deploy, so browsers must revalidate it instead of guessing a cache lifetime.
		c.Set(fiber.HeaderCacheControl, "no-cache")
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
		return c.Send(body)
	})
	api.Get("/openapi/", apiReferenceHandler)
	api.Get("/openapi/reference.js", apiReferenceScriptHandler)
}

// cacheStaticResponse keeps the week-long max-age for files whose URL fixes their bytes: hash= renders, and heads,
// potions and leather named after what they show. Other files in rendered/ and derived/ (the pack manifest) change
// in place, so clients revalidate them.
func cacheStaticResponse(c fiber.Ctx) error {
	path := c.Path()
	if !strings.Contains(path, "hash=") && (strings.HasPrefix(path, "/cache/rendered/") || strings.HasPrefix(path, "/cache/derived/")) {
		c.Set(fiber.HeaderCacheControl, "public, no-cache")
	}
	return nil
}

//go:embed api-reference/index.html
var apiReferenceHTML []byte

//go:embed api-reference/index.js
var apiReferenceJS []byte

//go:embed api-reference/api.md
var apiReferenceIntro []byte

func apiReferenceHandler(c fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return c.Send(apiReferenceHTML)
}

func apiReferenceScriptHandler(c fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, fiber.MIMETextJavaScriptCharsetUTF8)
	return c.Send(apiReferenceJS)
}
