# VmOrbit — Production Deployment Guide

Complete step-by-step guide to deploy VmOrbit on a production Linux server using Docker.

---

## Prerequisites

### Server Requirements

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| CPU | 2 cores | 4 cores |
| RAM | 4 GB | 8 GB |
| Disk | 40 GB SSD | 100 GB SSD |
| OS | Ubuntu 22.04 LTS / Debian 12 / RHEL 9 | Ubuntu 24.04 LTS |
| Network | Public IP with ports 80, 443 open | Static IP |

### Domain Name

You need a domain (e.g., `vmorbit.yourdomain.com`) pointed to your server's public IP via DNS A record.

---

## Step 1: Server Initial Setup

SSH into your server:

```bash
ssh root@your-server-ip
```

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

```bash
sudo ufw allow OpenSSH
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
sudo ufw status
```

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

# Add your user to docker group (avoids needing sudo for docker commands)
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

### 2.3 Install Certbot (for free SSL certificates)

```bash
sudo apt install -y certbot
```

---

## Step 3: Clone the Repository

```bash
cd /opt
sudo mkdir vmorbit && sudo chown $USER:$USER vmorbit
git clone https://github.com/tchandu17/VmOrbit.git vmorbit
cd vmorbit
```

---

## Step 4: Configure SSL/TLS Certificates

### Option A: Let's Encrypt (free, recommended)

```bash
# Stop anything on port 80 first
sudo certbot certonly --standalone -d vmorbit.yourdomain.com

# Certificates will be at:
#   /etc/letsencrypt/live/vmorbit.yourdomain.com/fullchain.pem
#   /etc/letsencrypt/live/vmorbit.yourdomain.com/privkey.pem

# Copy to the nginx/ssl directory
sudo cp /etc/letsencrypt/live/vmorbit.yourdomain.com/fullchain.pem nginx/ssl/fullchain.pem
sudo cp /etc/letsencrypt/live/vmorbit.yourdomain.com/privkey.pem nginx/ssl/privkey.pem
sudo chown $USER:$USER nginx/ssl/*.pem
```

### Option B: Self-signed (for testing only)

```bash
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout nginx/ssl/privkey.pem \
  -out nginx/ssl/fullchain.pem \
  -subj "/CN=vmorbit.yourdomain.com"
```

### Auto-renewal (Let's Encrypt)

```bash
# Add a cron job to renew and copy certs
sudo crontab -e
```

Add this line:

```
0 3 * * * certbot renew --quiet --post-hook "cp /etc/letsencrypt/live/vmorbit.yourdomain.com/fullchain.pem /opt/vmorbit/nginx/ssl/fullchain.pem && cp /etc/letsencrypt/live/vmorbit.yourdomain.com/privkey.pem /opt/vmorbit/nginx/ssl/privkey.pem && docker restart vmorbit_nginx"
```

---

## Step 5: Configure Environment Variables

### 5.1 Create the production env file

```bash
cp .env.production.example .env.production
```

### 5.2 Generate secrets and edit the file

```bash
# Generate a JWT secret
openssl rand -base64 64

# Generate an encryption key for provider credentials
openssl rand -hex 32

# Generate strong passwords for DB and Redis
openssl rand -base64 32
```

### 5.3 Edit `.env.production`

```bash
nano .env.production
```

Fill in the values — here's what to change:

```env
# ── Application
APP_VERSION=1.0.0

# ── Database (use the same password in both DB_PASSWORD and VMORBIT_DATABASE_PASSWORD)
DB_USER=vmorbit
DB_PASSWORD=<your-generated-db-password>
DB_NAME=vmorbit
VMORBIT_DATABASE_USER=vmorbit
VMORBIT_DATABASE_PASSWORD=<your-generated-db-password>
VMORBIT_DATABASE_DBNAME=vmorbit

# ── Redis (use the same password in both REDIS_PASSWORD and VMORBIT_REDIS_PASSWORD)
REDIS_PASSWORD=<your-generated-redis-password>
VMORBIT_REDIS_PASSWORD=<your-generated-redis-password>

# ── JWT
VMORBIT_JWT_SECRET=<your-generated-jwt-secret>

# ── Encryption
VMORBIT_ENCRYPTION_KEY=<your-generated-hex-key>

# ── CORS & Frontend URL (replace with your actual domain)
VMORBIT_SERVER_CORS_ORIGINS=https://vmorbit.yourdomain.com
NEXT_PUBLIC_API_URL=https://vmorbit.yourdomain.com
```

---

## Step 6: Build and Start the Application

### 6.1 Build all containers

```bash
docker compose -f docker-compose.production.yml --env-file .env.production build
```

This will:
- Build the Go backend (multi-stage, produces a ~15MB static binary)
- Build the Next.js frontend (standalone output)
- Pull PostgreSQL 16, Redis 7, and Nginx images

### 6.2 Start the stack

```bash
docker compose -f docker-compose.production.yml --env-file .env.production up -d
```

### 6.3 Verify all containers are running

```bash
docker compose -f docker-compose.production.yml ps
```

Expected output — all services should show `healthy`:

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
# Run the seed command inside the backend container's network
docker compose -f docker-compose.production.yml exec backend /vmorbit seed

# Or run it separately if the above doesn't work:
docker run --rm --network vmorbit_vmorbit_net \
  --env-file .env.production \
  -e VMORBIT_DATABASE_HOST=postgres \
  -e VMORBIT_REDIS_HOST=redis \
  vmorbit/backend:latest /vmorbit seed
```

> **Note:** If the seed command isn't built into the main binary, you can run it from your local machine pointing to the server's DB, or build a separate seed image.

---

## Step 8: Verify the Deployment

### 8.1 Health check

```bash
curl -k https://vmorbit.yourdomain.com/health
# Expected: {"status":"ok"}
```

### 8.2 Open in browser

Navigate to `https://vmorbit.yourdomain.com` — you should see the login page.

### 8.3 Default credentials (from seed)

Check your `cmd/seed/main.go` for the default admin credentials. Typically:
- **Username:** `admin@vmorbit.local`
- **Password:** `admin123` (change immediately after first login)

---

## Step 9: Post-Deployment Configuration

### 9.1 Add your first hypervisor

1. Log in to the dashboard
2. Go to **Hypervisors** → **Add Hypervisor**
3. Enter your vCenter/Proxmox/ESXi connection details
4. Click **Test Connection** to verify
5. Click **Sync** to pull inventory

### 9.2 Set up automatic certificate renewal

Already configured in Step 4 if using Let's Encrypt.

### 9.3 Configure backups

The `vmorbit_backup` container runs automatic daily PostgreSQL backups with 7-day retention. Backups are stored in the `backup_data` Docker volume.

To manually trigger a backup:

```bash
docker exec vmorbit_backup pg_dump -U vmorbit -d vmorbit > /backups/manual_$(date +%Y%m%d).sql
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

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Backend can't connect to DB | Check `VMORBIT_DATABASE_PASSWORD` matches `DB_PASSWORD` in `.env.production` |
| Frontend shows blank page | Verify `NEXT_PUBLIC_API_URL` matches your actual domain |
| 502 Bad Gateway | Backend hasn't started yet — check `docker logs vmorbit_backend` |
| SSL errors | Verify cert files exist in `nginx/ssl/` and are readable |
| WebSocket not connecting | Ensure nginx config has the `/ws` location block (it does by default) |
| Container keeps restarting | Check logs: `docker logs vmorbit_<service>` |
| Port 80/443 already in use | Stop Apache/Nginx on host: `sudo systemctl stop apache2 nginx` |

---

## Architecture Diagram

```
Internet
    │
    ▼
┌─────────────────────────────────────────┐
│  Nginx (ports 80, 443)                  │
│  - TLS termination                      │
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
│ ┌──────┐ ┌───────┐                     │
│ │Postgres│ │ Redis │                     │
│ │ :5432  │ │ :6379 │                     │
│ └────────┘ └───────┘                     │
└─────────────────────────────────────────┘
         Docker Network (vmorbit_net)
```

---

## Software Summary

| Software | Version | Purpose |
|----------|---------|---------|
| Docker Engine | 24+ | Container runtime |
| Docker Compose | v2+ | Multi-container orchestration |
| Git | 2.x | Clone repository |
| Certbot | latest | SSL certificate management |
| Ubuntu/Debian | 22.04+ / 12+ | Host operating system |

All other software (Go, Node.js, PostgreSQL, Redis, Nginx) runs inside Docker containers — no host installation needed.
