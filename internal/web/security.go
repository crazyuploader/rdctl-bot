package web

import (
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
)

// IPManager handles IP banning and tracking
type IPManager struct {
	mu           sync.Mutex
	authFailures map[string][]time.Time
	bannedIPs    map[string]time.Time
	banDuration  time.Duration
	failLimit    int
	failWindow   time.Duration
}

// NewIPManager creates a new IPManager
func NewIPManager(banDurationSeconds, failLimit, failWindowSeconds int) *IPManager {
	return &IPManager{
		authFailures: make(map[string][]time.Time),
		bannedIPs:    make(map[string]time.Time),
		banDuration:  time.Duration(banDurationSeconds) * time.Second,
		failLimit:    failLimit,
		failWindow:   time.Duration(failWindowSeconds) * time.Second,
	}
}

// Middleware checks if an IP is banned
func (m *IPManager) Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		ip := c.IP()
		if m.IsBanned(ip) {
			log.Printf("Blocked request from banned IP: %s", ip)
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"error":   "Your IP is temporarily banned due to excessive authentication failures.",
			})
		}
		return c.Next()
	}
}

// RegisterAuthFailure records a failed authentication attempt
func (m *IPManager) RegisterAuthFailure(ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clean up old failures
	now := time.Now()
	failures, exists := m.authFailures[ip]
	if !exists {
		failures = []time.Time{}
	}

	// Filter out failures older than the window
	validFailures := []time.Time{}
	for _, t := range failures {
		if now.Sub(t) < m.failWindow {
			validFailures = append(validFailures, t)
		}
	}

	// Add new failure
	validFailures = append(validFailures, now)
	m.authFailures[ip] = validFailures

	// Check if limit reached
	if len(validFailures) >= m.failLimit {
		log.Printf("Banning IP %s for %v after %d failed attempts", ip, m.banDuration, len(validFailures))
		m.bannedIPs[ip] = now.Add(m.banDuration)
		delete(m.authFailures, ip) // Clear failures after ban
	}
}

// IsBanned checks if an IP is currently banned
func (m *IPManager) IsBanned(ip string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	banExpiry, banned := m.bannedIPs[ip]
	if !banned {
		return false
	}

	if time.Now().After(banExpiry) {
		delete(m.bannedIPs, ip) // Unban
		return false
	}

	return true
}

// GetClientIP gets the client IP dealing with proxies if needed
// Assuming fiber.Config ProxyHeader is set elsewhere or use c.IP() directly which handles it
func GetClientIP(c *fiber.Ctx) string {
	// If X-Forwarded-For is set and trusted, it might contain multiple IPs "client, proxy1, proxy2"
	// We want the first one (client)
	ips := c.Get("X-Forwarded-For")
	if ips != "" {
		parts := strings.Split(ips, ",")
		return strings.TrimSpace(parts[0])
	}
	return c.IP()
}
