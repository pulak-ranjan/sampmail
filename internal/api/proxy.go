package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pulak-ranjan/sampmail/internal/models"
)

// =====================================
// PROXY MANAGEMENT API
// =====================================

// GET /api/proxies
func (s *Server) handleListProxies(w http.ResponseWriter, r *http.Request) {
	var proxies []models.Proxy
	if err := s.Store.DB.Order("priority ASC").Find(&proxies).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, proxies)
}

// POST /api/proxies
func (s *Server) handleCreateProxy(w http.ResponseWriter, r *http.Request) {
	var proxy models.Proxy
	if err := json.NewDecoder(r.Body).Decode(&proxy); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Validate required fields
	if proxy.Host == "" || proxy.Port == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "host and port are required"})
		return
	}

	if proxy.Type == "" {
		proxy.Type = "socks5" // Default
	}

	if proxy.UseFor == "" {
		proxy.UseFor = "both"
	}

	proxy.IsActive = true
	proxy.IsHealthy = true
	proxy.CreatedAt = time.Now()
	proxy.UpdatedAt = time.Now()

	if err := s.Store.DB.Create(&proxy).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, proxy)
}

// GET /api/proxies/{id}
func (s *Server) handleGetProxy(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	
	var proxy models.Proxy
	if err := s.Store.DB.First(&proxy, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "proxy not found"})
		return
	}

	writeJSON(w, http.StatusOK, proxy)
}

// PUT /api/proxies/{id}
func (s *Server) handleUpdateProxy(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var proxy models.Proxy
	if err := s.Store.DB.First(&proxy, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "proxy not found"})
		return
	}

	var update models.Proxy
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Update fields
	if update.Name != "" {
		proxy.Name = update.Name
	}
	if update.Type != "" {
		proxy.Type = update.Type
	}
	if update.Host != "" {
		proxy.Host = update.Host
	}
	if update.Port != 0 {
		proxy.Port = update.Port
	}
	if update.Username != "" {
		proxy.Username = update.Username
	}
	if update.Password != "" {
		proxy.Password = update.Password
	}
	if update.UseFor != "" {
		proxy.UseFor = update.UseFor
	}
	proxy.Priority = update.Priority
	proxy.IsActive = update.IsActive
	proxy.UpdatedAt = time.Now()

	if err := s.Store.DB.Save(&proxy).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, proxy)
}

// DELETE /api/proxies/{id}
func (s *Server) handleDeleteProxy(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	if err := s.Store.DB.Delete(&models.Proxy{}, id).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "proxy deleted"})
}

// POST /api/proxies/{id}/test
func (s *Server) handleTestProxy(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var proxy models.Proxy
	if err := s.Store.DB.First(&proxy, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "proxy not found"})
		return
	}

	// Test proxy connectivity with a real TCP dial.
	// This verifies that the host:port is reachable before marking it healthy.
	addr := fmt.Sprintf("%s:%d", proxy.Host, proxy.Port)
	start := time.Now()
	conn, dialErr := net.DialTimeout("tcp", addr, 5*time.Second)
	latency := time.Since(start)

	proxy.LastCheck = time.Now()
	if dialErr != nil {
		proxy.IsHealthy = false
		if err := s.Store.DB.Save(&proxy).Error; err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update proxy status"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"message": "proxy unreachable: " + dialErr.Error(),
			"latency": latency.String(),
		})
		return
	}
	conn.Close()

	proxy.IsHealthy = true
	if err := s.Store.DB.Save(&proxy).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update proxy status"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "proxy is healthy",
		"latency": latency.String(),
	})
}

// =====================================
// SENDING IP MANAGEMENT API
// =====================================

// GET /api/sending-ips
func (s *Server) handleListSendingIPs(w http.ResponseWriter, r *http.Request) {
	var ips []models.SendingIP
	if err := s.Store.DB.Order("id ASC").Find(&ips).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, ips)
}

// POST /api/sending-ips
func (s *Server) handleCreateSendingIP(w http.ResponseWriter, r *http.Request) {
	var ip models.SendingIP
	if err := json.NewDecoder(r.Body).Decode(&ip); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if ip.IPAddress == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ip_address is required"})
		return
	}

	ip.IsActive = true
	ip.WarmupStage = 0
	ip.DailySendLimit = 100 // Start with low limit
	ip.ReputationScore = 50.0
	ip.CreatedAt = time.Now()
	ip.UpdatedAt = time.Now()

	if err := s.Store.DB.Create(&ip).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, ip)
}

// PUT /api/sending-ips/{id}
func (s *Server) handleUpdateSendingIP(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var ip models.SendingIP
	if err := s.Store.DB.First(&ip, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "IP not found"})
		return
	}

	var update models.SendingIP
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if update.Hostname != "" {
		ip.Hostname = update.Hostname
	}
	ip.IsActive = update.IsActive
	if update.DailySendLimit > 0 {
		ip.DailySendLimit = update.DailySendLimit
	}
	if update.DomainID > 0 {
		ip.DomainID = update.DomainID
	}
	ip.UpdatedAt = time.Now()

	if err := s.Store.DB.Save(&ip).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, ip)
}

// DELETE /api/sending-ips/{id}
func (s *Server) handleDeleteSendingIP(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	if err := s.Store.DB.Delete(&models.SendingIP{}, id).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "IP deleted"})
}
