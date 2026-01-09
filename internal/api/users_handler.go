package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

// UsersHandler handles user management endpoints
type UsersHandler struct {
	Store *store.Store
}

// NewUsersHandler creates a new UsersHandler
func NewUsersHandler(st *store.Store) *UsersHandler {
	return &UsersHandler{Store: st}
}

// ListUsers returns all admin users
func (h *UsersHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.Store.ListAdmins()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Remove password hashes from response
	type UserResponse struct {
		ID           uint   `json:"id"`
		Email        string `json:"email"`
		IsSuperAdmin bool   `json:"is_super_admin"`
		TwoFAEnabled bool   `json:"two_fa_enabled"`
	}

	var response []UserResponse
	for _, u := range users {
		response = append(response, UserResponse{
			ID:           u.ID,
			Email:        u.Email,
			IsSuperAdmin: u.IsSuperAdmin,
			TwoFAEnabled: u.TwoFactorEnabled,
		})
	}

	writeJSON(w, http.StatusOK, response)
}

// CreateUser creates a new admin user
func (h *UsersHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email        string `json:"email"`
		Password     string `json:"password"`
		IsSuperAdmin bool   `json:"is_super_admin"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Email and password are required"})
		return
	}

	// Check if user already exists
	existing, _ := h.Store.GetAdminByEmail(req.Email)
	if existing != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "User with this email already exists"})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to hash password"})
		return
	}

	user := &models.AdminUser{
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		IsSuperAdmin: req.IsSuperAdmin,
	}

	if err := h.Store.CreateAdmin(user); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":             user.ID,
		"email":          user.Email,
		"is_super_admin": user.IsSuperAdmin,
	})
}

// UpdateUser updates an admin user
func (h *UsersHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid user ID"})
		return
	}

	user, err := h.Store.GetAdminByID(uint(id))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "User not found"})
		return
	}

	var req struct {
		Email        *string `json:"email"`
		Password     *string `json:"password"`
		IsSuperAdmin *bool   `json:"is_super_admin"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	if req.Email != nil {
		user.Email = *req.Email
	}

	if req.Password != nil && *req.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to hash password"})
			return
		}
		user.PasswordHash = string(hashedPassword)
	}

	if req.IsSuperAdmin != nil {
		user.IsSuperAdmin = *req.IsSuperAdmin
	}

	if err := h.Store.UpdateAdmin(user); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":             user.ID,
		"email":          user.Email,
		"is_super_admin": user.IsSuperAdmin,
	})
}

// DeleteUser deletes an admin user
func (h *UsersHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid user ID"})
		return
	}

	// Prevent deleting yourself
	currentUser := r.Context().Value("user").(*models.AdminUser)
	if currentUser.ID == uint(id) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Cannot delete your own account"})
		return
	}

	if err := h.Store.DeleteAdmin(uint(id)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "User deleted successfully"})
}
