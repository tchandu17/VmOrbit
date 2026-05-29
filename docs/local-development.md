# VMOrbit — Local Development Guide

## What Went Wrong (and Why)

The `indexes.go` file contained **5 SQL index definitions that referenced columns which don't exist** on their respective database tables. PostgreSQL rejected them at startup, causing a fatal crash before the server could bind to a port.

The specific mismatches were:

| Index | Problem |
|---|---|
| `idx_platform_events_created` | `platform_events` table has no `deleted_at` column — it is an append-only event log, not a soft-deletable entity |
| `idx_platform_events_hypervisor_created` | Same — `deleted_at` does not exist |
| `idx_infra_metrics_hypervisor_collected` | `infrastructure_metrics` is a **platform-wide aggregate** — it has no `hypervisor_id` column |
| `idx_capacity_history_hypervisor_recorded` | The column is named `collected_at`, not `recorded_at` |
| `idx_vm_usage_stats_vm_collected` | `vm_usage_stats` is a single upserted row per VM — it has no `collected_at` time-series column |

All five have been fixed in `internal/infrastructure/database/indexes.go`.

---

## Architecture Overview

VMOrbit is **two separate processes** that must both be running:

```
Browser
  │
  ▼
Frontend (Next.js)  :3001   ← what you open in the browser
  │  proxies /api/* calls
  ▼
Backend (Go/Gin)    :8080   ← REST API + WebSocket, no HTML
  │
  ├── PostgreSQL    :5432   ← via Docker
  └── Redis         :6380   ← via Docker (host port 6380 → container 6379)
```

Opening `http://localhost:8080` directly in a browser gives "404 page not found" — that is expected. The backend only serves `/api/v1/...` routes. **Always use `http://localhost:3001`.**

---

## Prerequisites

- [Go 1.22+](https://go.dev/dl/)
- [Node.js 20+](https://nodejs.org/) and npm
- [Docker Desktop](https://www.docker.com/products/docker-desktop/) (for Postgres + Redis)

---

## Step-by-Step: Bring the Application Up

### Step 1 — Start the database and cache

```bash
# From the project root (D:\VMOrbit)
make docker-up
```

This starts two Docker containers:
- `vmorbit_postgres` — PostgreSQL 16 on port **5432**
- `vmorbit_redis` — Redis 7 on port **6380**

Wait a few seconds for them to become healthy. You can verify:

```bash
docker compose ps
```

Both should show `healthy`.

### Step 2 — Start the backend

Open a terminal in `D:\VMOrbit` and run:

```bash
make run
```

The server starts on **http://localhost:8080**. You will see Gin route registration logs followed by:

```
{"level":"info","msg":"VmOrbit server starting","addr":":8080"}
```

You can verify it is healthy:

```
http://localhost:8080/health   → {"service":"vmOrbit","status":"ok",...}
http://localhost:8080/ready    → {"status":"ready",...}
```

> **Note:** The warning `VMORBIT_ENCRYPTION_KEY is not set` is safe to ignore locally.
> It uses a built-in development key. Never ignore this in production.

### Step 3 — Start the frontend

Open a **second terminal** in `D:\VMOrbit\frontend` and run:

```bash
# First time only — install dependencies
npm install

# Start the dev server
npm run dev
```

The frontend starts on **http://localhost:3001**.

### Step 4 — Open the app

Navigate to **http://localhost:3001** in your browser.

---

## Default Login

The seed command creates a default admin user. If you haven't seeded yet:

```bash
# From D:\VMOrbit
go run ./cmd/seed/main.go
```

Then log in at `http://localhost:3001` with the credentials printed by the seed command.

---

## Stopping Everything

```bash
# Stop Docker containers (Postgres + Redis)
make docker-down

# Stop the backend: Ctrl+C in its terminal
# Stop the frontend: Ctrl+C in its terminal
```

---

## Quick Reference

| Command | What it does |
|---|---|
| `make docker-up` | Start Postgres + Redis |
| `make docker-down` | Stop Postgres + Redis |
| `make run` | Start the Go backend on :8080 |
| `npm run dev` (in `frontend/`) | Start the Next.js frontend on :3001 |
| `go run ./cmd/seed/main.go` | Seed the database with initial data |
| `go run ./cmd/migrate/main.go` | Run migrations manually |
| `make build` | Compile the backend binary to `bin/vmorbit` |
| `make test` | Run all Go tests |

---

## Troubleshooting

### "port already in use" on :8080

A previous backend process is still running. Find and kill it:

```powershell
# Find the PID
netstat -ano | findstr ":8080"

# Kill it (replace 1234 with the actual PID)
taskkill /PID 1234 /F
```

### "port already in use" on :3001

Same approach:

```powershell
netstat -ano | findstr ":3001"
taskkill /PID 1234 /F
```

### Backend crashes at startup with "column does not exist"

This means the database has stale schema from before the index fix. Drop and recreate:

```bash
make docker-down
docker volume rm vmorbit_postgres_data
make docker-up
make run
```

### Frontend shows "Backend unreachable" (502)

The backend is not running. Start it with `make run` first.

### Docker containers not starting

Make sure Docker Desktop is running, then:

```bash
docker compose down -v
make docker-up
```
