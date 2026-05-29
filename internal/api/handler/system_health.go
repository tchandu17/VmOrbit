package handler

import (
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/vmOrbit/backend/pkg/logger"
	"gorm.io/gorm"
)

// SystemHealthHandler serves GET /api/v1/system/health — the backend data
// source for the System Health dashboard page.
type SystemHealthHandler struct {
	db          *gorm.DB
	redisClient *redis.Client
	log         logger.Logger
	startTime   time.Time
}

// NewSystemHealthHandler creates a new SystemHealthHandler.
func NewSystemHealthHandler(
	db *gorm.DB,
	redisClient *redis.Client,
	_ interface{}, // reserved for future task-service injection
	log logger.Logger,
) *SystemHealthHandler {
	return &SystemHealthHandler{
		db:          db,
		redisClient: redisClient,
		log:         log,
		startTime:   time.Now(),
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Response types
// ─────────────────────────────────────────────────────────────────────────────

type SystemHealthResponse struct {
	Timestamp  string         `json:"timestamp"`
	UptimeSecs int64          `json:"uptime_secs"`
	API        APIHealth      `json:"api"`
	Database   DatabaseHealth `json:"database"`
	Cache      CacheHealth    `json:"cache"`
	Tasks      TaskQueueHealth `json:"tasks"`
	Runtime    RuntimeHealth  `json:"runtime"`
}

type APIHealth struct {
	Status string `json:"status"`
}

type DatabaseHealth struct {
	Status     string  `json:"status"`
	OpenConns  int     `json:"open_connections"`
	IdleConns  int     `json:"idle_connections"`
	InUseConns int     `json:"in_use_connections"`
	WaitCount  int64   `json:"wait_count"`
	LatencyMs  float64 `json:"latency_ms"`
}

type CacheHealth struct {
	Status       string  `json:"status"`
	UsedMemoryMB float64 `json:"used_memory_mb"`
	HitRatePct   float64 `json:"hit_rate_percent"`
	LatencyMs    float64 `json:"latency_ms"`
}

type TaskQueueHealth struct {
	PendingTasks int              `json:"pending_tasks"`
	RunningTasks int              `json:"running_tasks"`
	QueueDepths  map[string]int64 `json:"queue_depths"`
	TotalQueued  int64            `json:"total_queued"`
}

type RuntimeHealth struct {
	Goroutines  int     `json:"goroutines"`
	HeapAllocMB float64 `json:"heap_alloc_mb"`
	HeapSysMB   float64 `json:"heap_sys_mb"`
	GCPauseMs   float64 `json:"gc_pause_ms"`
	NumGC       uint32  `json:"num_gc"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Handler
// ─────────────────────────────────────────────────────────────────────────────

// GetHealth returns a comprehensive system health snapshot.
func (h *SystemHealthHandler) GetHealth(c *gin.Context) {
	ctx := c.Request.Context()

	resp := SystemHealthResponse{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		UptimeSecs: int64(time.Since(h.startTime).Seconds()),
		API:        APIHealth{Status: "ok"},
	}

	// ── Database ──────────────────────────────────────────────────────────────
	dbStart := time.Now()
	dbStatus := "ok"
	if err := h.db.WithContext(ctx).Exec("SELECT 1").Error; err != nil {
		dbStatus = "error"
		h.log.Warn("health: db ping failed", logger.Error(err))
	}
	dbLatencyMs := float64(time.Since(dbStart).Milliseconds())

	sqlDB, _ := h.db.DB()
	dbHealth := DatabaseHealth{Status: dbStatus, LatencyMs: dbLatencyMs}
	if sqlDB != nil {
		s := sqlDB.Stats()
		dbHealth.OpenConns = s.OpenConnections
		dbHealth.IdleConns = s.Idle
		dbHealth.InUseConns = s.InUse
		dbHealth.WaitCount = s.WaitCount
	}
	resp.Database = dbHealth

	// ── Redis ─────────────────────────────────────────────────────────────────
	cacheStart := time.Now()
	cacheStatus := "ok"
	var usedMemMB, hitRate float64

	if err := h.redisClient.Ping(ctx).Err(); err != nil {
		cacheStatus = "error"
		h.log.Warn("health: redis ping failed", logger.Error(err))
	} else {
		if info, err := h.redisClient.Info(ctx, "memory", "stats").Result(); err == nil {
			usedMemMB = redisInfoFloat(info, "used_memory") / 1024 / 1024
			hits := redisInfoFloat(info, "keyspace_hits")
			misses := redisInfoFloat(info, "keyspace_misses")
			if hits+misses > 0 {
				hitRate = hits / (hits + misses) * 100
			}
		}
	}
	resp.Cache = CacheHealth{
		Status:       cacheStatus,
		UsedMemoryMB: usedMemMB,
		HitRatePct:   hitRate,
		LatencyMs:    float64(time.Since(cacheStart).Milliseconds()),
	}

	// ── Task queues ───────────────────────────────────────────────────────────
	queueDepths := make(map[string]int64, 10)
	var totalQueued int64
	for i := 1; i <= 10; i++ {
		key := "task:queue:" + strconv.Itoa(i)
		n, _ := h.redisClient.LLen(ctx, key).Result()
		if n > 0 {
			queueDepths["p"+strconv.Itoa(i)] = n
		}
		totalQueued += n
	}

	var pendingCount, runningCount int64
	h.db.WithContext(ctx).Raw(
		"SELECT COUNT(*) FROM tasks WHERE status = 'pending' AND deleted_at IS NULL",
	).Scan(&pendingCount)
	h.db.WithContext(ctx).Raw(
		"SELECT COUNT(*) FROM tasks WHERE status = 'running' AND deleted_at IS NULL",
	).Scan(&runningCount)

	resp.Tasks = TaskQueueHealth{
		PendingTasks: int(pendingCount),
		RunningTasks: int(runningCount),
		QueueDepths:  queueDepths,
		TotalQueued:  totalQueued,
	}

	// ── Go runtime ────────────────────────────────────────────────────────────
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	var lastGCPauseMs float64
	if mem.NumGC > 0 {
		lastGCPauseMs = float64(mem.PauseNs[(mem.NumGC+255)%256]) / 1e6
	}
	resp.Runtime = RuntimeHealth{
		Goroutines:  runtime.NumGoroutine(),
		HeapAllocMB: float64(mem.HeapAlloc) / 1024 / 1024,
		HeapSysMB:   float64(mem.HeapSys) / 1024 / 1024,
		GCPauseMs:   lastGCPauseMs,
		NumGC:       mem.NumGC,
	}

	c.JSON(http.StatusOK, gin.H{"data": resp})
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// redisInfoFloat extracts a numeric value from Redis INFO output by key name.
func redisInfoFloat(info, key string) float64 {
	prefix := key + ":"
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, prefix) {
			val := strings.TrimPrefix(line, prefix)
			if f, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
				return f
			}
		}
	}
	return 0
}
