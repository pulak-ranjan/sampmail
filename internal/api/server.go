package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/pulak-ranjan/sampmail/internal/config"
	"github.com/pulak-ranjan/sampmail/internal/core"
	"github.com/pulak-ranjan/sampmail/internal/logger"
	"github.com/pulak-ranjan/sampmail/internal/middleware/custom"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

type Server struct {
	Store  *store.Store
	WS     *core.WebhookService
	Router chi.Router
}

func NewServer(st *store.Store, ws *core.WebhookService) *Server {
	s := &Server{Store: st, WS: ws}

	// Wire up WebSocket origin validator using app settings
	wsAllowedOrigins = func(origin string) bool {
		settings, err := st.GetSettings()
		if err != nil || settings.AllowedOrigins == "" {
			return false
		}
		for _, a := range strings.Split(settings.AllowedOrigins, ",") {
			if strings.TrimSpace(a) == origin {
				return true
			}
		}
		return false
	}

	s.Router = s.routes()
	return s
}

func (s *Server) routes() chi.Router {
	r := chi.NewRouter()

	cfg := config.Get()

	// Core middleware - order matters!
	r.Use(custom.RequestIDMiddleware)                       // Add request ID first
	r.Use(custom.SecurityHeadersMiddleware)                 // Security headers
	r.Use(middleware.RealIP)                                // Extract real IP
	r.Use(middleware.Logger)                                // Request logging
	r.Use(middleware.Recoverer)                             // Panic recovery
	r.Use(custom.TimeoutMiddleware(cfg.RequestTimeout))     // Request timeout
	r.Use(custom.MaxBodySizeMiddleware(cfg.MaxRequestBody)) // Limit body size

	// Metrics middleware (if enabled)
	if cfg.MetricsEnabled {
		r.Use(custom.MetricsMiddleware)
	}

	// Rate limiting - use Redis if available, else in-memory
	r.Use(custom.GetRateLimiter("general").Limit)

	// Dynamic CORS for Credentials support
	r.Use(cors.Handler(cors.Options{
		AllowOriginFunc: func(r *http.Request, origin string) bool {
			// In development, allow localhost
			if strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "http://127.0.0.1") {
				return true
			}

			settings, err := s.Store.GetSettings()
			if err != nil {
				return false
			}

			// If no origins configured, default to denying external access (safe default)
			if settings.AllowedOrigins == "" {
				return false
			}

			allowed := strings.Split(settings.AllowedOrigins, ",")
			for _, a := range allowed {
				if strings.TrimSpace(a) == origin {
					return true
				}
			}
			return false
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Temp-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// --- Public Routes ---
	r.With(custom.AuthLimiter.Limit).Post("/api/auth/register", s.handleRegister)
	r.With(custom.AuthLimiter.Limit).Post("/api/auth/login", s.handleLogin)
	r.With(custom.AuthLimiter.Limit).Post("/api/auth/verify-2fa", s.handleVerify2FA)

	// --- Protected Routes ---
	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)

		// WebSocket (real-time updates)
		r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
			admin := getAdminFromContext(r.Context())
			org := getOrganizationFromContext(r.Context())
			var orgID uint
			if org != nil {
				orgID = org.ID
			}
			HandleWebSocket(w, r, admin.ID, orgID)
		})

		// Auth & Profile
		r.Get("/api/auth/me", s.handleMe)
		r.Post("/api/auth/logout", s.handleLogout)
		r.With(custom.AuthLimiter.Limit).Post("/api/auth/setup-2fa", s.handleSetup2FA)
		r.With(custom.AuthLimiter.Limit).Post("/api/auth/enable-2fa", s.handleEnable2FA)
		r.With(custom.AuthLimiter.Limit).Post("/api/auth/disable-2fa", s.handleDisable2FA)
		r.Post("/api/auth/theme", s.handleSetTheme)
		r.Get("/api/auth/sessions", s.handleListSessions)

		// Dashboard
		r.Get("/api/dashboard/stats", s.handleGetDashboardStats)

		// Settings
		r.Get("/api/settings", s.handleGetSettings)
		r.Post("/api/settings", s.handleSetSettings)
		r.Get("/api/config/runtime", s.handleGetRuntimeConfig)
		r.Post("/api/config/runtime", s.handleSetRuntimeConfig)
		r.Get("/api/config/bot", s.handleGetBotConfig)
		r.Post("/api/config/bot", s.handleSetBotConfig)

		// Domains
		r.Get("/api/domains", s.handleListDomains)
		r.Post("/api/domains", s.handleCreateDomain)
		r.Get("/api/domains/{id}", s.handleGetDomain)
		r.Put("/api/domains/{id}", s.handleUpdateDomain)
		r.Delete("/api/domains/{id}", s.handleDeleteDomain)

		// Senders
		r.Get("/api/domains/{domainID}/senders", s.handleListSenders)
		r.Post("/api/domains/{domainID}/senders", s.handleCreateSender)
		r.Get("/api/senders/{id}", s.handleGetSender)
		r.Put("/api/senders/{id}", s.handleUpdateSender)
		r.Delete("/api/senders/{id}", s.handleDeleteSender)
		r.Post("/api/domains/{domainID}/senders/{id}/setup", s.handleSetupSender)

		// Bounce Accounts
		r.Get("/api/bounces", s.handleListBounce)
		r.Post("/api/bounces", s.handleSaveBounceAccount)
		r.Delete("/api/bounces/{bounceID}", s.handleDeleteBounceAccount)
		r.Post("/api/bounces/apply", s.handleApplyBounceAccounts)

		// System IPs
		r.Get("/api/system/ips", s.handleListIPs)
		r.Post("/api/system/ips", s.handleAddIP)
		r.Post("/api/system/ips/configure", s.handleConfigureIP) // <--- NEW ROUTE
		r.Delete("/api/system/ips/{id}", s.handleDeleteIP)
		r.Post("/api/system/ips/bulk", s.handleBulkAddIPs)
		r.Post("/api/system/ips/cidr", s.handleAddIPsByCIDR)
		r.Post("/api/system/ips/detect", s.handleDetectIPs)

		// DKIM
		r.Get("/api/dkim/records", s.handleListDKIM)
		r.Post("/api/dkim/generate", s.handleGenerateDKIM)

		// DMARC & DNS
		r.Get("/api/dmarc/{domainID}", s.handleGetDMARC)
		r.Post("/api/dmarc/{domainID}", s.handleSetDMARC)
		r.Get("/api/dmarc/reports", s.handleGetDMARCReports) // Frontend compatibility
		r.Get("/api/dmarc/stats", s.handleGetDMARCStats)     // Frontend compatibility
		r.Get("/api/dns/{domainID}", s.handleGetAllDNS)

		// Lists (aliases for /v2/lists for frontend compatibility)
		listHandler := NewListHandler(s.Store)
		r.Get("/api/lists", listHandler.ListLists)
		r.Post("/api/lists", listHandler.CreateList)
		r.Get("/api/lists/{id}", listHandler.GetList)
		r.Delete("/api/lists/{id}", listHandler.DeleteList)
		r.Get("/api/lists/{id}/contacts", listHandler.ListSubscribers)
		r.Post("/api/lists/{id}/contacts", listHandler.AddSubscriber)

		// Stats
		r.Get("/api/stats", s.handleGetStatsSummary) // Alias for frontend compatibility
		r.Get("/api/stats/domains", s.handleGetDomainStats)
		r.Get("/api/stats/domains/{domain}", s.handleGetSingleDomainStats)
		r.Get("/api/stats/summary", s.handleGetStatsSummary)
		r.Post("/api/stats/refresh", s.handleRefreshStats)

		// Queue Management
		r.Get("/api/queue", s.handleGetQueue)
		r.Get("/api/queue/stats", s.handleGetQueueStats)
		r.Delete("/api/queue/{id}", s.handleDeleteQueueMessage)
		r.Post("/api/queue/flush", s.handleFlushQueue)
		r.Post("/api/queue/retry/{id}", s.handleRetryQueueMessage)
		r.Post("/api/queue/retry/all", s.handleRetryAllDeferred)
		r.Delete("/api/queue/bounced", s.handleDeleteBounced)
		r.Get("/api/queue/domains/kumo", s.handleGetKumoDomainStats)

		// KumoMTA Service Control
		r.Get("/api/kumo/status", s.handleGetKumoStatus)
		r.Post("/api/kumo/start", s.handleStartKumo)
		r.Post("/api/kumo/stop", s.handleStopKumo)
		r.Post("/api/kumo/restart", s.handleRestartKumo)
		r.Post("/api/kumo/reload", s.handleReloadKumo)
		r.Get("/api/kumo/logs", s.handleGetKumoLogs)

		// Webhooks - Settings
		r.Get("/api/webhooks/settings", s.handleGetWebhookSettings)
		r.Post("/api/webhooks/settings", s.handleSetWebhookSettings)
		r.Post("/api/webhooks/test", s.handleTestWebhook)
		r.Get("/api/webhooks/logs", s.handleGetWebhookLogs)
		r.Post("/api/webhooks/check-bounces", s.handleCheckBounces)

		// Webhooks - CRUD (for WebhooksPage.jsx)
		r.Get("/api/webhooks", s.handleListWebhooks)
		r.Post("/api/webhooks", s.handleCreateWebhook)
		r.Delete("/api/webhooks/{id}", s.handleDeleteWebhook)
		r.Post("/api/webhooks/{id}/test", s.handleTestSingleWebhook)

		// System Tools & Actions (Guardian)
		r.Post("/api/system/check-blacklist", s.handleCheckBlacklist)
		r.Post("/api/system/check-security", s.handleCheckSecurity)
		r.Post("/api/system/action/block-ip", s.handleBlockIP)
		r.Post("/api/tools/send-test", s.handleSendTestEmail)

		// AI Chat & Analysis
		r.Post("/api/system/ai-analyze", s.handleAIAnalyze)
		r.Get("/api/ai/history", s.handleGetChatHistory)
		r.Post("/api/ai/chat", s.handleAIChat)
		r.Get("/api/ai/usage", s.handleGetAIUsageStats)

		// Warmup Routes
		r.Get("/api/warmup", s.handleGetWarmupList)
		r.Post("/api/warmup/{id}", s.handleUpdateWarmup)

		// API Keys Routes
		r.Get("/api/keys", s.handleListKeys)
		r.Post("/api/keys", s.handleCreateKey)
		r.Delete("/api/keys/{id}", s.handleDeleteKey)

		// Config
		r.Get("/api/config/preview", s.handlePreviewConfig)
		r.Post("/api/config/apply", s.handleApplyConfig)

		// Logs
		r.Get("/api/logs/kumomta", s.handleLogsKumo)
		r.Get("/api/logs/dovecot", s.handleLogsDovecot)
		r.Get("/api/logs/fail2ban", s.handleLogsFail2ban)

		// AI Bounce Analysis
		r.Post("/api/bounces/analyze", s.handleAIBounceAnalysis)
		r.Get("/api/bounces", s.handleListBounces)
		r.Get("/api/bounces/stats", s.handleBounceStats)

		// System Health
		r.Get("/api/system/health", s.handleSystemHealth)
		r.Get("/api/system/services", s.handleSystemServices)
		r.Get("/api/system/ports", s.handleSystemPorts)

		// SSL/HTTPS Management
		sslHandler := NewSSLHandler()
		r.Get("/api/system/ssl", sslHandler.HandleGetSSLStatus)
		r.Post("/api/system/ssl/install", sslHandler.HandleInstallSSL)
		r.Post("/api/system/ssl/renew", sslHandler.HandleRenewSSL)

		// Version & Updates
		r.Get("/api/version", s.handleGetVersion)
		r.Get("/api/updates/status", s.handleUpdateStatus)
		r.Post("/api/updates/check", s.handleCheckForUpdates)
		r.Post("/api/updates/download", s.handleDownloadUpdate)
		r.Post("/api/updates/apply", s.handleApplyUpdate)
		r.Post("/api/updates/rollback", s.handleRollback)
		r.Get("/api/updates/changelog", s.handleUpdateChangelog)

		// Service Manager - One-click install and control
		serviceManager := NewServiceHandlerWithStore(s.Store)
		r.Get("/api/services/status", serviceManager.HandleGetStatus)
		r.Post("/api/services/kumomta/install", serviceManager.HandleInstall)
		r.Post("/api/services/kumomta/start", serviceManager.HandleStart)
		r.Post("/api/services/kumomta/stop", serviceManager.HandleStop)
		r.Post("/api/services/kumomta/restart", serviceManager.HandleRestart)
		r.Post("/api/services/dovecot/install", serviceManager.HandleInstall)
		r.Post("/api/services/dovecot/start", serviceManager.HandleStart)
		r.Post("/api/services/dovecot/stop", serviceManager.HandleStop)
		r.Post("/api/services/dovecot/restart", serviceManager.HandleRestart)
		r.Post("/api/services/reacher/install", serviceManager.HandleInstall)
		r.Post("/api/services/reacher/start", serviceManager.HandleStart)
		r.Post("/api/services/reacher/stop", serviceManager.HandleStop)
		r.Post("/api/services/reacher/restart", serviceManager.HandleRestart)

		// Bulk Import (rate limited to prevent abuse)
		r.With(custom.ImportLimiter.Limit).Post("/api/import/csv", s.handleCSVImport)            // Sync (limited to 1MB)
		r.With(custom.ImportLimiter.Limit).Post("/api/import/csv/async", s.handleCSVImportAsync) // Async (up to 50MB)
		r.Get("/api/import/status/{jobId}", s.handleImportStatus)

		// Proxy Management
		r.Get("/api/proxies", s.handleListProxies)
		r.Post("/api/proxies", s.handleCreateProxy)
		r.Get("/api/proxies/{id}", s.handleGetProxy)
		r.Put("/api/proxies/{id}", s.handleUpdateProxy)
		r.Delete("/api/proxies/{id}", s.handleDeleteProxy)
		r.Post("/api/proxies/{id}/test", s.handleTestProxy)

		// Sending IP Management
		r.Get("/api/sending-ips", s.handleListSendingIPs)
		r.Post("/api/sending-ips", s.handleCreateSendingIP)
		r.Put("/api/sending-ips/{id}", s.handleUpdateSendingIP)
		r.Delete("/api/sending-ips/{id}", s.handleDeleteSendingIP)

		// Campaigns
		r.Route("/api/campaigns", NewCampaignHandler(s.Store).Routes)

		// Suppression List Management
		suppression := NewSuppressionHandler(s.Store)
		r.Route("/api/suppressions", suppressionRoutes(suppression))

		// Tracking (Public, no auth)
		// Note: TrackingHandler methods need to be wrapped or unprotected.
		// Since this group is protected, we must move tracking OUTSIDE or use skip logic.
	})

	// --- Tracking Routes (Unprotected but rate limited) ---
	tracking := NewTrackingHandler(s.Store)
	r.With(custom.TrackingLimiter.Limit).Get("/api/track/open/{id}", tracking.HandleTrackOpen)
	r.With(custom.TrackingLimiter.Limit).Get("/api/track/click/{id}", tracking.HandleTrackClick)

	// --- Unsubscribe Routes (Unprotected - users click these from emails) ---
	unsub := NewUnsubscribeHandler(s.Store)
	r.With(custom.TrackingLimiter.Limit).Get("/api/unsubscribe/{token}", unsub.HandleUnsubscribePage)
	r.With(custom.TrackingLimiter.Limit).Post("/api/unsubscribe/{token}", unsub.HandleUnsubscribeConfirm)

	// --- Analytics (Protected) ---
	r.Group(func(r chi.Router) {
		r.Use(s.authMiddleware)
		analytics := NewAnalyticsHandler(s.Store)
		r.Get("/api/analytics/top-leads", analytics.GetTopLeads)
		r.Get("/api/analytics/campaign-summary", analytics.GetCampaignSummary)

		// Enhanced Analytics V2
		analyticsV2 := NewAnalyticsHandlerV2(s.Store)
		r.Get("/api/analytics/dashboard", analyticsV2.Dashboard)
		r.Get("/api/analytics/campaigns/{id}", analyticsV2.Campaign)
		r.Get("/api/analytics/domains", analyticsV2.Domains)
		r.Get("/api/analytics/deliverability", analyticsV2.Deliverability)
		r.Get("/api/analytics/realtime", analyticsV2.Realtime)

		// Templates
		templates := NewTemplateHandler(s.Store)
		r.Get("/api/templates", templates.List)
		r.Post("/api/templates", templates.Create)
		r.Get("/api/templates/library", templates.Library)
		r.Get("/api/templates/categories", templates.Categories)
		r.Get("/api/templates/{id}", templates.Get)
		r.Put("/api/templates/{id}", templates.Update)
		r.Delete("/api/templates/{id}", templates.Delete)
		r.Post("/api/templates/{id}/clone", templates.Clone)
		r.Post("/api/templates/{id}/preview", templates.Preview)

		// Tags
		tags := NewTagHandler(s.Store)
		r.Get("/api/tags", tags.List)
		r.Post("/api/tags", tags.Create)
		r.Put("/api/tags/{id}", tags.Update)
		r.Delete("/api/tags/{id}", tags.Delete)
		r.Get("/api/subscribers/{id}/tags", tags.GetSubscriberTags)
		r.Post("/api/subscribers/{id}/tags", tags.AddToSubscriber)
		r.Delete("/api/subscribers/{id}/tags", tags.RemoveFromSubscriber)

		// Segments
		segments := NewSegmentHandler(s.Store)
		r.Get("/api/segments", segments.List)
		r.Post("/api/segments", segments.Create)
		r.Get("/api/segments/{id}", segments.Get)
		r.Put("/api/segments/{id}", segments.Update)
		r.Delete("/api/segments/{id}", segments.Delete)
		r.Get("/api/segments/{id}/subscribers", segments.GetSubscribers)
		r.Post("/api/segments/{id}/refresh", segments.Refresh)

		contacts := NewContactHandler(s.Store)
		r.With(custom.VerifyLimiter.Limit).Post("/api/contacts/verify", contacts.HandleVerifyEmail)
		r.With(custom.VerifyLimiter.Limit).Post("/api/contacts/verify-batch", contacts.HandleVerifyBatch)
		r.Post("/api/lists/{id}/clean", contacts.HandleCleanList)

		// Automation & WhatsApp
		wa := NewWhatsAppHandler(s.Store)
		r.Post("/api/whatsapp/send", wa.HandleSend)
		r.Post("/api/whatsapp/webhook", wa.HandleWebhook)

		// Telegram Bot Webhook
		r.Post("/api/telegram/webhook", s.handleTelegramWebhook)

		// Discord Bot Webhook
		r.Post("/api/discord/webhook", s.handleDiscordWebhook)

		// =====================================
		// API V2 - Templates, Automations, Campaigns
		// =====================================

		// Group that requires Organization Context
		r.Group(func(r chi.Router) {
			r.Use(s.organizationMiddleware) // <--- Enforce Organization Context

			// Templates V2 - Drag & Drop Builder
			templateV2 := NewTemplateHandlerV2(s.Store)
			r.Get("/api/v2/templates", templateV2.ListTemplates)
			r.Post("/api/v2/templates", templateV2.CreateTemplate)
			r.Get("/api/v2/templates/{id}", templateV2.GetTemplate)
			r.Put("/api/v2/templates/{id}", templateV2.UpdateTemplate)
			r.Delete("/api/v2/templates/{id}", templateV2.DeleteTemplate)
			r.Post("/api/v2/templates/generate", templateV2.GenerateWithAI)
			r.Post("/api/v2/templates/preview", templateV2.PreviewTemplate)
			r.Get("/api/v2/templates/library", templateV2.GetBuiltInTemplates)
			r.Get("/api/v2/templates/library/{templateId}", templateV2.GetBuiltInTemplateContent)
			r.Get("/api/v2/templates/variables", templateV2.GetVariables)

			// Automations V2 - Drag & Drop Workflow Builder
			automationV2 := NewAutomationHandlerV2(s.Store)
			r.Get("/api/v2/automations", automationV2.ListAutomations)
			r.Post("/api/v2/automations", automationV2.CreateAutomation)
			r.Get("/api/v2/automations/{id}", automationV2.GetAutomation)
			r.Put("/api/v2/automations/{id}", automationV2.UpdateAutomation)
			r.Delete("/api/v2/automations/{id}", automationV2.DeleteAutomation)
			r.Post("/api/v2/automations/{id}/activate", automationV2.ActivateAutomation)
			r.Post("/api/v2/automations/{id}/pause", automationV2.PauseAutomation)
			r.Get("/api/v2/automations/{id}/stats", automationV2.GetAutomationStats)
			r.Get("/api/v2/automations/{id}/runs", automationV2.GetAutomationRuns)
			r.Get("/api/v2/automations/node-types", automationV2.GetNodeTypes)

			// Campaigns V2 - Enhanced with personalization
			campaignV2 := NewCampaignHandlerV2(s.Store)
			r.Get("/api/v2/campaigns", campaignV2.ListCampaigns)
			r.Get("/api/v2/campaigns/stats", campaignV2.GetCampaignStats)
			r.Post("/api/v2/campaigns", campaignV2.CreateCampaign)
			r.Get("/api/v2/campaigns/{id}", campaignV2.GetCampaign)
			r.Put("/api/v2/campaigns/{id}", campaignV2.UpdateCampaign)
			r.Delete("/api/v2/campaigns/{id}", campaignV2.DeleteCampaign)
			r.Post("/api/v2/campaigns/{id}/preview", campaignV2.PreviewCampaign)

			// Subscriber Lists
			listHandler := NewListHandler(s.Store)
			r.Get("/api/v2/lists", listHandler.ListLists)
			r.Post("/api/v2/lists", listHandler.CreateList)
			r.Get("/api/v2/lists/{id}", listHandler.GetList)
			r.Put("/api/v2/lists/{id}", listHandler.UpdateList)
			r.Delete("/api/v2/lists/{id}", listHandler.DeleteList)
			r.Get("/api/v2/lists/{id}/subscribers", listHandler.ListSubscribers)
			r.Post("/api/v2/lists/{id}/subscribers", listHandler.AddSubscriber)
			r.Delete("/api/v2/lists/{id}/subscribers/{contactId}", listHandler.RemoveSubscriber)
			r.Post("/api/v2/lists/{id}/subscribers/{contactId}/unsubscribe", listHandler.UnsubscribeFromList)
			r.Post("/api/v2/lists/{id}/import", listHandler.ImportSubscribers)
			r.Get("/api/v2/lists/{id}/export", listHandler.ExportSubscribers)
			r.Post("/api/v2/lists/{id}/copy", listHandler.CopySubscribers)
			r.Get("/api/v2/lists/{id}/stats", listHandler.GetListStats)

			keyV2 := NewAPIKeyHandlerV2(s.Store)
			r.Get("/api/v2/keys", keyV2.ListKeys)
			r.Post("/api/v2/keys", keyV2.CreateKey)
			r.Delete("/api/v2/keys/{id}", keyV2.DeleteKey)

			tagV2 := NewTagHandlerV2(s.Store)
			r.Get("/api/v2/tags", tagV2.List)
			r.Post("/api/v2/tags", tagV2.Create)
			r.Put("/api/v2/tags/{id}", tagV2.Update)
			r.Delete("/api/v2/tags/{id}", tagV2.Delete)

			segmentV2 := NewSegmentHandlerV2(s.Store)
			r.Get("/api/v2/segments", segmentV2.List)
			r.Post("/api/v2/segments", segmentV2.Create)
			r.Delete("/api/v2/segments/{id}", segmentV2.Delete)
			r.Post("/api/v2/segments/{id}/refresh", segmentV2.Refresh)

			suppV2 := NewSuppressionHandlerV2(s.Store)
			r.Get("/api/v2/suppressions", suppV2.List)
			r.Post("/api/v2/suppressions", suppV2.Add)
			r.Delete("/api/v2/suppressions/{id}", suppV2.Delete)

			webhookV2 := NewWebhookHandlerV2(s.Store)
			r.Get("/api/v2/webhooks", webhookV2.List)
			r.Post("/api/v2/webhooks", webhookV2.Create)
			r.Put("/api/v2/webhooks/{id}", webhookV2.Update)
			r.Delete("/api/v2/webhooks/{id}", webhookV2.Delete)
			r.Post("/api/v2/webhooks/test", webhookV2.TestURL)
			r.Post("/api/v2/webhooks/{id}/test", webhookV2.TestByID)

			// Analytics V2 (Tenant Scoped)
			analyticsV2 := NewAnalyticsHandlerV2(s.Store)
			r.Get("/api/v2/analytics/dashboard", analyticsV2.DashboardTenant)
			r.Get("/api/v2/analytics/deliverability", analyticsV2.DeliverabilityTenant)
		})

		// Superadmin Management
		r.Group(func(r chi.Router) {
			// Superadmin routes don't strictly require Org Context (they manage Orgs)
			// Implementation checks IsSuperAdmin inside handlers
			adminOrgHandler := NewOrganizationHandler(s.Store)
			r.Get("/api/v2/admin/organizations", adminOrgHandler.ListOrganizations)
			r.Post("/api/v2/admin/organizations", adminOrgHandler.CreateOrganization)
			r.Delete("/api/v2/admin/organizations/{id}", adminOrgHandler.DeleteOrganization)
			r.Get("/api/v2/admin/organizations/{id}/members", adminOrgHandler.GetOrganizationMembers)

			// User Management (Super Admin only)
			usersHandler := NewUsersHandler(s.Store)
			r.Get("/api/admin/users", usersHandler.ListUsers)
			r.Post("/api/admin/users", usersHandler.CreateUser)
			r.Put("/api/admin/users/{id}", usersHandler.UpdateUser)
			r.Delete("/api/admin/users/{id}", usersHandler.DeleteUser)
			r.Get("/api/admin/users/{id}/organizations", usersHandler.ListUserOrgs)
			r.Post("/api/admin/users/{id}/organizations", usersHandler.AssignUserToOrg)
			r.Delete("/api/admin/users/{id}/organizations/{org_id}", usersHandler.RemoveUserFromOrg)
		})

		// Organizations (Multi-tenancy)
		// Available globally to authenticated users (to select/switch)
		orgHandler := NewOrganizationHandler(s.Store)
		r.Get("/api/v2/organizations/{id}", orgHandler.GetOrganization)
		r.Put("/api/v2/organizations/{id}", orgHandler.UpdateOrganization)
		r.Get("/api/v2/organizations/{id}/usage", orgHandler.GetOrganizationUsage)

		// Per-tenant tracking & unsubscribe domain configuration
		orgDomainHandler := NewOrgDomainHandler(s.Store)
		r.Get("/api/v2/organizations/{id}/domain-config", orgDomainHandler.GetConfig)
		r.Put("/api/v2/organizations/{id}/domain-config", orgDomainHandler.UpdateConfig)
		r.Post("/api/v2/organizations/{id}/domain-config/verify-tracking", orgDomainHandler.VerifyTracking)
		r.Post("/api/v2/organizations/{id}/domain-config/verify-unsubscribe", orgDomainHandler.VerifyUnsubscribe)
	})

	// --- Metrics endpoint (if enabled) ---
	if cfg.MetricsEnabled {
		r.Handle(cfg.MetricsPath, custom.MetricsHandler())
	}

	// --- Health check endpoints (unprotected for K8s probes) ---
	r.Get("/health", s.handleHealthCheck)
	r.Get("/health/live", s.handleLivenessCheck)
	r.Get("/health/ready", s.handleReadinessCheck)

	// --- Internal Policy API (for KumoMTA Lua integration) ---
	// Protected by IP whitelist (localhost only) - no auth tokens
	r.Route("/api/internal/policy", func(r chi.Router) {
		r.Use(internalOnlyMiddleware) // Restrict to localhost
		policy := NewPolicyHandler(s.Store)
		r.Get("/sending-ips", policy.HandleGetSendingIPs)
		r.Get("/domains", policy.HandleGetDomains)
		r.Get("/sender-rate", policy.HandleGetSenderRate)
		r.Post("/log-event", policy.HandleLogEvent)
		r.Get("/suppression-check", policy.HandleSuppressionCheck)
	})

	// --- Static File Server for Frontend ---
	// Serve the React frontend from web/dist directory
	staticDir := "./web/dist"
	fs := http.FileServer(http.Dir(staticDir))

	// Handle all non-API routes as static files with SPA fallback
	r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
		path := req.URL.Path

		// Try to serve the exact file
		fullPath := staticDir + path
		if _, err := http.Dir(staticDir).Open(path); err == nil {
			// Set correct MIME types
			if strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".mjs") {
				w.Header().Set("Content-Type", "application/javascript")
			} else if strings.HasSuffix(path, ".css") {
				w.Header().Set("Content-Type", "text/css")
			}
			fs.ServeHTTP(w, req)
			return
		}

		// For SPA: serve index.html for any unmatched routes
		_ = fullPath // unused when falling through
		http.ServeFile(w, req, staticDir+"/index.html")
	})

	return r
}

// ... (Rest of file same as before) ...

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.WithComponent("api").Error("json encode failed", "error", err)
	}
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	tokenHash := hashSessionToken(token)
	s.Store.DeleteSession(tokenHash)
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}

// handleHealthCheck returns detailed health status
func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	hc := core.GetHealthChecker()
	if hc == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy", "error": "health checker not initialized"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	health := hc.Check(ctx)

	status := http.StatusOK
	if health.Status == core.HealthStatusUnhealthy {
		status = http.StatusServiceUnavailable
	} else if health.Status == core.HealthStatusDegraded {
		status = http.StatusOK // Still return 200 for degraded
	}

	writeJSON(w, status, health)
}

// handleLivenessCheck returns 200 if the process is alive (for K8s liveness probe)
func (s *Server) handleLivenessCheck(w http.ResponseWriter, r *http.Request) {
	hc := core.GetHealthChecker()
	if hc == nil || !hc.Liveness() {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// handleReadinessCheck returns 200 if ready to serve traffic (for K8s readiness probe)
func (s *Server) handleReadinessCheck(w http.ResponseWriter, r *http.Request) {
	hc := core.GetHealthChecker()
	if hc == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if !hc.Readiness(ctx) {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
