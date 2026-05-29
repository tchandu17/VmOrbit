# VMOrbit — Monitoring & Observability Setup

Guide for deploying the Prometheus + Grafana monitoring stack alongside VMOrbit.

---

## Overview

VMOrbit exposes Prometheus metrics at `/metrics` and includes a pre-configured monitoring stack:

| Component | Purpose | Port |
|-----------|---------|------|
| Prometheus | Metrics collection & alerting | 9090 (localhost only) |
| Grafana | Dashboards & visualization | 3000 (localhost only) |
| Backend `/metrics` | Prometheus scrape target | 8080 (internal) |

---

## Quick Start

### 1. Start the monitoring stack

```bash
# Start production + monitoring together
docker compose \
  -f docker-compose.production.yml \
  -f docker-compose.monitoring.yml \
  --env-file .env.production up -d

# Or use the Makefile shortcut
make docker-monitoring-up
```

### 2. Access Grafana

- **URL:** `http://localhost:3000/grafana/` (or via nginx at `/grafana/` from internal networks)
- **Default credentials:** `admin` / value of `GRAFANA_ADMIN_PASSWORD` in `.env.production`
- **Change the password** on first login

### 3. Access Prometheus

- **URL:** `http://localhost:9090` (localhost only — not exposed externally)
- Use for ad-hoc queries and alert rule testing

---

## Metrics Exposed

### HTTP Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `vmorbit_http_requests_total` | Counter | method, path, status | Total HTTP requests |
| `vmorbit_http_request_duration_seconds` | Histogram | method, path | Request latency |
| `vmorbit_http_requests_in_flight` | Gauge | — | Currently active requests |

### Task Engine Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `vmorbit_tasks_total` | Counter | type, status | Tasks processed |
| `vmorbit_tasks_duration_seconds` | Histogram | type | Task execution time |
| `vmorbit_tasks_queue_depth` | Gauge | priority | Queue depth per priority |
| `vmorbit_tasks_workers_active` | Gauge | — | Active worker count |
| `vmorbit_tasks_retries_total` | Counter | type | Retry attempts |

### WebSocket Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `vmorbit_websocket_connections_active` | Gauge | — | Active WS connections |
| `vmorbit_websocket_messages_total` | Counter | room, direction | Messages sent/received |
| `vmorbit_websocket_subscriptions` | Gauge | room | Subscriptions per room |

### Provider Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `vmorbit_provider_operations_total` | Counter | provider, operation, status | Provider API calls |
| `vmorbit_provider_operation_duration_seconds` | Histogram | provider, operation | Call latency |
| `vmorbit_provider_connections_active` | Gauge | provider | Active connections |
| `vmorbit_provider_health_status` | Gauge | provider, hypervisor_id | Health (1=up, 0=down) |

### Inventory Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `vmorbit_inventory_sync_duration_seconds` | Histogram | provider | Sync duration |
| `vmorbit_inventory_vms_total` | Gauge | hypervisor_id, provider | VMs per hypervisor |
| `vmorbit_inventory_syncs_total` | Counter | provider, status | Sync operations |

### Database Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `vmorbit_database_query_duration_seconds` | Histogram | operation | Query latency |
| `vmorbit_database_connections_open` | Gauge | — | Open connections |
| `vmorbit_database_connections_idle` | Gauge | — | Idle connections |

### Go Runtime Metrics (automatic)

Standard Go metrics are also exposed: `go_goroutines`, `go_memstats_*`, `process_*`.

---

## Pre-configured Dashboards

### VMOrbit Production Overview

Located at: `monitoring/grafana/dashboards/vmorbit-overview.json`

Panels:
- Service status, uptime, request rate, error rate
- WebSocket connections, active workers
- Request latency percentiles (p50, p95, p99)
- Requests by status code
- Task queue depth and duration
- Provider operation latency and health
- Heap memory, goroutines, DB connections

---

## Alert Rules

Pre-configured alerts in `monitoring/prometheus/rules/vmorbit.yml`:

| Alert | Condition | Severity |
|-------|-----------|----------|
| VMOrbitBackendDown | Backend unreachable for 1m | Critical |
| HighErrorRate | >5% 5xx responses for 5m | Warning |
| HighLatency | p95 > 2s for 5m | Warning |
| TaskQueueBacklog | Queue > 500 for 5m | Warning |
| TaskFailureRate | >10% failures for 10m | Warning |
| ProviderUnhealthy | Provider down for 5m | Warning |
| ProviderHighLatency | p95 > 30s for 5m | Warning |
| HighMemoryUsage | Heap > 400MB for 10m | Warning |
| HighGoroutineCount | > 1000 for 5m | Warning |
| DatabaseConnectionPoolExhausted | > 90% capacity for 5m | Critical |

### Adding Alertmanager (optional)

To receive alert notifications, add Alertmanager:

```yaml
# In docker-compose.monitoring.yml, add:
alertmanager:
  image: prom/alertmanager:v0.27.0
  container_name: vmorbit_alertmanager
  volumes:
    - ./monitoring/alertmanager/alertmanager.yml:/etc/alertmanager/alertmanager.yml:ro
  ports:
    - "127.0.0.1:9093:9093"
  networks:
    - vmorbit_net
```

Then uncomment the `alerting` section in `monitoring/prometheus/prometheus.yml`.

---

## Customization

### Adding custom dashboards

Place JSON dashboard files in `monitoring/grafana/dashboards/`. They are auto-provisioned on Grafana startup.

### Modifying scrape intervals

Edit `monitoring/prometheus/prometheus.yml`:
```yaml
global:
  scrape_interval: 15s  # Change to 5s for higher resolution (more storage)
```

### Data retention

Prometheus retains data for 30 days by default. Adjust in `docker-compose.monitoring.yml`:
```yaml
command:
  - "--storage.tsdb.retention.time=90d"
  - "--storage.tsdb.retention.size=20GB"
```

---

## Useful PromQL Queries

```promql
# Request rate per endpoint
sum(rate(vmorbit_http_requests_total[5m])) by (path)

# Slowest endpoints (p95)
histogram_quantile(0.95, sum(rate(vmorbit_http_request_duration_seconds_bucket[5m])) by (le, path))

# Task throughput
sum(rate(vmorbit_tasks_total[5m])) by (type)

# Provider error rate
sum(rate(vmorbit_provider_operations_total{status="error"}[5m])) by (provider)

# Memory growth rate
deriv(go_memstats_heap_alloc_bytes{job="vmorbit-backend"}[1h])
```

---

## Stopping the Monitoring Stack

```bash
make docker-monitoring-down
# or
docker compose -f docker-compose.monitoring.yml down
```

Data is persisted in Docker volumes (`prometheus_data`, `grafana_data`). To remove all monitoring data:

```bash
docker compose -f docker-compose.monitoring.yml down -v
```
