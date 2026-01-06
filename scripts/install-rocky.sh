#!/bin/bash
#
# SampMail Installation Script for Rocky Linux / RHEL / CentOS / AlmaLinux
# https://github.com/pulak-ranjan/sampmail
#
# Tested on:
#   - Rocky Linux 9
#   - RHEL 9
#   - AlmaLinux 9
#   - CentOS Stream 9
#
# Usage:
#   sudo bash install-rocky.sh [OPTIONS]
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
            echo "SampMail Installation Script for Rocky/RHEL/CentOS/AlmaLinux"
            echo ""
            echo "Usage: sudo bash install-rocky.sh [OPTIONS]"
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
echo -e "${BLUE}║     SampMail Installer for Rocky/RHEL/CentOS/AlmaLinux        ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Detect package manager
if command -v dnf &> /dev/null; then
    PKG_MGR="dnf"
else
    PKG_MGR="yum"
fi

# Generate random password for PostgreSQL
generate_password() {
    openssl rand -base64 24 | tr -d '/+=' | head -c 24
}

# Step 1: Install EPEL and base dependencies
log_info "Installing EPEL release and dependencies..."
$PKG_MGR install -y epel-release
$PKG_MGR install -y curl wget unzip ca-certificates policycoreutils-python-utils \
    dnf-plugins-core bind-utils firewalld fail2ban fail2ban-firewalld nano

# Enable firewalld and fail2ban early
systemctl enable --now firewalld 2>/dev/null || true
systemctl enable --now fail2ban 2>/dev/null || true

# Disable postfix if present (conflicts with SMTP)
systemctl disable --now postfix 2>/dev/null || true

# Step 2: Install PostgreSQL
log_info "Installing PostgreSQL..."
if ! command -v psql &> /dev/null; then
    # Install PostgreSQL 15
    $PKG_MGR install -y https://download.postgresql.org/pub/repos/yum/reporpms/EL-9-x86_64/pgdg-redhat-repo-latest.noarch.rpm 2>/dev/null || true
    $PKG_MGR -qy module disable postgresql 2>/dev/null || true
    $PKG_MGR install -y postgresql15-server postgresql15

    # Initialize and start PostgreSQL
    /usr/pgsql-15/bin/postgresql-15-setup initdb
    systemctl enable postgresql-15
    systemctl start postgresql-15
    log_success "PostgreSQL 15 installed"
else
    log_info "PostgreSQL already installed"
    # Ensure it's running
    systemctl start postgresql-15 2>/dev/null || systemctl start postgresql 2>/dev/null || true
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
    # Add local MD5 auth for sampmail user before the default entries
    sed -i "/^# TYPE/a local   ${PG_DATABASE}      ${PG_USER}                              md5" "$PG_HBA"
    sed -i "/^# TYPE/a host    ${PG_DATABASE}      ${PG_USER}      127.0.0.1/32            md5" "$PG_HBA"
fi

# Reload PostgreSQL to apply changes
systemctl reload postgresql-15 2>/dev/null || systemctl reload postgresql 2>/dev/null || true
log_success "PostgreSQL configured (user: ${PG_USER}, database: ${PG_DATABASE})"

# Step 3b: Install Redis (for automation engine scaling)
log_info "Installing Redis..."
if ! command -v redis-server &> /dev/null; then
    $PKG_MGR install -y redis
    systemctl enable redis
    systemctl start redis
    log_success "Redis installed and started"
else
    log_info "Redis already installed"
    systemctl start redis 2>/dev/null || true
fi

# Step 4: Install Docker
log_info "Installing Docker..."
if ! command -v docker &> /dev/null; then
    $PKG_MGR install -y yum-utils
    yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
    $PKG_MGR install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
    systemctl enable docker
    systemctl start docker
    log_success "Docker installed"
else
    log_info "Docker already installed"
    systemctl start docker
fi

# Step 5: Create user and directories
log_info "Creating sampmail user and directories..."
id sampmail &>/dev/null || useradd -r -s /sbin/nologin -d "$DATA_DIR" sampmail
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
    log_warn "Binary not found. Please build from source."
    log_info "See: https://github.com/pulak-ranjan/sampmail#building-from-source"
    exit 1
}

tar -xzf sampmail.tar.gz -C "$INSTALL_DIR"
rm -f sampmail.tar.gz

# Handle nested directory from old release format
if [[ -d "$INSTALL_DIR/sampmail" ]]; then
    # Move binary out first to avoid name collision
    if [[ -f "$INSTALL_DIR/sampmail/sampmail" ]]; then
        mv "$INSTALL_DIR/sampmail/sampmail" "$INSTALL_DIR/sampmail-bin"
    fi
    mv "$INSTALL_DIR/sampmail/"* "$INSTALL_DIR/" 2>/dev/null || true
    rmdir "$INSTALL_DIR/sampmail" 2>/dev/null || rm -rf "$INSTALL_DIR/sampmail" 2>/dev/null || true
    if [[ -f "$INSTALL_DIR/sampmail-bin" ]]; then
        mv "$INSTALL_DIR/sampmail-bin" "$INSTALL_DIR/sampmail"
    fi
fi

chmod +x "$INSTALL_DIR/sampmail"
chown -R sampmail:sampmail "$INSTALL_DIR"
log_success "SampMail installed to $INSTALL_DIR"

# Step 7: Configure SELinux
if command -v getenforce &> /dev/null && [[ $(getenforce) != "Disabled" ]]; then
    log_info "Configuring SELinux..."

    # Set proper SELinux context for binary
    chcon -t bin_t "$INSTALL_DIR/sampmail"
    restorecon -Rv "$INSTALL_DIR" 2>/dev/null || true

    # Allow network connections
    setsebool -P httpd_can_network_connect 1 2>/dev/null || true
    setsebool -P httpd_can_network_connect_db 1 2>/dev/null || true

    # Set proper context for data directory
    semanage fcontext -a -t var_lib_t "${DATA_DIR}(/.*)?" 2>/dev/null || true
    restorecon -Rv "$DATA_DIR" 2>/dev/null || true

    # Set proper context for log directory
    semanage fcontext -a -t var_log_t "/var/log/sampmail(/.*)?" 2>/dev/null || true
    restorecon -Rv /var/log/sampmail 2>/dev/null || true

    log_success "SELinux configured"
fi

# Helper: Find a free port starting from a base port
find_free_port() {
    local port=$1
    while ss -tuln | grep -q ":$port "; do
        ((port++))
    done
    echo "$port"
}

# Helper: Check if a specific port is in use
is_port_occupied() {
    local port=$1
    ss -tuln | grep -q ":$port "
}

# Step 0: Pre-flight Port Check
log_info "Running pre-flight port checks..."
if ! command -v ss &> /dev/null; then
    $PKG_MGR install -y iproute
fi

if is_port_occupied 80; then
    log_warn "Port 80 is in use. Nginx might fail to start if you don't stop the existing service."
fi

# Detect dynamic ports for internal services
REACHER_PORT=$(find_free_port 8080)
if [[ "$REACHER_PORT" != "8080" ]]; then
    log_warn "Port 8080 is busy. Moving Reacher to port $REACHER_PORT"
fi

SAMPMAIL_PORT=$(find_free_port 9000)
if [[ "$SAMPMAIL_PORT" != "9000" ]]; then
    log_warn "Port 9000 is busy. Moving SampMail Backend to port $SAMPMAIL_PORT"
fi

# Step 8: Generate config
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
# SECURITY: Bind only to localhost. Nginx handles external traffic.
SAMPMAIL_LISTEN_ADDR=127.0.0.1:$SAMPMAIL_PORT
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
REACHER_URL=http://127.0.0.1:$REACHER_PORT

# Performance
SAMPMAIL_CAMPAIGN_WORKERS=50
SAMPMAIL_DB_MAX_OPEN_CONNS=100
SAMPMAIL_DB_MAX_IDLE_CONNS=10

# Redis (for automation engine scaling)
SAMPMAIL_REDIS_ADDR=127.0.0.1:6379
EOF

chmod 640 "$CONFIG_FILE"
chown root:sampmail "$CONFIG_FILE"
log_success "Configuration saved to $CONFIG_FILE"

# Step 9: Install nginx for frontend
log_info "Installing nginx..."
if ! command -v nginx &> /dev/null; then
    $PKG_MGR install -y nginx
    log_success "Nginx installed"
else
    log_info "Nginx already installed"
fi

# Configure nginx to serve frontend and proxy API
cat > /etc/nginx/conf.d/sampmail.conf << NGINX
server {
    listen 80 default_server;
    listen [::]:80 default_server;
    server_name _;

    root /opt/sampmail/web;
    index index.html;

    # Gzip compression
    gzip on;
    gzip_types text/plain text/css application/json application/javascript text/xml application/xml;

    # Frontend - serve static files, fallback to index.html for SPA
    location / {
        try_files \$uri \$uri/ /index.html;
    }

    # API - proxy to backend
    location /api/ {
        proxy_pass http://127.0.0.1:$SAMPMAIL_PORT;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }

    # Health check endpoint
    location /health {
        proxy_pass http://127.0.0.1:$SAMPMAIL_PORT;
        proxy_set_header Host \$host;
    }
}
NGINX

# Allow nginx to read web files (SELinux)
if command -v setsebool &> /dev/null; then
    setsebool -P httpd_can_network_connect 1 2>/dev/null || true
    chcon -Rt httpd_sys_content_t /opt/sampmail/web 2>/dev/null || true
fi

# Remove default nginx config if exists
rm -f /etc/nginx/conf.d/default.conf 2>/dev/null || true

systemctl enable nginx
log_success "Nginx configured"

# Install Certbot for Let's Encrypt SSL
log_info "Installing Certbot for SSL..."
$PKG_MGR install -y certbot python3-certbot-nginx
log_success "Certbot installed - run 'sudo certbot --nginx' to enable HTTPS"

# Step 10: Create systemd services
log_info "Creating systemd services..."

cat > /etc/systemd/system/sampmail.service << EOF
[Unit]
Description=SampMail Email Marketing Platform
After=network.target postgresql-15.service docker.service redis.service
Wants=postgresql-15.service redis.service

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

# Step 11: Setup Reacher
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

# Step 12: Configure firewall ports
log_info "Configuring firewall ports..."

# Standard SMTP and web ports
firewall-cmd --permanent --add-service=http 2>/dev/null || true
firewall-cmd --permanent --add-service=https 2>/dev/null || true
firewall-cmd --permanent --add-port=25/tcp 2>/dev/null || true
firewall-cmd --permanent --add-port=587/tcp 2>/dev/null || true
firewall-cmd --permanent --add-port=465/tcp 2>/dev/null || true

# SECURITY: Close port 9000 to public - only nginx accesses backend locally
firewall-cmd --permanent --remove-port=9000/tcp 2>/dev/null || true

firewall-cmd --reload 2>/dev/null || true
log_success "Firewall ports configured"

# SELinux: Allow nginx to proxy to backend
if command -v semanage &> /dev/null; then
    semanage port -a -t http_port_t -p tcp 9000 2>/dev/null || true
fi

# Step 13: Install KumoMTA (if requested)
if [[ "$INSTALL_KUMOMTA" == true ]]; then
    log_info "Installing KumoMTA..."

    # Try multiple installation methods
    KUMOMTA_INSTALLED=false

    # Method 1: Official repo using dnf config-manager (same as kumomta-ui)
    if dnf config-manager --add-repo https://openrepo.kumomta.com/files/kumomta-rocky.repo 2>/dev/null; then
        if $PKG_MGR install -y kumomta 2>/dev/null; then
            KUMOMTA_INSTALLED=true
            log_success "KumoMTA installed from official repo"
        fi
    fi

    # Method 2: Try direct from GitHub releases
    if [[ "$KUMOMTA_INSTALLED" != true ]]; then
        log_warn "Official repo failed. Trying GitHub releases..."
        KUMOMTA_URL="https://github.com/KumoCorp/kumomta/releases/latest/download/kumomta-el9.x86_64.rpm"
        if curl -fsSL -o /tmp/kumomta.rpm "$KUMOMTA_URL" 2>/dev/null; then
            if $PKG_MGR install -y /tmp/kumomta.rpm 2>/dev/null; then
                KUMOMTA_INSTALLED=true
                log_success "KumoMTA installed from GitHub"
            fi
        fi
    fi

    # Method 3: Provide manual instructions
    if [[ "$KUMOMTA_INSTALLED" != true ]]; then
        log_warn "Could not auto-install KumoMTA. Please install manually:"
        echo "    See: https://docs.kumomta.com/installation/linux/"
    else
        systemctl enable kumomta 2>/dev/null || true
        mkdir -p /opt/kumomta/etc/policy 2>/dev/null || true
    fi
fi

# Step 14: Enable and start services
systemctl daemon-reload
systemctl enable reacher sampmail

log_info "Starting services..."
systemctl start nginx
systemctl start reacher
sleep 3
systemctl start sampmail

# Wait for sampmail to start
sleep 5

# Check service status
SAMPMAIL_STATUS=$(systemctl is-active sampmail 2>/dev/null || echo "failed")
REACHER_STATUS=$(systemctl is-active reacher 2>/dev/null || echo "failed")
NGINX_STATUS=$(systemctl is-active nginx 2>/dev/null || echo "failed")

# Get server IP
SERVER_IP=$(hostname -I | awk '{print $1}')

# Done!
echo ""
echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${GREEN}  ✓ SampMail Installation Complete!${NC}"
echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
echo ""
echo "  Services:"
echo "    - SampMail:   $SAMPMAIL_STATUS"
echo "    - Nginx:      $NGINX_STATUS"
echo "    - Reacher:    $REACHER_STATUS"
echo "    - PostgreSQL: $(systemctl is-active postgresql-15 2>/dev/null || systemctl is-active postgresql 2>/dev/null || echo 'unknown')"
echo ""
echo -e "  ${GREEN}Access UI:${NC} http://${SERVER_IP}"
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
    echo "    1. Install KumoMTA: sudo bash install-rocky.sh --with-kumomta"
    echo "    2. Configure your domain in SampMail UI"
    echo "    3. Setup SSL with: sudo certbot --nginx"
else
    echo "    1. Configure KumoMTA: /opt/kumomta/etc/policy"
    echo "    2. Configure your domain in SampMail UI"
    echo "    3. Setup SSL with: sudo certbot --nginx"
fi
echo ""
echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"

