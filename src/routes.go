package src

import (
	"fmt"
	"log"
	"os"
	notenoughupdates "skycrypt/src/NotEnoughUpdates"
	"skycrypt/src/api"
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

	if os.Getenv("FIBER_PREFORK_CHILD") == "" {
		log.Println("No .env file found, using environment variables")
	}

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

	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	mongoDBName := os.Getenv("MONGO_DB_NAME")
	if mongoDBName == "" {
		mongoDBName = "SkyCrypt"
	}

	err = db.InitMongo(mongoURI, mongoDBName)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %v", err)
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

		notenoughupdates.StartUpdateLoop()

		if err := api.LoadSkyBlockItems(false); err != nil {
			return fmt.Errorf("error loading SkyBlock items: %v", err)
		}

		go func() {
			if err := lib.StartCustomResources(); err != nil {
				log.Printf("failed to start custom resources: %v", err)
			}
		}()

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

		if err := notenoughupdates.ParseNEURepository(); err != nil {
			return fmt.Errorf("failed to parse NEU repository: %v", err)
		}

		if err := lib.PrepareCustomResourceCache(); err != nil {
			log.Printf("failed to load custom resource index: %v", err)
		}

		lib.WarmCustomResourceRendererAsync(1500 * time.Millisecond)
	}

	return nil
}

func SetupRoutes(app *fiber.App) {
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed,
	}))

	// Assets folder
	app.Static("/assets", "assets")
	app.Static("/cache", "cache")

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

	api := app.Group("/api")

	api.Get("/source", routes.SourceHandler)

	// Forensic dashboard (live in-memory report)
	if utility.IsForensicsEnabled() {
		api.Get("/forensics", forensics.DashboardHandler())
		api.Get("/forensics/dashboard", forensics.DashboardHTMLHandler())
	}

	// Documentation - serve openapi files directly
	api.Static("/openapi/doc.json", "./docs/swagger.json")

	api.Get("/openapi/", func(c *fiber.Ctx) error {
		html := `<!DOCTYPE html>
					<html>
					<head>
						<title>API Documentation</title>
						<meta charset="utf-8" />
						<meta name="viewport" content="width=device-width, initial-scale=1" />
					</head>
					<body>
						<script id="api-reference" data-url="/api/openapi/doc.json"></script>
						<script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
					</body>
					</html>`
		c.Set("Content-Type", "text/html")
		return c.SendString(html)
	})

	// USERNAME AND UUID RESOLVING
	api.Get("/uuid/:username", routes.ServerAPITokenMiddleware, routes.UUIDHandler)
	api.Get("/username/:uuid", routes.UsernameHandler)

	// STATS ENDPOINTS
	api.Get("/stats/:uuid/:profileId", routes.ServerAPITokenMiddleware, routes.StatsHandler)
	api.Get("/stats/:uuid", routes.ServerAPITokenMiddleware, routes.SelectedProfileStatsHandler)

	api.Get("/playerStats/:uuid/:profileId", routes.ServerAPITokenMiddleware, routes.PlayerStatsHandler)

	api.Get("/networth/:uuid/:profileId", routes.ServerAPITokenMiddleware, routes.NetworthHandler)

	api.Get("/combined/:uuid/:profileId", routes.ServerAPITokenMiddleware, routes.CombinedHandler)

	api.Get("/inventory/:uuid/:profileId", routes.ServerAPITokenMiddleware, routes.InventoryHandler)
	api.Get("/inventory/search/:uuid/:profileId/:searchParam", routes.ServerAPITokenMiddleware, routes.InventorySearchHandler)

	api.Get("/garden/:uuid/:profileId", routes.ServerAPITokenMiddleware, routes.GardenHandler)

	api.Get("/embed/:uuid/:profileId", routes.ServerAPITokenMiddleware, routes.EmbedHandler)
	api.Get("/embed/:uuid", routes.ServerAPITokenMiddleware, routes.SelectedProfileEmbedHandler)

	// RENDERING ENDPOINTS
	api.Get("/head/:textureId", routes.HeadHandlers)
	api.Get("/item/:itemId/resolve", routes.ItemResolveHandler)
	api.Get("/item/:itemId", routes.ItemHandlers)
	api.Get("/potion/:type/:color", routes.PotionHandlers)
	api.Get("/leather/:type/:color", routes.LeatherHandlers)
	api.Get("/constants/stats", routes.StatsConstantsHandler)
	api.Get("/constants/packs", routes.ResourcePacksConstantsHandler)

	// OTHER
	api.Get("/emojis", routes.EmojisHandler)
}
