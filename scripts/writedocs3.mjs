import { writeFileSync } from 'fs';
import { join } from 'path';

const root = 'D:\\Mail Sender\\sampmail\\sampmail';

const readme = `# SampMail

SampMail is a self-hosted email marketing platform built with Go and React. It gives you complete control over your email infrastructure, subscriber data, and sending pipeline. There are no per-email fees after initial setup and no third-party access to your data.

---

## Table of Contents

- [Overview](#overview)
- [Features](#features)
- [System Requirements](#system-requirements)
- [Installation](#installation)
- [Configuration](#configuration)
- [Dashboard](#dashboard)
- [API Documentation](#api-documentation)
- [Security](#security)
- [Updates](#updates)
- [Contributing](#contributing)
- [License](#license)
- [Support](#support)

---

## Overview

SampMail handles the full email marketing lifecycle: subscriber management, campaign authoring, sending via KumoMTA, open and click tracking, bounce processing, suppression list management, and delivery analytics. All components run on infrastructure you own.

The backend is written in Go (Chi router, GORM, PostgreSQL or SQLite). The frontend is React with Tailwind CSS. The sending engine delegates to KumoMTA on port 587 with a connection pool and circuit breaker. Email verification is handled by an optional Reacher integration.

---

## Features

### Email Campaigns

- HTML and plain-text composition
- MJML template support
- Personalization with merge tags (name, email, organization, custom fields)
- A/B testing
- Scheduled sending with timezone support
- Real-time open and click tracking with HMAC-signed links

### Marketing Automation

- Trigger-based workflows (subscribe, open, click, date-based)
- Drip sequences
- Behavioral branching
- Lead scoring

### Analytics

- Real-time open and click counters (atomic, race-free)
- Bounce rate, complaint rate, unsubscribe rate
- Deliverability trend data
- Per-campaign and aggregate reporting

### Compliance and Deliverability

- DKIM signing (per-domain keys stored encrypted)
- SPF and DMARC guidance
- RFC-8058 one-click unsubscribe header
- List-Unsubscribe header
- Automatic bounce classification (hard, soft, FBL complaint)
- Suppression list enforced on every send
- GDPR-ready unsubscribe flow

### Infrastructure

- KumoMTA integration (high-performance MTA on port 587)
- SMTP connection pool with configurable concurrency
- Circuit breaker on SMTP failures
- Redis-backed rate limiting with in-memory fallback
- Multi-tenant organizations with full data isolation
- Optional Reacher integration for email address verification
- SOCKS5 and HTTP proxy rotation manager

### V2 Features (0.2.0)

- Multi-tenant organization model with role-based access
- Organization switcher in the sidebar
- Admin dashboard with system-wide health and service status
- One-click install, start, and stop for KumoMTA, Dovecot, and Reacher from the dashboard
- AI writing assistant (OpenAI, DeepSeek, Google Gemini)
- Enhanced REST API with pagination and cursor support
- Disabled public registration (admin-controlled user provisioning)

---

## System Requirements

### Minimum

| Component | Requirement                       |
|-----------|-----------------------------------|
| CPU       | 2 cores                           |
| RAM       | 4 GB                              |
| Storage   | 20 GB SSD                         |
| OS        | Ubuntu 22.04+ or Rocky Linux 9+   |

### Recommended for Production

| Component | Requirement            |
|-----------|------------------------|
| CPU       | 4+ cores               |
| RAM       | 8+ GB                  |
| Storage   | 100+ GB SSD            |
| Database  | PostgreSQL 14+         |
| Cache     | Redis 7+               |

### Software Dependencies

- Go 1.21+
- Node.js 18+
- PostgreSQL 14+ (recommended) or SQLite (development only)
- Redis 7+ (optional, for rate limiting and caching)
- Nginx (recommended as reverse proxy for TLS termination)

---

## Installation

### Quick Install (Ubuntu / Debian VPS)

The install script handles all dependencies: Go, Node.js, PostgreSQL, Redis, KumoMTA, and Nginx. It auto-detects your server's public IP, generates all secrets, and configures the service.

\`\`\`bash
sudo apt update && sudo apt install -y git curl
git clone https://github.com/pulak-ranjan/sampmail.git
cd sampmail
sudo bash scripts/install.sh
\`\`\`

To override the auto-detected URL:

\`\`\`bash
sudo bash scripts/install.sh --url https://mail.yourdomain.com
\`\`\`

### Docker Compose

\`\`\`bash
curl -fsSL https://get.docker.com | sudo sh
git clone https://github.com/pulak-ranjan/sampmail.git
cd sampmail
cp .env.example .env
# Edit .env and set SAMPMAIL_SECRET and SAMPMAIL_BASE_URL
sudo docker compose up -d
\`\`\`

### Manual Installation

See [docs/INSTALLATION.md](docs/INSTALLATION.md) for step-by-step manual installation, Nginx and TLS configuration, KumoMTA setup, and building from source.

---

## Configuration

Copy the example environment file and edit the required values:

\`\`\`bash
cp .env.example .env
\`\`\`

### Required Variables

\`\`\`env
# Master secret: all encryption and session keys are derived from this value.
# Generate with: openssl rand -base64 32
SAMPMAIL_SECRET=your-secret-here

# Public base URL of this installation.
# Used in tracking links, unsubscribe URLs, and RFC-8058 headers.
SAMPMAIL_BASE_URL=https://mail.yourdomain.com

# PostgreSQL password
SAMPMAIL_PG_PASSWORD=your-database-password
\`\`\`

### Common Optional Variables

\`\`\`env
# Server
SAMPMAIL_LISTEN_ADDR=0.0.0.0:9000

# Database (defaults to SQLite if not set)
SAMPMAIL_DB_DRIVER=postgres
SAMPMAIL_PG_HOST=localhost
SAMPMAIL_PG_PORT=5432
SAMPMAIL_PG_USER=sampmail
SAMPMAIL_PG_DBNAME=sampmail

# Redis
SAMPMAIL_REDIS_ADDR=localhost:6379

# SMTP (KumoMTA)
SAMPMAIL_SMTP_ADDR=localhost:587
SAMPMAIL_SMTP_MAX_CONNS=50

# AI assistant (optional)
SAMPMAIL_OPENAI_API_KEY=sk-...
\`\`\`

See [docs/CONFIGURATION.md](docs/CONFIGURATION.md) for the complete variable reference and key derivation details.

---

## Dashboard

After installation, open the address shown by the install script in your browser.

Default credentials created on first run:

\`\`\`
Email:    admin@localhost
Password: admin123
\`\`\`

Change the password and email address immediately after first login. Enable two-factor authentication for all administrator accounts.

### Dashboard Sections

| Section          | Description                                               |
|------------------|-----------------------------------------------------------|
| Campaigns        | Create, schedule, and monitor email campaigns             |
| Lists            | Manage subscriber lists and segments                      |
| Templates        | Author and version HTML and MJML templates                |
| Contacts         | Import CSV, view subscriber profiles and activity         |
| Automations      | Build trigger-based workflow sequences                    |
| Domains          | Configure sending domains and DKIM keys                   |
| Analytics        | Open, click, bounce, and unsubscribe reporting            |
| System (admin)   | Service health, KumoMTA control, update management        |
| Organizations    | Manage tenants and user assignments (superadmin)          |

---

## API Documentation

The REST API uses bearer token authentication. Obtain a token from the login endpoint and include it in the Authorization header of subsequent requests.

\`\`\`bash
# Login
curl -s -X POST http://localhost:9000/api/auth/login \\
  -H "Content-Type: application/json" \\
  -d '{"email":"admin@localhost","password":"admin123"}'

# List campaigns
curl -s http://localhost:9000/api/v2/campaigns \\
  -H "Authorization: Bearer YOUR_TOKEN"

# Create a campaign
curl -s -X POST http://localhost:9000/api/v2/campaigns \\
  -H "Authorization: Bearer YOUR_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"name":"Welcome Series","subject":"Welcome to our list","status":"draft"}'
\`\`\`

See [docs/API.md](docs/API.md) for the complete endpoint reference including request bodies, response shapes, and error codes.

---

## Security

- All encryption and session keys are derived from \`SAMPMAIL_SECRET\` using PBKDF2-SHA256 (100,000 iterations) with separate salts. A single secret is the only credential you need to manage.
- Session tokens are 32 random bytes. The raw token is never stored; only its HMAC-SHA256 hash is persisted.
- Passwords are hashed with bcrypt at cost factor 12 or higher.
- Tracking links (open pixel and click redirect) are HMAC-signed. Requests with invalid signatures return 403.
- Rate limiting is enforced on authentication, 2FA, and password reset endpoints.
- TOTP two-factor authentication is available for all user accounts.

See [docs/SECURITY.md](docs/SECURITY.md) for the full security model, hardening checklist, and vulnerability reporting instructions.

---

## Updates

### Checking the Current Version

\`\`\`bash
curl -s http://localhost:9000/api/system/health | jq .version
\`\`\`

### Manual Update

\`\`\`bash
sudo systemctl stop sampmail
pg_dump sampmail > backup_$(date +%Y%m%d).sql

cd /opt/sampmail
git fetch origin
git checkout v0.2.1   # replace with target version

go build -o sampmail ./cmd/server
cd web && npm ci && npm run build && cd ..

sudo systemctl start sampmail
\`\`\`

### Rollback

\`\`\`bash
sudo systemctl stop sampmail
git checkout v0.2.0
go build -o sampmail ./cmd/server
sudo systemctl start sampmail
\`\`\`

---

## Contributing

1. Fork the repository.
2. Create a feature branch: \`git checkout -b feature/my-feature\`
3. Make your changes, add tests where applicable.
4. Format Go code with \`gofmt\` and lint with \`golangci-lint run\`.
5. Format frontend code with the project ESLint and Prettier configuration.
6. Open a pull request against the \`main\` branch.

See [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) for the full contribution guide.

---

## License

SampMail is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0).

This means:

- You may use, modify, and distribute SampMail freely.
- If you run a modified version as a network service, you must make the modified source code available to users of that service.
- Derivative works must be licensed under the same terms.

For businesses that need to keep modifications private or white-label the product, a commercial license is available. Contact cloudnesh@gmail.com.

See [LICENSE](LICENSE) for the full license text.

---

## Support

### Documentation

| Document                              | Contents                                                 |
|---------------------------------------|----------------------------------------------------------|
| [docs/INSTALLATION.md](docs/INSTALLATION.md) | Full install guide, Nginx, TLS, KumoMTA, troubleshooting |
| [docs/CONFIGURATION.md](docs/CONFIGURATION.md) | All environment variables and key derivation             |
| [docs/API.md](docs/API.md)            | Complete REST API reference                              |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | System design, request lifecycle, data flow              |
| [docs/CAMPAIGNS.md](docs/CAMPAIGNS.md) | Campaign lifecycle, tracking, deliverability             |
| [docs/SECURITY.md](docs/SECURITY.md)  | Security model, hardening, vulnerability reporting       |
| [docs/CHANGELOG.md](docs/CHANGELOG.md) | Release history                                          |

### Community and Commercial Support

- GitHub Issues: bug reports and feature requests
- GitHub Discussions: questions and usage help
- Commercial support and custom development: cloudnesh@gmail.com

---

## Acknowledgments

SampMail is built on the following open-source projects:

- [KumoMTA](https://kumomta.com/) - High-performance mail transfer agent
- [Reacher](https://reacher.email/) - Email address verification
- [Go](https://golang.org/) - Backend language
- [Chi](https://github.com/go-chi/chi) - HTTP router
- [GORM](https://gorm.io/) - ORM and migration layer
- [React](https://reactjs.org/) - Frontend framework
- [Tailwind CSS](https://tailwindcss.com/) - Utility-first CSS framework
- [Redis](https://redis.io/) - Rate limiting and caching
`;

writeFileSync(join(root, 'README.md'), readme, 'utf8');
console.log('README.md written');
