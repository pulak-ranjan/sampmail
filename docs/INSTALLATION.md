# SampMail Installation Guide

This guide covers every method of deploying SampMail on a Linux server.

SampMail integrates with two external services:

- **KumoMTA** (https://kumomta.com) — the mail transfer agent that delivers email to recipient mail servers.
- **Reacher** (https://reacher.email) — the email verification engine that validates addresses before sending.

Both can be installed by the install script or connected separately after the fact.

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Install](#quick-install)
- [Docker Compose Install](#docker-compose-install)
- [Manual Install](#manual-install)
- [Building from Source](#building-from-source)
- [KumoMTA Setup](#kumomta-setup)
- [Reacher Setup](#reacher-setup)
- [Nginx Reverse Proxy and TLS](#nginx-reverse-proxy-and-tls)
- [Configuration Reference](#configuration-reference)
- [Upgrading](#upgrading)
- [Uninstalling](#uninstalling)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

| Requirement | Minimum | Notes |
|---|---|---|
| OS | Ubuntu 22.04, Debian 12, or Rocky 9 | Other Linux distributions work but are untested |
| CPU | 2 cores | 4 or more recommended for production |
| RAM | 2 GB | 4 GB recommended |
| Disk | 20 GB | More for large contact lists and log retention |
| PostgreSQL | 15 or 16 | SQLite is available for development only |
| Redis | 7 | Optional but strongly recommended for production |
| Open ports | 9000 (HTTP), 587 (SMTP submission) | Do not expose port 25 publicly |

---

## Quick Install

The bundled universal shell script installs PostgreSQL, Docker (for Reacher), KumoMTA, downloads the SampMail binary, creates a systemd service, and writes a fully configured `/etc/sampmail.env` with all secrets auto-generated.

### 🚀 Standard Installation
Works on **Ubuntu (22.04+), Debian (12+), Rocky Linux (9+), AlmaLinux (9+), and RHEL 9**.

```bash
curl -fsSL https://raw.githubusercontent.com/pulak-ranjan/sampmail/main/scripts/install.sh | bash -s -- --with-kumomta
```

### 🛠️ Custom Options

**Override Base URL (Default is Auto-detected):**
```bash
curl -fsSL https://raw.githubusercontent.com/pulak-ranjan/sampmail/main/scripts/install.sh \
  | bash -s -- --url https://mail.yourdomain.com --with-kumomta
```

**Install without Reacher (Email Verification):**
```bash
curl -fsSL https://raw.githubusercontent.com/pulak-ranjan/sampmail/main/scripts/install.sh \
  | bash -s -- --with-kumomta --no-reacher
```

### What the script does automatically

| Step | Detail |
|---|---|
| Detects the public IP | Queries AWS, GCP, and Azure instance metadata APIs. Falls back to ifconfig.io, then the local NIC. |
| Generates SAMPMAIL_SECRET | `openssl rand -base64 32`. Unique per installation. Written to /etc/sampmail.env. |
| Generates the PostgreSQL password | 24-character alphanumeric random string. |
| Writes /etc/sampmail.env | All variables pre-filled. File permissions set to 600. |
| Creates a systemd service | Auto-starts on boot. Restarts on failure after 5 seconds. |
| Seeds the hostname on first startup | The detected base URL hostname is written into the database. Tracking links and unsubscribe URLs work immediately without any UI configuration. |

### Switching to HTTPS after initial setup

Once a domain is pointing at the server and a certificate is issued:

```bash
sudo nano /etc/sampmail.env
# Update:
SAMPMAIL_BASE_URL=https://mail.yourdomain.com

sudo systemctl restart sampmail
```

SampMail detects the updated value on startup and writes the new hostname to the
database automatically.

---

## Docker Compose Install

```bash
mkdir -p /opt/sampmail && cd /opt/sampmail

curl -fsSL https://raw.githubusercontent.com/pulak-ranjan/sampmail/main/docker-compose.yml \
  -o docker-compose.yml

SERVER_IP=\$(curl -sf --max-time 5 https://ifconfig.io || hostname -I | awk '{print \$1}')

printf 'SAMPMAIL_SECRET=%s\n' "\$(openssl rand -base64 32)" > .env
printf 'POSTGRES_PASSWORD=%s\n' "\$(openssl rand -base64 24 | tr -d /+= | head -c 24)" >> .env
printf 'SAMPMAIL_BASE_URL=http://%s:9000\n' "\$SERVER_IP" >> .env
cat >> .env <<'ENV'
SAMPMAIL_DB_DRIVER=postgres
POSTGRES_USER=sampmail
POSTGRES_DB=sampmail
SAMPMAIL_REDIS_ADDR=redis:6379
ENV

chmod 600 .env
docker compose up -d
```

The `.env` file contains secrets and must not be committed to version control.

### Docker service overview

| Service | Image | Purpose |
|---|---|---|
| sampmail | Built locally | Main application |
| postgres | postgres:16-alpine | Primary database |
| redis | redis:7-alpine | Distributed rate limiting |
| reacher | reacherhq/backend:latest | Email address verification |
| kumomta | ghcr.io/kumomta/kumomta:latest | Mail transfer agent |

KumoMTA exposes port 587 for authenticated submission only. Port 25 is not mapped to
the host.

---

## Manual Install

### 1. Create the service user and directories

```bash
useradd -r -s /bin/false -d /var/lib/sampmail sampmail
mkdir -p /opt/sampmail /var/lib/sampmail /var/lib/sampmail/backups /var/log/sampmail
chown -R sampmail:sampmail /var/lib/sampmail /var/log/sampmail
```

### 2. Install PostgreSQL 15

```bash
# Ubuntu or Debian
sh -c 'echo "deb https://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" \
  > /etc/apt/sources.list.d/pgdg.list'
wget -qO - https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add -
apt-get update && apt-get install -y postgresql-15
systemctl enable postgresql && systemctl start postgresql
```

### 3. Create the database

```bash
PG_PASSWORD=\$(openssl rand -base64 24 | tr -d '/+=' | head -c 24)
echo "Save this password: \$PG_PASSWORD"

sudo -u postgres psql -c "CREATE USER sampmail WITH PASSWORD '\$PG_PASSWORD';"
sudo -u postgres psql -c "CREATE DATABASE sampmail OWNER sampmail;"
sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE sampmail TO sampmail;"
```

### 4. Download the binary

```bash
curl -fsSL \
  https://github.com/pulak-ranjan/sampmail/releases/latest/download/sampmail-linux-amd64.tar.gz \
  | tar -xz -C /opt/sampmail

chmod +x /opt/sampmail/sampmail
chown -R sampmail:sampmail /opt/sampmail
```

### 5. Write the configuration file

```bash
APP_SECRET=\$(openssl rand -base64 32)
SERVER_IP=\$(curl -sf --max-time 5 https://ifconfig.io || hostname -I | awk '{print \$1}')

cat > /etc/sampmail.env <<CONFIG
SAMPMAIL_SECRET=\$APP_SECRET
SAMPMAIL_BASE_URL=http://\$SERVER_IP:9000
SAMPMAIL_LISTEN_ADDR=0.0.0.0:9000
SAMPMAIL_ENV=production
SAMPMAIL_DATA_DIR=/var/lib/sampmail
SAMPMAIL_DB_DRIVER=postgres
SAMPMAIL_PG_HOST=127.0.0.1
SAMPMAIL_PG_PORT=5432
SAMPMAIL_PG_USER=sampmail
SAMPMAIL_PG_PASSWORD=\$PG_PASSWORD
SAMPMAIL_PG_DATABASE=sampmail
SAMPMAIL_PG_SSLMODE=disable
SAMPMAIL_SMTP_ADDR=127.0.0.1:587
REACHER_URL=http://127.0.0.1:8080
SAMPMAIL_CAMPAIGN_WORKERS=50
SAMPMAIL_BCRYPT_COST=12
SAMPMAIL_LOG_LEVEL=info
SAMPMAIL_LOG_JSON=true
CONFIG

chmod 600 /etc/sampmail.env
chown root:sampmail /etc/sampmail.env
```

### 6. Create the systemd service

```bash
cat > /etc/systemd/system/sampmail.service <<'SERVICE'
[Unit]
Description=SampMail Email Marketing Platform
After=network.target postgresql.service
Wants=postgresql.service

[Service]
Type=simple
User=sampmail
Group=sampmail
EnvironmentFile=/etc/sampmail.env
WorkingDirectory=/opt/sampmail
ExecStart=/opt/sampmail/sampmail
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/sampmail /var/log/sampmail
PrivateTmp=true

[Install]
WantedBy=multi-user.target
SERVICE

systemctl daemon-reload
systemctl enable sampmail
systemctl start sampmail
```

---

## Building from Source

Requirements: Go 1.21 or later, Bun, Git.

```bash
git clone https://github.com/pulak-ranjan/sampmail
cd sampmail

# Build the React frontend
cd web && bun install && bun run build && cd ..

# Build the Go binary with version information embedded at compile time
VERSION=\$(cat VERSION 2>/dev/null || echo "dev")
BUILD_TIME=\$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=\$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

go build \
  -ldflags "-X github.com/pulak-ranjan/sampmail/internal/core.Version=\$VERSION \
            -X github.com/pulak-ranjan/sampmail/internal/core.BuildTime=\$BUILD_TIME \
            -X github.com/pulak-ranjan/sampmail/internal/core.GitCommit=\$GIT_COMMIT" \
  -o sampmail \
  ./cmd/server/
```

The frontend build output at `web/dist/` is served directly by the binary. No separate
web server is required.

---

## KumoMTA Setup

KumoMTA handles actual delivery of email to recipient mail servers. SampMail connects
to it over authenticated SMTP submission on port 587. Port 25 is used by KumoMTA to
reach other mail servers and must never be exposed to the public internet.

### Install KumoMTA

```bash
# Ubuntu or Debian
curl -fsSL https://openrepo.kumomta.com/files/kumomta-repo-setup.deb.sh | bash
apt-get update && apt-get install -y kumomta

# Rocky Linux or RHEL
curl -fsSL https://openrepo.kumomta.com/files/kumomta-repo-setup.rpm.sh | bash
yum install -y kumomta

systemctl enable kumomta && systemctl start kumomta
```

### Connect SampMail to KumoMTA

In `/etc/sampmail.env`:

```
SAMPMAIL_SMTP_ADDR=127.0.0.1:587
SAMPMAIL_KUMO_DIR=/opt/kumomta
SAMPMAIL_LOG_DIR=/var/log/kumomta
```

SampMail maintains a persistent SMTP connection pool (5 to 100 connections by default).
All campaign email passes through this pool.

### Internal policy API

SampMail exposes localhost-only endpoints that KumoMTA queries for real-time sending
policy decisions:

| Endpoint | Purpose |
|---|---|
| GET /api/internal/policy/sending-ips | Returns the list of IPs configured for outbound sending |
| GET /api/internal/policy/domains | Returns configured sending domains |
| GET /api/internal/policy/sender-rate | Returns per-sender rate limits |
| POST /api/internal/policy/log-event | KumoMTA posts delivery events (successes, bounces, deferrals) here |
| GET /api/internal/policy/suppression-check | Pre-send suppression check for a given address |

These endpoints bind to 127.0.0.1 only and are not reachable from outside the server.

---

## Reacher Setup

Reacher validates whether an email address can receive mail before a campaign sends.
This protects sender reputation by eliminating undeliverable addresses from contact lists.

### Option 1: Docker (recommended)

```bash
docker run -d \
  --name reacher \
  --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  -e RCH_ENABLE_BULK=true \
  reacherhq/backend:latest
```

In `/etc/sampmail.env`:

```
REACHER_URL=http://127.0.0.1:8080
```

### Option 2: Reacher Cloud

```
REACHER_URL=https://api.reacher.email
REACHER_API_KEY=your_api_key
```

### Option 3: No Reacher

SampMail falls back to direct SMTP verification from the server. Accuracy is lower for
major providers such as Gmail, Outlook, and Yahoo because they block direct SMTP probes.
Reacher handles these via its own IP infrastructure.

---

## Nginx Reverse Proxy and TLS

Running SampMail behind Nginx is the standard production configuration. Nginx handles
TLS termination and allows SampMail to run as a non-root process on port 9000 while
being publicly accessible on port 443.

### Issue a certificate

```bash
apt-get install -y certbot python3-certbot-nginx
certbot certonly --standalone -d mail.yourdomain.com
```

### Nginx site configuration

Create `/etc/nginx/sites-available/sampmail`:

```nginx
server {
    listen 80;
    server_name mail.yourdomain.com;
    return 301 https://\$host\$request_uri;
}

server {
    listen 443 ssl http2;
    server_name mail.yourdomain.com;

    ssl_certificate /etc/letsencrypt/live/mail.yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/mail.yourdomain.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers on;

    client_max_body_size 50m;

    location / {
        proxy_pass http://127.0.0.1:9000;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_read_timeout 300s;
    }
}
```

```bash
ln -s /etc/nginx/sites-available/sampmail /etc/nginx/sites-enabled/
nginx -t && systemctl reload nginx
```

Update `/etc/sampmail.env`:

```
SAMPMAIL_BASE_URL=https://mail.yourdomain.com
SAMPMAIL_TRUST_PROXY=true
SAMPMAIL_TRUSTED_PROXY_CIDR=127.0.0.1/32
```

```bash
sudo systemctl restart sampmail
```

---

## Configuration Reference

All configuration is read from environment variables. There is no configuration file
format. For the systemd service, variables come from `/etc/sampmail.env`. For Docker
Compose, they come from `.env`.

### Required

| Variable | Description |
|---|---|
| SAMPMAIL_SECRET | Master secret, minimum 32 characters. Auto-generated by install.sh. Used to derive AES-256 encryption keys and session signing keys via PBKDF2 with 100,000 iterations. Do not change after first run — doing so invalidates all active sessions. |
| SAMPMAIL_BASE_URL | The public URL that external email recipients can reach. Used to build tracking pixel URLs, click redirect URLs, and unsubscribe links in every campaign email. Auto-detected by install.sh. Update when switching to a custom domain or HTTPS. Example: https://mail.yourdomain.com |
| SAMPMAIL_PG_PASSWORD | PostgreSQL password. Required when SAMPMAIL_DB_DRIVER=postgres. Auto-generated by install.sh. |

### Server

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_LISTEN_ADDR | 127.0.0.1:9000 | Address and port the HTTP server binds to |
| SAMPMAIL_ENV | development | Set to production for JSON logging and production defaults |
| SAMPMAIL_REQUEST_TIMEOUT | 30s | Maximum duration per HTTP request |
| SAMPMAIL_MAX_REQUEST_BODY | 10485760 | Maximum request body in bytes |
| SAMPMAIL_SHUTDOWN_TIMEOUT | 30s | Grace period for in-flight requests during graceful shutdown |

### Database

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_DB_DRIVER | sqlite | Database engine: sqlite or postgres |
| SAMPMAIL_PG_HOST | localhost | PostgreSQL hostname |
| SAMPMAIL_PG_PORT | 5432 | PostgreSQL port |
| SAMPMAIL_PG_USER | | PostgreSQL username |
| SAMPMAIL_PG_DATABASE | | Database name |
| SAMPMAIL_PG_SSLMODE | require | SSL mode: require, disable, or verify-full |
| SAMPMAIL_DB_MAX_OPEN_CONNS | 100 | Maximum open connections in the pool |
| SAMPMAIL_DB_MAX_IDLE_CONNS | 10 | Minimum idle connections kept alive |
| DATABASE_URL | | Full PostgreSQL connection string. Parsed automatically when set. Example: postgres://user:pass@host:5432/db?sslmode=disable |

### Redis

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_REDIS_ADDR | | Redis address, e.g. redis:6379. When not set, rate limiting uses in-memory counters that reset on restart. |
| SAMPMAIL_REDIS_PASSWORD | | Redis AUTH password |
| SAMPMAIL_REDIS_DB | 0 | Redis logical database number |

### SMTP and KumoMTA

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_SMTP_ADDR | kumomta:587 | KumoMTA submission host and port |
| SAMPMAIL_SMTP_MAX_CONNS | 100 | Maximum connections in the SMTP pool |
| SAMPMAIL_SMTP_MIN_CONNS | 5 | Minimum connections kept alive |
| SAMPMAIL_SMTP_CONN_TIMEOUT | 10s | SMTP connection timeout |
| SAMPMAIL_KUMO_DIR | /opt/kumomta | KumoMTA installation directory |
| SAMPMAIL_LOG_DIR | /var/log/kumomta | KumoMTA log directory, read for bounce processing |
| SAMPMAIL_KUMO_API_URL | http://127.0.0.1:8000 | KumoMTA HTTP management API |

### Email verification

| Variable | Default | Description |
|---|---|---|
| REACHER_URL | http://reacher:8080 | Reacher service base URL |
| REACHER_API_KEY | | API key for Reacher Cloud. Omit for self-hosted Reacher. |

### Campaigns

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_CAMPAIGN_WORKERS | 50 | Number of concurrent sending goroutines per campaign |
| SAMPMAIL_IMPORT_BATCH_SIZE | 500 | Rows per database transaction during CSV import |

### Security

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_BCRYPT_COST | 12 | bcrypt cost factor. Values 12 to 14 are appropriate for production. |
| SAMPMAIL_FIREWALL_ENABLED | true | Automatically block IPs that exhibit attack patterns |
| SAMPMAIL_TRUST_PROXY | false | Trust X-Forwarded-For and X-Forwarded-Proto headers. Enable only when behind a trusted reverse proxy. |
| SAMPMAIL_TRUSTED_PROXY_CIDR | | Comma-separated CIDR ranges whose proxy headers are trusted. Example: 127.0.0.1/32,10.0.0.0/8 |

### TLS (use Nginx instead for production)

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_TLS_ENABLED | false | Enable TLS directly on the application HTTP server |
| SAMPMAIL_TLS_CERT | | Path to the PEM certificate file |
| SAMPMAIL_TLS_KEY | | Path to the PEM private key file |

### Logging and monitoring

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_LOG_LEVEL | info | Log verbosity: debug, info, warn, or error |
| SAMPMAIL_LOG_JSON | true in production | Emit structured JSON log lines |
| SAMPMAIL_METRICS_ENABLED | true | Expose a Prometheus-compatible metrics endpoint |
| SAMPMAIL_METRICS_PATH | /metrics | Path for the metrics endpoint |

### Data

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_DATA_DIR | /var/lib/sampmail | Root directory for database files and configuration backups |
| SAMPMAIL_BACKUP_RETENTION | 3 | Number of configuration backup copies to retain |

---

## Upgrading

### Systemd installation

```bash
curl -fsSL \
  https://github.com/pulak-ranjan/sampmail/releases/latest/download/sampmail-linux-amd64.tar.gz \
  | tar -xz -C /opt/sampmail

chmod +x /opt/sampmail/sampmail
chown sampmail:sampmail /opt/sampmail/sampmail
systemctl restart sampmail
journalctl -u sampmail -n 50
```

Database schema changes apply automatically on startup via GORM auto-migrations. No
manual migration step is required.

### Docker Compose

```bash
cd /opt/sampmail
docker compose pull
docker compose up -d
```

---

## Uninstalling

```bash
systemctl stop sampmail
systemctl disable sampmail
rm /etc/systemd/system/sampmail.service
systemctl daemon-reload
rm -rf /opt/sampmail /var/lib/sampmail /var/log/sampmail /etc/sampmail.env

sudo -u postgres psql -c "DROP DATABASE sampmail;"
sudo -u postgres psql -c "DROP USER sampmail;"
```

---

## Troubleshooting

### Service fails to start

```bash
journalctl -u sampmail -n 100 --no-pager
```

| Error message | Cause | Fix |
|---|---|---|
| SAMPMAIL_SECRET is required | /etc/sampmail.env is missing or unreadable by the service | Verify the file exists with mode 600 and is owned by root:sampmail |
| failed to connect to PostgreSQL | Database not running or credentials are incorrect | systemctl status postgresql |
| address already in use | Port 9000 is occupied by another process | ss -tlnp to find and stop the conflicting process |
| SAMPMAIL_SECRET must be at least 32 characters | Secret value is too short | Generate a new one: openssl rand -base64 32 |

### Tracking links use the wrong address

`SAMPMAIL_BASE_URL` is set to an internal or incorrect address.

```bash
grep SAMPMAIL_BASE_URL /etc/sampmail.env
# Must be the URL that external recipients can reach
# Correct example: https://mail.yourdomain.com

sudo systemctl restart sampmail
```

SampMail updates the hostname stored in the database on every restart when the value
has changed.

### Emails are not being sent

1. Verify KumoMTA is running: `systemctl status kumomta`
2. Check SMTP pool health: `curl http://localhost:9000/health`
3. Confirm `SAMPMAIL_SMTP_ADDR` points to the correct address and port
4. View per-recipient error messages in the UI: Campaigns > select campaign > Recipients

### High bounce rate after a campaign

1. Verify Reacher is running: `curl http://localhost:8080/health`
2. Clean the contact list before the next send: Lists > select list > Clean List
3. Review the suppression list for entries that should be removed
4. Verify DKIM, SPF, and DMARC DNS records: Domains > DNS Check

### Database disk usage growing unexpectedly

```bash
du -sh /var/lib/sampmail/*
journalctl --vacuum-size=1G
```

If using SQLite with a large number of contacts or tracking events, switch to
PostgreSQL. SQLite does not reclaim space automatically after deletions and is not
suitable for production deployments with millions of records.
