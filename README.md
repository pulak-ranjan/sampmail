# SampMail - Self-Hosted Email Marketing Platform

<p align="center">
  <img src="docs/logo/sampmail-logo.png" alt="SampMail Logo" width="200">
</p>

<p align="center">
  <strong>Enterprise-grade email marketing with full control over your infrastructure</strong>
</p>

<p align="center">
  <a href="#features">Features</a> •
  <a href="#installation">Installation</a> •
  <a href="#configuration">Configuration</a> •
  <a href="#api-documentation">API</a> •
  <a href="#license">License</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-0.2.0-blue.svg" alt="Version 0.2.0">
  <img src="https://img.shields.io/badge/license-AGPL--3.0-green.svg" alt="License AGPL-3.0">
  <img src="https://img.shields.io/badge/go-1.21+-00ADD8.svg" alt="Go 1.21+">
  <img src="https://img.shields.io/badge/node-18+-339933.svg" alt="Node 18+">
</p>

---

## 📋 Table of Contents

- [Overview](#overview)
- [Features](#features)
- [System Requirements](#system-requirements)
- [Installation](#installation)
- [Configuration](#configuration)
- [Dashboard](#dashboard)
- [API Documentation](#api-documentation)
- [Version Control & Updates](#version-control--updates)
- [Security](#security)
- [Contributing](#contributing)
- [License](#license)
- [Support](#support)

---

## Overview

SampMail is a powerful, self-hosted email marketing platform designed for businesses that need complete control over their email infrastructure. Built with Go for performance and React for a modern user experience.

### Why SampMail?

- **Full Control**: Own your data and infrastructure
- **Cost Effective**: No per-email fees after setup
- **Privacy First**: Your subscriber data never leaves your servers
- **Scalable**: Handle millions of emails with proper infrastructure
- **Extensible**: REST API for custom integrations

---

## Features

### 📧 Email Campaigns
- Drag-and-drop email builder
- MJML template support
- A/B testing
- Scheduled sending
- Real-time analytics

### 🤖 Marketing Automation
- Visual automation builder
- Trigger-based workflows
- Behavioral targeting
- Lead scoring
- Drip campaigns

### 📊 Analytics & Reporting
- Real-time open/click tracking
- Bounce management
- Engagement scoring
- Custom reports
- Export capabilities

### 🔐 Security & Compliance
- DKIM/SPF/DMARC support
- Automatic bounce handling
- Suppression list management
- Rate limiting
- GDPR-ready unsubscribe

### 🛠 Infrastructure
- **Powered by KumoMTA** (High-performance delivery engine)
- Multi-domain support
- IP warmup management
- SMTP connection pooling
- Circuit breaker patterns
- Health monitoring
- **Proxy Rotation Manager** (SOCKS5/HTTP support)

### 🆕 V2 Features (0.2.0)
- **Admin Dashboard** - System-wide health, services, security management
- **User Dashboard** - Org-scoped campaign stats, performance, activity
- **Service Manager** - One-click install/start/stop for KumoMTA, Dovecot, Reacher
- Multi-tenant organizations with isolation
- Organization Switcher in sidebar
- **AI-powered Agent** (OpenAI, DeepSeek, Google Gemini)
- Advanced personalization engine
- Enhanced API with pagination
- Disabled public registration (admin-only user creation)

---

## System Requirements

### Minimum Requirements
| Component | Requirement |
|-----------|-------------|
| CPU | 2 cores |
| RAM | 4 GB |
| Storage | 20 GB SSD |
| OS | Ubuntu 22.04+ / Rocky Linux 9+ |

### Recommended for Production
| Component | Requirement |
|-----------|-------------|
| CPU | 4+ cores |
| RAM | 8+ GB |
| Storage | 100+ GB SSD |
| Database | PostgreSQL 14+ |
| Cache | Redis 7+ |

### Software Dependencies
- Go 1.21+
- Node.js 18+
- PostgreSQL 14+ (recommended) or SQLite
- Redis 7+ (optional, for rate limiting)
- Nginx (reverse proxy)

---

## Installation

### Prerequisites (Fresh VPS)

```bash
# Update system and install git (if not installed)
sudo apt update && sudo apt upgrade -y
sudo apt install -y git curl
```

### Quick Install (Ubuntu/Debian)

```bash
# Download and run installer (recommended - handles all dependencies)
curl -sSL https://raw.githubusercontent.com/pulak-ranjan/sampmail/main/scripts/install.sh | sudo bash

# Or clone and install manually
git clone https://github.com/pulak-ranjan/sampmail.git
cd sampmail
sudo ./scripts/install-ubuntu.sh
```

### Quick Install (Rocky Linux/RHEL)

```bash
# Update system and install git (if not installed)
sudo dnf update -y
sudo dnf install -y git curl

# Clone and install
git clone https://github.com/pulak-ranjan/sampmail.git
cd sampmail
sudo ./scripts/install-rocky.sh
```

### Docker Installation

```bash
# Install Docker (if not installed)
curl -fsSL https://get.docker.com | sudo sh

# Clone and run
git clone https://github.com/pulak-ranjan/sampmail.git
cd sampmail
sudo docker-compose up -d
```

### Manual Installation

See [INSTALLATION.md](docs/INSTALLATION.md) for detailed manual installation instructions.

---

## Configuration

### Environment Variables

Copy the example configuration:

```bash
cp .env.example .env
```

Key configuration options:

```env
# Server
SAMPMAIL_LISTEN_ADDR=:9000
SAMPMAIL_BASE_URL=https://mail.yourdomain.com

# Database (PostgreSQL recommended for production)
SAMPMAIL_DATABASE_DRIVER=postgres
SAMPMAIL_POSTGRES_HOST=localhost
SAMPMAIL_POSTGRES_PORT=5432
SAMPMAIL_POSTGRES_USER=sampmail
SAMPMAIL_POSTGRES_PASSWORD=your_secure_password
SAMPMAIL_POSTGRES_DATABASE=sampmail

# Redis (optional but recommended)
SAMPMAIL_REDIS_ADDR=localhost:6379

# SMTP
SAMPMAIL_SMTP_ADDR=localhost:25
SAMPMAIL_SMTP_MAX_CONNS=50

# Security
SAMPMAIL_JWT_SECRET=your_32_char_secret_here
SAMPMAIL_ENCRYPTION_KEY=your_32_char_key_here

# AI Features (optional)
SAMPMAIL_OPENAI_API_KEY=sk-...
```

See [.env.example](.env.example) for all available options.

---

## Dashboard

Access the dashboard at `http://your-server:9000` after installation.

### Default Credentials
```
Email: admin@localhost
Password: admin123
```

⚠️ **Change these immediately after first login!**

### Dashboard Features
- Campaign management
- Subscriber lists
- Template builder
- Analytics
- Domain configuration
- System health monitoring
- **One-click updates** (see below)

---

## API Documentation

Full API documentation is available at `/api/docs` when running the server.

### Quick Examples

```bash
# Authentication
curl -X POST http://localhost:9000/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@localhost","password":"admin123"}'

# List campaigns
curl http://localhost:9000/api/campaigns \
  -H "Authorization: Bearer YOUR_TOKEN"

# Create campaign
curl -X POST http://localhost:9000/api/campaigns \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"My Campaign","subject":"Hello!","html_content":"<p>Hi {{name}}</p>"}'
```

See [API.md](docs/API.md) for complete documentation.

---

## Version Control & Updates

SampMail includes a built-in version control system with one-click updates from the dashboard.

### Current Version

```
Version: 0.1.12
Release Date: December 29, 2025
Release Channel: stable
```

### Version Information API

```bash
# Check current version
GET /api/version

# Response:
{
  "version": "0.1.12",
  "build_time": "2025-12-29T00:00:00Z",
  "git_commit": "abc123...",
  "channel": "stable",
  "go_version": "go1.21",
  "os": "linux",
  "arch": "amd64"
}
```

### Automatic Update Check

SampMail checks for updates every 24 hours and displays notifications in the dashboard when updates are available.

### One-Click Updates

1. Navigate to **Settings → System → Updates** in the dashboard
2. Click **Check for Updates**
3. Review the changelog
4. Click **Update to v2.x.x**
5. Wait for automatic restart

### Update Channels

| Channel | Description | Recommended For |
|---------|-------------|-----------------|
| `lts` | Long Term Support - stable, security fixes only | Production |
| `stable` | Regular stable releases | Most users |
| `beta` | Pre-release features | Testing |

Configure in `.env`:
```env
SAMPMAIL_UPDATE_CHANNEL=lts
```

### Manual Updates

```bash
# Stop the service
sudo systemctl stop sampmail

# Backup database
pg_dump sampmail > backup_$(date +%Y%m%d).sql

# Pull latest version
cd /opt/sampmail
git fetch origin
git checkout v2.1.0  # or latest tag

# Rebuild
go build -o sampmail ./cmd/server
cd web && npm install && npm run build && cd ..

# Run migrations
./sampmail migrate

# Start service
sudo systemctl start sampmail
```

### Rollback

```bash
# Stop service
sudo systemctl stop sampmail

# Checkout previous version
git checkout v2.0.0

# Rebuild and restart
go build -o sampmail ./cmd/server
sudo systemctl start sampmail
```

---

## Security

### Security Features
- JWT authentication with refresh tokens
- TOTP two-factor authentication
- Rate limiting (Redis-backed)
- Input validation and sanitization
- SQL injection prevention (GORM)
- XSS protection
- CSRF tokens
- Encrypted password storage (bcrypt)
- Encrypted sensitive data (AES-256)

### Security Best Practices

1. **Change default credentials immediately**
2. **Use HTTPS** with valid SSL certificates
3. **Enable 2FA** for admin accounts
4. **Regular backups** of database and DKIM keys
5. **Keep updated** via one-click updates
6. **Monitor logs** for suspicious activity

### Reporting Vulnerabilities

Please report security vulnerabilities to: cloudnesh@gmail.com

See [SECURITY.md](docs/SECURITY.md) for our security policy.

---

## Contributing

We welcome contributions! Please read our [Contributing Guide](docs/CONTRIBUTING.md) before submitting PRs.

### Development Setup

```bash
# Clone repository
git clone https://github.com/pulak-ranjan/sampmail.git
cd sampmail

# Install Go dependencies
go mod download

# Install Node dependencies
cd web && npm install && cd ..

# Run in development mode
make dev
```

### Code Style
- Go: `gofmt` and `golint`
- JavaScript: ESLint with Prettier
- Commits: Conventional Commits format

---

## License

SampMail is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**.

```
SampMail - Self-Hosted Email Marketing Platform
Copyright (C) 2025 Pulak Ranjan

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
```

### What AGPL-3.0 Means

- ✅ **Free to use** for personal and internal purposes
- ✅ **Free to modify** and create derivative works
- ✅ **Free to distribute** copies
- ⚠️ **Must disclose source** of any modifications
- ⚠️ **Network use is distribution** - if you run a modified version as a service, you must provide source code to users
- ⚠️ **Must use same license** for derivative works

### Commercial License

For businesses that want to use SampMail without open source obligations (keep modifications private, white-label, etc.), a commercial license is available. Contact: cloudnesh@gmail.com

See [LICENSE.md](LICENSE.md) for the full license text.

---

## Support

### Documentation
- [Installation Guide](docs/INSTALLATION.md)
- [API Reference](docs/API.md)
- [DNS & IP Reputation Guide](docs/DNS_AND_REPUTATION.md) ⚠️ **Important**
- [Security Policy](docs/SECURITY.md)
- [Changelog](docs/CHANGELOG.md)

### Community
- GitHub Issues: Bug reports and feature requests
- GitHub Discussions: Questions and community support

### Commercial Support
For enterprise support, custom development, or consulting:
- Email: cloudnesh@gmail.com
- Website: https://pulak-ranjan.github.io/me/

---

## Acknowledgments

SampMail is built with these amazing open-source projects:

### Core Infrastructure
- [KumoMTA](https://kumomta.com/) - High-performance MTA (Mail Transfer Agent)
- [Reacher.email](https://reacher.email/) - Email verification service

### Backend
- [Go](https://golang.org/) - Backend language
- [Chi](https://github.com/go-chi/chi) - HTTP router
- [GORM](https://gorm.io/) - ORM
- [Redis](https://redis.io/) - Caching and delayed queue

### Frontend
- [React](https://reactjs.org/) - Frontend framework
- [Tailwind CSS](https://tailwindcss.com/) - Styling
- [Lucide](https://lucide.dev/) - Icons

---

<p align="center">
  Made with ❤️ by the SampMail Community
</p>
