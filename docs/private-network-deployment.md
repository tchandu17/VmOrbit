# VmOrbit — Private Network Deployment Guide

Complete step-by-step guide to deploy VmOrbit on a private/internal network (no public IP, no domain name required).

---

## Overview

This guide is for deploying VmOrbit on an internal network where:
- The server has a **private IP only** (e.g., `192.168.1.50`, `10.0.0.100`)
- No public internet exposure is needed
- Users access the dashboard from the same LAN/VPN
- No domain name or DNS is required

---

## Prerequisites

### Server Requirements

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| CPU | 2 cores | 4 cores |
| RAM | 4 GB | 8 GB |
| Disk | 40 GB SSD | 100 GB SSD |
| OS | Ubuntu 22.04 LTS / Debian 12 / RHEL 9 | Ubuntu 24.04 LTS |
| Network | Private/static IP on your LAN | Static IP recommended |

### What You Need

- A Linux server on your internal network
- SSH access to the server
- The server's private IP address (run `ip addr` or `hostname -I` to find it)

### What You Do NOT Need

- A public IP address
- A domain name
- Let's Encrypt / Certbot
- DNS configuration

---

## Step 1: Server Initial Setup

SSH into your server from another machine on the same network:

```bash
ssh root@192.168.1.50
```

> Replace `192.168.1.50` with your server's actual private IP throughout this guide.

### 1.1 Update the system

```bash
apt update && apt upgrade -y
```

### 1.2 Create a non-root user (recommended)

```bash
adduser vmorbit
usermod -aG sudo vmorbit
su - vmorbit
```

### 1.3 Configure firewall

Only allow SSH and web traffic from your internal network:

```bash
sudo ufw allow OpenSSH
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
sudo ufw status
```

> **Optional (more restrictive):** If you want to limit access to only your subnet:
> ```bash
> sudo ufw allow from 192.168.1.0/24 to any port 80
> sudo ufw allow from 192.168.1.0/24 to any port 443
> ```

---

## Step 2: Install Required Software

### 2.1 Install Docker Engine

```bash
# Remove old versions
sudo apt remove docker docker-engine docker.io containerd runc 2>/dev/null

# Install prerequisites
sudo apt install -y ca-certificates curl gnupg lsb-release

# Add Docker's official GPG key
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg

# Add Docker repository
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# Install Docker
sudo apt update
sudo apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# Add your user to docker group
sudo usermod -aG docker $USER
newgrp docker

# Verify installation
docker --version
docker compose version
```

### 2.2 Install Git

```bash
sudo apt install -y git
```

### 2.3 Install OpenSSL (for self-signed certificates)

Usually pre-installed, but verify:

```bash
openssl version
```

> **Note:** We do NOT need Certbot. Let's Encrypt requires a public domain — we'll use self-signed certificates instead.

---

## Step 3: Clone the Repository

```bash
cd /opt
sudo mkdir vmorbit && sudo chown $USER:$USER vmorbit
git clone https://github.com/tchandu17/VmOrbit.git vmorbit
cd vmorbit
```

---

## Step 4: Generate Self-Signed SSL Certificates

Since we're on a private network without a domain, we generate self-signed certificates. The `nginx/ssl/` directory already exists in the cloned repository.

```bash
# Generate a self-signed certificate valid for 10 years
openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
  -keyout nginx/ssl/privkey.pem \
  -out nginx/ssl/fullchain.pem \
  -subj "/C=US/ST=Local/L=Local/O=VmOrbit/CN=192.168.1.50" \
  -addext "subjectAltName=IP:192.168.1.50"
```

> **Important:** Replace `192.168.1.50` with your server's actual private IP in both the `-subj` and `-addext` fields.

### Verify the certificate was created

```bash
ls -la nginx/ssl/
# Should show fullchain.pem and privkey.pem
```

### (Optional) Eliminate browser SSL warnings

Since the certificate is self-signed, browsers will show a security warning. To avoid this, you can distribute the certificate to client machines:

**On client machines (Windows):**
1. Copy `nginx/ssl/fullchain.pem` to the client
2. Rename to `vmorbit-ca.crt`
3. Double-click → Install Certificate → Local Machine → Trusted Root Certification Authorities

**On client machines (macOS):**
```bash
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain vmorbit-ca.crt
```

**On client machines (Linux):**
```bash
sudo cp vmorbit-ca.crt /usr/local/share/ca-certificates/
sudo update-ca-certificates
```

---

## Step 5: Configure Environment Variables

### 5.1 Create the production env file

```bash
cp .env.production.example .env.production
```

### 5.2 Generate secrets

```bash
# Generate a JWT secret
openssl rand -base64 64

# Generate an encryption key for provider credentials
openssl rand -hex 32

# Generate a strong password for the database
openssl rand -base64 32

# Generate a strong password for Redis
openssl rand -base64 32
```

Copy each output — you'll paste them into the config file next.

### 5.3 Edit `.env.production`

```bash
nano .env.production
```

Here's the complete configuration — replace placeholder values with your generated secrets and your server's private IP:

```env
# ── Application
APP_VERSION=1.0.0

# ── Database
DB_USER=vmorbit
DB_PASSWORD=<paste-your-generated-db-password>
DB_NAME=vmorbit
VMORBIT_DATABASE_USER=vmorbit
VMORBIT_DATABASE_PASSWORD=<paste-your-generated-db-password>
VMORBIT_DATABASE_DBNAME=vmorbit
VMORBIT_DATABASE_SSLMODE=disable
VMORBIT_DATABASE_MAX_OPEN_CONNS=50
VMORBIT_DATABASE_MAX_IDLE_CONNS=15
VMORBIT_DATABASE_CONN_MAX_LIFETIME=5m

# ── Redis
REDIS_PASSWORD=<paste-your-generated-redis-password>
REDIS_MAX_MEMORY=512mb
VMORBIT_REDIS_PASSWORD=<paste-your-generated-redis-password>
VMORBIT_REDIS_DB=0
VMORBIT_REDIS_POOL_SIZE=20

# ── JWT
VMORBIT_JWT_SECRET=<paste-your-generated-jwt-secret>
VMORBIT_JWT_ACCESS_TOKEN_TTL=15m
VMORBIT_JWT_REFRESH_TOKEN_TTL=168h
VMORBIT_JWT_ISSUER=vmOrbit

# ── Server
VMORBIT_SERVER_PORT=8080
VMORBIT_SERVER_MODE=release
VMORBIT_SERVER_READ_TIMEOUT=15s
VMORBIT_SERVER_WRITE_TIMEOUT=15s
VMORBIT_SERVER_IDLE_TIMEOUT=60s

# ── CORS & Frontend URL — USE YOUR PRIVATE IP HERE
VMORBIT_SERVER_CORS_ORIGINS=https://192.168.1.50
NEXT_PUBLIC_API_URL=https://192.168.1.50

# ── Encryption
VMORBIT_ENCRYPTION_KEY=<paste-your-generated-hex-key>

# ── Task Engine
VMORBIT_TASK_ENGINE_WORKER_COUNT=10
VMORBIT_TASK_ENGINE_QUEUE_SIZE=1000
VMORBIT_TASK_ENGINE_MAX_RETRIES=3
VMORBIT_TASK_ENGINE_DEFAULT_TIMEOUT=10m

# ── Logging
VMORBIT_LOG_LEVEL=info
VMORBIT_LOG_FORMAT=json

# ── Metrics
VMORBIT_METRICS_ENABLED=true
VMORBIT_METRICS_PATH=/metrics

# ── Backup
BACKUP_RETENTION_DAYS=7
```

> **Key difference from public deployment:** `VMORBIT_SERVER_CORS_ORIGINS` and `NEXT_PUBLIC_API_URL` use `https://192.168.1.50` (your private IP) instead of a domain name.

Save and exit (`Ctrl+X`, then `Y`, then `Enter` in nano).

---

## Step 6: Build and Start the Application

### 6.1 Build all containers

```bash
docker compose -f docker-compose.production.yml --env-file .env.production build
```

This will:
- Build the Go backend (multi-stage, produces a ~15MB static binary)
- Build the Next.js frontend (standalone output, with your private IP baked in)
- Pull PostgreSQL 16, Redis 7, and Nginx images

> **Note:** The first build may take 5–10 minutes depending on your server's internet speed and CPU.

### 6.2 Start the stack

```bash
docker compose -f docker-compose.production.yml --env-file .env.production up -d
```

### 6.3 Verify all containers are running

```bash
docker compose -f docker-compose.production.yml ps
```

Expected output — all services should show `Up` or `healthy`:

```
NAME                STATUS              PORTS
vmorbit_backend     Up (healthy)        8080/tcp
vmorbit_frontend    Up (healthy)        3001/tcp
vmorbit_nginx       Up (healthy)        0.0.0.0:80->80/tcp, 0.0.0.0:443->443/tcp
vmorbit_postgres    Up (healthy)        5432/tcp
vmorbit_redis       Up (healthy)        6379/tcp
vmorbit_backup      Up
```

### 6.4 Check logs if something is wrong

```bash
# All services
docker compose -f docker-compose.production.yml logs

# Specific service
docker compose -f docker-compose.production.yml logs backend
docker compose -f docker-compose.production.yml logs frontend
docker compose -f docker-compose.production.yml logs postgres
```

---

## Step 7: Initialize the Database

The backend runs GORM AutoMigrate on startup, so tables are created automatically. To seed initial data (admin user, default roles):

```bash
docker compose -f docker-compose.production.yml exec backend /vmorbit seed
```

If that doesn't work, try:

```bash
docker run --rm --network vmorbit_vmorbit_net \
  --env-file .env.production \
  -e VMORBIT_DATABASE_HOST=postgres \
  -e VMORBIT_REDIS_HOST=redis \
  vmorbit/backend:latest /vmorbit seed
```

---

## Step 8: Verify the Deployment

### 8.1 Health check (from the server itself)

```bash
curl -k https://192.168.1.50/health
# Expected: {"status":"ok"}
```

> The `-k` flag tells curl to accept the self-signed certificate.

### 8.2 Health check (from another machine on the network)

```bash
curl -k https://192.168.1.50/health
```

### 8.3 Open in browser

From any machine on the same network, navigate to:

```
https://192.168.1.50
```

Your browser will show a certificate warning (because it's self-signed). Click **Advanced** → **Proceed** (or **Accept the Risk**) to continue.

You should see the VmOrbit login page.

### 8.4 Default credentials (from seed)

- **Username:** `admin@vmorbit.local`
- **Password:** `admin123`

> **Change this password immediately after first login.**

---

## Step 9: Post-Deployment Configuration

### 9.1 Add your first hypervisor

1. Log in to the dashboard
2. Go to **Hypervisors** → **Add Hypervisor**
3. Enter your vCenter/Proxmox/ESXi connection details
4. Click **Test Connection** to verify
5. Click **Sync** to pull inventory

### 9.2 Configure backups

The `vmorbit_backup` container runs automatic daily PostgreSQL backups with 7-day retention. Backups are stored in the `backup_data` Docker volume.

To manually trigger a backup:

```bash
docker compose -f docker-compose.production.yml exec backup \
  pg_dump -U vmorbit -d vmorbit > /tmp/manual_backup_$(date +%Y%m%d).sql
```

---

## Common Operations

### Restart the stack

```bash
docker compose -f docker-compose.production.yml --env-file .env.production restart
```

### Update to a new version

```bash
cd /opt/vmorbit
git pull origin main
docker compose -f docker-compose.production.yml --env-file .env.production build
docker compose -f docker-compose.production.yml --env-file .env.production up -d
```

### View real-time logs

```bash
docker compose -f docker-compose.production.yml logs -f backend
```

### Stop everything

```bash
docker compose -f docker-compose.production.yml down
```

### Stop and remove all data (destructive!)

```bash
docker compose -f docker-compose.production.yml down -v
```

---

## Changing the Server IP

If your server's IP changes (e.g., DHCP reassignment), you need to:

1. **Regenerate the SSL certificate** with the new IP:
   ```bash
   openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
     -keyout nginx/ssl/privkey.pem \
     -out nginx/ssl/fullchain.pem \
     -subj "/C=US/ST=Local/L=Local/O=VmOrbit/CN=NEW_IP" \
     -addext "subjectAltName=IP:NEW_IP"
   ```

2. **Update `.env.production`:**
   ```env
   VMORBIT_SERVER_CORS_ORIGINS=https://NEW_IP
   NEXT_PUBLIC_API_URL=https://NEW_IP
   ```

3. **Rebuild and restart** (frontend needs rebuild because `NEXT_PUBLIC_API_URL` is baked in at build time):
   ```bash
   docker compose -f docker-compose.production.yml --env-file .env.production build frontend
   docker compose -f docker-compose.production.yml --env-file .env.production up -d
   ```

> **Tip:** Use a static IP assignment on your server to avoid this. Configure it in `/etc/netplan/` (Ubuntu) or `/etc/network/interfaces` (Debian).

---

## Optional: HTTP-Only Mode (No SSL)

If your network is fully trusted (isolated lab, air-gapped environment), you can skip HTTPS entirely. This requires modifying the Nginx configuration.

### Steps:

1. **Edit `nginx/nginx.conf`** — replace the SSL server block with a plain HTTP block:
   ```nginx
   server {
       listen 80;
       server_name _;

       # ... keep all location blocks the same, just remove SSL directives
   }
   ```

2. **Update `.env.production`:**
   ```env
   VMORBIT_SERVER_CORS_ORIGINS=http://192.168.1.50
   NEXT_PUBLIC_API_URL=http://192.168.1.50
   ```

3. **Rebuild:**
   ```bash
   docker compose -f docker-compose.production.yml --env-file .env.production build
   docker compose -f docker-compose.production.yml --env-file .env.production up -d
   ```

4. **Access via:** `http://192.168.1.50`

> **Warning:** HTTP mode transmits login credentials and hypervisor passwords in plain text over the network. Only use this in fully isolated/trusted environments.

---

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Backend can't connect to DB | Check `VMORBIT_DATABASE_PASSWORD` matches `DB_PASSWORD` in `.env.production` |
| Frontend shows blank page | Verify `NEXT_PUBLIC_API_URL` matches your server's IP with correct protocol |
| 502 Bad Gateway | Backend hasn't started yet — check `docker logs vmorbit_backend` |
| Browser shows "connection refused" | Verify firewall allows port 443 from your subnet |
| Browser SSL warning | Expected with self-signed certs — click through, or install the cert on clients |
| WebSocket not connecting | Ensure nginx config has the `/ws` location block (it does by default) |
| Container keeps restarting | Check logs: `docker compose -f docker-compose.production.yml logs <service>` |
| Can't access from other machines | Check firewall rules and that client is on the same network/VLAN |
| IP changed, app broken | Follow the "Changing the Server IP" section above |

---

## Architecture Diagram

```
Internal Network (LAN/VPN)
    │
    ▼
┌─────────────────────────────────────────┐
│  Nginx (ports 80, 443)                  │
│  - TLS termination (self-signed cert)   │
│  - Rate limiting                        │
│  - Security headers                     │
│  - HTTP → HTTPS redirect                │
├─────────────────────────────────────────┤
│         │                    │           │
│         ▼                    ▼           │
│  ┌─────────────┐    ┌──────────────┐    │
│  │  Backend    │    │  Frontend    │    │
│  │  (Go:8080)  │    │  (Next:3001) │    │
│  └──────┬──────┘    └──────────────┘    │
│         │                               │
│    ┌────┴────┐                          │
│    ▼         ▼                          │
│ ┌────────┐ ┌───────┐                   │
│ │Postgres│ │ Redis │                   │
│ │ :5432  │ │ :6379 │                   │
│ └────────┘ └───────┘                   │
└─────────────────────────────────────────┘
     Docker Network (vmorbit_net)
     Server IP: 192.168.1.50
```

---

## Summary of Differences from Public Deployment

| Aspect | Public Deployment | Private Network |
|--------|-------------------|-----------------|
| IP | Public IP | Private IP (e.g., 192.168.1.x) |
| Domain | Required | Not needed |
| SSL | Let's Encrypt (free, auto-renew) | Self-signed (10-year validity) |
| Certbot | Required | Not needed |
| DNS | A record pointing to server | Not needed |
| Browser warning | None | Yes (unless cert is installed on clients) |
| Access | Anywhere on internet | Same LAN/VPN only |
| Firewall | Open to world | Restrict to subnet |

---

## Software Installed on Host

| Software | Version | Purpose |
|----------|---------|---------|
| Docker Engine | 24+ | Container runtime |
| Docker Compose | v2+ | Multi-container orchestration |
| Git | 2.x | Clone repository |
| OpenSSL | (pre-installed) | Generate self-signed certificates |

All other software (Go, Node.js, PostgreSQL, Redis, Nginx) runs inside Docker containers — no additional host installation needed.
