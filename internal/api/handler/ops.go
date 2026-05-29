package handler

// ─────────────────────────────────────────────────────────────────────────────
// OpsHandler — operational probe endpoints
//
// Endpoints:
//   GET /health  — liveness probe (always 200 if process is alive)
//   GET /ready   — readiness probe (200 only when DB + Redis are reachable)
//   GET /status  — extended status for ops dashboards
//
// These are intentionally unauthenticated so load balancers and orchestrators
// can call them without credentials.
// ─────────────────────────────────────────────────────────────────────────────

import (
	"context"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/vmOrbit/backend/pkg/logger"
	"gorm.io/gorm"
)

// OpsHandler serves liveness, readiness, and status probes.
type OpsHandler struct {
	db          *gorm.DB
	redisClient *redis.Client
	log         logger.Logger
	startTime   time.Time
	version     string
}

// NewOpsHandler creates a new OpsHandler.
func NewOpsHandler(db *gorm.DB, redisClient *redis.Client, log logger.Logger, version string) *OpsHandler {
	return &OpsHandler{
		db:          db,
		redisClient: redisClient,
		log:         log,
		startTime:   time.Now(),
		version:     version,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /health — liveness probe
// Returns 200 as long as the process is running. Used by Docker HEALTHCHECK
// and Kubernetes liveness probes. Never returns 5xx.
// ─────────────────────────────────────────────────────────────────────────────

func (h *OpsHandler) Liveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "vmOrbit",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /ready — readiness probe
// Returns 200 only when the service can handle traffic (DB + Redis reachable).
// Returns 503 during startup or when dependencies are unavailable.
// ─────────────────────────────────────────────────────────────────────────────

func (h *OpsHandler) Readiness(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	type checkResult struct {
		name string
		ok   bool
		err  string
	}

	var wg sync.WaitGroup
	results := make([]checkResult, 2)

	// Check DB
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := h.db.WithContext(ctx).Exec("SELECT 1").Error; err != nil {
			results[0] = checkResult{name: "database", ok: false, err: err.Error()}
		} else {
			results[0] = checkResult{name: "database", ok: true}
		}
	}()

	// Check Redis
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := h.redisClient.Ping(ctx).Err(); err != nil {
			results[1] = checkResult{name: "redis", ok: false, err: err.Error()}
		} else {
			results[1] = checkResult{name: "redis", ok: true}
		}
	}()

	wg.Wait()

	checks := make(map[string]interface{}, len(results))
	allOK := true
	for _, r := range results {
		if r.ok {
			checks[r.name] = "ok"
		} else {
			checks[r.name] = r.err
			allOK = false
		}
	}

	status := "ready"
	code := http.StatusOK
	if !allOK {
		status = "not_ready"
		code = http.StatusServiceUnavailable
	}

	c.JSON(code, gin.H{
		"status": status,
		"checks": checks,
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /status — extended operational status
// Returns version, uptime, and dependency health for ops dashboards.
// ─────────────────────────────────────────────────────────────────────────────

func (h *OpsHandler) Status(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	uptime := time.Since(h.startTime)

	// DB check
	dbStart := time.Now()
	dbStatus := "ok"
	dbErr := ""
	if err := h.db.WithContext(ctx).Exec("SELECT 1").Error; err != nil {
		dbStatus = "error"
		dbErr = err.Error()
	}
	dbLatency := time.Since(dbStart)

	// Redis check
	redisStart := time.Now()
	redisStatus := "ok"
	redisErr := ""
	if err := h.redisClient.Ping(ctx).Err(); err != nil {
		redisStatus = "error"
		redisErr = err.Error()
	}
	redisLatency := time.Since(redisStart)

	// Runtime
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	dbInfo := gin.H{
		"status":     dbStatus,
		"latency_ms": dbLatency.Milliseconds(),
	}
	if dbErr != "" {
		dbInfo["error"] = dbErr
	}

	redisInfo := gin.H{
		"status":     redisStatus,
		"latency_ms": redisLatency.Milliseconds(),
	}
	if redisErr != "" {
		redisInfo["error"] = redisErr
	}

	c.JSON(http.StatusOK, gin.H{
		"service":    "vmOrbit",
		"version":    h.version,
		"status":     "running",
		"uptime":     uptime.String(),
		"uptime_sec": int64(uptime.Seconds()),
		"started_at": h.startTime.UTC().Format(time.RFC3339),
		"time":       time.Now().UTC().Format(time.RFC3339),
		"dependencies": gin.H{
			"database": dbInfo,
			"redis":    redisInfo,
		},
		"runtime": gin.H{
			"goroutines":    runtime.NumGoroutine(),
			"heap_alloc_mb": float64(mem.HeapAlloc) / 1024 / 1024,
			"num_gc":        mem.NumGC,
		},
	})
}
