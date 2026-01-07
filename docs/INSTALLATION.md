# Installation Guide

Complete installation instructions for SampMail.

**SampMail is powered by:**
- [KumoMTA](https://kumomta.com/) - High-performance Mail Transfer Agent
- [Reacher](https://reacher.email/) - Email Verification Service

---

## Table of Contents

- [Quick Install (Recommended)](#quick-install-recommended)
- [Docker Installation](#docker-installation)
- [Manual Installation](#manual-installation)
- [Building from Source](#building-from-source)
- [Reacher Setup](#reacher-setup)
- [KumoMTA Integration](#kumomta-integration)
- [Production Deployment](#production-deployment)
- [Configuration Reference](#configuration-reference)
- [Uninstallation](#uninstallation)
- [Troubleshooting](#troubleshooting)

---

## Quick Install (Recommended)

### One-Line Install

**Ubuntu/Debian:**
```bash
curl -fsSL https://raw.githubusercontent.com/pulak-ranjan/sampmail/main/scripts/install-ubuntu.sh | sudo bash
```

**Rocky/RHEL/CentOS/AlmaLinux:**
```bash
curl -fsSL https://raw.githubusercontent.com/pulak-ranjan/sampmail/main/scripts/install-rocky.sh | sudo bash
```

**Any Linux (with Docker):**
```bash
curl -fsSL https://raw.githubusercontent.com/pulak-ranjan/sampmail/main/scripts/install.sh | sudo bash -s -- --docker
```

### What Gets Installed

| Component | Description |
|-----------|-------------|
| SampMail | Main application (`/opt/sampmail`) |
| Reacher | Email verification (Docker container) |
| Systemd services | `sampmail.service`, `reacher.service` |
| Configuration | `/etc/sampmail.env` |
| Data directory | `/var/lib/sampmail` |

### Install Options

```bash
# Install with KumoMTA
curl -fsSL .../install.sh | sudo bash -s -- --with-kumomta

# Skip Reacher (if using external)
curl -fsSL .../install.sh | sudo bash -s -- --no-reacher

# Specific version
curl -fsSL .../install.sh | sudo bash -s -- --version 1.0.0

# Docker Compose installation
curl -fsSL .../install.sh | sudo bash -s -- --docker
```

---

## Docker Installation

The easiest way to run SampMail with all dependencies.

### 1. Clone Repository

```bash
git clone https://github.com/pulak-ranjan/sampmail.git
cd sampmail
```

### 2. Configure Environment

```bash
# Generate a secure secret key
echo "SAMPMAIL_SECRET=$(openssl rand -base64 32)" > .env
```

### 3. Start Services

```bash
# Start SampMail + Reacher
docker-compose up -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f sampmail
```

### 4. Create Super Admin Account

> **Note (v0.2.0+):** Public registration is disabled. Create your admin account via CLI:

```bash
# Docker installation
docker exec sampmail ./sampmail user create admin@example.com "YourPassword123" --role super_admin

# Binary installation
cd /opt/sampmail
./sampmail user create admin@example.com "YourPassword123" --role super_admin
```

### 5. Access UI

Open http://localhost:9000 and login with your admin account.

**First Steps:**
1. Login as Super Admin
2. Go to **Admin Station > Organizations**
3. Create your first organization
4. Use the **Org Switcher** in sidebar to enter User Dashboard

### What Gets Started

| Service | Port | Description |
|---------|------|-------------|
| SampMail | 9000 | Web UI and API |
| Reacher | 8080 | Email verification |

---

## Binary Installation

For servers with KumoMTA already installed.

### 1. Download Release

```bash
# Download latest release
curl -LO https://github.com/pulak-ranjan/sampmail/releases/latest/download/sampmail-linux-amd64.tar.gz

# Extract
tar -xzf sampmail-linux-amd64.tar.gz
cd sampmail
```

### 2. Configure

```bash
# Create environment file
sudo cat > /etc/sampmail.env << 'EOF'
SAMPMAIL_SECRET=your-32-character-minimum-secret-key
SAMPMAIL_LISTEN_ADDR=127.0.0.1:9000
SAMPMAIL_KUMO_DIR=/opt/kumomta
SAMPMAIL_DATA_DIR=/var/lib/sampmail
REACHER_URL=http://localhost:8080
EOF

sudo chmod 600 /etc/sampmail.env
```

### 3. Create Systemd Service

```bash
sudo cat > /etc/systemd/system/sampmail.service << 'EOF'
[Unit]
Description=SampMail Email Platform
After=network.target kumomta.service

[Service]
Type=simple
User=sampmail
Group=sampmail
EnvironmentFile=/etc/sampmail.env
ExecStart=/opt/sampmail/sampmail
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# Create user and directories
sudo useradd -r -s /bin/false sampmail
sudo mkdir -p /var/lib/sampmail /opt/sampmail
sudo chown sampmail:sampmail /var/lib/sampmail

# Move binary
sudo mv sampmail /opt/sampmail/
sudo chown sampmail:sampmail /opt/sampmail/sampmail

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable sampmail
sudo systemctl start sampmail
```

### 4. Verify

```bash
# Check status
sudo systemctl status sampmail

# Test API
curl http://127.0.0.1:9000/api/system/health
```

---

## Building from Source

### 1. Clone Repository

```bash
git clone https://github.com/pulak-ranjan/sampmail.git
cd sampmail
```

### 2. Build Backend

```bash
# Download dependencies
go mod download

# Build binary
CGO_ENABLED=1 go build -ldflags="-s -w" -o sampmail ./cmd/server
```

### 3. Build Frontend

```bash
cd web
bun install
bun run build
cd ..
```

### 4. Run

```bash
export SAMPMAIL_SECRET="your-32-char-secret-key"
./sampmail
```

---

## Reacher Setup

SampMail uses [Reacher](https://reacher.email/) for email verification.

### Option 1: Docker (Included in docker-compose.yml)

```bash
# Already started with docker-compose up
docker-compose ps reacher
```

### Option 2: Standalone Docker

```bash
docker run -d \
  --name reacher \
  -p 8080:8080 \
  -e RCH_ENABLE_BULK=true \
  reacherhq/backend:latest
```

### Option 3: Reacher Cloud

If using [Reacher Cloud](https://reacher.email/):

```bash
export REACHER_URL="https://api.reacher.email"
export REACHER_API_KEY="your-api-key"
```

### Verification Features

| Check | Description |
|-------|-------------|
| Syntax | Valid email format |
| MX Records | Domain has mail servers |
| SMTP | Mailbox exists |
| Disposable | Temp email detection |
| Role-based | info@, support@, etc. |
| Catch-all | Accepts any address |

---

## KumoMTA Integration

SampMail generates configuration for KumoMTA.

### Prerequisites

1. Install KumoMTA: https://docs.kumomta.com/

2. Configure SampMail:
```bash
export SAMPMAIL_KUMO_DIR="/opt/kumomta"
export SAMPMAIL_LOG_DIR="/var/log/kumomta"
export SAMPMAIL_SMTP_ADDR="127.0.0.1:25"
```

### Generated Files

SampMail creates these in `$SAMPMAIL_KUMO_DIR/etc/policy/`:

| File | Purpose |
|------|---------|
| `init.lua` | Main KumoMTA config |
| `sources.toml` | IP/egress sources |
| `queues.toml` | Queue settings |
| `dkim_data.toml` | DKIM policies |
| `auth.toml` | SMTP credentials |

### DKIM Keys

Generated in `$SAMPMAIL_KUMO_DIR/etc/dkim/`:

```
dkim/
└── example.com/
    ├── newsletter.key    # Private key
    └── newsletter.pub    # Public key (add to DNS)
```

---

## Production Deployment

### Nginx Reverse Proxy

```nginx
upstream sampmail {
    server 127.0.0.1:9000;
    keepalive 32;
}

server {
    listen 443 ssl http2;
    server_name mail.example.com;

    ssl_certificate /etc/letsencrypt/live/mail.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/mail.example.com/privkey.pem;

    location / {
        proxy_pass http://sampmail;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

server {
    listen 80;
    server_name mail.example.com;
    return 301 https://$host$request_uri;
}
```

### Environment for Production

```bash
# /etc/sampmail.env
SAMPMAIL_SECRET=<openssl rand -base64 32>
SAMPMAIL_LISTEN_ADDR=127.0.0.1:9000
SAMPMAIL_KUMO_DIR=/opt/kumomta
SAMPMAIL_DATA_DIR=/var/lib/sampmail
SAMPMAIL_LOG_DIR=/var/log/kumomta

# Behind reverse proxy
SAMPMAIL_TRUST_PROXY=true
SAMPMAIL_TRUSTED_PROXY_CIDR=127.0.0.1/32

# Performance
SAMPMAIL_CAMPAIGN_WORKERS=100

# Reacher
REACHER_URL=http://127.0.0.1:8080
```

### File Permissions

```bash
# Database
chmod 600 /var/lib/sampmail/sampmail.db
chown sampmail:sampmail /var/lib/sampmail/sampmail.db

# DKIM keys
chmod 600 /opt/kumomta/etc/dkim/*/*.key
chmod 644 /opt/kumomta/etc/dkim/*/*.pub

# Environment file
chmod 600 /etc/sampmail.env
```

---

## Configuration Reference

### All Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SAMPMAIL_SECRET` | *required* | Encryption key (min 32 chars) |
| `SAMPMAIL_LISTEN_ADDR` | `127.0.0.1:9000` | HTTP bind address |
| `SAMPMAIL_KUMO_DIR` | `/opt/kumomta` | KumoMTA directory |
| `SAMPMAIL_DATA_DIR` | `/var/lib/sampmail` | Database & backups |
| `SAMPMAIL_LOG_DIR` | `/var/log/kumomta` | KumoMTA logs |
| `SAMPMAIL_SPOOL_DIR` | `/var/spool/kumomta` | Mail spool |
| `SAMPMAIL_SMTP_ADDR` | `127.0.0.1:25` | SMTP server |
| `SAMPMAIL_TRUST_PROXY` | `false` | Trust proxy headers |
| `SAMPMAIL_TRUSTED_PROXY_CIDR` | - | Trusted proxy CIDRs |
| `SAMPMAIL_CAMPAIGN_WORKERS` | `50` | Sending concurrency |
| `SAMPMAIL_IMPORT_BATCH_SIZE` | `500` | CSV import batch |
| `SAMPMAIL_FIREWALL_ENABLED` | `true` | IP blocking |
| `REACHER_URL` | `http://reacher:8080` | Reacher API |
| `REACHER_API_KEY` | - | Reacher Cloud key |

### Generate Secret Key

```bash
# Option 1: OpenSSL
openssl rand -base64 32

# Option 2: /dev/urandom
head -c 32 /dev/urandom | base64
```

---

## Troubleshooting

### Common Issues

**Port already in use:**
```bash
lsof -i :9000
# Kill process or change SAMPMAIL_LISTEN_ADDR
```

**Database permission denied:**
```bash
sudo chown sampmail:sampmail /var/lib/sampmail/sampmail.db
sudo chmod 600 /var/lib/sampmail/sampmail.db
```

**Cannot connect to KumoMTA:**
```bash
# Check KumoMTA is running
systemctl status kumomta

# Test SMTP
nc -zv 127.0.0.1 25
```

**Reacher not responding:**
```bash
# Check Reacher health
curl http://localhost:8080/health

# Check container logs
docker logs reacher
```

### Logs

```bash
# SampMail logs
journalctl -u sampmail -f

# Docker logs
docker-compose logs -f sampmail
docker-compose logs -f reacher
```

### Getting Help

- [GitHub Issues](https://github.com/pulak-ranjan/sampmail/issues)
- [Discussions](https://github.com/pulak-ranjan/sampmail/discussions)

---

## Uninstallation

To completely remove SampMail:

```bash
# Download and run uninstall script
curl -fsSL https://raw.githubusercontent.com/pulak-ranjan/sampmail/main/scripts/uninstall.sh | sudo bash
```

Or manually:

```bash
# Stop services
sudo systemctl stop sampmail reacher

# Disable services
sudo systemctl disable sampmail reacher

# Remove files
sudo rm -rf /opt/sampmail
sudo rm -f /etc/sampmail.env
sudo rm -f /etc/systemd/system/sampmail.service
sudo rm -f /etc/systemd/system/reacher.service

# Remove data (optional)
sudo rm -rf /var/lib/sampmail

# Remove user
sudo userdel sampmail

# Stop Reacher container
sudo docker stop reacher && sudo docker rm reacher
```
