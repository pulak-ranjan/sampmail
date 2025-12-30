#!/bin/bash
#
# SampMail Installation Script for Ubuntu/Debian
# https://github.com/pulak-ranjan/sampmail
#
# Tested on:
#   - Ubuntu 22.04 LTS
#   - Ubuntu 24.04 LTS
#   - Debian 12
#
# Usage:
#   sudo bash install-ubuntu.sh [OPTIONS]
#
# Options:
#   --with-kumomta    Also install KumoMTA mail server
#   --help            Show this help
#

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
SAMPMAIL_VERSION="${SAMPMAIL_VERSION:-latest}"
INSTALL_DIR="/opt/sampmail"
DATA_DIR="/var/lib/sampmail"
CONFIG_FILE="/etc/sampmail.env"
INSTALL_KUMOMTA=false

# Database credentials (auto-generated)
PG_USER="sampmail"
PG_DATABASE="sampmail"
PG_PASSWORD=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --with-kumomta)
            INSTALL_KUMOMTA=true
            shift
            ;;
        --help)
            echo "SampMail Installation Script for Ubuntu/Debian"
            echo ""
            echo "Usage: sudo bash install-ubuntu.sh [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --with-kumomta    Also install KumoMTA mail server"
            echo "  --help            Show this help"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# Check root
[[ $EUID -ne 0 ]] && log_error "Run as root: sudo bash $0"

echo ""
echo -e "${BLUE}╔═══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║        SampMail Installer for Ubuntu/Debian                   ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Generate random password for PostgreSQL
generate_password() {
    openssl rand -base64 24 | tr -d '/+=' | head -c 24
}

# Step 1: Update system
log_info "Updating system packages..."
apt-get update -qq
apt-get install -y -qq curl wget unzip ca-certificates gnupg lsb-release

# Step 2: Install PostgreSQL
log_info "Installing PostgreSQL..."
if ! command -v psql &> /dev/null; then
    # Add PostgreSQL official repository
    sh -c 'echo "deb https://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list'
    wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add -
    apt-get update -qq
    apt-get install -y -qq postgresql-15

    # Start PostgreSQL
    systemctl enable postgresql
    systemctl start postgresql
    log_success "PostgreSQL 15 installed"
else
    log_info "PostgreSQL already installed"
    systemctl start postgresql
fi

# Step 3: Generate PostgreSQL credentials
PG_PASSWORD=$(generate_password)
log_info "Configuring PostgreSQL database..."

# Create database and user
sudo -u postgres psql -c "DROP DATABASE IF EXISTS ${PG_DATABASE};" 2>/dev/null || true
sudo -u postgres psql -c "DROP USER IF EXISTS ${PG_USER};" 2>/dev/null || true
sudo -u postgres psql -c "CREATE USER ${PG_USER} WITH PASSWORD '${PG_PASSWORD}';"
sudo -u postgres psql -c "CREATE DATABASE ${PG_DATABASE} OWNER ${PG_USER};"
sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE ${PG_DATABASE} TO ${PG_USER};"

# Configure pg_hba.conf for local connections
PG_HBA=$(sudo -u postgres psql -t -c "SHOW hba_file;" | xargs)
if ! grep -q "sampmail" "$PG_HBA" 2>/dev/null; then
    # Add local MD5 auth for sampmail user
    sed -i "/^# TYPE/a local   ${PG_DATABASE}      ${PG_USER}                              md5" "$PG_HBA"
    sed -i "/^# TYPE/a host    ${PG_DATABASE}      ${PG_USER}      127.0.0.1/32            md5" "$PG_HBA"
fi

# Reload PostgreSQL to apply changes
systemctl reload postgresql
log_success "PostgreSQL configured (user: ${PG_USER}, database: ${PG_DATABASE})"

# Step 4: Install Docker
log_info "Installing Docker..."
if ! command -v docker &> /dev/null; then
    curl -fsSL https://get.docker.com | sh
    systemctl enable docker
    systemctl start docker
    log_success "Docker installed"
else
    log_info "Docker already installed"
    systemctl start docker
fi

# Step 5: Create user and directories
log_info "Creating sampmail user and directories..."
id sampmail &>/dev/null || useradd -r -s /bin/false -d "$DATA_DIR" sampmail
mkdir -p "$INSTALL_DIR" "$DATA_DIR" "$DATA_DIR/backups" /var/log/sampmail
chown -R sampmail:sampmail "$DATA_DIR" /var/log/sampmail

# Step 6: Download SampMail
log_info "Downloading SampMail..."
cd /tmp
if [[ "$SAMPMAIL_VERSION" == "latest" ]]; then
    DOWNLOAD_URL="https://github.com/pulak-ranjan/sampmail/releases/latest/download/sampmail-linux-amd64.tar.gz"
else
    DOWNLOAD_URL="https://github.com/pulak-ranjan/sampmail/releases/download/v${SAMPMAIL_VERSION}/sampmail-linux-amd64.tar.gz"
fi

curl -fsSL -o sampmail.tar.gz "$DOWNLOAD_URL" || {
    log_warn "Binary not found. Building from source..."

    # Install Go
    if ! command -v go &> /dev/null; then
        log_info "Installing Go..."
        curl -fsSL "https://go.dev/dl/go1.21.5.linux-amd64.tar.gz" -o go.tar.gz
        rm -rf /usr/local/go && tar -C /usr/local -xzf go.tar.gz
        export PATH=$PATH:/usr/local/go/bin
    fi

    # Install Bun
    if ! command -v bun &> /dev/null; then
        log_info "Installing Bun..."
        curl -fsSL https://bun.sh/install | bash
        export PATH=$PATH:$HOME/.bun/bin
    fi

    # Clone and build
    git clone https://github.com/pulak-ranjan/sampmail.git /tmp/sampmail-src
    cd /tmp/sampmail-src
    go build -o sampmail ./cmd/server
    cd web && bun install && bun run build && cd ..
    mv sampmail "$INSTALL_DIR/"
    cp -r web/dist "$INSTALL_DIR/web"
}

if [[ -f /tmp/sampmail.tar.gz ]]; then
    tar -xzf sampmail.tar.gz -C "$INSTALL_DIR"
    rm -f sampmail.tar.gz

    # Handle nested directory from old release format
    if [[ -d "$INSTALL_DIR/sampmail" ]]; then
        if [[ -f "$INSTALL_DIR/sampmail/sampmail" ]]; then
            mv "$INSTALL_DIR/sampmail/sampmail" "$INSTALL_DIR/sampmail-bin"
        fi
        mv "$INSTALL_DIR/sampmail/"* "$INSTALL_DIR/" 2>/dev/null || true
        rmdir "$INSTALL_DIR/sampmail" 2>/dev/null || rm -rf "$INSTALL_DIR/sampmail" 2>/dev/null || true
        if [[ -f "$INSTALL_DIR/sampmail-bin" ]]; then
            mv "$INSTALL_DIR/sampmail-bin" "$INSTALL_DIR/sampmail"
        fi
    fi
fi

chmod +x "$INSTALL_DIR/sampmail"
chown -R sampmail:sampmail "$INSTALL_DIR"
log_success "SampMail installed to $INSTALL_DIR"

# Step 7: Generate config
log_info "Creating configuration..."
SECRET=$(openssl rand -base64 32)

cat > "$CONFIG_FILE" << EOF
# SampMail Configuration
# Generated on $(date)
#
# IMPORTANT: Keep this file secure! It contains database credentials.

# Application Secret (required, min 32 chars)
SAMPMAIL_SECRET=$SECRET

# Server Settings
# Listen on all interfaces for external access
SAMPMAIL_LISTEN_ADDR=0.0.0.0:9000
SAMPMAIL_ENV=production

# Data directories
SAMPMAIL_DATA_DIR=$DATA_DIR
SAMPMAIL_KUMO_DIR=/opt/kumomta
SAMPMAIL_LOG_DIR=/var/log/kumomta

# SMTP (KumoMTA)
SAMPMAIL_SMTP_ADDR=127.0.0.1:25

# Database Configuration (PostgreSQL)
SAMPMAIL_DB_DRIVER=postgres
SAMPMAIL_PG_HOST=127.0.0.1
SAMPMAIL_PG_PORT=5432
SAMPMAIL_PG_USER=${PG_USER}
SAMPMAIL_PG_PASSWORD=${PG_PASSWORD}
SAMPMAIL_PG_DATABASE=${PG_DATABASE}
SAMPMAIL_PG_SSLMODE=disable

# Reacher (Email Verification)
REACHER_URL=http://127.0.0.1:8080

# Performance
SAMPMAIL_CAMPAIGN_WORKERS=50
SAMPMAIL_DB_MAX_OPEN_CONNS=100
SAMPMAIL_DB_MAX_IDLE_CONNS=10
EOF

chmod 600 "$CONFIG_FILE"
chown root:sampmail "$CONFIG_FILE"
log_success "Configuration saved to $CONFIG_FILE"

# Step 8: Create systemd service for SampMail
log_info "Creating systemd services..."

cat > /etc/systemd/system/sampmail.service << EOF
[Unit]
Description=SampMail Email Marketing Platform
After=network.target postgresql.service docker.service
Wants=postgresql.service

[Service]
Type=simple
User=sampmail
Group=sampmail
EnvironmentFile=$CONFIG_FILE
ExecStart=$INSTALL_DIR/sampmail
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$DATA_DIR /var/log/sampmail
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

# Step 9: Setup Reacher
log_info "Setting up Reacher (email verification)..."

docker pull reacherhq/backend:latest

cat > /etc/systemd/system/reacher.service << EOF
[Unit]
Description=Reacher Email Verification
After=docker.service
Requires=docker.service

[Service]
Type=simple
ExecStartPre=-/usr/bin/docker stop reacher
ExecStartPre=-/usr/bin/docker rm reacher
ExecStart=/usr/bin/docker run --rm --name reacher -p 127.0.0.1:8080:8080 -e RCH_ENABLE_BULK=true reacherhq/backend:latest
ExecStop=/usr/bin/docker stop reacher
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

# Step 10: Install KumoMTA (if requested)
if [[ "$INSTALL_KUMOMTA" == true ]]; then
    log_info "Installing KumoMTA..."
    curl -fsSL https://openrepo.kumomta.com/files/kumomta-repo-setup.deb.sh | bash
    apt-get update
    apt-get install -y kumomta
    systemctl enable kumomta
    log_success "KumoMTA installed"
fi

# Step 11: Configure firewall (if UFW is installed)
if command -v ufw &> /dev/null; then
    log_info "Configuring UFW firewall..."
    ufw allow 9000/tcp comment 'SampMail Web UI' 2>/dev/null || true
    ufw allow 25/tcp comment 'SMTP' 2>/dev/null || true
    ufw allow 587/tcp comment 'SMTP Submission' 2>/dev/null || true
    ufw allow 80/tcp comment 'HTTP' 2>/dev/null || true
    ufw allow 443/tcp comment 'HTTPS' 2>/dev/null || true
    log_success "Firewall configured"
fi

# Step 12: Enable and start services
systemctl daemon-reload
systemctl enable reacher sampmail

log_info "Starting services..."
systemctl start reacher
sleep 3
systemctl start sampmail

# Wait for sampmail to start
sleep 5

# Check service status
SAMPMAIL_STATUS=$(systemctl is-active sampmail 2>/dev/null || echo "failed")
REACHER_STATUS=$(systemctl is-active reacher 2>/dev/null || echo "failed")

# Get server IP
SERVER_IP=$(hostname -I | awk '{print $1}')

# Done!
echo ""
echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}  SampMail Installation Complete!${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
echo ""
echo "  Services:"
echo "    - SampMail:   $SAMPMAIL_STATUS"
echo "    - Reacher:    $REACHER_STATUS"
echo "    - PostgreSQL: $(systemctl is-active postgresql 2>/dev/null || echo 'unknown')"
echo ""
echo -e "  ${GREEN}Access UI:${NC} http://${SERVER_IP}:9000"
echo ""
echo "  Database Credentials (saved in $CONFIG_FILE):"
echo "    - Host:     127.0.0.1"
echo "    - Port:     5432"
echo "    - User:     ${PG_USER}"
echo "    - Password: ${PG_PASSWORD}"
echo "    - Database: ${PG_DATABASE}"
echo ""
echo "  Commands:"
echo "    sudo systemctl status sampmail"
echo "    sudo journalctl -u sampmail -f"
echo ""
echo "  Config: $CONFIG_FILE"
echo ""
echo -e "${YELLOW}  Next Steps:${NC}"
if [[ "$INSTALL_KUMOMTA" != true ]]; then
    echo "    1. Install KumoMTA: sudo bash install-ubuntu.sh --with-kumomta"
    echo "    2. Configure your domain in SampMail UI"
    echo "    3. Setup reverse proxy (nginx) for HTTPS"
else
    echo "    1. Configure KumoMTA: /opt/kumomta/etc/policy"
    echo "    2. Configure your domain in SampMail UI"
    echo "    3. Setup reverse proxy (nginx) for HTTPS"
fi
echo ""
echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
