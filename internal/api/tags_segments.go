package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
	"gorm.io/gorm"
)

// =====================================
// TAGS
// =====================================

type TagHandler struct {
	Store *store.Store
}

func NewTagHandler(st *store.Store) *TagHandler {
	return &TagHandler{Store: st}
}

// GET /api/tags
func (h *TagHandler) List(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	var tags []models.Tag
	if err := h.Store.DB.Order("name").Find(&tags).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Add subscriber counts
	type TagWithCount struct {
		models.Tag
		SubscriberCount int `json:"subscriber_count"`
	}

	result := make([]TagWithCount, len(tags))
	for i, tag := range tags {
		var count int64
		h.Store.DB.Model(&models.SubscriberTag{}).Where("tag_id = ?", tag.ID).Count(&count)
		result[i] = TagWithCount{Tag: tag, SubscriberCount: int(count)}
	}

	writeJSON(w, http.StatusOK, result)
}

// POST /api/tags
func (h *TagHandler) Create(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	var tag models.Tag
	if err := json.NewDecoder(r.Body).Decode(&tag); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if tag.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	if tag.Color == "" {
		tag.Color = "#6366f1" // Default indigo
	}

	tag.CreatedAt = time.Now()

	if err := h.Store.DB.Create(&tag).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "tag already exists"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, tag)
}

// PUT /api/tags/:id
func (h *TagHandler) Update(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var tag models.Tag
	if err := h.Store.DB.First(&tag, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "tag not found"})
		return
	}

	var updates models.Tag
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if updates.Name != "" {
		tag.Name = updates.Name
	}
	if updates.Color != "" {
		tag.Color = updates.Color
	}

	if err := h.Store.DB.Save(&tag).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, tag)
}

// DELETE /api/tags/:id
func (h *TagHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	// Delete tag associations first
	h.Store.DB.Where("tag_id = ?", id).Delete(&models.SubscriberTag{})

	if err := h.Store.DB.Delete(&models.Tag{}, id).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// POST /api/subscribers/:id/tags
func (h *TagHandler) AddToSubscriber(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	contactID, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var req struct {
		TagIDs []uint `json:"tag_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	for _, tagID := range req.TagIDs {
		st := models.SubscriberTag{
			ContactID: uint(contactID),
			TagID:     tagID,
			AddedAt:   time.Now(),
		}
		// Ignore duplicate errors
		h.Store.DB.Create(&st)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "tags added"})
}

// DELETE /api/subscribers/:id/tags
func (h *TagHandler) RemoveFromSubscriber(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	contactID, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var req struct {
		TagIDs []uint `json:"tag_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	for _, tagID := range req.TagIDs {
		h.Store.DB.Where("contact_id = ? AND tag_id = ?", contactID, tagID).Delete(&models.SubscriberTag{})
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "tags removed"})
}

// GET /api/subscribers/:id/tags
func (h *TagHandler) GetSubscriberTags(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	contactID, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var subscriberTags []models.SubscriberTag
	h.Store.DB.Where("contact_id = ?", contactID).Find(&subscriberTags)

	var tagIDs []uint
	for _, st := range subscriberTags {
		tagIDs = append(tagIDs, st.TagID)
	}

	var tags []models.Tag
	if len(tagIDs) > 0 {
		h.Store.DB.Where("id IN ?", tagIDs).Find(&tags)
	}

	writeJSON(w, http.StatusOK, tags)
}

// =====================================
// SEGMENTS
// =====================================

type SegmentHandler struct {
	Store *store.Store
}

func NewSegmentHandler(st *store.Store) *SegmentHandler {
	return &SegmentHandler{Store: st}
}

// SegmentCondition represents a single condition in a segment
type SegmentCondition struct {
	Field    string `json:"field"`    // email, first_name, tag, custom_field_key
	Operator string `json:"operator"` // equals, not_equals, contains, not_contains, starts_with, ends_with, is_empty, is_not_empty
	Value    string `json:"value"`
	Combiner string `json:"combiner"` // and, or (for next condition)
}

// GET /api/segments
func (h *SegmentHandler) List(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	var segments []models.Segment
	if err := h.Store.DB.Order("name").Find(&segments).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, segments)
}

// GET /api/segments/:id
func (h *SegmentHandler) Get(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var segment models.Segment
	if err := h.Store.DB.First(&segment, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "segment not found"})
		return
	}

	// Recalculate count if dynamic
	if segment.IsDynamic {
		count := h.calculateSegmentCount(segment)
		segment.CachedCount = count
		segment.LastComputed = time.Now()
		if err := h.Store.DB.Save(&segment).Error; err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update segment count"})
			return
		}
	}

	writeJSON(w, http.StatusOK, segment)
}

// POST /api/segments
func (h *SegmentHandler) Create(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	var segment models.Segment
	if err := json.NewDecoder(r.Body).Decode(&segment); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if segment.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	segment.IsDynamic = true
	segment.CreatedAt = time.Now()
	segment.UpdatedAt = time.Now()

	// Calculate initial count
	segment.CachedCount = h.calculateSegmentCount(segment)
	segment.LastComputed = time.Now()

	if err := h.Store.DB.Create(&segment).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, segment)
}

// PUT /api/segments/:id
func (h *SegmentHandler) Update(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var segment models.Segment
	if err := h.Store.DB.First(&segment, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "segment not found"})
		return
	}

	var updates models.Segment
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	segment.Name = updates.Name
	segment.Description = updates.Description
	segment.Conditions = updates.Conditions
	segment.UpdatedAt = time.Now()

	// Recalculate count
	segment.CachedCount = h.calculateSegmentCount(segment)
	segment.LastComputed = time.Now()

	if err := h.Store.DB.Save(&segment).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, segment)
}

// DELETE /api/segments/:id
func (h *SegmentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	if err := h.Store.DB.Delete(&models.Segment{}, id).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// GET /api/segments/:id/subscribers
func (h *SegmentHandler) GetSubscribers(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var segment models.Segment
	if err := h.Store.DB.First(&segment, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "segment not found"})
		return
	}

	// Pagination
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit := 50
	offset := (page - 1) * limit

	subscribers := h.getSubscribersForSegment(segment, limit, offset)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"subscribers": subscribers,
		"page":        page,
		"limit":       limit,
		"total":       segment.CachedCount,
	})
}

// POST /api/segments/:id/refresh
func (h *SegmentHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var segment models.Segment
	if err := h.Store.DB.First(&segment, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "segment not found"})
		return
	}

	segment.CachedCount = h.calculateSegmentCount(segment)
	segment.LastComputed = time.Now()
	if err := h.Store.DB.Save(&segment).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to refresh segment"})
		return
	}

	writeJSON(w, http.StatusOK, segment)
}

// calculateSegmentCount builds and executes the segment query
func (h *SegmentHandler) calculateSegmentCount(segment models.Segment) int {
	var conditions []SegmentCondition
	if err := json.Unmarshal([]byte(segment.Conditions), &conditions); err != nil {
		return 0
	}

	if len(conditions) == 0 {
		// No conditions = all subscribers
		var count int64
		h.Store.DB.Model(&models.Contact{}).Count(&count)
		return int(count)
	}

	query := h.Store.DB.Model(&models.Contact{})
	query = h.applyConditions(query, conditions)

	var count int64
	query.Count(&count)
	return int(count)
}

// getSubscribersForSegment returns paginated subscribers matching segment
func (h *SegmentHandler) getSubscribersForSegment(segment models.Segment, limit, offset int) []models.Contact {
	var conditions []SegmentCondition
	if err := json.Unmarshal([]byte(segment.Conditions), &conditions); err != nil {
		return nil
	}

	query := h.Store.DB.Model(&models.Contact{})
	query = h.applyConditions(query, conditions)

	var contacts []models.Contact
	query.Limit(limit).Offset(offset).Find(&contacts)
	return contacts
}

// applyConditions builds the WHERE clause from segment conditions
func (h *SegmentHandler) applyConditions(query *gorm.DB, conditions []SegmentCondition) *gorm.DB {
	for i, cond := range conditions {
		var clause string
		var args []interface{}

		switch cond.Field {
		case "email":
			clause, args = h.buildStringCondition("email", cond.Operator, cond.Value)
		case "first_name":
			clause, args = h.buildStringCondition("first_name", cond.Operator, cond.Value)
		case "last_name":
			clause, args = h.buildStringCondition("last_name", cond.Operator, cond.Value)
		case "tag":
			// Tag condition - check if subscriber has tag
			if cond.Operator == "has" || cond.Operator == "equals" {
				clause = "id IN (SELECT contact_id FROM subscriber_tags st JOIN tags t ON st.tag_id = t.id WHERE t.name = ?)"
				args = []interface{}{cond.Value}
			} else if cond.Operator == "not_has" || cond.Operator == "not_equals" {
				clause = "id NOT IN (SELECT contact_id FROM subscriber_tags st JOIN tags t ON st.tag_id = t.id WHERE t.name = ?)"
				args = []interface{}{cond.Value}
			}
		case "list_id":
			clause = "list_id = ?"
			args = []interface{}{cond.Value}
		case "is_valid":
			if cond.Value == "true" {
				clause = "is_valid = ?"
				args = []interface{}{true}
			} else {
				clause = "is_valid = ? OR is_valid IS NULL"
				args = []interface{}{false}
			}
		case "created_at":
			clause, args = h.buildDateCondition("created_at", cond.Operator, cond.Value)
		default:
			// Custom field - check subscriber_field_values
			clause = "id IN (SELECT contact_id FROM subscriber_field_values sfv JOIN custom_fields cf ON sfv.field_id = cf.id WHERE cf.field_key = ? AND sfv.value LIKE ?)"
			args = []interface{}{cond.Field, "%" + cond.Value + "%"}
		}

		if clause == "" {
			continue
		}

		// Apply with AND/OR combiner
		if i == 0 || cond.Combiner == "and" || cond.Combiner == "" {
			query = query.Where(clause, args...)
		} else if cond.Combiner == "or" {
			query = query.Or(clause, args...)
		}
	}

	return query
}

func (h *SegmentHandler) buildStringCondition(field, operator, value string) (string, []interface{}) {
	switch operator {
	case "equals":
		return field + " = ?", []interface{}{value}
	case "not_equals":
		return field + " != ?", []interface{}{value}
	case "contains":
		return field + " LIKE ?", []interface{}{"%" + value + "%"}
	case "not_contains":
		return field + " NOT LIKE ?", []interface{}{"%" + value + "%"}
	case "starts_with":
		return field + " LIKE ?", []interface{}{value + "%"}
	case "ends_with":
		return field + " LIKE ?", []interface{}{"%" + value}
	case "is_empty":
		return "(" + field + " = '' OR " + field + " IS NULL)", nil
	case "is_not_empty":
		return field + " != '' AND " + field + " IS NOT NULL", nil
	}
	return "", nil
}

func (h *SegmentHandler) buildDateCondition(field, operator, value string) (string, []interface{}) {
	switch operator {
	case "before":
		return field + " < ?", []interface{}{value}
	case "after":
		return field + " > ?", []interface{}{value}
	case "on":
		return "DATE(" + field + ") = DATE(?)", []interface{}{value}
	case "within_days":
		days, _ := strconv.Atoi(value)
		return field + " >= datetime('now', '-" + strconv.Itoa(days) + " days')", nil
	}
	return "", nil
}
