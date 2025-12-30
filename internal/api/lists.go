package api

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pulak-ranjan/sampmail/internal/core"
	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

// =====================================
// SUBSCRIBER LIST MODELS
// =====================================

// SubscriberList represents a mailing list
type SubscriberList struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	OrganizationID    uint      `gorm:"index" json:"organization_id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	Type              string    `json:"type"` // static, dynamic
	DoubleOptin       bool      `json:"double_optin"`
	WelcomeEmailID    uint      `json:"welcome_email_id"`
	UnsubscribeEmailID uint     `json:"unsubscribe_email_id"`
	SubscriberCount   int       `json:"subscriber_count"`
	ActiveCount       int       `json:"active_count"`
	UnsubscribedCount int       `json:"unsubscribed_count"`
	BouncedCount      int       `json:"bounced_count"`
	CreatedBy         uint      `json:"created_by"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// ListSubscriber represents a subscriber in a list
type ListSubscriber struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	ListID        uint       `gorm:"index" json:"list_id"`
	ContactID     uint       `gorm:"index" json:"contact_id"`
	Status        string     `json:"status"` // active, unsubscribed, bounced, pending
	Source        string     `json:"source"` // import, api, form, manual
	SubscribedAt  time.Time  `json:"subscribed_at"`
	UnsubscribedAt *time.Time `json:"unsubscribed_at"`
}

// =====================================
// LIST HANDLER
// =====================================

// ListHandler handles subscriber list operations
type ListHandler struct {
	store *store.Store
}

// NewListHandler creates a new list handler
func NewListHandler(st *store.Store) *ListHandler {
	return &ListHandler{store: st}
}

// ListLists returns all subscriber lists
func (h *ListHandler) ListLists(w http.ResponseWriter, r *http.Request) {
	var lists []SubscriberList
	if err := h.store.DB.Order("created_at DESC").Find(&lists).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, lists)
}

// GetList returns a single list with statistics
func (h *ListHandler) GetList(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var list SubscriberList
	if err := h.store.DB.First(&list, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "list not found"})
		return
	}

	// Get fresh counts
	var activeCount, unsubCount, bouncedCount int64
	h.store.DB.Model(&ListSubscriber{}).Where("list_id = ? AND status = ?", id, "active").Count(&activeCount)
	h.store.DB.Model(&ListSubscriber{}).Where("list_id = ? AND status = ?", id, "unsubscribed").Count(&unsubCount)
	h.store.DB.Model(&ListSubscriber{}).Where("list_id = ? AND status = ?", id, "bounced").Count(&bouncedCount)

	list.ActiveCount = int(activeCount)
	list.UnsubscribedCount = int(unsubCount)
	list.BouncedCount = int(bouncedCount)
	list.SubscriberCount = list.ActiveCount + list.UnsubscribedCount + list.BouncedCount

	writeJSON(w, http.StatusOK, list)
}

// CreateList creates a new subscriber list
func (h *ListHandler) CreateList(w http.ResponseWriter, r *http.Request) {
	var list SubscriberList
	if err := json.NewDecoder(r.Body).Decode(&list); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if list.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	list.Type = "static"
	list.CreatedAt = time.Now()
	list.UpdatedAt = time.Now()

	if err := h.store.DB.Create(&list).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, list)
}

// UpdateList updates a subscriber list
func (h *ListHandler) UpdateList(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var list SubscriberList
	if err := h.store.DB.First(&list, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "list not found"})
		return
	}

	var updates SubscriberList
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	updates.UpdatedAt = time.Now()

	if err := h.store.DB.Model(&list).Updates(updates).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, list)
}

// DeleteList deletes a subscriber list
func (h *ListHandler) DeleteList(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	// Delete subscribers first
	h.store.DB.Where("list_id = ?", id).Delete(&ListSubscriber{})

	if err := h.store.DB.Delete(&SubscriberList{}, id).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// =====================================
// SUBSCRIBER OPERATIONS
// =====================================

// ListSubscribers returns subscribers in a list
func (h *ListHandler) ListSubscribers(w http.ResponseWriter, r *http.Request) {
	listID, _ := strconv.Atoi(chi.URLParam(r, "id"))
	status := r.URL.Query().Get("status")
	search := r.URL.Query().Get("search")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}

	// Join with contacts to get full info
	query := h.store.DB.Table("list_subscribers ls").
		Select("ls.*, cv.email, cv.first_name, cv.last_name, cv.company, cv.lead_score").
		Joins("LEFT JOIN contact_v2s cv ON ls.contact_id = cv.id").
		Where("ls.list_id = ?", listID)

	if status != "" {
		query = query.Where("ls.status = ?", status)
	}
	if search != "" {
		query = query.Where("cv.email LIKE ? OR cv.first_name LIKE ? OR cv.last_name LIKE ?",
			"%"+search+"%", "%"+search+"%", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var subscribers []map[string]interface{}
	if err := query.
		Order("ls.subscribed_at DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Scan(&subscribers).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":       subscribers,
		"total":      total,
		"page":       page,
		"limit":      limit,
		"totalPages": (total + int64(limit) - 1) / int64(limit),
	})
}

// AddSubscriber adds a subscriber to a list
func (h *ListHandler) AddSubscriber(w http.ResponseWriter, r *http.Request) {
	listID, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var req struct {
		Email     string                 `json:"email"`
		FirstName string                 `json:"first_name"`
		LastName  string                 `json:"last_name"`
		Fields    map[string]interface{} `json:"fields"`
		Source    string                 `json:"source"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if req.Email == "" || !strings.Contains(req.Email, "@") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid email required"})
		return
	}

	// Find or create contact
	var contact models.ContactV2
	if err := h.store.DB.Where("email = ?", req.Email).First(&contact).Error; err != nil {
		// Create new contact
		contact = models.ContactV2{
			Email:        req.Email,
			FirstName:    req.FirstName,
			LastName:     req.LastName,
			Status:       "active",
			SubscribedAt: time.Now(),
		}
		if req.Fields != nil {
			contact.CustomFields = models.JSONMap(req.Fields)
		}
		h.store.DB.Create(&contact)
	}

	// Check if already in list
	var existing ListSubscriber
	if err := h.store.DB.Where("list_id = ? AND contact_id = ?", listID, contact.ID).First(&existing).Error; err == nil {
		// Already exists, update status if needed
		if existing.Status != "active" {
			existing.Status = "active"
			existing.SubscribedAt = time.Now()
			h.store.DB.Save(&existing)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":     "already_subscribed",
			"subscriber": existing,
		})
		return
	}

	// Add to list
	subscriber := ListSubscriber{
		ListID:       uint(listID),
		ContactID:    contact.ID,
		Status:       "active",
		Source:       req.Source,
		SubscribedAt: time.Now(),
	}
	if subscriber.Source == "" {
		subscriber.Source = "api"
	}

	if err := h.store.DB.Create(&subscriber).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Update list count
	h.store.DB.Model(&SubscriberList{}).Where("id = ?", listID).
		UpdateColumn("subscriber_count", h.store.DB.Raw("subscriber_count + 1"))

	// Trigger automation
	core.TriggerAutomation("trigger_contact_added", contact.ID, map[string]interface{}{
		"list_id": listID,
		"source":  subscriber.Source,
	})

	writeJSON(w, http.StatusCreated, subscriber)
}

// RemoveSubscriber removes a subscriber from a list
func (h *ListHandler) RemoveSubscriber(w http.ResponseWriter, r *http.Request) {
	listID, _ := strconv.Atoi(chi.URLParam(r, "id"))
	contactID, _ := strconv.Atoi(chi.URLParam(r, "contactId"))

	result := h.store.DB.Where("list_id = ? AND contact_id = ?", listID, contactID).
		Delete(&ListSubscriber{})

	if result.RowsAffected == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "subscriber not found"})
		return
	}

	// Update list count
	h.store.DB.Model(&SubscriberList{}).Where("id = ?", listID).
		UpdateColumn("subscriber_count", h.store.DB.Raw("subscriber_count - 1"))

	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// UnsubscribeFromList unsubscribes a contact from a list
func (h *ListHandler) UnsubscribeFromList(w http.ResponseWriter, r *http.Request) {
	listID, _ := strconv.Atoi(chi.URLParam(r, "id"))
	contactID, _ := strconv.Atoi(chi.URLParam(r, "contactId"))

	now := time.Now()
	result := h.store.DB.Model(&ListSubscriber{}).
		Where("list_id = ? AND contact_id = ?", listID, contactID).
		Updates(map[string]interface{}{
			"status":          "unsubscribed",
			"unsubscribed_at": now,
		})

	if result.RowsAffected == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "subscriber not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "unsubscribed"})
}

// ImportSubscribers imports subscribers from CSV
func (h *ListHandler) ImportSubscribers(w http.ResponseWriter, r *http.Request) {
	listID, _ := strconv.Atoi(chi.URLParam(r, "id"))

	// Parse multipart form
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB max
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to parse form"})
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file required"})
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// Read header
	header, err := reader.Read()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid CSV"})
		return
	}

	// Map header to field indices
	fieldMap := make(map[string]int)
	for i, h := range header {
		fieldMap[strings.ToLower(strings.TrimSpace(h))] = i
	}

	emailIdx, ok := fieldMap["email"]
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email column required"})
		return
	}

	var imported, skipped, failed int

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			failed++
			continue
		}

		if len(record) <= emailIdx {
			failed++
			continue
		}

		email := strings.TrimSpace(record[emailIdx])
		if email == "" || !strings.Contains(email, "@") {
			skipped++
			continue
		}

		// Get optional fields
		firstName := ""
		lastName := ""
		company := ""

		if idx, ok := fieldMap["first_name"]; ok && len(record) > idx {
			firstName = strings.TrimSpace(record[idx])
		}
		if idx, ok := fieldMap["firstname"]; ok && len(record) > idx {
			firstName = strings.TrimSpace(record[idx])
		}
		if idx, ok := fieldMap["last_name"]; ok && len(record) > idx {
			lastName = strings.TrimSpace(record[idx])
		}
		if idx, ok := fieldMap["lastname"]; ok && len(record) > idx {
			lastName = strings.TrimSpace(record[idx])
		}
		if idx, ok := fieldMap["company"]; ok && len(record) > idx {
			company = strings.TrimSpace(record[idx])
		}

		// Find or create contact
		var contact models.ContactV2
		if err := h.store.DB.Where("email = ?", email).First(&contact).Error; err != nil {
			contact = models.ContactV2{
				Email:        email,
				FirstName:    firstName,
				LastName:     lastName,
				Company:      company,
				Status:       "active",
				SubscribedAt: time.Now(),
			}
			h.store.DB.Create(&contact)
		}

		// Check if already in list
		var existing ListSubscriber
		if err := h.store.DB.Where("list_id = ? AND contact_id = ?", listID, contact.ID).First(&existing).Error; err == nil {
			skipped++
			continue
		}

		// Add to list
		subscriber := ListSubscriber{
			ListID:       uint(listID),
			ContactID:    contact.ID,
			Status:       "active",
			Source:       "import",
			SubscribedAt: time.Now(),
		}
		if err := h.store.DB.Create(&subscriber).Error; err != nil {
			failed++
			continue
		}

		imported++
	}

	// Update list count
	h.store.DB.Model(&SubscriberList{}).Where("id = ?", listID).
		UpdateColumn("subscriber_count", h.store.DB.Raw(fmt.Sprintf("subscriber_count + %d", imported)))

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"imported": imported,
		"skipped":  skipped,
		"failed":   failed,
		"total":    imported + skipped + failed,
	})
}

// ExportSubscribers exports subscribers as CSV
func (h *ListHandler) ExportSubscribers(w http.ResponseWriter, r *http.Request) {
	listID, _ := strconv.Atoi(chi.URLParam(r, "id"))
	status := r.URL.Query().Get("status")

	query := h.store.DB.Table("list_subscribers ls").
		Select("cv.email, cv.first_name, cv.last_name, cv.company, cv.phone, ls.status, ls.subscribed_at").
		Joins("LEFT JOIN contact_v2s cv ON ls.contact_id = cv.id").
		Where("ls.list_id = ?", listID)

	if status != "" {
		query = query.Where("ls.status = ?", status)
	}

	var subscribers []struct {
		Email        string
		FirstName    string
		LastName     string
		Company      string
		Phone        string
		Status       string
		SubscribedAt time.Time
	}

	if err := query.Find(&subscribers).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=list_%d_export.csv", listID))

	writer := csv.NewWriter(w)
	writer.Write([]string{"email", "first_name", "last_name", "company", "phone", "status", "subscribed_at"})

	for _, s := range subscribers {
		writer.Write([]string{
			s.Email,
			s.FirstName,
			s.LastName,
			s.Company,
			s.Phone,
			s.Status,
			s.SubscribedAt.Format(time.RFC3339),
		})
	}

	writer.Flush()
}

// CopySubscribers copies subscribers between lists
func (h *ListHandler) CopySubscribers(w http.ResponseWriter, r *http.Request) {
	sourceID, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var req struct {
		TargetListID uint   `json:"target_list_id"`
		Status       string `json:"status"` // Optional filter
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	// Get source subscribers
	query := h.store.DB.Model(&ListSubscriber{}).Where("list_id = ?", sourceID)
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}

	var sourceSubscribers []ListSubscriber
	if err := query.Find(&sourceSubscribers).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var copied, skipped int
	for _, s := range sourceSubscribers {
		// Check if already in target
		var existing ListSubscriber
		if err := h.store.DB.Where("list_id = ? AND contact_id = ?", req.TargetListID, s.ContactID).First(&existing).Error; err == nil {
			skipped++
			continue
		}

		newSub := ListSubscriber{
			ListID:       req.TargetListID,
			ContactID:    s.ContactID,
			Status:       "active",
			Source:       "copy",
			SubscribedAt: time.Now(),
		}
		if err := h.store.DB.Create(&newSub).Error; err != nil {
			continue
		}
		copied++
	}

	// Update target list count
	h.store.DB.Model(&SubscriberList{}).Where("id = ?", req.TargetListID).
		UpdateColumn("subscriber_count", h.store.DB.Raw(fmt.Sprintf("subscriber_count + %d", copied)))

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"copied":  copied,
		"skipped": skipped,
	})
}

// GetListStats returns detailed list statistics
func (h *ListHandler) GetListStats(w http.ResponseWriter, r *http.Request) {
	listID, _ := strconv.Atoi(chi.URLParam(r, "id"))

	// Basic counts
	var activeCount, unsubCount, bouncedCount, pendingCount int64
	h.store.DB.Model(&ListSubscriber{}).Where("list_id = ? AND status = ?", listID, "active").Count(&activeCount)
	h.store.DB.Model(&ListSubscriber{}).Where("list_id = ? AND status = ?", listID, "unsubscribed").Count(&unsubCount)
	h.store.DB.Model(&ListSubscriber{}).Where("list_id = ? AND status = ?", listID, "bounced").Count(&bouncedCount)
	h.store.DB.Model(&ListSubscriber{}).Where("list_id = ? AND status = ?", listID, "pending").Count(&pendingCount)

	// Growth over time (last 30 days)
	var growth []struct {
		Date  string
		Count int
	}
	h.store.DB.Raw(`
		SELECT DATE(subscribed_at) as date, COUNT(*) as count
		FROM list_subscribers
		WHERE list_id = ? AND subscribed_at > datetime('now', '-30 days')
		GROUP BY DATE(subscribed_at)
		ORDER BY date
	`, listID).Scan(&growth)

	// Top sources
	var sources []struct {
		Source string
		Count  int
	}
	h.store.DB.Raw(`
		SELECT source, COUNT(*) as count
		FROM list_subscribers
		WHERE list_id = ?
		GROUP BY source
		ORDER BY count DESC
		LIMIT 5
	`, listID).Scan(&sources)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total":        activeCount + unsubCount + bouncedCount + pendingCount,
		"active":       activeCount,
		"unsubscribed": unsubCount,
		"bounced":      bouncedCount,
		"pending":      pendingCount,
		"growth":       growth,
		"sources":      sources,
	})
}
