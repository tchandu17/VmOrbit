# VMOrbit — Operations Runbook

## Service Management

### Start all services
```bash
make docker-prod-up
# or
docker compose -f docker-compose.production.yml --env-file .env.production up -d
```

### Stop all services
```bash
make docker-prod-down
```

### Restart a single service
```bash
docker compose -f docker-compose.production.yml restart backend
docker compose -f docker-compose.production.yml restart frontend
docker compose -f docker-compose.production.yml restart nginx
```

### View logs
```bash
make docker-prod-logs
# or per service:
docker compose -f docker-compose.production.yml logs -f backend --tail=200
```

### Check service status
```bash
docker compose -f docker-compose.production.yml ps
make health-check
```

---

## Common Operational Tasks

### Enable maintenance mode (before upgrades/migrations)
```bash
curl -X POST https://vmorbit.example.com/api/v1/system/maintenance/enable \
  -H "Authorization: Bearer <admin-token>" \
  -H "Content-Type: application/json" \
  -d '{"reason": "Scheduled maintenance"}'
```

### Disable maintenance mode
```bash
curl -X POST https://vmorbit.example.com/api/v1/system/maintenance/disable \
  -H "Authorization: Bearer <admin-token>"
```

### Trigger inventory sync for a hypervisor
```bash
curl -X POST https://vmorbit.example.com/api/v1/hypervisors/<id>/sync \
  -H "Authorization: Bearer <token>"
```

### Check task queue depth
```bash
docker exec vmorbit_redis redis-cli -a <redis_password> \
  LLEN task:queue:1  # priority 1 (highest)
```

### Clear stuck tasks (emergency)
```bash
# List running tasks
curl https://vmorbit.example.com/api/v1/tasks?status=running \
  -H "Authorization: Bearer <token>"

# Cancel a specific task
curl -X DELETE https://vmorbit.example.com/api/v1/tasks/<task-id> \
  -H "Authorization: Bearer <token>"
```

---

## Troubleshooting

### Backend won't start

1. Check logs: `docker compose -f docker-compose.production.yml logs backend`
2. Verify `.env.production` has all required variables
3. Check DB connectivity: `docker exec vmorbit_postgres pg_isready -U vmorbit`
4. Check Redis: `docker exec vmorbit_redis redis-cli -a <password> ping`

### High task queue depth

1. Check queue depths: `GET /api/v1/system/health` → `tasks.queue_depths`
2. Check for stuck running tasks: `GET /api/v1/tasks?status=running`
3. Increase worker count: set `VMORBIT_TASK_ENGINE_WORKER_COUNT=20` and restart backend
4. Check provider connectivity: `GET /api/v1/providers/health`

### Database connection pool exhausted

Symptoms: `too many connections` errors in backend logs.

1. Check current connections: `GET /api/v1/system/health` → `database.open_connections`
2. Increase pool: set `VMORBIT_DATABASE_MAX_OPEN_CONNS=100` and restart
3. Check for long-running queries:
   ```sql
   SELECT pid, now() - pg_stat_activity.query_start AS duration, query
   FROM pg_stat_activity
   WHERE state = 'active' AND duration > interval '30 seconds';
   ```

### Redis memory full

1. Check memory: `GET /api/v1/system/health` → `cache.used_memory_mb`
2. Increase limit: set `REDIS_MAX_MEMORY=1gb` and restart Redis
3. Redis uses `allkeys-lru` eviction — old cache entries are evicted automatically

### WebSocket connections dropping

1. Check nginx `proxy_read_timeout` (should be 3600s for WebSocket)
2. Check backend WebSocket hub logs
3. Verify firewall allows long-lived connections

### Provider connection failures

1. Check provider health: `GET /api/v1/providers/health`
2. Test connection: `POST /api/v1/hypervisors/<id>/test-connection`
3. Common causes:
   - TLS certificate issues: set `tls_verify=false` for self-signed certs
   - Network firewall blocking port 443/8006
   - Expired credentials

---

## Performance Tuning

### PostgreSQL
The production compose file includes tuned PostgreSQL parameters. Key settings:
- `shared_buffers=256MB` — increase to 25% of RAM for dedicated DB servers
- `max_connections=200` — reduce if connection pool exhaustion occurs
- `work_mem=4MB` — increase for complex analytics queries

### Redis
- `maxmemory=512mb` — increase if task logs are being evicted
- `appendfsync=everysec` — balance between durability and performance

### Backend
- `VMORBIT_TASK_ENGINE_WORKER_COUNT` — increase for higher task throughput
- `VMORBIT_DATABASE_MAX_OPEN_CONNS` — increase for higher API concurrency
- `VMORBIT_REDIS_POOL_SIZE` — should be ≥ worker_count + 10

---

## Monitoring Alerts (Recommended)

Set up alerts for:

| Metric | Threshold | Action |
|---|---|---|
| `/ready` returns 503 | Any | Page on-call |
| DB latency | > 100ms | Investigate queries |
| Redis memory | > 80% | Increase limit |
| Task queue depth | > 500 | Check workers |
| Running tasks | > 50 | Check for stuck tasks |
| Goroutines | > 1000 | Check for goroutine leak |
| Heap memory | > 400MB | Check for memory leak |
