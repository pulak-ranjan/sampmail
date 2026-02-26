package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"

	"github.com/pulak-ranjan/sampmail/internal/config"
	"github.com/pulak-ranjan/sampmail/internal/core"
	"github.com/pulak-ranjan/sampmail/internal/middleware/custom"
	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
	"github.com/pulak-ranjan/sampmail/internal/validation"
)

type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	TOTPCode string `json:"totp_code,omitempty"`
}

type authResponse struct {
	Token       string `json:"token,omitempty"`
	Email       string `json:"email"`
	Requires2FA bool   `json:"requires_2fa,omitempty"`
	TempToken   string `json:"temp_token,omitempty"`
}

type setup2FARequest struct {
	Password string `json:"password"`
}

type setup2FAResponse struct {
	Secret string `json:"secret"`
	URI    string `json:"uri"`
}

type verify2FARequest struct {
	Code string `json:"code"`
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate secure token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// hashSessionToken hashes a session token for secure storage
// We return the raw token to the user but store the hash
func hashSessionToken(token string) string {
	cfg := config.Get()
	h := sha256.New()
	h.Write(cfg.DeriveSessionKey())
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

func validateEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

func validatePassword(password string) bool {
	if len(password) < 8 {
		return false
	}
	hasLetter := false
	hasNumber := false
	for _, c := range password {
		if unicode.IsLetter(c) {
			hasLetter = true
		}
		if unicode.IsDigit(c) {
			hasNumber = true
		}
	}
	return hasLetter && hasNumber
}

func getClientIP(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		parts := strings.Split(ip, ",")
		return strings.TrimSpace(parts[0])
	}
	ip = r.Header.Get("X-Real-IP")
	if ip != "" {
		return ip
	}
	ip = r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

// ----------------------
// 2FA Brute Force Protection (Redis-backed with in-memory fallback — fail-secure)
// ----------------------

const (
	twoFAMaxAttempts  = 5
	twoFALockDuration = 15 * time.Minute
)

// inMemory2FA is the in-process fallback store used when Redis is unavailable.
// It maps adminID → (attempts, lockedUntil).
var inMemory2FA struct {
	sync.Mutex
	attempts map[uint]int
	locked   map[uint]time.Time
}

func init() {
	inMemory2FA.attempts = make(map[uint]int)
	inMemory2FA.locked = make(map[uint]time.Time)
}

func twoFAAttemptsKey(adminID uint) string {
	return fmt.Sprintf("2fa:attempts:%d", adminID)
}

func twoFALockKey(adminID uint) string {
	return fmt.Sprintf("2fa:locked:%d", adminID)
}

// check2FALocked returns true if the admin is locked out from 2FA attempts.
// Falls back to the in-memory store when Redis is unavailable (fail-secure).
func check2FALocked(adminID uint) bool {
	rdb := custom.GetRedisClient()
	if rdb != nil {
		val, err := rdb.Get(context.Background(), twoFALockKey(adminID)).Result()
		return err == nil && val == "1"
	}
	// In-memory fallback: check if still within lock window
	inMemory2FA.Lock()
	defer inMemory2FA.Unlock()
	if until, ok := inMemory2FA.locked[adminID]; ok {
		if time.Now().Before(until) {
			return true
		}
		// Lock expired — clean up
		delete(inMemory2FA.locked, adminID)
		delete(inMemory2FA.attempts, adminID)
	}
	return false
}

// record2FAFailure increments failure count and locks if threshold exceeded.
// Falls back to in-memory when Redis is unavailable.
func record2FAFailure(adminID uint) {
	rdb := custom.GetRedisClient()
	if rdb != nil {
		ctx := context.Background()
		key := twoFAAttemptsKey(adminID)
		count, _ := rdb.Incr(ctx, key).Result()
		rdb.Expire(ctx, key, twoFALockDuration)
		if count >= twoFAMaxAttempts {
			rdb.Set(ctx, twoFALockKey(adminID), "1", twoFALockDuration)
			rdb.Del(ctx, key)
		}
		return
	}
	// In-memory fallback
	inMemory2FA.Lock()
	defer inMemory2FA.Unlock()
	inMemory2FA.attempts[adminID]++
	if inMemory2FA.attempts[adminID] >= twoFAMaxAttempts {
		inMemory2FA.locked[adminID] = time.Now().Add(twoFALockDuration)
		delete(inMemory2FA.attempts, adminID)
	}
}

// clear2FAAttempts clears the brute force counters on successful verification.
func clear2FAAttempts(adminID uint) {
	rdb := custom.GetRedisClient()
	if rdb != nil {
		ctx := context.Background()
		rdb.Del(ctx, twoFAAttemptsKey(adminID))
		rdb.Del(ctx, twoFALockKey(adminID))
		return
	}
	// In-memory fallback
	inMemory2FA.Lock()
	defer inMemory2FA.Unlock()
	delete(inMemory2FA.attempts, adminID)
	delete(inMemory2FA.locked, adminID)
}

// POST /api/auth/register
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	v := validation.New()
	v.Required("email", req.Email).Email("email", req.Email)
	v.Required("password", req.Password).MinLength("password", req.Password, 8)

	if !v.Valid() {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"errors": v.Errors()})
		return
	}

	if !validatePassword(req.Password) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password must have at least 1 letter and 1 number"})
		return
	}

	count, _ := s.Store.AdminCount()
	if count > 0 {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin already exists"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), config.Get().BcryptCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to hash password"})
		return
	}

	admin := &models.AdminUser{
		Email:        req.Email,
		PasswordHash: string(hash),
		Theme:        "dark", // Default theme
	}

	if err := s.Store.CreateAdmin(admin); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create admin"})
		return
	}

	// Generate token and store hash for security
	token, err := generateToken()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate session token"})
		return
	}
	tokenHash := hashSessionToken(token)
	ip := getClientIP(r)
	userAgent := r.UserAgent()

	if err := s.Store.CreateSession(admin.ID, tokenHash, ip, userAgent, 7*24*time.Hour); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create session"})
		return
	}

	writeJSON(w, http.StatusOK, authResponse{Token: token, Email: admin.Email})
}

// POST /api/auth/login
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	v := validation.New()
	v.Required("email", req.Email).Email("email", req.Email)
	v.Required("password", req.Password)

	if !v.Valid() {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"errors": v.Errors()})
		return
	}

	admin, err := s.Store.GetAdminByEmail(req.Email)
	if err != nil {
		if err == store.ErrNotFound {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to find user"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	// Check if 2FA is enabled
	if admin.TwoFactorEnabled && admin.TwoFactorSecret != "" {
		// If no TOTP code provided, return that 2FA is required
		if req.TOTPCode == "" {
			tempToken, err := generateToken()
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate token"})
				return
			}
			tempTokenHash := hashSessionToken(tempToken)
			// Store temp token hash with short expiry (5 minutes)
			s.Store.CreateSession(admin.ID, "2fa:"+tempTokenHash, getClientIP(r), r.UserAgent(), 5*time.Minute)
			writeJSON(w, http.StatusOK, authResponse{
				Email:       admin.Email,
				Requires2FA: true,
				TempToken:   tempToken,
			})
			return
		}

		// Validate TOTP code with brute force protection
		if check2FALocked(admin.ID) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "2FA locked due to too many attempts, try again in 15 minutes"})
			return
		}
		if !core.ValidateTOTP(admin.TwoFactorSecret, req.TOTPCode) {
			record2FAFailure(admin.ID)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid 2FA code"})
			return
		}
		clear2FAAttempts(admin.ID)
	}

	// Create full session with hashed token
	token, err := generateToken()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate session token"})
		return
	}
	tokenHash := hashSessionToken(token)
	ip := getClientIP(r)
	userAgent := r.UserAgent()

	if err := s.Store.CreateSession(admin.ID, tokenHash, ip, userAgent, 7*24*time.Hour); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create session"})
		return
	}

	// Fetch user's organizations
	var memberships []models.OrganizationUser
	// Preload the Organization data
	if err := s.Store.DB.Preload("Organization").Where("admin_id = ?", admin.ID).Find(&memberships).Error; err != nil {
		// Log error but allow login (user might be pending or superadmin with no orgs)
		s.Store.LogError(err)
	}

	orgs := make([]map[string]interface{}, 0)
	for _, m := range memberships {
		// We can't access m.Organization fields directly if it wasn't preloaded correctly or if struct is different
		// But Preload("Organization") should populate it if Organization struct has basic fields
		// We'll trust GORM here or manually fetch if needed.
		// Assuming OrganizationUser has Organization struct field OR we need to fetch manually.
		// "Organization" field is not in OrganizationUser definition in models_v2.go (Wait, let me check)
		// models_v2.go: Users []OrganizationUser `json:"users,omitempty" gorm:"foreignKey:OrganizationID"` in Organization
		// But OrganizationUser has: OrganizationID uint
		// Use manual fetch or join if relation isn't defined on OrganizationUser side.

		// For safety/speed, let's just fetch org details manually for now since models.go might lack the reverse pointer
		var org models.Organization
		if err := s.Store.DB.First(&org, m.OrganizationID).Error; err == nil {
			orgs = append(orgs, map[string]interface{}{
				"id":   org.ID,
				"name": org.Name,
				"slug": org.Slug,
				"role": m.Role,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token":          token,
		"email":          admin.Email,
		"is_super_admin": admin.IsSuperAdmin,
		"organizations":  orgs,
	})
}

// POST /api/auth/verify-2fa
func (s *Server) handleVerify2FA(w http.ResponseWriter, r *http.Request) {
	tempToken := r.Header.Get("X-Temp-Token")
	if tempToken == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing temp token"})
		return
	}

	var req verify2FARequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	// Hash the temp token for lookup (consistent with how it was stored)
	tempTokenHash := hashSessionToken(tempToken)
	
	// Find admin by temp token (stored with "2fa:" prefix in the token field)
	admin, err := s.Store.GetAdminBySessionToken("2fa:" + tempTokenHash)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired temp token"})
		return
	}

	// Validate TOTP with brute force protection
	if check2FALocked(admin.ID) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "2FA locked due to too many attempts, try again in 15 minutes"})
		return
	}
	if !core.ValidateTOTP(admin.TwoFactorSecret, req.Code) {
		record2FAFailure(admin.ID)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid 2FA code"})
		return
	}
	clear2FAAttempts(admin.ID)

	// Delete temp session (use the same hashed token)
	s.Store.DeleteSession("2fa:" + tempTokenHash)

	// Create full session with hashed token
	token, err := generateToken()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate session token"})
		return
	}
	tokenHash := hashSessionToken(token)
	ip := getClientIP(r)
	userAgent := r.UserAgent()

	if err := s.Store.CreateSession(admin.ID, tokenHash, ip, userAgent, 7*24*time.Hour); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create session"})
		return
	}

	writeJSON(w, http.StatusOK, authResponse{Token: token, Email: admin.Email})
}

// GET /api/auth/me
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	if admin == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	// Fetch user's organizations for multi-tenancy support
	var memberships []models.OrganizationUser
	s.Store.DB.Where("admin_id = ?", admin.ID).Find(&memberships)

	orgs := make([]map[string]interface{}, 0)
	for _, m := range memberships {
		var org models.Organization
		if err := s.Store.DB.First(&org, m.OrganizationID).Error; err == nil {
			orgs = append(orgs, map[string]interface{}{
				"id":   org.ID,
				"name": org.Name,
				"slug": org.Slug,
				"role": m.Role,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"email":          admin.Email,
		"theme":          admin.Theme,
		"has_2fa":        admin.TwoFactorEnabled,
		"is_super_admin": admin.IsSuperAdmin,
		"organizations":  orgs,
	})
}

// POST /api/auth/setup-2fa
func (s *Server) handleSetup2FA(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	if admin == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	var req setup2FARequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid password"})
		return
	}

	// Generate new secret
	secret, err := core.GenerateTOTPSecret()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate secret"})
		return
	}

	// Store secret (but don't enable yet)
	admin.TwoFactorSecret = secret
	if err := s.Store.UpdateAdmin(admin); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save secret"})
		return
	}

	uri := core.GenerateTOTPURI(secret, admin.Email)

	writeJSON(w, http.StatusOK, setup2FAResponse{
		Secret: secret,
		URI:    uri,
	})
}

// POST /api/auth/enable-2fa
func (s *Server) handleEnable2FA(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	if admin == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	var req verify2FARequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if admin.TwoFactorSecret == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "2FA not set up"})
		return
	}

	// Validate code
	if !core.ValidateTOTP(admin.TwoFactorSecret, req.Code) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid code"})
		return
	}

	// Enable 2FA
	admin.TwoFactorEnabled = true
	if err := s.Store.UpdateAdmin(admin); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to enable 2FA"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "2fa enabled"})
}

// POST /api/auth/disable-2fa
func (s *Server) handleDisable2FA(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	if admin == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	var req struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid password"})
		return
	}

	// Verify TOTP
	if admin.TwoFactorEnabled && !core.ValidateTOTP(admin.TwoFactorSecret, req.Code) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid 2FA code"})
		return
	}

	// Disable 2FA
	admin.TwoFactorEnabled = false
	admin.TwoFactorSecret = ""
	if err := s.Store.UpdateAdmin(admin); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to disable 2FA"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "2fa disabled"})
}

// POST /api/auth/theme
func (s *Server) handleSetTheme(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	if admin == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	var req struct {
		Theme string `json:"theme"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if req.Theme != "dark" && req.Theme != "light" && req.Theme != "system" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid theme"})
		return
	}

	admin.Theme = req.Theme
	if err := s.Store.UpdateAdmin(admin); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save theme"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "theme": req.Theme})
}

// GET /api/auth/sessions
func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	if admin == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	sessions, err := s.Store.ListSessionsByAdmin(admin.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list sessions"})
		return
	}

	type sessionDTO struct {
		ID        uint      `json:"id"`
		DeviceIP  string    `json:"device_ip"`
		UserAgent string    `json:"user_agent"`
		CreatedAt time.Time `json:"created_at"`
		ExpiresAt time.Time `json:"expires_at"`
	}

	out := make([]sessionDTO, 0, len(sessions))
	for _, sess := range sessions {
		// Don't expose temp 2FA sessions
		if strings.HasPrefix(sess.Token, "2fa:") {
			continue
		}
		out = append(out, sessionDTO{
			ID:        sess.ID,
			DeviceIP:  sess.DeviceIP,
			UserAgent: sess.UserAgent,
			CreatedAt: sess.CreatedAt,
			ExpiresAt: sess.ExpiresAt,
		})
	}

	writeJSON(w, http.StatusOK, out)
}
