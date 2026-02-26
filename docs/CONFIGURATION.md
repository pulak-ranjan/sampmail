# SampMail Configuration Reference

SampMail is configured entirely through environment variables. There is no configuration
file format other than the `.env`-style key-value files read by the systemd service
unit and Docker Compose.

For the systemd installation, variables are loaded from `/etc/sampmail.env`.
For Docker Compose, they are loaded from the `.env` file in the Compose project
directory.

---

## How variables are resolved

When the process starts, `config.Get()` is called exactly once and the result is
cached for the lifetime of the process. Variables are read with the following priority:

1. Explicit environment variable set in the shell or EnvironmentFile.
2. Default value defined in `internal/config/config.go`.

There is no live reload. To apply a configuration change, restart the process.

---

## SAMPMAIL_BASE_URL — how it works

This variable is the most important one to get right for a working installation.

**What it controls:**

Every tracking link, click redirect URL, and unsubscribe URL embedded in campaign
emails is built from this value. If it is wrong, clicks and opens will not be tracked,
and the unsubscribe page will not be reachable.

**Auto-detection:**

If `SAMPMAIL_BASE_URL` is not set, the server derives a fallback from
`SAMPMAIL_LISTEN_ADDR`:

- `0.0.0.0:9000` becomes `http://localhost:9000`
- `[::]:9000` becomes `http://localhost:9000`
- Any other address is used as-is with the `http://` scheme.

If `SAMPMAIL_TLS_ENABLED=true`, the scheme becomes `https://`.

**Auto-seeding into the database:**

On first startup, the hostname extracted from `SAMPMAIL_BASE_URL` is written to the
`app_settings` table as `main_hostname`. Subsequent restarts only update the stored
value if `main_hostname` is currently empty. This means:

- A fresh installation works immediately with no UI configuration.
- After the first startup, operators can update `main_hostname` in the UI without
  it being overwritten on the next restart.
- To force an update from the environment variable, clear `main_hostname` in the UI
  settings and restart.

---

## SAMPMAIL_SECRET — key derivation

The master secret is used as input to PBKDF2-SHA256 with 100,000 iterations to derive
two 32-byte keys:

| Key | Salt |
|---|---|
| Encryption key | sampmail-encryption-key-v1 |
| Session key | sampmail-session-key-v1 |

These keys are used for AES-256 encryption of stored credentials and for HMAC-SHA256
signing of session tokens and tracking links respectively.

Changing this value after first use is a destructive operation. All active sessions
become invalid, and any data encrypted with the derived encryption key becomes
unreadable.

---

## Full variable reference

### Application identity

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_SECRET | Required | Master secret. Minimum 32 characters. |
| SAMPMAIL_BASE_URL | Derived from SAMPMAIL_LISTEN_ADDR | Public-facing base URL for tracking and unsubscribe links. Example: https://mail.yourdomain.com |
| SAMPMAIL_ENV | development | Set to production for JSON logging and production behavior. |
| SAMPMAIL_VERSION | dev | Version string. Set at build time. |

### HTTP server

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_LISTEN_ADDR | 127.0.0.1:9000 | Host and port the HTTP server listens on. Use 0.0.0.0:9000 to accept connections on all interfaces. |
| SAMPMAIL_REQUEST_TIMEOUT | 30s | Maximum time allowed to process a single request. |
| SAMPMAIL_MAX_REQUEST_BODY | 10485760 | Maximum request body size in bytes. The async import endpoint uses a separate 50 MB limit. |
| SAMPMAIL_SHUTDOWN_TIMEOUT | 30s | Grace period for in-flight requests when a shutdown signal is received. |

### TLS (optional — use Nginx instead)

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_TLS_ENABLED | false | Enable TLS on the application HTTP server directly. |
| SAMPMAIL_TLS_CERT | | Path to the PEM-format TLS certificate file. Required when TLS is enabled. |
| SAMPMAIL_TLS_KEY | | Path to the PEM-format TLS private key file. Required when TLS is enabled. |

### Proxy

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_TRUST_PROXY | false | Trust X-Forwarded-For and X-Forwarded-Proto headers from the upstream proxy. Must be set to true when running behind Nginx or another reverse proxy. |
| SAMPMAIL_TRUSTED_PROXY_CIDR | | Comma-separated list of CIDR ranges from which proxy headers are trusted. Example: 127.0.0.1/32,10.0.0.0/8 |

### Database

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_DB_DRIVER | sqlite | Database engine. Use postgres for production. |
| DATABASE_URL | | Full PostgreSQL connection string. Parsed automatically when set. Takes precedence over individual SAMPMAIL_PG_* variables. Example: postgres://user:pass@host:5432/db?sslmode=disable |
| SAMPMAIL_PG_HOST | localhost | PostgreSQL server hostname. |
| SAMPMAIL_PG_PORT | 5432 | PostgreSQL server port. |
| SAMPMAIL_PG_USER | | PostgreSQL username. Required when using the postgres driver. |
| SAMPMAIL_PG_PASSWORD | | PostgreSQL password. Required when using the postgres driver. |
| SAMPMAIL_PG_DATABASE | | PostgreSQL database name. Required when using the postgres driver. |
| SAMPMAIL_PG_SSLMODE | require | PostgreSQL SSL mode: require, disable, or verify-full. Use disable only on localhost connections. |
| SAMPMAIL_DB_MAX_OPEN_CONNS | 100 | Maximum number of open connections in the database connection pool. |
| SAMPMAIL_DB_MAX_IDLE_CONNS | 10 | Number of idle connections kept alive in the pool at all times. |

### Redis

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_REDIS_ADDR | | Redis server address, e.g. redis:6379 or 127.0.0.1:6379. When not set, all rate limiting uses in-memory counters. |
| SAMPMAIL_REDIS_PASSWORD | | Redis AUTH password. |
| SAMPMAIL_REDIS_DB | 0 | Redis logical database number (0-15). |

### SMTP and KumoMTA

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_SMTP_ADDR | kumomta:587 | KumoMTA SMTP submission address (host:port). |
| SAMPMAIL_SMTP_MAX_CONNS | 100 | Maximum number of connections in the SMTP pool. Must be at least as large as SAMPMAIL_CAMPAIGN_WORKERS. |
| SAMPMAIL_SMTP_MIN_CONNS | 5 | Number of connections kept alive in the pool at all times. |
| SAMPMAIL_SMTP_CONN_TIMEOUT | 10s | Timeout for establishing a new SMTP connection. |
| SAMPMAIL_KUMO_DIR | /opt/kumomta | KumoMTA installation root. Used for DKIM key management. |
| SAMPMAIL_LOG_DIR | /var/log/kumomta | Directory where KumoMTA writes delivery logs. Read by the bounce processor. |
| SAMPMAIL_SPOOL_DIR | /var/spool/kumomta | KumoMTA spool directory. |
| SAMPMAIL_KUMO_API_URL | http://127.0.0.1:8000 | KumoMTA HTTP management API URL. |

### Email verification

| Variable | Default | Description |
|---|---|---|
| REACHER_URL | http://reacher:8080 | Reacher service base URL. |
| REACHER_API_KEY | | API key for Reacher Cloud. Leave empty for self-hosted Reacher. |

### Campaigns

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_CAMPAIGN_WORKERS | 50 | Number of goroutines sending emails concurrently per campaign. Higher values increase throughput but also CPU and SMTP pool usage. |
| SAMPMAIL_IMPORT_BATCH_SIZE | 500 | Number of rows processed per database transaction during CSV import. Lower values use less memory. |

### Security

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_BCRYPT_COST | 12 | bcrypt cost factor for password hashing. Each increment doubles the computation time. Values 12 to 14 are appropriate for production. |
| SAMPMAIL_FIREWALL_ENABLED | true | Automatically block IP addresses that exhibit attack patterns such as repeated auth failures. |

### File paths

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_DATA_DIR | /var/lib/sampmail | Root directory for the SQLite database file and configuration backups. |
| SAMPMAIL_KUMO_DIR | /opt/kumomta | See SMTP and KumoMTA section above. |
| SAMPMAIL_HOME_DIR | /home | System home directory. Used internally for certain path operations. |

### Logging

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_LOG_LEVEL | info | Log verbosity: debug, info, warn, or error. Use debug only for troubleshooting — it is very verbose. |
| SAMPMAIL_LOG_JSON | true when SAMPMAIL_ENV=production | Emit structured JSON log lines. Set to false for human-readable output during development. |

### Monitoring

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_METRICS_ENABLED | true | Expose a Prometheus-compatible metrics endpoint. |
| SAMPMAIL_METRICS_PATH | /metrics | URL path for the Prometheus scrape endpoint. |

### Backups

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_BACKUP_RETENTION | 3 | Maximum number of configuration backup copies retained in the data directory. Older copies are deleted automatically. |

---

## Example: minimal production configuration

```bash
# /etc/sampmail.env
# Permissions: 600 — readable only by root and the sampmail group

SAMPMAIL_SECRET=YourMinimum32CharacterSecretHere1234
SAMPMAIL_BASE_URL=https://mail.yourdomain.com

SAMPMAIL_LISTEN_ADDR=0.0.0.0:9000
SAMPMAIL_ENV=production

SAMPMAIL_DB_DRIVER=postgres
SAMPMAIL_PG_HOST=127.0.0.1
SAMPMAIL_PG_USER=sampmail
SAMPMAIL_PG_PASSWORD=YourDatabasePassword
SAMPMAIL_PG_DATABASE=sampmail
SAMPMAIL_PG_SSLMODE=disable

SAMPMAIL_TRUST_PROXY=true
SAMPMAIL_TRUSTED_PROXY_CIDR=127.0.0.1/32

SAMPMAIL_SMTP_ADDR=127.0.0.1:587
REACHER_URL=http://127.0.0.1:8080

SAMPMAIL_LOG_JSON=true
SAMPMAIL_LOG_LEVEL=info
```

---

## Example: Docker Compose .env

```bash
# .env — do not commit to version control

SAMPMAIL_SECRET=YourMinimum32CharacterSecretHere1234
POSTGRES_PASSWORD=YourDatabasePassword
SAMPMAIL_BASE_URL=https://mail.yourdomain.com

SAMPMAIL_DB_DRIVER=postgres
POSTGRES_USER=sampmail
POSTGRES_DB=sampmail
SAMPMAIL_REDIS_ADDR=redis:6379
```

---

## Verifying the configuration

To confirm the application starts with a valid configuration:

```bash
systemctl start sampmail
journalctl -u sampmail -n 30 --no-pager
```

The startup log shows:

- The resolved listen address.
- The database driver and connection target.
- Whether Redis was successfully connected.
- The SMTP pool status.
- The Reacher URL.
- The auto-seeded hostname (on first run).

If any required variable is missing or invalid, the process exits immediately with a
descriptive error message before attempting any network connections.
