#!/bin/bash
#
# SampMail Uninstallation Script
# https://github.com/pulak-ranjan/sampmail
#

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Check root
[[ $EUID -ne 0 ]] && { echo -e "${RED}Run as root${NC}"; exit 1; }

echo ""
echo -e "${YELLOW}╔═══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${YELLOW}║           SampMail Uninstallation                             ║${NC}"
echo -e "${YELLOW}╚═══════════════════════════════════════════════════════════════╝${NC}"
echo ""

read -p "Are you sure you want to uninstall SampMail? [y/N] " -n 1 -r
echo
[[ ! $REPLY =~ ^[Yy]$ ]] && exit 1

echo ""
read -p "Delete data directory (/var/lib/sampmail)? [y/N] " -n 1 -r
DELETE_DATA=$REPLY
echo ""

# Stop services
echo -e "${YELLOW}Stopping services...${NC}"
systemctl stop sampmail 2>/dev/null || true
systemctl stop reacher 2>/dev/null || true

# Disable services
systemctl disable sampmail 2>/dev/null || true
systemctl disable reacher 2>/dev/null || true

# Remove service files
rm -f /etc/systemd/system/sampmail.service
rm -f /etc/systemd/system/reacher.service
systemctl daemon-reload

# Remove binary
rm -rf /opt/sampmail

# Remove config
rm -f /etc/sampmail.env
rm -f /etc/sampmail.env.bak

# Remove logs
rm -rf /var/log/sampmail

# Optionally remove data
if [[ $DELETE_DATA =~ ^[Yy]$ ]]; then
    echo -e "${YELLOW}Removing data directory...${NC}"
    rm -rf /var/lib/sampmail
fi

# Remove user
userdel sampmail 2>/dev/null || true

# Stop Reacher container
docker stop reacher 2>/dev/null || true
docker rm reacher 2>/dev/null || true

echo ""
echo -e "${GREEN}✓ SampMail uninstalled${NC}"
echo ""

if [[ ! $DELETE_DATA =~ ^[Yy]$ ]]; then
    echo "Data preserved at: /var/lib/sampmail"
    echo "To remove: sudo rm -rf /var/lib/sampmail"
fi
