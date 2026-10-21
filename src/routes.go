package src

import (
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
	"time"

	skyhelpernetworthgo "github.com/SkyCryptWebsite/SkyHelper-Networth-Go"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/etag"
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
	if os.Getenv("FIBER_PREFORK_CHILD") == "" {
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
	app.Static("/assets", "assets", fiber.Static{MaxAge: 24 * 60 * 60})
	app.Static("/cache", "cache", fiber.Static{MaxAge: 7 * 24 * 60 * 60})

	if os.Getenv("DEV") != "true" {
		if os.Getenv("FIBER_PREFORK_CHILD") == "" {
			fmt.Println("[ENVIROMENT] Running in production mode.")
		}

		app.Use(etag.New())
		/*
			app.Use("/api", cache.New(cache.Config{
				Expiration:   5 * time.Minute,
				CacheControl: true,
			})
		*/
	}

	// api.shiiyu.moe serves the API reference at its root and the endpoints under /api.
	app.Get("/", apiReferenceHandler)

	api := app.Group("/api")

	api.Get("/source", routes.SourceHandler)

	// Forensic dashboard (live in-memory report)
	if utility.IsForensicsEnabled() {
		api.Get("/forensics", forensics.DashboardHandler())
		api.Get("/forensics/dashboard", forensics.DashboardHTMLHandler())
	}

	// Documentation - serve openapi files directly
	api.Static("/openapi/doc.json", "./docs/swagger.json")
	api.Get("/openapi/", apiReferenceHandler)

	// USERNAME AND UUID RESOLVING
	api.Get("/uuid/:username", routes.APIKeyMiddleware, routes.UUIDHandler)
	api.Get("/username/:uuid", routes.UsernameHandler)

	// STATS ENDPOINTS
	api.Get("/stats/:uuid/:profileId", routes.APIKeyMiddleware, routes.StatsHandler)
	api.Get("/stats/:uuid", routes.APIKeyMiddleware, routes.SelectedProfileStatsHandler)

	api.Get("/playerStats/:uuid/:profileId", routes.APIKeyMiddleware, routes.PlayerStatsHandler)

	api.Get("/networth/:uuid/:profileId", routes.APIKeyMiddleware, routes.NetworthHandler)

	api.Get("/combined/:uuid/:profileId", routes.APIKeyMiddleware, routes.CombinedHandler)

	api.Get("/inventory/:uuid/:profileId", routes.APIKeyMiddleware, routes.InventoryHandler)
	api.Get("/inventory/search/:uuid/:profileId/:searchParam", routes.APIKeyMiddleware, routes.InventorySearchHandler)

	api.Get("/garden/:uuid/:profileId", routes.APIKeyMiddleware, routes.GardenHandler)

	api.Get("/embed/:uuid/:profileId", routes.APIKeyMiddleware, routes.EmbedHandler)
	api.Get("/embed/:uuid", routes.APIKeyMiddleware, routes.SelectedProfileEmbedHandler)

	// RENDERING ENDPOINTS
	api.Get("/head/:textureId", routes.HeadHandlers)
	api.Get("/item/:itemId/resolve", routes.ItemResolveHandler)
	api.Get("/item/:itemId", routes.ItemHandlers)
	api.Get("/potion/:type/:color", routes.PotionHandlers)
	api.Get("/leather/:type/:color", routes.LeatherHandlers)
	api.Get("/constants/stats", routes.StatsConstantsHandler)
	api.Get("/constants/packs", routes.ResourcePacksConstantsHandler)
	api.Get("/constants/enchantments", routes.EnchantmentsConstantsHandler)
}

func apiReferenceHandler(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/html")
	return c.SendString(`<!DOCTYPE html>
<html>
<head>
	<title>SkyCrypt API Reference</title>
	<meta charset="utf-8" />
	<meta name="viewport" content="width=device-width, initial-scale=1" />
</head>
<body>
	<script id="api-reference" data-url="/api/openapi/doc.json"></script>
	<script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference@1.75.1"></script>
</body>
</html>`)
}
