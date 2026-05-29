# VMOrbit — Production Deployment Guide

## Prerequisites

- Docker Engine 24+ and Docker Compose v2
- A Linux server with at least 4 GB RAM and 20 GB disk
- A domain name with DNS pointing to your server
- TLS certificate (Let's Encrypt recommended)

---

## Quick Start

### 1. Clone and configure

```bash
git clone <repo> vmorbit && cd vmorbit

# Copy and fill in the environment file
cp .env.production.example .env.production
nano .env.production
```

### 2. Generate secrets

```bash
make gen-secrets
# Copy the output into .env.production
```

### 3. Set up TLS certificates

```bash
# Option A: Let's Encrypt (recommended)
certbot certonly --standalone -d vmorbit.example.com
cp /etc/letsencrypt/live/vmorbit.example.com/fullchain.pem nginx/ssl/
cp /etc/letsencrypt/live/vmorbit.example.com/privkey.pem nginx/ssl/

# Option B: Self-signed (dev/test only)
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout nginx/ssl/privkey.pem -out nginx/ssl/fullchain.pem \
  -subj "/CN=vmorbit.example.com"
```

### 4. Deploy

```bash
make deploy
# Or manually:
docker compose -f docker-compose.production.yml --env-file .env.production up -d
```

### 5. Verify

```bash
make health-check
# Or:
curl https://vmorbit.example.com/health
curl https://vmorbit.example.com/ready
curl https://vmorbit.example.com/status
```

---

## Environment Variables Reference

| Variable | Required | Description |
|---|---|---|
| `VMORBIT_JWT_SECRET` | ✅ | JWT signing secret (min 32 chars, use `openssl rand -base64 64`) |
| `VMORBIT_ENCRYPTION_KEY` | ✅ | AES-256 key for provider credentials (64 hex chars, use `openssl rand -hex 32`) |
| `DB_PASSWORD` | ✅ | PostgreSQL password |
| `REDIS_PASSWORD` | ✅ | Redis password |
| `VMORBIT_SERVER_CORS_ORIGINS` | ✅ | Comma-separated allowed origins (e.g. `https://vmorbit.example.com`) |
| `NEXT_PUBLIC_API_URL` | ✅ | Public URL for the frontend (e.g. `https://vmorbit.example.com`) |
| `BACKUP_RETENTION_DAYS` | ❌ | Days to keep backups (default: 7) |
| `REDIS_MAX_MEMORY` | ❌ | Redis memory limit (default: 512mb) |

---

## Architecture

```
Internet
    │
    ▼
[Nginx :443]  ← TLS termination, rate limiting, security headers
    │
    ├── /api/*  ──────────────────► [Backend :8080]  ← Go API server
    ├── /ws     ──────────────────► [Backend :8080]  ← WebSocket
    ├── /health, /ready, /status ► [Backend :8080]  ← Probes
    └── /*      ──────────────────► [Frontend :3001] ← Next.js
                                         │
                                    [PostgreSQL :5432]
                                    [Redis :6379]
                                    [Backup service]
```

---

## Health Probes

| Endpoint | Auth | Purpose |
|---|---|---|
| `GET /health` | None | Liveness — always 200 if process is alive |
| `GET /ready` | None | Readiness — 200 only when DB + Redis are reachable |
| `GET /status` | None | Extended status with dependency health and runtime metrics |
| `GET /metrics` | IP-restricted | Prometheus metrics |
| `GET /api/v1/system/health` | JWT | Deep system health dashboard data |

---

## Scaling Recommendations

### Vertical scaling (single node)
- Backend: 2–4 vCPU, 1–2 GB RAM handles ~500 concurrent users
- PostgreSQL: 4 vCPU, 4 GB RAM, SSD storage
- Redis: 1 vCPU, 1 GB RAM

### Horizontal scaling
The backend is stateless (sessions in Redis) and can be scaled horizontally:

```yaml
# In docker-compose.production.yml, add replicas:
backend:
  deploy:
    replicas: 3
```

Update nginx upstream to round-robin across instances.

### Task engine scaling
Increase `VMORBIT_TASK_ENGINE_WORKER_COUNT` (default 10) for higher task throughput.
Each worker holds one Redis connection — ensure `VMORBIT_REDIS_POOL_SIZE` ≥ workers + 5.

---

## Updating

```bash
# Pull latest code
git pull

# Deploy new version (builds images, runs rolling update)
make deploy

# Or with explicit version
APP_VERSION=v1.2.3 make deploy
```

### Rollback

```bash
make deploy-rollback
```

---

## Maintenance Mode

Enable maintenance mode before schema migrations or major upgrades:

```bash
# Via API (requires admin JWT)
curl -X POST https://vmorbit.example.com/api/v1/system/maintenance/enable \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"reason": "Database migration in progress"}'

# Via UI: Dashboard → Administration → Maintenance Mode
```

---

## Monitoring

### Prometheus + Grafana

The `/metrics` endpoint exposes Prometheus metrics. Add to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: vmorbit
    static_configs:
      - targets: ['vmorbit-server:8080']
    metrics_path: /metrics
```

### Log aggregation

Backend logs are structured JSON (when `VMORBIT_LOG_FORMAT=json`). Ship to your log aggregator:

```bash
# View logs
make docker-prod-logs

# Or per service
docker compose -f docker-compose.production.yml logs -f backend
```

---

## Security Hardening Checklist

- [ ] `VMORBIT_ENCRYPTION_KEY` set to a random 32-byte hex value
- [ ] `VMORBIT_JWT_SECRET` set to a random 64-byte base64 value
- [ ] Strong passwords for PostgreSQL and Redis
- [ ] HTTPS enabled with valid TLS certificate
- [ ] CORS origins restricted to your domain
- [ ] `/metrics` endpoint restricted to internal networks (nginx config)
- [ ] Firewall: only ports 80 and 443 exposed externally
- [ ] PostgreSQL and Redis not exposed on public interfaces
- [ ] Regular backup verification
- [ ] RBAC policies configured before go-live
