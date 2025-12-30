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

# Step 1: Update system and install dependencies
log_info "Installing dependencies..."
$PKG_MGR install -y curl wget unzip ca-certificates policycoreutils-python-utils

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

# Step 9: Create systemd services
log_info "Creating systemd services..."

cat > /etc/systemd/system/sampmail.service << EOF
[Unit]
Description=SampMail Email Marketing Platform
After=network.target postgresql-15.service docker.service
Wants=postgresql-15.service

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

# Step 10: Setup Reacher
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

# Step 11: Configure firewall
if command -v firewall-cmd &> /dev/null; then
    log_info "Configuring firewall..."
    firewall-cmd --permanent --add-port=9000/tcp 2>/dev/null || true
    firewall-cmd --permanent --add-port=25/tcp 2>/dev/null || true
    firewall-cmd --permanent --add-port=587/tcp 2>/dev/null || true
    firewall-cmd --permanent --add-port=80/tcp 2>/dev/null || true
    firewall-cmd --permanent --add-port=443/tcp 2>/dev/null || true
    firewall-cmd --reload 2>/dev/null || true
    log_success "Firewall configured"
fi

# Step 12: Install KumoMTA (if requested)
if [[ "$INSTALL_KUMOMTA" == true ]]; then
    log_info "Installing KumoMTA..."
    curl -fsSL https://openrepo.kumomta.com/files/kumomta-repo-setup.rpm.sh | bash
    $PKG_MGR install -y kumomta
    systemctl enable kumomta
    log_success "KumoMTA installed"
fi

# Step 13: Enable and start services
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
echo "    - PostgreSQL: $(systemctl is-active postgresql-15 2>/dev/null || systemctl is-active postgresql 2>/dev/null || echo 'unknown')"
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
    echo "    1. Install KumoMTA: sudo bash install-rocky.sh --with-kumomta"
    echo "    2. Configure your domain in SampMail UI"
    echo "    3. Setup reverse proxy (nginx) for HTTPS"
else
    echo "    1. Configure KumoMTA: /opt/kumomta/etc/policy"
    echo "    2. Configure your domain in SampMail UI"
    echo "    3. Setup reverse proxy (nginx) for HTTPS"
fi
echo ""
echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
