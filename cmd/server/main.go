package main

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/api"
	"github.com/pulak-ranjan/sampmail/internal/config"
	"github.com/pulak-ranjan/sampmail/internal/core"
	"github.com/pulak-ranjan/sampmail/internal/logger"
	"github.com/pulak-ranjan/sampmail/internal/middleware/custom"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

// Version info (set at build time)
var (
	Version   = "dev"
	BuildTime = ""
	GitCommit = ""
)

func main() {
	log := logger.WithComponent("main")

	// Load and validate configuration
	cfg := config.Get()
	if err := cfg.Validate(); err != nil {
		logger.Fatal("configuration error", "error", err)
	}

	log.Info("starting SampMail",
		"version", Version,
		"build_time", BuildTime,
		"git_commit", GitCommit)

	// Ensure data directory exists
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		log.Warn("failed to create data directory", "error", err)
	}

	// Initialize database
	var st *store.Store
	var err error

	switch cfg.DatabaseDriver {
	case "postgres":
		pgConfig := &store.PostgresConfig{
			Host:            cfg.PostgresHost,
			Port:            cfg.PostgresPort,
			User:            cfg.PostgresUser,
			Password:        cfg.PostgresPassword,
			Database:        cfg.PostgresDatabase,
			SSLMode:         cfg.PostgresSSLMode,
			MaxOpenConns:    cfg.DBMaxOpenConns,
			MaxIdleConns:    cfg.DBMaxIdleConns,
			ConnMaxLifetime: time.Hour,
			ConnMaxIdleTime: 10 * time.Minute,
		}
		st, err = store.NewPostgresStore(pgConfig)
		if err != nil {
			logger.Fatal("failed to connect to PostgreSQL", "error", err)
		}
		// Run GORM AutoMigrate for PostgreSQL
		// This handles all table creation including V2 models
		// Note: V2 raw SQL migrations are skipped for PostgreSQL as they conflict with GORM
		if err := store.MigratePostgres(st); err != nil {
			logger.Fatal("failed to migrate PostgreSQL", "error", err)
		}
		log.Info("connected to PostgreSQL",
			"host", cfg.PostgresHost,
			"database", cfg.PostgresDatabase)

	default: // sqlite
		st, err = store.NewStore(cfg.DatabasePath())
		if err != nil {
			logger.Fatal("failed to open database", "error", err)
		}
		// Run V2 migrations for SQLite
		if err := store.RunV2Migrations(st.DB); err != nil {
			logger.Fatal("failed to run V2 migrations", "error", err)
		}
		log.Info("using SQLite database", "path", cfg.DatabasePath())
	}

	// Initialize AtomicOps for race-condition-free operations
	atomicOps := store.NewAtomicOps(st.DB)
	_ = atomicOps // Will be used by handlers

	// Initialize Redis rate limiters if configured
	if cfg.RedisAddr != "" {
		redisConfig := &custom.RedisConfig{
			Addr:         cfg.RedisAddr,
			Password:     cfg.RedisPassword,
			DB:           cfg.RedisDB,
			PoolSize:     100,
			MinIdleConns: 10,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
		}
		if err := custom.InitRedisRateLimiters(redisConfig); err != nil {
			log.Warn("failed to initialize Redis rate limiters, using in-memory",
				"error", err)
		} else {
			log.Info("Redis rate limiters initialized", "addr", cfg.RedisAddr)
		}
	}

	// Initialize circuit breakers
	core.InitCircuitBreakers(func(name string, from, to core.CircuitState) {
		log.Warn("circuit breaker state changed",
			"name", name,
			"from", from.String(),
			"to", to.String())
	})

	// Initialize SMTP connection pool (using fixed version)
	smtpConfig := core.DefaultSMTPPoolConfig(cfg.SMTPAddr)
	smtpConfig.MaxConnections = cfg.SMTPMaxConns
	smtpConfig.MinConnections = cfg.SMTPMinConns
	smtpConfig.ConnectTimeout = cfg.SMTPConnTimeout
	if err := core.InitSMTPPool(smtpConfig); err != nil {
		log.Warn("failed to initialize SMTP pool", "error", err)
	} else {
		log.Info("SMTP connection pool initialized",
			"addr", cfg.SMTPAddr,
			"max_conns", cfg.SMTPMaxConns)
	}

	// Initialize Reacher (email verification engine)
	core.SetupReacher()

	// Initialize Health Checker
	core.InitHealthChecker(st, Version)

	// Initialize Automation Engine with email sender
	emailSender := core.NewSMTPEmailSender("noreply@localhost")
	core.InitAutomationEngine(st, emailSender)
	log.Info("automation engine initialized")

	// Seed built-in email templates
	api.SeedBuiltInTemplates(st)

	// Initialize Core Services
	ws := core.NewWebhookService(st)
	srv := api.NewServer(st, ws)

	// Create HTTP server with production timeouts
	httpServer := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           srv.Router,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}

	// Configure TLS if enabled
	if cfg.TLSEnabled {
		httpServer.TLSConfig = &tls.Config{
			MinVersion:               tls.VersionTLS12,
			PreferServerCipherSuites: true,
			CurvePreferences: []tls.CurveID{
				tls.CurveP256,
				tls.X25519,
			},
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
		}
	}

	// Channel to receive shutdown signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Channel to indicate scheduler should stop
	schedulerCtx, cancelScheduler := context.WithCancel(context.Background())
	schedulerDone := make(chan struct{})

	// Start Background Scheduler
	go func() {
		startScheduler(schedulerCtx, st, ws)
		close(schedulerDone)
	}()

	// Start Automation Engine
	automationEngine := core.GetAutomationEngine()
	if automationEngine != nil {
		if err := automationEngine.Start(); err != nil {
			log.Error("failed to start automation engine", "error", err)
		} else {
			log.Info("automation engine started")
		}
	}

	// Start HTTP server in a goroutine
	serverErrors := make(chan error, 1)
	go func() {
		log.Info("starting HTTP server",
			"address", cfg.ListenAddr,
			"tls_enabled", cfg.TLSEnabled,
			"trust_proxy", cfg.TrustProxy)

		if cfg.TLSEnabled {
			serverErrors <- httpServer.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
		} else {
			serverErrors <- httpServer.ListenAndServe()
		}
	}()

	// Block until we receive a signal or server error
	select {
	case err := <-serverErrors:
		if err != http.ErrServerClosed {
			logger.Fatal("server error", "error", err)
		}
	case sig := <-shutdown:
		log.Info("shutdown signal received", "signal", sig)

		// Create a context with timeout for graceful shutdown
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		// Signal scheduler to stop
		cancelScheduler()

		// Stop automation engine
		if automationEngine != nil {
			log.Info("stopping automation engine...")
			automationEngine.Stop()
		}

		// Attempt graceful shutdown of HTTP server
		log.Info("shutting down HTTP server...")
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Error("HTTP server shutdown error", "error", err)
			httpServer.Close()
		}

		// Close SMTP pool
		if pool := core.GetSMTPPool(); pool != nil {
			log.Info("closing SMTP connection pool...")
			pool.Close()
		}

		// Wait for scheduler to finish
		log.Info("waiting for scheduler to finish...")
		select {
		case <-schedulerDone:
			log.Info("scheduler stopped gracefully")
		case <-ctx.Done():
			log.Warn("scheduler shutdown timed out")
		}

		log.Info("server stopped gracefully")
	}
}

func startScheduler(ctx context.Context, st *store.Store, ws *core.WebhookService) {
	log := logger.WithComponent("scheduler")
	log.Info("starting background scheduler")

	// Initialize services
	cs := core.NewCampaignService(st)
	bp := core.NewBounceProcessor(st)

	// Run startup checks (with panic recovery)
	go func() {
		defer logger.LogPanic("startup_checks")

		log.Info("running initial startup checks")

		if err := core.ProcessDailyWarmup(st); err != nil {
			log.Error("warmup startup check failed", "error", err)
		}

		if err := cs.ResumeInterruptedCampaigns(); err != nil {
			log.Error("campaign resumption failed", "error", err)
		}

		ws.RunSecurityAudit()
		ws.CheckBlacklists(false)
		ws.CheckBounceRates()

		if err := core.EnsureRecentBackup(); err != nil {
			log.Error("backup startup check failed", "error", err)
		}

		// Process bounces with streaming (memory-safe)
		if err := bp.ProcessKumoMTALogs(); err != nil {
			log.Error("bounce log processing failed", "error", err)
		}
		if err := bp.ProcessMaildirsStreaming(); err != nil {
			log.Error("maildir processing failed", "error", err)
		}
	}()

	dailyTicker := time.NewTicker(24 * time.Hour)
	hourlyTicker := time.NewTicker(1 * time.Hour)
	warmupTicker := time.NewTicker(5 * time.Minute)
	bounceTicker := time.NewTicker(10 * time.Minute)
	automationTicker := time.NewTicker(30 * time.Second) // Process automation runs

	defer func() {
		dailyTicker.Stop()
		hourlyTicker.Stop()
		warmupTicker.Stop()
		bounceTicker.Stop()
		automationTicker.Stop()
	}()

	for {
		select {
		case <-ctx.Done():
			log.Info("scheduler received shutdown signal")
			return

		case <-warmupTicker.C:
			func() {
				defer logger.LogPanic("warmup_task")
				if err := core.ProcessDailyWarmup(st); err != nil {
					log.Error("warmup error", "error", err)
				}
				if err := cs.StartScheduledCampaigns(); err != nil {
					log.Error("scheduled campaign error", "error", err)
				}
			}()

		case <-bounceTicker.C:
			func() {
				defer logger.LogPanic("bounce_task")
				if err := bp.ProcessKumoMTALogs(); err != nil {
					log.Error("bounce log error", "error", err)
				}
				// Use streaming version to avoid memory bombs
				if err := bp.ProcessMaildirsStreaming(); err != nil {
					log.Error("maildir error", "error", err)
				}
			}()

		case <-automationTicker.C:
			func() {
				defer logger.LogPanic("automation_task")
				engine := core.GetAutomationEngine()
				if engine != nil {
					engine.ProcessDelayedActions()
				}
			}()

		case <-dailyTicker.C:
			func() {
				defer logger.LogPanic("daily_task")
				log.Info("running daily tasks")

				if stats, err := core.GetAllDomainsStats(1); err == nil {
					ws.SendDailySummary(stats)
				}

				ws.RunSecurityAudit()

				if err := core.BackupConfig(); err != nil {
					log.Error("backup failed", "error", err)
				} else {
					log.Info("configuration backed up")
				}
			}()

		case <-hourlyTicker.C:
			func() {
				defer logger.LogPanic("hourly_task")
				log.Info("running hourly tasks")
				ws.CheckBlacklists(false)
				ws.CheckBounceRates()
			}()
		}
	}
}
