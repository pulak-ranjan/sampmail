package api

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pulak-ranjan/sampmail/internal/core"
	"github.com/pulak-ranjan/sampmail/internal/models"
)

// ImportJob tracks async import progress
type ImportJob struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"` // pending, processing, completed, failed
	Progress  int       `json:"progress"`
	Total     int       `json:"total"`
	CreatedAt time.Time `json:"created_at"`
	Error     string    `json:"error,omitempty"`
	Stats     *ImportStats `json:"stats,omitempty"`
}

type ImportStats struct {
	DomainsCreated int      `json:"domains_created"`
	SendersCreated int      `json:"senders_created"`
	IPsAdded       int      `json:"ips_added"`
	BouncesCreated int      `json:"bounces_created"`
	DKIMGenerated  int      `json:"dkim_generated"`
	Errors         []string `json:"errors"`
}

// In-memory job tracker (use Redis in production for multi-instance)
var importJobs = make(map[string]*ImportJob)

// POST /api/import/csv - Async version
// Returns 202 Accepted with job ID for tracking
func (s *Server) handleCSVImportAsync(w http.ResponseWriter, r *http.Request) {
	// Max 50MB for async processing
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file too large (max 50MB)"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file required"})
		return
	}
	defer file.Close()

	// Create job ID
	jobID := uuid.New().String()

	// Save file to temp location for async processing
	tempDir := filepath.Join(os.TempDir(), "sampmail-imports")
	os.MkdirAll(tempDir, 0755)
	tempFile := filepath.Join(tempDir, jobID+".csv")

	dst, err := os.Create(tempFile)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save file"})
		return
	}
	
	// Copy with size limit
	written, err := io.CopyN(dst, file, 50<<20) // 50MB limit
	dst.Close()
	if err != nil && err != io.EOF {
		os.Remove(tempFile)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save file"})
		return
	}

	// Create job tracker
	job := &ImportJob{
		ID:        jobID,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	importJobs[jobID] = job

	// Process async
	go s.processCSVImportAsync(jobID, tempFile)

	// Return 202 Accepted with job ID
	w.Header().Set("Location", "/api/import/status/"+jobID)
	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"job_id":  jobID,
		"status":  "pending",
		"message": "Import job created. Check status at /api/import/status/" + jobID,
		"file":    header.Filename,
		"size":    written,
	})
}

// GET /api/import/status/{jobId} - Check import job status
func (s *Server) handleImportStatus(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("id")
	if jobID == "" {
		// Try path param
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) > 0 {
			jobID = parts[len(parts)-1]
		}
	}

	job, exists := importJobs[jobID]
	if !exists {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}

	writeJSON(w, http.StatusOK, job)
}

// processCSVImportAsync handles the actual import in background
func (s *Server) processCSVImportAsync(jobID, filePath string) {
	job := importJobs[jobID]
	job.Status = "processing"

	defer func() {
		// Cleanup temp file
		os.Remove(filePath)
		
		// Recover from panics
		if r := recover(); r != nil {
			job.Status = "failed"
			job.Error = "internal error during import"
		}
	}()

	file, err := os.Open(filePath)
	if err != nil {
		job.Status = "failed"
		job.Error = "failed to read file"
		return
	}
	defer file.Close()

	// FIXED: Count lines by streaming instead of ReadAll to prevent memory exhaustion
	// A 50MB CSV with ReadAll can use 200MB+ RAM due to slice overhead
	lineCount := 0
	scanner := bufio.NewScanner(file)
	// Set larger buffer for long lines (1MB max line length)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		lineCount++
	}
	if err := scanner.Err(); err != nil {
		job.Status = "failed"
		job.Error = "failed to scan file"
		return
	}
	job.Total = lineCount - 1 // Subtract header row

	// Reset file to beginning
	if _, err := file.Seek(0, 0); err != nil {
		job.Status = "failed"
		job.Error = "failed to reset file"
		return
	}

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1

	// Read header
	header, err := reader.Read()
	if err != nil {
		job.Status = "failed"
		job.Error = "invalid CSV format"
		return
	}

	// Process using the sync logic but with progress tracking
	stats := &ImportStats{
		Errors: []string{},
	}
	job.Stats = stats

	// Helper to clean BOM and whitespace
	cleanHeader := func(h string) string {
		h = strings.TrimPrefix(h, "\ufeff")
		return strings.ToLower(strings.TrimSpace(h))
	}

	// Map column indices
	colMap := make(map[string]int)
	for i, col := range header {
		colMap[cleanHeader(col)] = i
	}

	domainIdx, hasDomain := colMap["domain"]
	if !hasDomain {
		job.Status = "failed"
		job.Error = "missing 'domain' column"
		return
	}

	senderIdx := colMap["sender"]
	localPartIdx := colMap["localpart"]
	ipIdx := colMap["ip"]
	passIdx := colMap["password"]
	bounceIdx := colMap["bounce"]
	bouncePassIdx := colMap["bounce_password"]

	domainCache := make(map[string]*models.Domain)
	processedSenders := make(map[string]bool)

	lineNum := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("line %d: invalid format", lineNum+2))
			lineNum++
			continue
		}
		
		lineNum++
		job.Progress = lineNum

		if domainIdx >= len(record) {
			continue
		}
		domainName := strings.TrimSpace(record[domainIdx])
		if domainName == "" {
			continue
		}

		// FIXED: Validate domain format BEFORE database insertion
		if !isValidDomainName(domainName) {
			stats.Errors = append(stats.Errors, fmt.Sprintf("line %d: invalid domain format: %s", lineNum+1, domainName))
			continue
		}

		// Get or create domain
		domain, exists := domainCache[domainName]
		if !exists {
			existing, err := s.Store.GetDomainByName(domainName)
			if err != nil {
				domain = &models.Domain{
					Name:            domainName,
					MailHost:        "mail." + domainName,
					BounceHost:      "bounce." + domainName,
					DMARCPolicy:     "none",
					DMARCPercentage: 100,
				}
				if err := s.Store.CreateDomain(domain); err != nil {
					stats.Errors = append(stats.Errors, "failed to create domain: "+domainName)
					continue
				}
				stats.DomainsCreated++
			} else {
				domain = existing
			}
			domainCache[domainName] = domain
		}

		// Get sender/localpart
		localPart := ""
		if localPartIdx > 0 && localPartIdx < len(record) {
			localPart = strings.TrimSpace(record[localPartIdx])
		}
		if localPart == "" && senderIdx > 0 && senderIdx < len(record) {
			senderEmail := strings.TrimSpace(record[senderIdx])
			if parts := strings.Split(senderEmail, "@"); len(parts) == 2 {
				localPart = parts[0]
			} else {
				localPart = senderEmail
			}
		}

		if localPart == "" {
			continue
		}

		// FIXED: Validate localpart format BEFORE database insertion
		if !isValidLocalPart(localPart) {
			stats.Errors = append(stats.Errors, fmt.Sprintf("line %d: invalid localpart format: %s", lineNum+1, localPart))
			continue
		}

		// Get IP
		ip := ""
		if ipIdx > 0 && ipIdx < len(record) {
			ip = strings.TrimSpace(record[ipIdx])
			// Validate IP format
			if ip != "" && !isValidIP(ip) {
				stats.Errors = append(stats.Errors, fmt.Sprintf("line %d: invalid IP format: %s", lineNum+1, ip))
				ip = "" // Clear invalid IP but continue processing
			}
		}

		// Get password
		password := ""
		if passIdx > 0 && passIdx < len(record) {
			rawPassword := strings.TrimSpace(record[passIdx])
			if rawPassword != "" {
				encrypted, err := core.EncryptPassword(rawPassword)
				if err == nil {
					password = encrypted
				}
			}
		}

		// Get bounce info
		customBounce := ""
		if bounceIdx > 0 && bounceIdx < len(record) {
			customBounce = strings.TrimSpace(record[bounceIdx])
			// Validate bounce username
			if customBounce != "" && !isValidLocalPart(customBounce) {
				stats.Errors = append(stats.Errors, fmt.Sprintf("line %d: invalid bounce username: %s", lineNum+1, customBounce))
				customBounce = ""
			}
		}
		bouncePass := ""
		if bouncePassIdx > 0 && bouncePassIdx < len(record) {
			bouncePass = strings.TrimSpace(record[bouncePassIdx])
		}

		// Check if already processed
		senderKey := domainName + ":" + localPart
		if processedSenders[senderKey] {
			continue
		}
		processedSenders[senderKey] = true

		// Create sender
		sender := &models.Sender{
			DomainID:     domain.ID,
			LocalPart:    localPart,
			Email:        localPart + "@" + domainName,
			IP:           ip,
			SMTPPassword: password,
		}
		if err := s.Store.CreateSender(sender); err != nil {
			stats.Errors = append(stats.Errors, "failed to create sender: "+sender.Email)
			continue
		}
		stats.SendersCreated++

		// Add IP
		if ip != "" {
			ipModel := &models.SystemIP{Value: ip, CreatedAt: time.Now()}
			if err := s.Store.CreateSystemIP(ipModel); err == nil {
				stats.IPsAdded++
			}
		}

		// Generate DKIM
		if err := core.GenerateDKIMKey(domainName, localPart); err == nil {
			stats.DKIMGenerated++
		}

		// Create bounce account
		bounceUser := "b-" + localPart
		if customBounce != "" {
			bounceUser = customBounce
		}
		if err := core.CreateBounceAccountWithPassword(bounceUser, domainName, bouncePass, "Imported via CSV", s.Store); err == nil {
			stats.BouncesCreated++
		}
	}

	job.Status = "completed"
}

// ===========================================
// INPUT VALIDATION FUNCTIONS
// Prevent DB pollution with malicious data
// ===========================================

// isValidDomainName validates domain format to prevent path traversal and XSS
func isValidDomainName(domain string) bool {
	if len(domain) == 0 || len(domain) > 253 {
		return false
	}
	
	// Must not contain path traversal sequences
	if strings.Contains(domain, "..") || strings.Contains(domain, "/") || strings.Contains(domain, "\\") {
		return false
	}
	
	// Must not contain null bytes or control characters
	for _, r := range domain {
		if r < 32 || r == 127 {
			return false
		}
	}
	
	// Must not contain HTML/script tags
	if strings.Contains(domain, "<") || strings.Contains(domain, ">") {
		return false
	}
	
	// Basic domain format: alphanumeric, hyphens, dots
	// Each label 1-63 chars, starts/ends with alphanumeric
	domainRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)
	return domainRegex.MatchString(domain)
}

// isValidLocalPart validates email local part to prevent path traversal
func isValidLocalPart(localPart string) bool {
	if len(localPart) == 0 || len(localPart) > 64 {
		return false
	}
	
	// Must not contain path traversal sequences
	if strings.Contains(localPart, "..") || strings.Contains(localPart, "/") || strings.Contains(localPart, "\\") {
		return false
	}
	
	// Must not contain null bytes or control characters
	for _, r := range localPart {
		if r < 32 || r == 127 {
			return false
		}
	}
	
	// Must not contain HTML/script tags
	if strings.Contains(localPart, "<") || strings.Contains(localPart, ">") {
		return false
	}
	
	// Allowed: alphanumeric, dots, hyphens, underscores, plus
	// Must start with alphanumeric
	localPartRegex := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._+-]*$`)
	return localPartRegex.MatchString(localPart)
}

// isValidIP validates IPv4 or IPv6 address format
func isValidIP(ip string) bool {
	// Use net.ParseIP for proper validation
	return net.ParseIP(ip) != nil
}

// POST /api/import/csv - Synchronous version (DEPRECATED - use async)
// Kept for backward compatibility but limited to small files
func (s *Server) handleCSVImport(w http.ResponseWriter, r *http.Request) {
	// Limit sync imports to 1MB to prevent timeouts
	r.ParseMultipartForm(1 << 20)

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file required"})
		return
	}
	defer file.Close()

	// Check file size - redirect to async for large files
	if header.Size > 1<<20 { // 1MB
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
			"error":   "file too large for sync import (max 1MB)",
			"message": "Use POST /api/import/csv/async for larger files",
		})
		return
	}

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1

	// Read header
	header_row, err := reader.Read()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid CSV"})
		return
	}

	cleanHeader := func(h string) string {
		h = strings.TrimPrefix(h, "\ufeff")
		return strings.ToLower(strings.TrimSpace(h))
	}

	colMap := make(map[string]int)
	for i, col := range header_row {
		colMap[cleanHeader(col)] = i
	}

	domainIdx, hasDomain := colMap["domain"]
	if !hasDomain {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing 'domain' column"})
		return
	}

	senderIdx := colMap["sender"]
	localPartIdx := colMap["localpart"]
	ipIdx := colMap["ip"]
	passIdx := colMap["password"]
	bounceIdx := colMap["bounce"]
	bouncePassIdx := colMap["bounce_password"]

	var stats struct {
		DomainsCreated int      `json:"domains_created"`
		SendersCreated int      `json:"senders_created"`
		IPsAdded       int      `json:"ips_added"`
		BouncesCreated int      `json:"bounces_created"`
		DKIMGenerated  int      `json:"dkim_generated"`
		Errors         []string `json:"errors"`
	}

	domainCache := make(map[string]*models.Domain)
	processedSenders := make(map[string]bool)

	lineNum := 1
	for {
		lineNum++
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			stats.Errors = append(stats.Errors, "line "+string(rune(lineNum))+": invalid format")
			continue
		}

		if domainIdx >= len(record) {
			continue
		}
		domainName := strings.TrimSpace(record[domainIdx])
		if domainName == "" {
			continue
		}

		// SECURITY: Validate domain format BEFORE database insertion
		if !isValidDomainName(domainName) {
			stats.Errors = append(stats.Errors, fmt.Sprintf("line %d: invalid domain format: %s", lineNum, domainName))
			continue
		}

		domain, exists := domainCache[domainName]
		if !exists {
			existing, err := s.Store.GetDomainByName(domainName)
			if err != nil {
				domain = &models.Domain{
					Name:            domainName,
					MailHost:        "mail." + domainName,
					BounceHost:      "bounce." + domainName,
					DMARCPolicy:     "none",
					DMARCPercentage: 100,
				}
				if err := s.Store.CreateDomain(domain); err != nil {
					stats.Errors = append(stats.Errors, "failed to create domain: "+domainName)
					continue
				}
				stats.DomainsCreated++
			} else {
				domain = existing
			}
			domainCache[domainName] = domain
		}

		localPart := ""
		if localPartIdx > 0 && localPartIdx < len(record) {
			localPart = strings.TrimSpace(record[localPartIdx])
		}
		if localPart == "" && senderIdx > 0 && senderIdx < len(record) {
			senderEmail := strings.TrimSpace(record[senderIdx])
			if parts := strings.Split(senderEmail, "@"); len(parts) == 2 {
				localPart = parts[0]
			} else {
				localPart = senderEmail
			}
		}

		if localPart == "" {
			continue
		}

		// SECURITY: Validate localpart format BEFORE database insertion
		if !isValidLocalPart(localPart) {
			stats.Errors = append(stats.Errors, fmt.Sprintf("line %d: invalid localpart format: %s", lineNum, localPart))
			continue
		}

		ip := ""
		if ipIdx > 0 && ipIdx < len(record) {
			ip = strings.TrimSpace(record[ipIdx])
			// Validate IP format
			if ip != "" && !isValidIP(ip) {
				stats.Errors = append(stats.Errors, fmt.Sprintf("line %d: invalid IP format: %s", lineNum, ip))
				ip = "" // Clear invalid IP but continue
			}
		}

		password := ""
		if passIdx > 0 && passIdx < len(record) {
			rawPassword := strings.TrimSpace(record[passIdx])
			if rawPassword != "" {
				encrypted, err := core.EncryptPassword(rawPassword)
				if err == nil {
					password = encrypted
				}
			}
		}

		customBounce := ""
		if bounceIdx > 0 && bounceIdx < len(record) {
			customBounce = strings.TrimSpace(record[bounceIdx])
			// Validate bounce username
			if customBounce != "" && !isValidLocalPart(customBounce) {
				stats.Errors = append(stats.Errors, fmt.Sprintf("line %d: invalid bounce username: %s", lineNum, customBounce))
				customBounce = ""
			}
		}

		bouncePass := ""
		if bouncePassIdx > 0 && bouncePassIdx < len(record) {
			bouncePass = strings.TrimSpace(record[bouncePassIdx])
		}

		senderKey := domainName + ":" + localPart
		if processedSenders[senderKey] {
			continue
		}
		processedSenders[senderKey] = true

		sender := &models.Sender{
			DomainID:     domain.ID,
			LocalPart:    localPart,
			Email:        localPart + "@" + domainName,
			IP:           ip,
			SMTPPassword: password,
		}
		if err := s.Store.CreateSender(sender); err != nil {
			stats.Errors = append(stats.Errors, "failed to create sender: "+sender.Email)
			continue
		}
		stats.SendersCreated++

		if ip != "" {
			ipModel := &models.SystemIP{Value: ip, CreatedAt: time.Now()}
			if err := s.Store.CreateSystemIP(ipModel); err == nil {
				stats.IPsAdded++
			}
		}

		if err := core.GenerateDKIMKey(domainName, localPart); err == nil {
			stats.DKIMGenerated++
		}

		bounceUser := "b-" + localPart
		if customBounce != "" {
			bounceUser = customBounce
		}
		if err := core.CreateBounceAccountWithPassword(bounceUser, domainName, bouncePass, "Imported via CSV", s.Store); err == nil {
			stats.BouncesCreated++
		}
	}

	writeJSON(w, http.StatusOK, stats)
}
