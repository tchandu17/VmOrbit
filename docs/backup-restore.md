# VMOrbit — Backup & Restore Procedures

## Overview

VMOrbit uses automated PostgreSQL backups via the `backup` Docker service.

- **Schedule**: Daily at 02:00 UTC
- **Format**: Compressed SQL dump (`.sql.gz`)
- **Retention**: Configurable via `BACKUP_RETENTION_DAYS` (default: 7 days)
- **Storage**: `./backups/` directory on the host

---

## Automated Backups

The backup service starts automatically with the production stack:

```bash
docker compose -f docker-compose.production.yml up -d backup
```

Backup files are stored in `./backups/` with names like:
```
vmorbit_20260525_020000.sql.gz
vmorbit_20260525_020000-pre-deploy-v1.2.3.sql.gz  # labeled backup
```

---

## Manual Backup

```bash
# Basic backup
make backup
# or
./scripts/backup.sh

# Labeled backup (useful before upgrades)
./scripts/backup.sh --label pre-upgrade-v1.2.3

# Via Docker (if running in container)
docker exec vmorbit_postgres pg_dump -U vmorbit vmorbit | gzip > backup.sql.gz
```

---

## Restore Procedure

> ⚠️ **Warning**: Restore drops and recreates the database. Stop all services first.

### Step 1: Stop the backend

```bash
docker compose -f docker-compose.production.yml stop backend frontend
```

### Step 2: Restore

```bash
# Using the backup script (interactive confirmation required)
./scripts/backup.sh --restore ./backups/vmorbit_20260525_020000.sql.gz

# Or using make
BACKUP_FILE=./backups/vmorbit_20260525_020000.sql.gz make restore

# Manual restore
PGPASSWORD=<db_password> gunzip -c ./backups/vmorbit_20260525_020000.sql.gz | \
  psql -h localhost -U vmorbit -d vmorbit
```

### Step 3: Restart services

```bash
docker compose -f docker-compose.production.yml start backend frontend
```

### Step 4: Verify

```bash
curl https://vmorbit.example.com/ready
# Should return: {"status":"ready",...}
```

---

## Backup Verification

Backups are automatically verified with `gunzip -t` after creation. To manually verify:

```bash
gunzip -t ./backups/vmorbit_20260525_020000.sql.gz && echo "OK" || echo "CORRUPT"
```

---

## Backup Rotation

Old backups are automatically deleted after `BACKUP_RETENTION_DAYS` days. To manually clean up:

```bash
find ./backups -name "vmorbit_*.sql.gz" -mtime +7 -delete
```

---

## Off-site Backup (Recommended)

For production, copy backups to an off-site location:

```bash
# Example: sync to S3
aws s3 sync ./backups/ s3://your-bucket/vmorbit-backups/ \
  --exclude "*" --include "*.sql.gz"

# Example: rsync to remote server
rsync -avz ./backups/ backup-server:/backups/vmorbit/
```

Add this to a cron job after the daily backup completes.

---

## Disaster Recovery RTO/RPO

| Scenario | RPO | RTO |
|---|---|---|
| Single service failure | 0 (Redis/DB still running) | < 1 min (container restart) |
| Full server failure | Up to 24h (last backup) | 30–60 min (restore + restart) |
| Database corruption | Up to 24h (last backup) | 15–30 min (restore procedure) |

To reduce RPO, increase backup frequency by modifying `backup-entrypoint.sh`.
