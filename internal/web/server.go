package web

import (
	"log"

	"github.com/crazyuploader/rdctl-bot/internal/config"
	"github.com/crazyuploader/rdctl-bot/internal/db"
	"github.com/crazyuploader/rdctl-bot/internal/realdebrid"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

// Dependencies struct to hold all dependencies for the web handlers
type Dependencies struct {
	RDClient     *realdebrid.Client
	UserRepo     *db.UserRepository
	ActivityRepo *db.ActivityRepository
	TorrentRepo  *db.TorrentRepository
	DownloadRepo *db.DownloadRepository
	CommandRepo  *db.CommandRepository
	Config       *config.Config
}

// Start starts the Fiber web server
func Start(deps Dependencies) {
	app := fiber.New()

	// Middleware
	app.Use(logger.New())

	// Static files for the dashboard
	app.Static("/", "./web/static")

	// API group
	api := app.Group("/api", APIKeyAuth(deps.Config.Web.APIKey))

	// API Routes
	api.Get("/status", deps.GetStatus)
	api.Get("/torrents", deps.GetTorrents)
	api.Get("/torrents/:id", deps.GetTorrentInfo)
	api.Post("/torrents", deps.AddTorrent)
	api.Delete("/torrents/:id", deps.DeleteTorrent)
	api.Get("/downloads", deps.GetDownloads)
	api.Post("/unrestrict", deps.UnrestrictLink)
	api.Delete("/downloads/:id", deps.DeleteDownload)
	api.Get("/stats/user/:id", deps.GetUserStats)

	log.Printf("Starting web server on %s", deps.Config.Web.ListenAddr)
	if err := app.Listen(deps.Config.Web.ListenAddr); err != nil {
		log.Printf("Web server shut down or failed: %v", err)
	}
}
