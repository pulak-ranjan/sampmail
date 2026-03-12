#!/bin/bash
#
# SampMail Installation Script (Universal)
# https://github.com/pulak-ranjan/sampmail
#
# Powered by KumoMTA and Reacher
# License: AGPL-3.0 / Commercial
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/pulak-ranjan/sampmail/main/scripts/install.sh | bash
#
# Or with options:
#   curl -fsSL https://raw.githubusercontent.com/pulak-ranjan/sampmail/main/scripts/install.sh | bash -s -- --with-kumomta
#

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SAMPMAIL_VERSION="${SAMPMAIL_VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-/opt/sampmail}"
DATA_DIR="${DATA_DIR:-/var/lib/sampmail}"
CONFIG_FILE="/etc/sampmail.env"
SERVICE_USER="sampmail"

# Database credentials (auto-generated)
PG_USER="sampmail"
PG_DATABASE="sampmail"
PG_PASSWORD=""

# Flags
INSTALL_KUMOMTA=false
INSTALL_REACHER=true
USE_DOCKER=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --with-kumomta)
            INSTALL_KUMOMTA=true
            shift
            ;;
        --no-reacher)
            INSTALL_REACHER=false
            shift
            ;;
        --docker)
            USE_DOCKER=true
            shift
            ;;
        --version)
            SAMPMAIL_VERSION="$2"
            shift 2
            ;;
        --help)
            echo "SampMail Installation Script"
            echo ""
            echo "Usage: install.sh [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --with-kumomta    Also install KumoMTA"
            echo "  --no-reacher      Skip Reacher installation"
            echo "  --docker          Use Docker Compose installation"
            echo "  --version VERSION Specific version to install"
            echo "  --help            Show this help"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "This script must be run as root"
        exit 1
    fi
}

detect_os() {
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        OS=$ID
        OS_VERSION=$VERSION_ID
    else
        log_error "Cannot detect OS. /etc/os-release not found."
        exit 1
    fi

    log_info "Detected OS: $OS $OS_VERSION"
}

generate_password() {
    openssl rand -base64 24 | tr -d '/+=' | head -c 24
}

install_dependencies() {
    log_info "Installing dependencies..."

    case $OS in
        ubuntu|debian)
            apt-get update -qq
            apt-get install -y -qq curl wget unzip ca-certificates gnupg lsb-release
            ;;
        centos|rhel|rocky|almalinux)
            yum install -y -q curl wget unzip ca-certificates policycoreutils-python-utils
            ;;
        fedora)
            dnf install -y -q curl wget unzip ca-certificates policycoreutils-python-utils
            ;;
        *)
            log_warn "Unknown OS: $OS. Skipping dependency installation."
            ;;
    esac
}

install_postgresql() {
    log_info "Installing PostgreSQL..."

    case $OS in
        ubuntu|debian)
            if ! command -v psql &> /dev/null; then
                sh -c 'echo "deb https://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list'
                wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add -
                apt-get update -qq
                apt-get install -y -qq postgresql-15
                systemctl enable postgresql
                systemctl start postgresql
                log_success "PostgreSQL 15 installed"
            else
                log_info "PostgreSQL already installed"
                systemctl start postgresql
            fi
            ;;
        centos|rhel|rocky|almalinux)
            if ! command -v psql &> /dev/null; then
                yum install -y https://download.postgresql.org/pub/repos/yum/reporpms/EL-9-x86_64/pgdg-redhat-repo-latest.noarch.rpm 2>/dev/null || true
                yum -qy module disable postgresql 2>/dev/null || true
                yum install -y postgresql15-server postgresql15
                /usr/pgsql-15/bin/postgresql-15-setup initdb
                systemctl enable postgresql-15
                systemctl start postgresql-15
                log_success "PostgreSQL 15 installed"
            else
                log_info "PostgreSQL already installed"
                systemctl start postgresql-15 2>/dev/null || systemctl start postgresql 2>/dev/null || true
            fi
            ;;
        fedora)
            if ! command -v psql &> /dev/null; then
                dnf install -y postgresql-server postgresql
                postgresql-setup --initdb
                systemctl enable postgresql
                systemctl start postgresql
                log_success "PostgreSQL installed"
            else
                log_info "PostgreSQL already installed"
                systemctl start postgresql
            fi
            ;;
        *)
            log_warn "Automatic PostgreSQL installation not supported for $OS. Please install manually."
            ;;
    esac
}

configure_postgresql() {
    log_info "Configuring PostgreSQL database..."

    PG_PASSWORD=$(generate_password)

    # Create database and user
    # cd /tmp to avoid "Permission denied" warnings from sudo -u postgres
    (
        cd /tmp
        sudo -u postgres psql -c "DROP DATABASE IF EXISTS ${PG_DATABASE};" 2>/dev/null || true
        sudo -u postgres psql -c "DROP USER IF EXISTS ${PG_USER};" 2>/dev/null || true
        sudo -u postgres psql -c "CREATE USER ${PG_USER} WITH PASSWORD '${PG_PASSWORD}';"
        sudo -u postgres psql -c "CREATE DATABASE ${PG_DATABASE} OWNER ${PG_USER};"
        sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE ${PG_DATABASE} TO ${PG_USER};"
    )

    # Configure pg_hba.conf for local connections
    PG_HBA=$(sudo -u postgres psql -t -c "SHOW hba_file;" | xargs)
    if ! grep -q "sampmail" "$PG_HBA" 2>/dev/null; then
        sed -i "/^# TYPE/a local   ${PG_DATABASE}      ${PG_USER}                              md5" "$PG_HBA"
        sed -i "/^# TYPE/a host    ${PG_DATABASE}      ${PG_USER}      127.0.0.1/32            md5" "$PG_HBA"
    fi

    # Reload PostgreSQL
    systemctl reload postgresql-15 2>/dev/null || systemctl reload postgresql 2>/dev/null || true

    log_success "PostgreSQL configured (user: ${PG_USER}, database: ${PG_DATABASE})"
}

create_user() {
    log_info "Creating service user: $SERVICE_USER"

    if id "$SERVICE_USER" &>/dev/null; then
        log_info "User $SERVICE_USER already exists"
    else
        useradd -r -s /bin/false -d "$DATA_DIR" "$SERVICE_USER"
    fi
}

create_directories() {
    log_info "Creating directories..."

    mkdir -p "$INSTALL_DIR"
    mkdir -p "$DATA_DIR"
    mkdir -p "$DATA_DIR/backups"
    mkdir -p /var/log/sampmail

    chown -R "$SERVICE_USER:$SERVICE_USER" "$DATA_DIR"
    chown -R "$SERVICE_USER:$SERVICE_USER" /var/log/sampmail
}

download_sampmail() {
    log_info "Downloading SampMail $SAMPMAIL_VERSION..."

    if [[ "$SAMPMAIL_VERSION" == "latest" ]]; then
        DOWNLOAD_URL="https://github.com/pulak-ranjan/sampmail/releases/latest/download/sampmail-linux-amd64.tar.gz"
    else
        DOWNLOAD_URL="https://github.com/pulak-ranjan/sampmail/releases/download/v${SAMPMAIL_VERSION}/sampmail-linux-amd64.tar.gz"
    fi

    cd /tmp
    curl -fsSL -o sampmail.tar.gz "$DOWNLOAD_URL" || {
        log_error "Failed to download SampMail"
        exit 1
    }

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

    chmod +x "$INSTALL_DIR/sampmail"
    chown -R "$SERVICE_USER:$SERVICE_USER" "$INSTALL_DIR"

    log_success "SampMail downloaded to $INSTALL_DIR"
}

configure_selinux() {
    if command -v getenforce &> /dev/null && [[ $(getenforce) != "Disabled" ]]; then
        log_info "Configuring SELinux..."

        # Set proper SELinux context for binary
        chcon -t bin_t "$INSTALL_DIR/sampmail" 2>/dev/null || true
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
}

create_config() {
    log_info "Creating configuration..."

    if [[ -f "$CONFIG_FILE" ]]; then
        log_warn "Config file exists. Backing up to ${CONFIG_FILE}.bak"
        cp "$CONFIG_FILE" "${CONFIG_FILE}.bak"
    fi

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
    chown root:$SERVICE_USER "$CONFIG_FILE"

    log_success "Configuration created at $CONFIG_FILE"
}

create_systemd_service() {
    log_info "Creating systemd service..."

    # Determine PostgreSQL service name
    PG_SERVICE="postgresql"
    if systemctl list-units --type=service | grep -q "postgresql-15"; then
        PG_SERVICE="postgresql-15"
    fi

    cat > /etc/systemd/system/sampmail.service << EOF
[Unit]
Description=SampMail - Email Marketing Platform
Documentation=https://github.com/pulak-ranjan/sampmail
After=network.target ${PG_SERVICE}.service
Wants=${PG_SERVICE}.service

[Service]
Type=simple
User=$SERVICE_USER
Group=$SERVICE_USER
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

    systemctl daemon-reload

    log_success "Systemd service created"
}

install_docker() {
    log_info "Installing Docker..."

    if ! command -v docker &> /dev/null; then
        case $OS in
            centos|rhel|rocky|almalinux)
                log_info "Using manual Docker installation for $OS..."
                yum install -y yum-utils
                yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
                yum install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
                ;;
            *)
                curl -fsSL https://get.docker.com | sh
                ;;
        esac
        systemctl enable docker
        systemctl start docker
        log_success "Docker installed"
    else
        log_info "Docker already installed"
        systemctl start docker
    fi
}

install_reacher_docker() {
    if [[ "$INSTALL_REACHER" != true ]]; then
        log_info "Skipping Reacher installation"
        return
    fi

    log_info "Installing Reacher (email verification)..."

    docker pull reacherhq/backend:latest

    # Create Reacher systemd service
    cat > /etc/systemd/system/reacher.service << EOF
[Unit]
Description=Reacher Email Verification Service
Documentation=https://reacher.email
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

    systemctl daemon-reload
    systemctl enable reacher
    systemctl start reacher

    log_success "Reacher installed and running on port 8080"
}

install_kumomta() {
    if [[ "$INSTALL_KUMOMTA" != true ]]; then
        log_info "Skipping KumoMTA installation (use --with-kumomta to install)"
        return
    fi

    log_info "Installing KumoMTA..."

    case $OS in
        ubuntu|debian)
            curl -fsSL https://openrepo.kumomta.com/files/kumomta-repo-setup.deb.sh | bash
            apt-get update
            apt-get install -y kumomta
            ;;
        centos|rhel|rocky|almalinux|fedora)
            dnf -y install dnf-plugins-core
            dnf config-manager --add-repo https://openrepo.kumomta.com/files/kumomta-rocky.repo
            yum install -y kumomta
            ;;
        *)
            log_warn "Automatic KumoMTA installation not supported for $OS"
            log_info "Please install KumoMTA manually: https://docs.kumomta.com"
            return
            ;;
    esac

    systemctl enable kumomta

    log_success "KumoMTA installed"
}

configure_firewall() {
    log_info "Configuring firewall..."

    if command -v firewall-cmd &> /dev/null; then
        firewall-cmd --permanent --add-port=9000/tcp 2>/dev/null || true
        firewall-cmd --permanent --add-port=25/tcp 2>/dev/null || true
        firewall-cmd --permanent --add-port=587/tcp 2>/dev/null || true
        firewall-cmd --permanent --add-port=80/tcp 2>/dev/null || true
        firewall-cmd --permanent --add-port=443/tcp 2>/dev/null || true
        firewall-cmd --reload 2>/dev/null || true
        log_success "Firewall (firewalld) configured"
    elif command -v ufw &> /dev/null; then
        ufw allow 9000/tcp 2>/dev/null || true
        ufw allow 25/tcp 2>/dev/null || true
        ufw allow 587/tcp 2>/dev/null || true
        ufw allow 80/tcp 2>/dev/null || true
        ufw allow 443/tcp 2>/dev/null || true
        log_success "Firewall (UFW) configured"
    else
        log_info "No firewall detected. Skipping firewall configuration."
    fi
}

docker_compose_install() {
    log_info "Installing with Docker Compose..."

    # Check Docker
    if ! command -v docker &> /dev/null; then
        log_info "Installing Docker..."
        curl -fsSL https://get.docker.com | sh
        systemctl enable docker
        systemctl start docker
    fi

    # Check Docker Compose
    if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
        log_info "Installing Docker Compose..."
        curl -fsSL "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
        chmod +x /usr/local/bin/docker-compose
    fi

    # Create directory and download compose file
    mkdir -p "$INSTALL_DIR"
    cd "$INSTALL_DIR"

    curl -fsSL "https://raw.githubusercontent.com/pulak-ranjan/sampmail/main/docker-compose.yml" -o docker-compose.yml

    # Generate .env
    SECRET=$(openssl rand -base64 32)
    echo "SAMPMAIL_SECRET=$SECRET" > .env

    # Start services
    docker compose up -d

    log_success "SampMail started with Docker Compose"
    log_info "Access at: http://localhost:9000"
}

print_success() {
    SERVER_IP=$(hostname -I | awk '{print $1}')

    echo ""
    echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}  SampMail Installation Complete!${NC}"
    echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
    echo ""
    echo "  Installation directory: $INSTALL_DIR"
    echo "  Data directory: $DATA_DIR"
    echo "  Configuration: $CONFIG_FILE"
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
    echo "  Start SampMail:"
    echo "    sudo systemctl start sampmail"
    echo ""
    echo "  Enable on boot:"
    echo "    sudo systemctl enable sampmail"
    echo ""
    echo "  View logs:"
    echo "    sudo journalctl -u sampmail -f"
    echo ""
    echo -e "${YELLOW}  Next Steps:${NC}"
    if [[ "$INSTALL_KUMOMTA" != true ]]; then
        echo "    1. Install KumoMTA: bash install.sh --with-kumomta"
    fi
    echo "    2. Configure your domain in SampMail UI"
    echo "    3. Setup reverse proxy (nginx) for HTTPS"
    echo ""
    echo "  Documentation: https://github.com/pulak-ranjan/sampmail"
    echo ""
    echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
}

# Main installation flow
main() {
    echo ""
    echo -e "${BLUE}╔═══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║           SampMail Installation Script                        ║${NC}"
    echo -e "${BLUE}║   Powered by KumoMTA (kumomta.com) & Reacher (reacher.email)  ║${NC}"
    echo -e "${BLUE}╚═══════════════════════════════════════════════════════════════╝${NC}"
    echo ""

    check_root
    detect_os

    if [[ "$USE_DOCKER" == true ]]; then
        docker_compose_install
        exit 0
    fi

    install_dependencies
    install_postgresql
    configure_postgresql
    install_docker
    create_user
    create_directories
    download_sampmail
    configure_selinux
    create_config
    create_systemd_service
    install_reacher_docker
    install_kumomta
    configure_firewall

    # Enable and start sampmail
    systemctl enable sampmail
    systemctl start sampmail

    # Wait for service to start
    sleep 5

    print_success
}

main "$@"
