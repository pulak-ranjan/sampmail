package api

import (
	"encoding/json"
	"net/http"

	"github.com/pulak-ranjan/sampmail/internal/config"
	"github.com/pulak-ranjan/sampmail/internal/core"
	"github.com/pulak-ranjan/sampmail/internal/logger"
	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

type settingsDTO struct {
	MainHostname string `json:"main_hostname"`
	MainServerIP string `json:"main_server_ip"`
	RelayIPs     string `json:"relay_ips"`
	AIProvider   string `json:"ai_provider"`
	AIAPIKey    string `json:"ai_api_key,omitempty"`
}

// BotConfigDTO for bot settings
type BotConfigDTO struct {
	// Telegram
	TelegramEnabled  bool   `json:"telegram_enabled"`
	TelegramBotToken string `json:"telegram_bot_token,omitempty"`
	TelegramChatID  string `json:"telegram_chat_id"`

	// Discord
	DiscordEnabled   bool   `json:"discord_enabled"`
	DiscordBotToken string `json:"discord_bot_token,omitempty"`
	DiscordAppID    string `json:"discord_app_id"`
	DiscordPublicKey string `json:"discord_public_key"`
	DiscordGuildID  string `json:"discord_guild_id"`
}

// RuntimeConfigDTO for runtime settings that can be changed without restart
type RuntimeConfigDTO struct {
	UseKumoHTTPAPI    bool   `json:"use_kumo_http_api"`
	CampaignWorkers   int    `json:"campaign_workers"`
	SMTPMaxConns     int    `json:"smtp_max_conns"`
	UseReacherOnly   bool   `json:"use_reacher_only"`
	ReacherURL       string `json:"reacher_url"`
}

// GET /api/settings
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	st, err := s.Store.GetSettings()
	if err != nil && err != store.ErrNotFound {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load settings"})
		return
	}

	if st == nil {
		writeJSON(w, http.StatusOK, settingsDTO{})
		return
	}

	writeJSON(w, http.StatusOK, settingsDTO{
		MainHostname: st.MainHostname,
		MainServerIP: st.MainServerIP,
		RelayIPs:     st.MailWizzIP,
		AIProvider:   st.AIProvider,
		// AIAPIKey intentionally omitted - write-only
	})
}

// POST /api/settings
func (s *Server) handleSetSettings(w http.ResponseWriter, r *http.Request) {
	var dto settingsDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	existing, err := s.Store.GetSettings()
	if err != nil && err != store.ErrNotFound {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load settings"})
		return
	}

	if existing == nil {
		existing = &models.AppSettings{}
	}

	existing.MainHostname = dto.MainHostname
	existing.MainServerIP = dto.MainServerIP
	existing.MailWizzIP = dto.RelayIPs
	existing.AIProvider = dto.AIProvider

	if dto.AIAPIKey != "" {
		enc, err := core.Encrypt(dto.AIAPIKey)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to encrypt key"})
			return
		}
		existing.AIAPIKey = enc
	}

	if err := s.Store.UpsertSettings(existing); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save settings"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GET /api/config/runtime - Get runtime configuration
func (s *Server) handleGetRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	// Get current config values
	cfg := config.Get()

	// Try to get from database to see if there are overrides
	settings, err := s.Store.GetSettings()
	if err != nil && err != store.ErrNotFound {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load settings"})
		return
	}

	// Use DB values if set, otherwise fall back to env config
	useHTTP := cfg.UseKumoHTTPAPI
	workers := cfg.CampaignWorkers
	smtpConns := cfg.SMTPMaxConns
	reacherURL := cfg.ReacherURL

	if settings != nil {
		if settings.UseKumoHTTPAPI {
			useHTTP = settings.UseKumoHTTPAPI
		}
		if settings.CampaignWorkers > 0 {
			workers = settings.CampaignWorkers
		}
		if settings.SMTPMaxConns > 0 {
			smtpConns = settings.SMTPMaxConns
		}
		if settings.ReacherURL != "" {
			reacherURL = settings.ReacherURL
		}
	}

	writeJSON(w, http.StatusOK, RuntimeConfigDTO{
		UseKumoHTTPAPI:   useHTTP,
		CampaignWorkers:   workers,
		SMTPMaxConns:     smtpConns,
		UseReacherOnly:    settings != nil && settings.UseReacherOnly,
		ReacherURL:       reacherURL,
	})
}

// POST /api/config/runtime - Update runtime configuration
// Note: These settings are stored in database and override env vars on startup
func (s *Server) handleSetRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	var dto RuntimeConfigDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	// Get existing settings
	existing, err := s.Store.GetSettings()
	if err != nil && err != store.ErrNotFound {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load settings"})
		return
	}

	if existing == nil {
		existing = &models.AppSettings{}
	}

	// Update runtime config fields
	existing.UseKumoHTTPAPI = dto.UseKumoHTTPAPI
	existing.CampaignWorkers = dto.CampaignWorkers
	existing.SMTPMaxConns = dto.SMTPMaxConns
	existing.ReacherURL = dto.ReacherURL
	existing.UseReacherOnly = dto.UseReacherOnly

	if err := s.Store.UpsertSettings(existing); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save config"})
		return
	}

	// Reload config in memory
	config.ReloadConfig()

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "Runtime config updated. Some changes may require restart."})
}

// GET /api/config/bot - Get bot configuration
func (s *Server) handleGetBotConfig(w http.ResponseWriter, r *http.Request) {
	st, err := s.Store.GetSettings()
	if err != nil && err != store.ErrNotFound {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load settings"})
		return
	}

	if st == nil {
		writeJSON(w, http.StatusOK, BotConfigDTO{})
		return
	}

	writeJSON(w, http.StatusOK, BotConfigDTO{
		TelegramEnabled:   st.TelegramEnabled,
		TelegramChatID:   st.TelegramChatID,
		DiscordEnabled:  st.DiscordEnabled,
		DiscordAppID:    st.DiscordAppID,
		DiscordPublicKey: st.DiscordPublicKey,
		DiscordGuildID:  st.DiscordGuildID,
	})
}

// POST /api/config/bot - Save bot configuration
func (s *Server) handleSetBotConfig(w http.ResponseWriter, r *http.Request) {
	var dto BotConfigDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	// Get existing settings
	existing, err := s.Store.GetSettings()
	if err != nil && err != store.ErrNotFound {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load settings"})
		return
	}

	if existing == nil {
		existing = &models.AppSettings{}
	}

	// Update bot config
	existing.TelegramEnabled = dto.TelegramEnabled
	existing.TelegramChatID = dto.TelegramChatID
	existing.DiscordEnabled = dto.DiscordEnabled
	existing.DiscordAppID = dto.DiscordAppID
	existing.DiscordPublicKey = dto.DiscordPublicKey
	existing.DiscordGuildID = dto.DiscordGuildID

	// Token is write-only, store separately
	if dto.TelegramBotToken != "" {
		existing.TelegramBotToken = dto.TelegramBotToken
	}
	if dto.DiscordBotToken != "" {
		existing.DiscordBotToken = dto.DiscordBotToken
	}

	if err := s.Store.UpsertSettings(existing); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save bot config"})
		return
	}

	// Restart bot with new config
	core.RestartBotEngine()

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "Bot config saved. Restarting bots..."})
}

// RestartBotEngine restarts the bot engine with current settings
func RestartBotEngine() {
	// This would restart the bot with new settings
	logger.Info("Bot engine restart requested")
}
