# ============================================================
# SampMail - Self-hosted Email Marketing Platform
# 
# Powered by KumoMTA and Reacher
# AGPL-3.0 / Commercial License
# ============================================================

# Build stage - Backend
FROM golang:1.21-alpine AS backend-builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with version info
ARG VERSION=dev
ARG BUILD_TIME
ARG GIT_COMMIT

RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags="-s -w \
    -X main.Version=${VERSION} \
    -X main.BuildTime=${BUILD_TIME} \
    -X main.GitCommit=${GIT_COMMIT}" \
    -o sampmail ./cmd/server

# Build stage - Frontend (using Bun)
FROM oven/bun:1 AS frontend-builder

WORKDIR /app/web

# Copy package files
COPY web/package.json web/bun.lockb* ./
RUN bun install --frozen-lockfile

# Copy source and build
COPY web/ ./
RUN bun run build

# Final stage
FROM alpine:3.19

LABEL org.opencontainers.image.title="SampMail"
LABEL org.opencontainers.image.description="Self-hosted Email Marketing Platform powered by KumoMTA and Reacher"
LABEL org.opencontainers.image.source="https://github.com/pulak-ranjan/sampmail"
LABEL org.opencontainers.image.licenses="AGPL-3.0"

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    sqlite \
    tzdata \
    curl \
    && rm -rf /var/cache/apk/*

# Create non-root user with specific UID/GID for security
RUN addgroup -g 1000 sampmail && \
    adduser -u 1000 -G sampmail -s /sbin/nologin -D sampmail

# Create directories with proper permissions
RUN mkdir -p /var/lib/sampmail /opt/kumomta/etc /var/log/sampmail && \
    chown -R sampmail:sampmail /var/lib/sampmail /var/log/sampmail

# Copy binaries
COPY --from=backend-builder /app/sampmail /usr/local/bin/
COPY --from=frontend-builder /app/web/dist /var/www/sampmail

# Set ownership
RUN chown sampmail:sampmail /usr/local/bin/sampmail

# Set working directory
WORKDIR /var/lib/sampmail

# Switch to non-root user
USER sampmail

# Environment defaults
ENV SAMPMAIL_LISTEN_ADDR=0.0.0.0:9000 \
    SAMPMAIL_DATA_DIR=/var/lib/sampmail \
    SAMPMAIL_KUMO_DIR=/opt/kumomta \
    SAMPMAIL_LOG_DIR=/var/log/kumomta \
    SAMPMAIL_ENV=production \
    SAMPMAIL_LOG_JSON=true \
    SAMPMAIL_METRICS_ENABLED=true \
    SAMPMAIL_BCRYPT_COST=12 \
    REACHER_URL=http://reacher:8080

# Expose port
EXPOSE 9000

# Health check - use the new dedicated endpoint
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q --spider http://localhost:9000/health/ready || exit 1

# Security: Read-only root filesystem friendly
VOLUME ["/var/lib/sampmail"]

# Run
ENTRYPOINT ["sampmail"]
