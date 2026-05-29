// Package metrics provides Prometheus instrumentation for VMOrbit.
// It registers custom collectors for API requests, task engine, WebSocket,
// and provider operations.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ─────────────────────────────────────────────────────────────────────────────
// API Metrics
// ─────────────────────────────────────────────────────────────────────────────

var (
	// HTTPRequestsTotal counts total HTTP requests by method, path, and status.
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "vmorbit",
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "Total number of HTTP requests.",
	}, []string{"method", "path", "status"})

	// HTTPRequestDuration observes request latency in seconds.
	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "vmorbit",
		Subsystem: "http",
		Name:      "request_duration_seconds",
		Help:      "HTTP request duration in seconds.",
		Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"method", "path"})

	// HTTPRequestsInFlight tracks currently active requests.
	HTTPRequestsInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "vmorbit",
		Subsystem: "http",
		Name:      "requests_in_flight",
		Help:      "Number of HTTP requests currently being processed.",
	})
)

// ─────────────────────────────────────────────────────────────────────────────
// Task Engine Metrics
// ─────────────────────────────────────────────────────────────────────────────

var (
	// TasksTotal counts tasks by type and final status.
	TasksTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "vmorbit",
		Subsystem: "tasks",
		Name:      "total",
		Help:      "Total number of tasks processed.",
	}, []string{"type", "status"})

	// TaskDuration observes task execution time in seconds.
	TaskDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "vmorbit",
		Subsystem: "tasks",
		Name:      "duration_seconds",
		Help:      "Task execution duration in seconds.",
		Buckets:   []float64{0.5, 1, 5, 10, 30, 60, 120, 300, 600},
	}, []string{"type"})

	// TaskQueueDepth tracks the number of tasks waiting in each priority queue.
	TaskQueueDepth = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "vmorbit",
		Subsystem: "tasks",
		Name:      "queue_depth",
		Help:      "Number of tasks waiting in the queue by priority.",
	}, []string{"priority"})

	// TaskWorkersActive tracks the number of active task workers.
	TaskWorkersActive = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "vmorbit",
		Subsystem: "tasks",
		Name:      "workers_active",
		Help:      "Number of task workers currently executing a task.",
	})

	// TaskRetries counts task retry attempts.
	TaskRetries = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "vmorbit",
		Subsystem: "tasks",
		Name:      "retries_total",
		Help:      "Total number of task retry attempts.",
	}, []string{"type"})
)

// ─────────────────────────────────────────────────────────────────────────────
// WebSocket Metrics
// ─────────────────────────────────────────────────────────────────────────────

var (
	// WSConnectionsActive tracks active WebSocket connections.
	WSConnectionsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "vmorbit",
		Subsystem: "websocket",
		Name:      "connections_active",
		Help:      "Number of active WebSocket connections.",
	})

	// WSMessagesTotal counts WebSocket messages sent.
	WSMessagesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "vmorbit",
		Subsystem: "websocket",
		Name:      "messages_total",
		Help:      "Total WebSocket messages sent.",
	}, []string{"room", "direction"})

	// WSSubscriptions tracks active room subscriptions.
	WSSubscriptions = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "vmorbit",
		Subsystem: "websocket",
		Name:      "subscriptions",
		Help:      "Number of active subscriptions per room.",
	}, []string{"room"})
)

// ─────────────────────────────────────────────────────────────────────────────
// Provider Metrics
// ─────────────────────────────────────────────────────────────────────────────

var (
	// ProviderOperationsTotal counts provider API calls.
	ProviderOperationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "vmorbit",
		Subsystem: "provider",
		Name:      "operations_total",
		Help:      "Total provider operations by type and provider.",
	}, []string{"provider", "operation", "status"})

	// ProviderOperationDuration observes provider call latency.
	ProviderOperationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "vmorbit",
		Subsystem: "provider",
		Name:      "operation_duration_seconds",
		Help:      "Provider operation duration in seconds.",
		Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120},
	}, []string{"provider", "operation"})

	// ProviderConnectionsActive tracks active provider connections.
	ProviderConnectionsActive = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "vmorbit",
		Subsystem: "provider",
		Name:      "connections_active",
		Help:      "Number of active provider connections.",
	}, []string{"provider"})

	// ProviderHealthStatus tracks provider health (1=healthy, 0=unhealthy).
	ProviderHealthStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "vmorbit",
		Subsystem: "provider",
		Name:      "health_status",
		Help:      "Provider health status (1=healthy, 0=unhealthy).",
	}, []string{"provider", "hypervisor_id"})
)

// ─────────────────────────────────────────────────────────────────────────────
// Inventory Metrics
// ─────────────────────────────────────────────────────────────────────────────

var (
	// InventorySyncDuration observes inventory sync duration.
	InventorySyncDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "vmorbit",
		Subsystem: "inventory",
		Name:      "sync_duration_seconds",
		Help:      "Inventory sync duration in seconds.",
		Buckets:   []float64{1, 5, 10, 30, 60, 120, 300, 600},
	}, []string{"provider"})

	// InventoryVMsTotal tracks total VMs discovered per hypervisor.
	InventoryVMsTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "vmorbit",
		Subsystem: "inventory",
		Name:      "vms_total",
		Help:      "Total VMs discovered per hypervisor.",
	}, []string{"hypervisor_id", "provider"})

	// InventorySyncsTotal counts sync operations.
	InventorySyncsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "vmorbit",
		Subsystem: "inventory",
		Name:      "syncs_total",
		Help:      "Total inventory sync operations.",
	}, []string{"provider", "status"})
)

// ─────────────────────────────────────────────────────────────────────────────
// Database Metrics
// ─────────────────────────────────────────────────────────────────────────────

var (
	// DBQueryDuration observes database query latency.
	DBQueryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "vmorbit",
		Subsystem: "database",
		Name:      "query_duration_seconds",
		Help:      "Database query duration in seconds.",
		Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
	}, []string{"operation"})

	// DBConnectionsOpen tracks open database connections.
	DBConnectionsOpen = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "vmorbit",
		Subsystem: "database",
		Name:      "connections_open",
		Help:      "Number of open database connections.",
	})

	// DBConnectionsIdle tracks idle database connections.
	DBConnectionsIdle = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "vmorbit",
		Subsystem: "database",
		Name:      "connections_idle",
		Help:      "Number of idle database connections.",
	})
)
