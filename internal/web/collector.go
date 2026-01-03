package web

import (
	"log"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// RDCollector implements the prometheus.Collector interface
type RDCollector struct {
	deps          Dependencies
	cacheDuration time.Duration
	mu            sync.Mutex
	lastScrape    time.Time

	// Cache fields
	cachedTorrentCount   float64
	cachedDownloadCount  float64
	cachedTotalSize      float64
	cachedUserPoints     float64
	cachedPremiumSeconds float64
	cachedActiveCount    float64

	// Descriptors
	torrentsCountDesc  *prometheus.Desc
	downloadsCountDesc *prometheus.Desc
	torrentsSizeDesc   *prometheus.Desc
	userPointsDesc     *prometheus.Desc
	premiumSecondsDesc *prometheus.Desc
	activeCountDesc    *prometheus.Desc
}

// NewRDCollector creates a new RDCollector
func NewRDCollector(deps Dependencies) *RDCollector {
	return &RDCollector{
		deps:          deps,
		cacheDuration: 5 * time.Minute,

		torrentsCountDesc: prometheus.NewDesc(
			"rdctl_torrents_count",
			"Total number of torrents",
			nil, nil,
		),
		downloadsCountDesc: prometheus.NewDesc(
			"rdctl_downloads_count",
			"Total number of downloads",
			nil, nil,
		),
		torrentsSizeDesc: prometheus.NewDesc(
			"rdctl_torrents_total_size_bytes",
			"Total size of all torrents in bytes",
			nil, nil,
		),
		userPointsDesc: prometheus.NewDesc(
			"rdctl_user_fidelity_points",
			"User fidelity points",
			nil, nil,
		),
		premiumSecondsDesc: prometheus.NewDesc(
			"rdctl_user_premium_seconds_remaining",
			"Seconds remaining of premium status",
			nil, nil,
		),
		activeCountDesc: prometheus.NewDesc(
			"rdctl_torrents_active_count",
			"Number of currently active torrents",
			nil, nil,
		),
	}
}

// Describe sends the super-set of must-have descriptors
func (c *RDCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.torrentsCountDesc
	ch <- c.downloadsCountDesc
	ch <- c.torrentsSizeDesc
	ch <- c.userPointsDesc
	ch <- c.premiumSecondsDesc
	ch <- c.activeCountDesc
}

// Collect is called by the Prometheus registry when collecting metrics
func (c *RDCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check cache
	if time.Since(c.lastScrape) > c.cacheDuration {
		c.scrape()
	}

	// Emit metrics from cache
	ch <- prometheus.MustNewConstMetric(c.torrentsCountDesc, prometheus.GaugeValue, c.cachedTorrentCount)
	ch <- prometheus.MustNewConstMetric(c.downloadsCountDesc, prometheus.GaugeValue, c.cachedDownloadCount)
	ch <- prometheus.MustNewConstMetric(c.torrentsSizeDesc, prometheus.GaugeValue, c.cachedTotalSize)
	ch <- prometheus.MustNewConstMetric(c.userPointsDesc, prometheus.GaugeValue, c.cachedUserPoints)
	ch <- prometheus.MustNewConstMetric(c.premiumSecondsDesc, prometheus.GaugeValue, c.cachedPremiumSeconds)
	ch <- prometheus.MustNewConstMetric(c.activeCountDesc, prometheus.GaugeValue, c.cachedActiveCount)
}

func (c *RDCollector) scrape() {
	log.Println("Scraping Real-Debrid metrics (refreshing cache)...")

	// 1. Torrents
	// Pagination loop to fetch ALL torrents for total size
	var totalSize int64
	var totalCount int
	limit := 5000 // Optimization: Fetch up to 5000 torrents per call to minimize API requests
	offset := 0
	scrapeSuccess := true

	for {
		torrentsResult, err := c.deps.RDClient.GetTorrentsWithCount(limit, offset)
		if err != nil {
			log.Printf("Error scraping torrents (offset %d): %v", offset, err)
			scrapeSuccess = false
			break
		}

		if offset == 0 {
			totalCount = torrentsResult.TotalCount
		}

		for _, t := range torrentsResult.Torrents {
			totalSize += t.Bytes
		}

		if len(torrentsResult.Torrents) < limit {
			break
		}
		offset += limit
	}

	if scrapeSuccess {
		c.cachedTorrentCount = float64(totalCount)
		c.cachedTotalSize = float64(totalSize)
	}

	// 2. Downloads
	downloadsResult, err := c.deps.RDClient.GetDownloadsWithCount(1, 0)
	if err == nil {
		c.cachedDownloadCount = float64(downloadsResult.TotalCount)
	} else {
		log.Printf("Error scraping downloads: %v", err)
	}

	// 3. User Info (Points, Premium)
	user, err := c.deps.RDClient.GetUser()
	if err == nil {
		c.cachedUserPoints = float64(user.Points)
		c.cachedPremiumSeconds = float64(user.Premium)
	} else {
		log.Printf("Error scraping user: %v", err)
	}

	// 4. Active Count
	activeCount, err := c.deps.RDClient.GetActiveCount()
	if err == nil {
		c.cachedActiveCount = float64(activeCount.Nb)
	} else {
		log.Printf("Error scraping active count: %v", err)
	}

	c.lastScrape = time.Now()
}
