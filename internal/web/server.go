package web

import (
	"embed"
	"log"
	"net/http"

	"github.com/crazyuploader/rdctl-bot/internal/config"
	"github.com/crazyuploader/rdctl-bot/internal/db"
	"github.com/crazyuploader/rdctl-bot/internal/realdebrid"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

//go:embed static/*
var staticFiles embed.FS

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

// Server represents the web server instance
type Server struct {
	app    *fiber.App
	config *config.Config
}

// NewServer creates a new web server instance
func NewServer(deps Dependencies) *Server {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	// Middleware
	app.Use(logger.New())
	app.Use(recover.New())
	app.Use(cors.New())

	// Embed static files
	app.Use("/", filesystem.New(filesystem.Config{
		Root:       http.FS(staticFiles),
		PathPrefix: "static",
		Browse:     false,
	}))

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

	return &Server{
		app:    app,
		config: deps.Config,
	}
}

// Start starts the web server
func (s *Server) Start() error {
	log.Printf("Starting web server on %s", s.config.Web.ListenAddr)
	return s.app.Listen(s.config.Web.ListenAddr)
}

// Shutdown gracefully shuts down the web server
func (s *Server) Shutdown() error {
	return s.app.Shutdown()
}
