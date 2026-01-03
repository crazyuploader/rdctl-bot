package web

import (
	"context"
	"embed"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/crazyuploader/rdctl-bot/internal/config"
	"github.com/crazyuploader/rdctl-bot/internal/db"
	"github.com/crazyuploader/rdctl-bot/internal/realdebrid"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/earlydata"
	"github.com/gofiber/fiber/v2/middleware/favicon"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/limiter"
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
	TokenStore   *TokenStore
}

// Server represents the web server instance
type Server struct {
	app        *fiber.App
	config     *config.Config
	tokenStore *TokenStore
}

// NewServer creates a new web server instance
func NewServer(deps Dependencies) *Server {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ProxyHeader:           "X-Forwarded-For", // Standard proxy header
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			// Status code defaults to 500
			code := fiber.StatusInternalServerError

			// Retrieve the custom status code if it's a *fiber.Error
			var e *fiber.Error
			var rdErr *realdebrid.APIError

			if errors.As(err, &e) {
				code = e.Code
			} else if errors.As(err, &rdErr) {
				// Map Real-Debrid API errors to 502 (Bad Gateway) to distinguish from internal server errors
				// forcing the message to be shown below
				code = fiber.StatusBadGateway
			}

			// Log the error internally
			log.Printf("Web Error [%d]: %v", code, err)

			// Sanitize error message for the client
			message := "An unexpected error occurred"
			if code < 500 || rdErr != nil {
				// Show message for client errors (< 500) or upstream API errors
				message = err.Error()
			} else {
				// For 500+ errors (excluding upstream API errors), we don't want to leak internal details
				message = "Internal Server Error"
			}

			return c.Status(code).JSON(fiber.Map{
				"success": false,
				"error":   message,
			})
		},
	})

	// Middleware
	app.Use(compress.New())
	app.Use(earlydata.New())
	app.Use(favicon.New(
		favicon.Config{
			File: "./internal/web/static/favicon.svg",
			URL:  "/favicon.svg",
		},
	))
	app.Use(logger.New())
	app.Use(recover.New())
	app.Use(cors.New())

	// API group with dual auth (API key OR token)
	api := app.Group("/api")

	// Rate limiting
	if deps.Config.Web.Limiter.Enabled {
		api.Use(limiter.New(limiter.Config{
			Max:               deps.Config.Web.Limiter.Max,
			Expiration:        time.Duration(deps.Config.Web.Limiter.ExpirationSeconds) * time.Second,
			LimiterMiddleware: limiter.SlidingWindow{},
			LimitReached: func(c *fiber.Ctx) error {
				return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
					"success": false,
					"error":   "Too many requests, please try again later.",
				})
			},
		}))
	}

	// Token exchange endpoint - NO AUTH REQUIRED
	api.Post("/exchange-token", deps.ExchangeToken)

	// Apply DualAuth to the rest of the API routes
	api.Use(DualAuth(deps.Config.Web.APIKey, deps.TokenStore))

	// Auth endpoint to get current user info
	api.Get("/auth/me", deps.GetAuthInfo)

	// API Routes - Read operations (allowed for all authenticated users)
	api.Get("/status", deps.GetStatus)
	api.Get("/torrents", deps.GetTorrents)
	api.Get("/torrents/:id", deps.GetTorrentInfo)
	api.Post("/torrents", deps.AddTorrent)
	api.Get("/downloads", deps.GetDownloads)
	api.Post("/unrestrict", deps.UnrestrictLink)
	api.Get("/stats/user/:id", deps.GetUserStats)

	// Delete operations - Admin only
	api.Delete("/torrents/:id", AdminOnly(deps.TokenStore), deps.DeleteTorrent)
	api.Delete("/downloads/:id", AdminOnly(deps.TokenStore), deps.DeleteDownload)

	// Embed static files - Place this last to ensure API routes are matched first
	// or properly fall through if not found.
	app.Use("/", filesystem.New(filesystem.Config{
		Root:       http.FS(staticFiles),
		PathPrefix: "static",
		Browse:     false,
	}))

	return &Server{
		app:        app,
		config:     deps.Config,
		tokenStore: deps.TokenStore,
	}
}

// Start starts the web server
func (s *Server) Start() error {
	log.Printf("Starting web server on %s", s.config.Web.ListenAddr)
	return s.app.Listen(s.config.Web.ListenAddr)
}

// Shutdown gracefully shuts down the web server with context for timeout support
func (s *Server) Shutdown(ctx context.Context) error {
	return s.app.ShutdownWithContext(ctx)
}
