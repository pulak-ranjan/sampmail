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
)

type TemplateHandler struct {
	Store *store.Store
}

func NewTemplateHandler(st *store.Store) *TemplateHandler {
	return &TemplateHandler{Store: st}
}

// GET /api/templates
func (h *TemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	search := r.URL.Query().Get("search")

	var templates []models.Template
	query := h.Store.DB.Order("created_at DESC")

	if category != "" {
		query = query.Where("category = ?", category)
	}
	if search != "" {
		query = query.Where("name LIKE ? OR subject LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	if err := query.Find(&templates).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, templates)
}

// GET /api/templates/:id
func (h *TemplateHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var template models.Template
	if err := h.Store.DB.First(&template, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "template not found"})
		return
	}

	writeJSON(w, http.StatusOK, template)
}

// POST /api/templates
func (h *TemplateHandler) Create(w http.ResponseWriter, r *http.Request) {
	var template models.Template
	if err := json.NewDecoder(r.Body).Decode(&template); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	// Extract variables from content
	template.Variables = extractVariables(template.HTMLContent)
	template.CreatedAt = time.Now()
	template.UpdatedAt = time.Now()

	if err := h.Store.DB.Create(&template).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, template)
}

// PUT /api/templates/:id
func (h *TemplateHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var existing models.Template
	if err := h.Store.DB.First(&existing, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "template not found"})
		return
	}

	var updates models.Template
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	// Update fields
	existing.Name = updates.Name
	existing.Subject = updates.Subject
	existing.HTMLContent = updates.HTMLContent
	existing.TextContent = updates.TextContent
	existing.Category = updates.Category
	existing.Variables = extractVariables(updates.HTMLContent)
	existing.UpdatedAt = time.Now()

	if err := h.Store.DB.Save(&existing).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

// DELETE /api/templates/:id
func (h *TemplateHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var template models.Template
	if err := h.Store.DB.First(&template, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "template not found"})
		return
	}

	// Don't delete built-in templates
	if template.IsBuiltIn {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "cannot delete built-in template"})
		return
	}

	if err := h.Store.DB.Delete(&template).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// POST /api/templates/:id/clone
func (h *TemplateHandler) Clone(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var original models.Template
	if err := h.Store.DB.First(&original, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "template not found"})
		return
	}

	clone := models.Template{
		Name:        original.Name + " (Copy)",
		Subject:     original.Subject,
		HTMLContent: original.HTMLContent,
		TextContent: original.TextContent,
		Category:    original.Category,
		Variables:   original.Variables,
		IsPublic:    false,
		IsBuiltIn:   false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := h.Store.DB.Create(&clone).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, clone)
}

// GET /api/templates/library
func (h *TemplateHandler) Library(w http.ResponseWriter, r *http.Request) {
	var templates []models.Template
	if err := h.Store.DB.Where("is_public = ? OR is_built_in = ?", true, true).Find(&templates).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, templates)
}

// GET /api/templates/categories
func (h *TemplateHandler) Categories(w http.ResponseWriter, r *http.Request) {
	var categories []string
	h.Store.DB.Model(&models.Template{}).Distinct().Pluck("category", &categories)

	writeJSON(w, http.StatusOK, categories)
}

// POST /api/templates/:id/preview
func (h *TemplateHandler) Preview(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var template models.Template
	if err := h.Store.DB.First(&template, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "template not found"})
		return
	}

	var data map[string]string
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		data = make(map[string]string)
	}

	// Replace variables with provided data or defaults
	html := template.HTMLContent
	for key, value := range data {
		html = strings.ReplaceAll(html, "{{"+key+"}}", value)
	}

	// Replace remaining variables with placeholders
	html = replaceUnusedVariables(html)

	writeJSON(w, http.StatusOK, map[string]string{
		"html":    html,
		"subject": replaceVariablesInSubject(template.Subject, data),
	})
}

// Helper: Extract {{variable}} patterns from content
func extractVariables(content string) string {
	vars := make(map[string]bool)
	start := 0

	for {
		idx := strings.Index(content[start:], "{{")
		if idx == -1 {
			break
		}
		start += idx
		end := strings.Index(content[start:], "}}")
		if end == -1 {
			break
		}

		varName := strings.TrimSpace(content[start+2 : start+end])
		if varName != "" && !strings.Contains(varName, " ") {
			vars[varName] = true
		}
		start += end + 2
	}

	// Convert to JSON array
	varList := make([]string, 0, len(vars))
	for v := range vars {
		varList = append(varList, v)
	}

	jsonBytes, _ := json.Marshal(varList)
	return string(jsonBytes)
}

// Helper: Replace unused variables with styled placeholders
func replaceUnusedVariables(html string) string {
	// Simple approach: replace {{var}} with [var]
	result := html
	start := 0
	for {
		idx := strings.Index(result[start:], "{{")
		if idx == -1 {
			break
		}
		actualIdx := start + idx
		end := strings.Index(result[actualIdx:], "}}")
		if end == -1 {
			break
		}

		varName := result[actualIdx+2 : actualIdx+end]
		placeholder := "[" + varName + "]"
		result = result[:actualIdx] + placeholder + result[actualIdx+end+2:]
		start = actualIdx + len(placeholder)
	}
	return result
}

// Helper: Replace variables in subject
func replaceVariablesInSubject(subject string, data map[string]string) string {
	result := subject
	for key, value := range data {
		result = strings.ReplaceAll(result, "{{"+key+"}}", value)
	}
	return result
}

// SeedBuiltInTemplates creates starter templates if they don't exist
func SeedBuiltInTemplates(st *store.Store) {
	var count int64
	st.DB.Model(&models.Template{}).Where("is_built_in = ?", true).Count(&count)
	if count > 0 {
		return // Already seeded
	}

	templates := []models.Template{
		{
			Name:      "Simple Newsletter",
			Subject:   "{{newsletter_title}}",
			Category:  "newsletter",
			IsBuiltIn: true,
			IsPublic:  true,
			HTMLContent: `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { text-align: center; padding: 20px 0; border-bottom: 2px solid #eee; }
        .content { padding: 30px 0; }
        .footer { text-align: center; padding: 20px 0; border-top: 1px solid #eee; font-size: 12px; color: #666; }
        .button { display: inline-block; padding: 12px 24px; background: #4F46E5; color: white; text-decoration: none; border-radius: 6px; }
    </style>
</head>
<body>
    <div class="header">
        <h1>{{newsletter_title}}</h1>
    </div>
    <div class="content">
        <p>Hi {{first_name}},</p>
        {{content}}
        <p><a href="{{cta_url}}" class="button">{{cta_text}}</a></p>
    </div>
    <div class="footer">
        <p>You received this email because you subscribed to our newsletter.</p>
        <p><a href="{{unsubscribe_url}}">Unsubscribe</a></p>
    </div>
</body>
</html>`,
			TextContent: `{{newsletter_title}}

Hi {{first_name}},

{{content}}

{{cta_text}}: {{cta_url}}

---
Unsubscribe: {{unsubscribe_url}}`,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			Name:      "Welcome Email",
			Subject:   "Welcome to {{company_name}}!",
			Category:  "welcome",
			IsBuiltIn: true,
			IsPublic:  true,
			HTMLContent: `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; text-align: center; padding: 40px 20px; border-radius: 8px 8px 0 0; }
        .content { background: #fff; padding: 30px; border: 1px solid #eee; }
        .button { display: inline-block; padding: 14px 28px; background: #4F46E5; color: white; text-decoration: none; border-radius: 6px; font-weight: 600; }
        .footer { text-align: center; padding: 20px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="header">
        <h1>Welcome, {{first_name}}! 🎉</h1>
    </div>
    <div class="content">
        <p>Thanks for joining {{company_name}}!</p>
        <p>We're thrilled to have you on board. Here's what you can do next:</p>
        <ul>
            <li>Complete your profile</li>
            <li>Explore our features</li>
            <li>Reach out if you need help</li>
        </ul>
        <p style="text-align: center; padding: 20px 0;">
            <a href="{{dashboard_url}}" class="button">Get Started</a>
        </p>
        <p>Best,<br>The {{company_name}} Team</p>
    </div>
    <div class="footer">
        <p><a href="{{unsubscribe_url}}">Unsubscribe</a></p>
    </div>
</body>
</html>`,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			Name:      "Promotional Email",
			Subject:   "{{promo_headline}} - {{discount_percent}}% Off!",
			Category:  "promotional",
			IsBuiltIn: true,
			IsPublic:  true,
			HTMLContent: `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 0; padding: 0; background: #f5f5f5; }
        .container { max-width: 600px; margin: 0 auto; background: white; }
        .hero { background: linear-gradient(135deg, #f093fb 0%, #f5576c 100%); color: white; text-align: center; padding: 50px 20px; }
        .hero h1 { font-size: 36px; margin: 0; }
        .hero .discount { font-size: 72px; font-weight: bold; }
        .content { padding: 30px; text-align: center; }
        .button { display: inline-block; padding: 16px 32px; background: #f5576c; color: white; text-decoration: none; border-radius: 50px; font-weight: 600; font-size: 18px; }
        .footer { padding: 20px; text-align: center; font-size: 12px; color: #666; background: #f5f5f5; }
    </style>
</head>
<body>
    <div class="container">
        <div class="hero">
            <h1>{{promo_headline}}</h1>
            <div class="discount">{{discount_percent}}% OFF</div>
            <p>Use code: <strong>{{promo_code}}</strong></p>
        </div>
        <div class="content">
            <p>Hi {{first_name}},</p>
            <p>{{promo_description}}</p>
            <p>Offer valid until {{expiry_date}}</p>
            <p style="padding: 20px 0;">
                <a href="{{shop_url}}" class="button">Shop Now</a>
            </p>
        </div>
        <div class="footer">
            <p><a href="{{unsubscribe_url}}">Unsubscribe</a></p>
        </div>
    </div>
</body>
</html>`,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			Name:      "Transactional - Order Confirmation",
			Subject:   "Order Confirmed - #{{order_id}}",
			Category:  "transactional",
			IsBuiltIn: true,
			IsPublic:  true,
			HTMLContent: `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { background: #10B981; color: white; padding: 20px; text-align: center; border-radius: 8px 8px 0 0; }
        .content { border: 1px solid #e5e7eb; border-top: none; padding: 20px; }
        .order-info { background: #f9fafb; padding: 15px; border-radius: 6px; margin: 15px 0; }
        .footer { text-align: center; padding: 20px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="header">
        <h1>✓ Order Confirmed</h1>
    </div>
    <div class="content">
        <p>Hi {{first_name}},</p>
        <p>Thank you for your order! We've received it and will process it shortly.</p>
        <div class="order-info">
            <p><strong>Order Number:</strong> #{{order_id}}</p>
            <p><strong>Order Date:</strong> {{order_date}}</p>
            <p><strong>Total:</strong> {{order_total}}</p>
        </div>
        <p>You can track your order status at any time:</p>
        <p><a href="{{tracking_url}}">View Order Status</a></p>
        <p>Questions? Reply to this email or contact our support team.</p>
    </div>
    <div class="footer">
        <p>This is a transactional email for order #{{order_id}}</p>
    </div>
</body>
</html>`,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			Name:      "Minimal Text",
			Subject:   "{{subject}}",
			Category:  "minimal",
			IsBuiltIn: true,
			IsPublic:  true,
			HTMLContent: `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body { font-family: Georgia, serif; line-height: 1.8; color: #333; max-width: 580px; margin: 0 auto; padding: 40px 20px; }
        a { color: #4F46E5; }
        .footer { margin-top: 40px; padding-top: 20px; border-top: 1px solid #eee; font-size: 13px; color: #666; }
    </style>
</head>
<body>
    <p>Hi {{first_name}},</p>
    
    {{content}}
    
    <p>Best,<br>{{sender_name}}</p>
    
    <div class="footer">
        <p><a href="{{unsubscribe_url}}">Unsubscribe</a></p>
    </div>
</body>
</html>`,
			TextContent: `Hi {{first_name}},

{{content}}

Best,
{{sender_name}}

---
Unsubscribe: {{unsubscribe_url}}`,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	for _, t := range templates {
		t.Variables = extractVariables(t.HTMLContent)
		st.DB.Create(&t)
	}
}
