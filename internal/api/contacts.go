package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pulak-ranjan/sampmail/internal/core"
	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

type ContactHandler struct {
	Store *store.Store
}

func NewContactHandler(st *store.Store) *ContactHandler {
	return &ContactHandler{Store: st}
}

// buildVerifierOpts creates VerifierOptions from settings
func (h *ContactHandler) buildVerifierOpts() core.VerifierOptions {
	opts := core.VerifierOptions{
		HeloHost: "kumomta.local",
	}

	if s, err := h.Store.GetSettings(); err == nil {
		if s.MainHostname != "" {
			opts.HeloHost = s.MainHostname
		}
		if s.ProxyURL != "" {
			opts.ProxyURLs = append(opts.ProxyURLs, s.ProxyURL)
		}
		opts.ReacherURL = s.ReacherURL
		opts.ReacherAPIKey = s.ReacherAPIKey
		opts.ReacherBinPath = s.ReacherBinPath
	}

	// Fetch Source IPs
	if ips, err := h.Store.ListSystemIPs(); err == nil {
		for _, ip := range ips {
			opts.SourceIPs = append(opts.SourceIPs, ip.Value)
		}
	}

	// Fetch Active Proxies form DB
	var proxies []models.Proxy
	if err := h.Store.DB.Where("is_active = ?", true).Order("priority ASC").Find(&proxies).Error; err == nil {
		for _, p := range proxies {
			protocol := p.Type
			if protocol == "" {
				protocol = "socks5"
			}

			auth := ""
			if p.Username != "" {
				auth = url.QueryEscape(p.Username) + ":" + url.QueryEscape(p.Password) + "@"
			}

			// Construct URL: scheme://user:pass@host:port
			pURL := fmt.Sprintf("%s://%s%s:%d", protocol, auth, p.Host, p.Port)
			opts.ProxyURLs = append(opts.ProxyURLs, pURL)
		}
	}

	return opts
}

// POST /api/contacts/verify
func (h *ContactHandler) HandleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email          string `json:"email"`
		UseReacherOnly bool   `json:"use_reacher_only,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	opts := h.buildVerifierOpts()
	opts.UseReacherOnly = req.UseReacherOnly

	result := core.VerifyEmail(req.Email, opts)
	writeJSON(w, http.StatusOK, result)
}

// POST /api/contacts/verify-batch
func (h *ContactHandler) HandleVerifyBatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Emails         []string `json:"emails"`
		UseReacherOnly bool     `json:"use_reacher_only,omitempty"`
		Concurrency    int      `json:"concurrency,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if len(req.Emails) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no emails provided"})
		return
	}

	if len(req.Emails) > 100 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "max 100 emails per batch"})
		return
	}

	opts := h.buildVerifierOpts()
	opts.UseReacherOnly = req.UseReacherOnly

	concurrency := req.Concurrency
	if concurrency <= 0 || concurrency > 10 {
		concurrency = 5
	}

	results := core.VerifyEmailBatch(req.Emails, opts, concurrency)

	// Summary stats
	summary := map[string]int{
		"safe":    0,
		"risky":   0,
		"invalid": 0,
		"unknown": 0,
	}
	for _, r := range results {
		summary[r.IsReachable]++
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"results": results,
		"summary": summary,
	})
}

// POST /api/lists/{id}/clean
func (h *ContactHandler) HandleCleanList(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)

	// Fetch contacts
	var contacts []models.Contact
	if err := h.Store.DB.Where("list_id = ?", id).Find(&contacts).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}

	opts := h.buildVerifierOpts()

	// Run cleaning in background
	go func() {
		for _, c := range contacts {
			res := core.VerifyEmail(c.Email, opts)

			c.IsValid = (res.IsReachable == "safe")
			c.RiskScore = res.RiskScore
			c.VerifyLog = res.Log

			h.Store.DB.Save(&c)

			// Throttle to avoid overwhelming Reacher/SMTP servers
			time.Sleep(200 * time.Millisecond)
		}
	}()

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "cleaning_started",
		"count":  strconv.Itoa(len(contacts)),
	})
}
