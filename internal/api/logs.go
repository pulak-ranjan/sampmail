package api

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"strconv"

	"github.com/pulak-ranjan/sampmail/internal/core"
	"github.com/pulak-ranjan/sampmail/internal/models"
)

// GET /api/logs/kumomta?lines=100
func (s *Server) handleLogsKumo(w http.ResponseWriter, r *http.Request) {
	s.handleLogsForService(w, r, "kumomta")
}

// GET /api/logs/dovecot?lines=100
func (s *Server) handleLogsDovecot(w http.ResponseWriter, r *http.Request) {
	s.handleLogsForService(w, r, "dovecot")
}

// GET /api/logs/fail2ban?lines=100
func (s *Server) handleLogsFail2ban(w http.ResponseWriter, r *http.Request) {
	s.handleLogsForService(w, r, "fail2ban")
}

// POST /api/bounces/analyze
// Triggers AI analysis of bounces using Ollama
func (s *Server) handleAIBounceAnalysis(w http.ResponseWriter, r *http.Request) {
	// Check admin privileges
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	// Parse request body
	var req struct {
		OllamaURL string `json:"ollama_url"`
		Model     string `json:"model"`
		BatchSize int    `json:"batch_size"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Use defaults
	if req.OllamaURL == "" {
		req.OllamaURL = "http://localhost:11434"
	}
	if req.Model == "" {
		req.Model = "qwen2.5:0.5b"
	}
	if req.BatchSize <= 0 {
		req.BatchSize = 10
	}

	// Create bounce processor
	bp := core.NewBounceProcessor(s.Store)

	// Run analysis
	analyzed, err := bp.AnalyzeUnanalyzedBatches(req.OllamaURL, req.Model, req.BatchSize)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "analysis failed",
			"details": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"analyzed": analyzed,
		"message":  "AI bounce analysis completed",
	})
}

// GET /api/bounces
// Lists bounces with AI analysis
func (s *Server) handleListBounces(w http.ResponseWriter, r *http.Request) {
	// Check admin privileges
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	// Parse query params
	limit := 50
	offset := 0
	aiOnly := r.URL.Query().Get("ai_only") == "true"

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Get bounces from store
	var bounces []models.BounceEvent
	query := s.Store.DB.Model(&models.BounceEvent{})

	if aiOnly {
		query = query.Where("ai_analyzed_at IS NOT NULL")
	}

	// Get total count
	var total int64
	query.Count(&total)

	// Get paginated results
	query = query.Order("processed_at DESC").Limit(limit).Offset(offset)
	if err := query.Find(&bounces).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"bounces": bounces,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// GET /api/bounces/stats
// Returns bounce statistics with AI analysis summary
func (s *Server) handleBounceStats(w http.ResponseWriter, r *http.Request) {
	// Check admin privileges
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	var stats struct {
		Total             int64 `json:"total"`
		HardBounces       int64 `json:"hard_bounces"`
		SoftBounces       int64 `json:"soft_bounces"`
		Complaints        int64 `json:"complaints"`
		AIAnalyzed        int64 `json:"ai_analyzed"`
		AIAnalyzedValid   int64 `json:"ai_valid"`
		AIAnalyzedRisky   int64 `json:"ai_risky"`
		AIAnalyzedInvalid int64 `json:"ai_invalid"`
	}

	s.Store.DB.Model(&models.BounceEvent{}).Count(&stats.Total)
	s.Store.DB.Model(&models.BounceEvent{}).Where("bounce_type = ?", "hard").Count(&stats.HardBounces)
	s.Store.DB.Model(&models.BounceEvent{}).Where("bounce_type = ?", "soft").Count(&stats.SoftBounces)
	s.Store.DB.Model(&models.BounceEvent{}).Where("bounce_type = ?", "complaint").Count(&stats.Complaints)
	s.Store.DB.Model(&models.BounceEvent{}).Where("ai_analyzed_at IS NOT NULL").Count(&stats.AIAnalyzed)
	s.Store.DB.Model(&models.BounceEvent{}).Where("ai_email_quality = ?", "valid").Count(&stats.AIAnalyzedValid)
	s.Store.DB.Model(&models.BounceEvent{}).Where("ai_email_quality = ?", "risky").Count(&stats.AIAnalyzedRisky)
	s.Store.DB.Model(&models.BounceEvent{}).Where("ai_email_quality = ?", "invalid").Count(&stats.AIAnalyzedInvalid)

	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleLogsForService(w http.ResponseWriter, r *http.Request, service string) {
	linesStr := r.URL.Query().Get("lines")
	if linesStr == "" {
		linesStr = "100"
	}
	n, err := strconv.Atoi(linesStr)
	if err != nil || n <= 0 {
		n = 100
	}

	cmd := exec.Command("journalctl", "-u", service, "-n", strconv.Itoa(n), "--no-pager")
	out, err := cmd.CombinedOutput()
	if err != nil {
		s.Store.LogError(err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "failed to read logs",
			"info":  string(out),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"service": service,
		"logs":    string(out),
	})
}
