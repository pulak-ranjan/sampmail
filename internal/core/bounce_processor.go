package core

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

// BounceProcessor handles parsing and processing of bounce emails
type BounceProcessor struct {
	Store   *store.Store
	LogDir  string // Configurable log directory (default: /var/log/kumomta)
	HomeDir string // Configurable home directory (default: /home)
}

func NewBounceProcessor(st *store.Store) *BounceProcessor {
	return &BounceProcessor{
		Store:   st,
		LogDir:  "/var/log/kumomta",
		HomeDir: "/home",
	}
}

// =====================================
// PRE-COMPILED REGEXES (Bug #2 Fix)
// Compile once at package init, not per-email
// =====================================

var (
	// JSON log parsing patterns
	bounceClassRe  = regexp.MustCompile(`"bounce_class(?:ification)?"\s*:\s*"([^"]+)"`)
	recipientRe    = regexp.MustCompile(`"recipient"\s*:\s*"([^"]+)"`)
	responseCodeRe = regexp.MustCompile(`"response"\s*:\s*"(\d{3})[^"]*"`)
	campaignIDRe   = regexp.MustCompile(`"campaign"\s*:\s*"?(\d+)"?`)

	// NDR email parsing patterns
	finalRecipientRe = regexp.MustCompile(`(?i)Final-Recipient:\s*rfc822;\s*([^\s\r\n]+)`)
	origRecipientRe  = regexp.MustCompile(`(?i)Original-Recipient:\s*rfc822;\s*([^\s\r\n]+)`)
	diagnosticCodeRe = regexp.MustCompile(`(?i)Diagnostic-Code:\s*(?:smtp;)?\s*(.+?)(?:\r?\n[^\s]|$)`)
	statusRe         = regexp.MustCompile(`(?i)Status:\s*(\d\.\d\.\d)`)
	smtpCodeRe       = regexp.MustCompile(`(?i)(\d{3})\s+\d\.\d\.\d`)
	campaignHeaderRe = regexp.MustCompile(`(?i)X-Campaign:\s*(\d+)`)

	// Fallback recipient patterns
	recipientPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)<([^>]+@[^>]+)>.*(?:undeliverable|failed|rejected)`),
		regexp.MustCompile(`(?i)(?:failed|rejected|bounced).*<([^>]+@[^>]+)>`),
		regexp.MustCompile(`(?i)Delivery to the following recipient[s]? failed[:\s]*([^\s\r\n<]+@[^\s\r\n>]+)`),
	}

	// Hard bounce indicator patterns
	hardBouncePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)user unknown`),
		regexp.MustCompile(`(?i)mailbox not found`),
		regexp.MustCompile(`(?i)no such user`),
		regexp.MustCompile(`(?i)recipient rejected`),
		regexp.MustCompile(`(?i)address rejected`),
		regexp.MustCompile(`(?i)does not exist`),
		regexp.MustCompile(`(?i)invalid recipient`),
		regexp.MustCompile(`(?i)permanent failure`),
		regexp.MustCompile(`(?i)user doesn't exist`),
		regexp.MustCompile(`(?i)mailbox unavailable`),
	}

	// Error extraction patterns
	errorPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?im)^550[- ](.+?)$`),
		regexp.MustCompile(`(?im)^5\d\d[- ](.+?)$`),
		regexp.MustCompile(`(?im)Remote host said:\s*(.+?)$`),
		regexp.MustCompile(`(?im)The error.*?was:\s*(.+?)$`),
	}

	// Username validation (Bug #6 Fix - prevent path traversal)
	validUsernameRe = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,30}$`)
)

// Common hard bounce codes
var hardBounceCodes = map[string]bool{
	"550": true, "551": true, "552": true, "553": true, "554": true,
	"511": true, "512": true, "513": true,
	"5.1.0": true, "5.1.1": true, "5.1.2": true, "5.1.3": true,
	"5.2.1": true, "5.2.2": true, "5.4.1": true,
}

// BounceInfo contains parsed bounce information
type BounceInfo struct {
	OriginalRecipient string
	BounceType        string // "hard", "soft", "complaint"
	BounceCode        string
	DiagnosticMessage string
	CampaignID        uint
}

// =====================================
// LOG FILE PROCESSING (Bug #1 Fix)
// Track file offsets to prevent re-processing
// =====================================

// ProcessKumoMTALogs reads and processes NEW bounce information from KumoMTA logs
func (bp *BounceProcessor) ProcessKumoMTALogs() error {
	// Panic recovery (Bug #4 Fix)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[BounceProcessor] PANIC in ProcessKumoMTALogs: %v\n%s", r, debug.Stack())
		}
	}()

	files, err := filepath.Glob(filepath.Join(bp.LogDir, "*.log"))
	if err != nil {
		return fmt.Errorf("failed to list log files: %w", err)
	}

	processedCount := 0
	for _, filePath := range files {
		count, err := bp.processLogFileIncremental(filePath)
		if err != nil {
			log.Printf("[BounceProcessor] Error processing %s: %v", filePath, err)
			continue
		}
		processedCount += count
	}

	if processedCount > 0 {
		log.Printf("[BounceProcessor] Processed %d new bounces from logs", processedCount)
	}

	return nil
}

// processLogFileIncremental processes only NEW lines in a log file
func (bp *BounceProcessor) processLogFileIncremental(filePath string) (int, error) {
	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}

	// Get last processed state
	var state models.ProcessedLogFile
	bp.Store.DB.Where("file_path = ?", filePath).First(&state)

	// Check for log rotation (file got smaller = new file)
	if fileInfo.Size() < state.LastSize {
		log.Printf("[BounceProcessor] Log rotation detected for %s, resetting offset", filePath)
		state.LastOffset = 0
	}

	// If file hasn't grown, skip it
	if fileInfo.Size() <= state.LastOffset {
		return 0, nil
	}

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// Seek to last processed position
	if state.LastOffset > 0 {
		_, err = file.Seek(state.LastOffset, io.SeekStart)
		if err != nil {
			return 0, err
		}
	}

	// Process new lines
	scanner := bufio.NewScanner(file)
	// Increase buffer for long log lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	processed := 0
	for scanner.Scan() {
		line := scanner.Text()

		// Skip non-bounce entries
		if !strings.Contains(line, "Bounce") && !strings.Contains(line, "bounce") &&
			!strings.Contains(line, "reject") && !strings.Contains(line, "Reject") {
			continue
		}

		bounceInfo := bp.parseLogLine(line)
		if bounceInfo != nil && bounceInfo.OriginalRecipient != "" {
			if err := bp.recordBounce(bounceInfo); err == nil {
				processed++
			}
		}
	}

	// Update processing state
	currentOffset, _ := file.Seek(0, io.SeekCurrent)
	state.FilePath = filePath
	state.LastOffset = currentOffset
	state.LastSize = fileInfo.Size()
	state.ProcessedAt = time.Now()

	bp.Store.DB.Save(&state)

	return processed, scanner.Err()
}

// parseLogLine extracts bounce info from a JSON log line
func (bp *BounceProcessor) parseLogLine(line string) *BounceInfo {
	info := &BounceInfo{BounceType: "soft"}

	// Extract recipient
	if matches := recipientRe.FindStringSubmatch(line); len(matches) > 1 {
		info.OriginalRecipient = matches[1]
	}

	// Extract bounce code
	if matches := responseCodeRe.FindStringSubmatch(line); len(matches) > 1 {
		info.BounceCode = matches[1]
		if hardBounceCodes[matches[1]] {
			info.BounceType = "hard"
		}
	}

	// Extract campaign ID
	if matches := campaignIDRe.FindStringSubmatch(line); len(matches) > 1 {
		fmt.Sscanf(matches[1], "%d", &info.CampaignID)
	}

	// Extract bounce classification
	if matches := bounceClassRe.FindStringSubmatch(line); len(matches) > 1 {
		class := strings.ToLower(matches[1])
		if strings.Contains(class, "invalid") || strings.Contains(class, "unknown") ||
			strings.Contains(class, "bad_mailbox") || strings.Contains(class, "no_such_user") {
			info.BounceType = "hard"
		}
		info.DiagnosticMessage = matches[1]
	}

	return info
}

// =====================================
// MAILDIR PROCESSING
// =====================================

// ProcessMaildirs scans Maildir for bounce emails and processes them
func (bp *BounceProcessor) ProcessMaildirs() error {
	// Panic recovery (Bug #4 Fix)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[BounceProcessor] PANIC in ProcessMaildirs: %v\n%s", r, debug.Stack())
		}
	}()

	accounts, err := bp.Store.ListBounceAccounts()
	if err != nil {
		return err
	}

	totalProcessed := 0
	for _, acc := range accounts {
		// Validate username to prevent path traversal (Bug #6 Fix)
		if !validUsernameRe.MatchString(acc.Username) {
			log.Printf("[BounceProcessor] Invalid username skipped: %s", acc.Username)
			continue
		}

		// Build path safely
		maildir := filepath.Clean(filepath.Join(bp.HomeDir, acc.Username, "Maildir", "new"))

		// Verify the path is still under expected directory
		if !strings.HasPrefix(maildir, bp.HomeDir+"/") {
			log.Printf("[BounceProcessor] Path traversal attempt blocked: %s", acc.Username)
			continue
		}

		count, err := bp.processMaildir(maildir)
		if err != nil {
			log.Printf("[BounceProcessor] Error processing maildir for %s: %v", acc.Username, err)
			continue
		}
		totalProcessed += count
	}

	if totalProcessed > 0 {
		log.Printf("[BounceProcessor] Processed %d bounce emails from Maildirs", totalProcessed)
	}

	return nil
}

func (bp *BounceProcessor) processMaildir(dir string) (int, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	processed := 0
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filePath := filepath.Join(dir, file.Name())
		
		// FIXED: Check file size before reading to prevent memory bombs
		info, err := file.Info()
		if err != nil {
			continue
		}
		
		// Skip files larger than 1MB - legitimate bounce emails are small
		if info.Size() > 1*1024*1024 {
			log.Printf("[BounceProcessor] Skipping oversized file (%d bytes): %s", info.Size(), filePath)
			// Move to cur/ anyway to prevent re-processing
			curPath := filepath.Join(filepath.Dir(dir), "cur", file.Name())
			os.Rename(filePath, curPath)
			continue
		}
		
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		bounceInfo := bp.parseBounceEmail(string(content))
		if bounceInfo != nil && bounceInfo.OriginalRecipient != "" {
			if err := bp.recordBounce(bounceInfo); err == nil {
				processed++
				// Move to cur/ after processing
				curPath := filepath.Join(filepath.Dir(dir), "cur", file.Name())
				os.Rename(filePath, curPath)
			}
		}
	}

	return processed, nil
}

// ProcessMaildirsStreaming is a memory-safe version that processes files with size limits
// FIXED: Prevents memory bombs from large email files
func (bp *BounceProcessor) ProcessMaildirsStreaming() error {
	// Panic recovery
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[BounceProcessor] PANIC in ProcessMaildirsStreaming: %v\n%s", r, debug.Stack())
		}
	}()

	accounts, err := bp.Store.ListBounceAccounts()
	if err != nil {
		return err
	}

	totalProcessed := 0
	for _, acc := range accounts {
		// Validate username to prevent path traversal
		if !validUsernameRe.MatchString(acc.Username) {
			log.Printf("[BounceProcessor] Invalid username skipped: %s", acc.Username)
			continue
		}

		// Build path safely
		maildir := filepath.Clean(filepath.Join(bp.HomeDir, acc.Username, "Maildir", "new"))

		// Verify the path is still under expected directory
		if !strings.HasPrefix(maildir, bp.HomeDir+"/") {
			log.Printf("[BounceProcessor] Path traversal attempt blocked: %s", acc.Username)
			continue
		}

		count, err := bp.processMaildirStreaming(maildir)
		if err != nil {
			log.Printf("[BounceProcessor] Error processing maildir for %s: %v", acc.Username, err)
			continue
		}
		totalProcessed += count
	}

	if totalProcessed > 0 {
		log.Printf("[BounceProcessor] Processed %d bounce emails from Maildirs", totalProcessed)
	}

	return nil
}

// processMaildirStreaming processes emails with streaming to limit memory usage
func (bp *BounceProcessor) processMaildirStreaming(dir string) (int, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	processed := 0
	const maxFileSize = 512 * 1024      // 512KB max per file
	const maxHeaderSize = 32 * 1024     // 32KB for headers (where bounce info is)
	
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filePath := filepath.Join(dir, file.Name())
		
		// Check file size first
		info, err := file.Info()
		if err != nil {
			continue
		}
		
		// Skip oversized files
		if info.Size() > maxFileSize {
			log.Printf("[BounceProcessor] Skipping oversized file (%d bytes): %s", info.Size(), filePath)
			curPath := filepath.Join(filepath.Dir(dir), "cur", file.Name())
			os.Rename(filePath, curPath)
			continue
		}
		
		// Read only the headers (most bounce info is in headers)
		bounceInfo, err := bp.parseBouncEmailStreaming(filePath, maxHeaderSize)
		if err != nil {
			continue
		}
		
		if bounceInfo != nil && bounceInfo.OriginalRecipient != "" {
			if err := bp.recordBounce(bounceInfo); err == nil {
				processed++
			}
		}
		
		// Move to cur/ after processing (success or failure)
		curPath := filepath.Join(filepath.Dir(dir), "cur", file.Name())
		os.Rename(filePath, curPath)
	}

	return processed, nil
}

// parseBouncEmailStreaming reads only the first N bytes of a file
func (bp *BounceProcessor) parseBouncEmailStreaming(filePath string, maxBytes int64) (*BounceInfo, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	
	// Read limited amount
	reader := io.LimitReader(f, maxBytes)
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	
	return bp.parseBounceEmail(string(content)), nil
}

// =====================================
// EMAIL PARSING (Bug #5 Fix)
// Proper diagnostic extraction
// =====================================

func (bp *BounceProcessor) parseBounceEmail(content string) *BounceInfo {
	info := &BounceInfo{BounceType: "soft"}

	// 1. Extract recipient using pre-compiled regexes
	if matches := finalRecipientRe.FindStringSubmatch(content); len(matches) > 1 {
		info.OriginalRecipient = strings.TrimSpace(matches[1])
	} else if matches := origRecipientRe.FindStringSubmatch(content); len(matches) > 1 {
		info.OriginalRecipient = strings.TrimSpace(matches[1])
	} else {
		// Try fallback patterns
		for _, re := range recipientPatterns {
			if matches := re.FindStringSubmatch(content); len(matches) > 1 {
				info.OriginalRecipient = strings.TrimSpace(matches[1])
				break
			}
		}
	}

	// 2. Extract diagnostic code/message (Bug #5 Fix - extract actual error, not raw headers)
	if matches := diagnosticCodeRe.FindStringSubmatch(content); len(matches) > 1 {
		info.DiagnosticMessage = strings.TrimSpace(matches[1])
		// Try to extract SMTP code from diagnostic
		if codeMatches := smtpCodeRe.FindStringSubmatch(info.DiagnosticMessage); len(codeMatches) > 1 {
			info.BounceCode = codeMatches[1]
		}
	}

	// 3. Extract status code
	if matches := statusRe.FindStringSubmatch(content); len(matches) > 1 {
		code := matches[1]
		if info.BounceCode == "" {
			info.BounceCode = code
		}
		if strings.HasPrefix(code, "5") {
			info.BounceType = "hard"
		}
	}

	// 4. Check for hard bounce patterns
	for _, re := range hardBouncePatterns {
		if re.MatchString(content) {
			info.BounceType = "hard"
			break
		}
	}

	// 5. Extract campaign ID from header
	if matches := campaignHeaderRe.FindStringSubmatch(content); len(matches) > 1 {
		fmt.Sscanf(matches[1], "%d", &info.CampaignID)
	}

	// 6. If no diagnostic found, try to extract a meaningful error message
	if info.DiagnosticMessage == "" {
		info.DiagnosticMessage = bp.extractErrorMessage(content)
	}

	return info
}

// extractErrorMessage tries to find the actual error message, not raw headers
func (bp *BounceProcessor) extractErrorMessage(content string) string {
	for _, re := range errorPatterns {
		if matches := re.FindStringSubmatch(content); len(matches) > 1 {
			msg := strings.TrimSpace(matches[1])
			if len(msg) > 200 {
				msg = msg[:200]
			}
			return msg
		}
	}
	return "Unknown delivery failure"
}

// =====================================
// BOUNCE RECORDING
// =====================================

func (bp *BounceProcessor) recordBounce(info *BounceInfo) error {
	email := strings.ToLower(strings.TrimSpace(info.OriginalRecipient))
	if email == "" || !strings.Contains(email, "@") {
		return fmt.Errorf("invalid email")
	}

	// =====================================
	// ENHANCED BOUNCE CLASSIFICATION
	// =====================================
	// Classify bounce more precisely based on codes
	classification := bp.classifyBounce(info)
	info.BounceType = classification.Type

	// Create bounce event record
	event := &models.BounceEvent{
		Email:          email,
		BounceType:     info.BounceType,
		BounceCode:     info.BounceCode,
		DiagnosticCode: info.DiagnosticMessage,
		CampaignID:     info.CampaignID,
		ProcessedAt:    time.Now(),
	}

	if err := bp.Store.CreateBounceEvent(event); err != nil {
		return err
	}

	// =====================================
	// BOUNCE ACTIONS BY CLASSIFICATION
	// =====================================
	switch classification.Action {
	case "suppress":
		// Hard bounce - add to suppression immediately
		source := "bounce:" + classification.Reason
		if info.CampaignID > 0 {
			source = fmt.Sprintf("campaign:%d:%s", info.CampaignID, classification.Reason)
		}
		// orgID=0: system-level suppression applies across all orgs
		bp.Store.AddSuppression(0, email, classification.Reason, source)

		// Update contact status
		bp.Store.DB.Model(&models.ContactV2{}).
			Where("email = ?", email).
			Update("status", "bounced")

		log.Printf("[BounceProcessor] Hard bounce suppressed: %s (reason: %s)", email, classification.Reason)

	case "retry":
		// Soft bounce - schedule for retry if job queue available
		log.Printf("[BounceProcessor] Soft bounce recorded: %s (code: %s) - eligible for retry", email, info.BounceCode)
		// Note: Retry scheduling is handled by the retry policy in the campaign service
		// Here we just record it; the job queue will handle retries

	case "suppress_threshold":
		// Soft bounce threshold exceeded
		bp.Store.AddSuppression(0, email, "soft_bounce_threshold", "auto")
		log.Printf("[BounceProcessor] Soft bounce threshold exceeded: %s", email)

	case "complaint":
		// Spam complaint - suppress and alert
		bp.Store.AddSuppression(0, email, "complaint", "fbl")
		bp.Store.DB.Model(&models.ContactV2{}).
			Where("email = ?", email).
			Update("status", "complained")
		log.Printf("[BounceProcessor] Complaint processed: %s", email)
		
		// Update sender reputation
		if info.CampaignID > 0 {
			bp.decrementSenderReputation(info.CampaignID, 15)
		}
		
	case "alert":
		// Technical issue - alert but don't suppress
		log.Printf("[BounceProcessor] ALERT: Technical bounce for %s - %s: %s", 
			email, classification.Reason, info.DiagnosticMessage)
	}

	// Update campaign bounce stats
	if info.CampaignID > 0 {
		bp.updateCampaignBounceStats(info.CampaignID, info.BounceType)
	}

	return nil
}

// BounceClassification represents the result of bounce classification
type BounceClassification struct {
	Type     string // "hard", "soft", "complaint", "technical"
	Action   string // "suppress", "retry", "suppress_threshold", "complaint", "alert"
	Reason   string // Human-readable reason
	Retryable bool  // Whether this bounce is eligible for retry
}

// classifyBounce determines the precise classification of a bounce
func (bp *BounceProcessor) classifyBounce(info *BounceInfo) BounceClassification {
	class := BounceClassification{
		Type:      info.BounceType,
		Action:    "retry",
		Retryable: false,
	}
	
	// Check for specific bounce codes
	code := info.BounceCode
	
	// Hard bounce codes - permanent failures
	hardBounceReasons := map[string]string{
		"5.1.1": "mailbox_does_not_exist",
		"5.1.2": "domain_does_not_exist",
		"5.1.3": "invalid_mailbox_syntax",
		"5.1.4": "mailbox_ambiguous",
		"5.1.6": "mailbox_moved",
		"5.2.1": "mailbox_disabled",
		"5.2.2": "mailbox_full_permanent",
		"5.2.3": "message_too_long",
		"5.4.1": "no_answer_from_host",
		"5.4.4": "unable_to_route",
		"5.7.1": "delivery_not_authorized",
		"5.7.26": "dkim_spf_failure",
	}
	
	if reason, isHard := hardBounceReasons[code]; isHard {
		class.Type = "hard"
		class.Action = "suppress"
		class.Reason = reason
		class.Retryable = false
		return class
	}
	
	// Check SMTP codes
	smtpCode := extractSMTPCodeFromBounce(info.DiagnosticMessage)
	
	switch {
	case strings.HasPrefix(smtpCode, "550"):
		class.Type = "hard"
		class.Action = "suppress"
		class.Reason = "mailbox_unavailable"
		
	case strings.HasPrefix(smtpCode, "551"):
		class.Type = "hard"
		class.Action = "suppress"
		class.Reason = "user_not_local"
		
	case strings.HasPrefix(smtpCode, "552") && strings.Contains(strings.ToLower(info.DiagnosticMessage), "full"):
		// Mailbox full - this is actually a soft bounce
		class.Type = "soft"
		class.Action = "retry"
		class.Reason = "mailbox_full"
		class.Retryable = true
		
	case strings.HasPrefix(smtpCode, "553"):
		class.Type = "hard"
		class.Action = "suppress"
		class.Reason = "invalid_mailbox_name"
		
	case strings.HasPrefix(smtpCode, "554"):
		class.Type = "hard"
		class.Action = "suppress"
		class.Reason = "transaction_failed"
		
	// Soft bounce codes - temporary failures
	case strings.HasPrefix(smtpCode, "421"):
		class.Type = "soft"
		class.Action = "retry"
		class.Reason = "service_unavailable"
		class.Retryable = true
		
	case strings.HasPrefix(smtpCode, "450"):
		class.Type = "soft"
		class.Action = "retry"
		class.Reason = "mailbox_busy"
		class.Retryable = true
		
	case strings.HasPrefix(smtpCode, "451"):
		class.Type = "soft"
		class.Action = "retry"
		class.Reason = "local_error"
		class.Retryable = true
		
	case strings.HasPrefix(smtpCode, "452"):
		class.Type = "soft"
		class.Action = "retry"
		class.Reason = "insufficient_storage"
		class.Retryable = true
		
	case strings.HasPrefix(smtpCode, "4"):
		class.Type = "soft"
		class.Action = "retry"
		class.Reason = "temporary_failure"
		class.Retryable = true
	}
	
	// Check diagnostic message for patterns
	diagLower := strings.ToLower(info.DiagnosticMessage)
	
	// Hard bounce patterns
	hardPatterns := []string{
		"user unknown",
		"mailbox not found",
		"no such user",
		"recipient rejected",
		"address rejected",
		"does not exist",
		"invalid recipient",
		"permanent failure",
		"mailbox unavailable",
	}
	
	for _, pattern := range hardPatterns {
		if strings.Contains(diagLower, pattern) {
			class.Type = "hard"
			class.Action = "suppress"
			class.Reason = "permanent_failure"
			class.Retryable = false
			return class
		}
	}
	
	// Soft bounce patterns
	softPatterns := []string{
		"try again later",
		"temporarily unavailable",
		"greylist",
		"graylist",
		"rate limit",
		"throttl",
		"try again",
		"come back later",
	}
	
	for _, pattern := range softPatterns {
		if strings.Contains(diagLower, pattern) {
			class.Type = "soft"
			class.Action = "retry"
			class.Reason = "temporary_failure"
			class.Retryable = true
			return class
		}
	}
	
	// Check for complaint type
	if info.BounceType == "complaint" {
		class.Type = "complaint"
		class.Action = "complaint"
		class.Reason = "spam_complaint"
		return class
	}
	
	// Check soft bounce threshold
	if info.BounceType == "soft" {
		bounces, _ := bp.Store.GetBouncesByEmail(info.OriginalRecipient)
		softCount := 0
		for _, b := range bounces {
			if b.BounceType == "soft" {
				softCount++
			}
		}
		if softCount >= 3 {
			class.Action = "suppress_threshold"
			class.Reason = "soft_bounce_threshold_exceeded"
		}
	}
	
	return class
}

// extractSMTPCodeFromBounce extracts SMTP code from diagnostic message
func extractSMTPCodeFromBounce(diag string) string {
	re := regexp.MustCompile(`(\d{3})`)
	matches := re.FindStringSubmatch(diag)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// updateCampaignBounceStats updates campaign bounce statistics
func (bp *BounceProcessor) updateCampaignBounceStats(campaignID uint, bounceType string) {
	// Use a strict whitelist to prevent SQL injection — never interpolate
	// user-controlled strings directly into column names or raw SQL.
	var column string
	if bounceType == "soft" {
		column = "total_soft_bounced"
	} else {
		column = "total_bounced"
	}

	bp.Store.DB.Model(&models.Campaign{}).
		Where("id = ?", campaignID).
		UpdateColumn(column, gorm.Expr(column+" + 1"))
}

// decrementSenderReputation decreases sender reputation score
func (bp *BounceProcessor) decrementSenderReputation(campaignID uint, points int) {
	var campaign models.Campaign
	if err := bp.Store.DB.First(&campaign, campaignID).Error; err != nil {
		return
	}
	
	bp.Store.DB.Model(&models.Sender{}).
		Where("id = ?", campaign.SenderID).
		UpdateColumn("reputation_score", 
			bp.Store.DB.Raw("GREATEST(0, reputation_score - ?)", points))
}

// ProcessComplaint handles FBL (Feedback Loop) complaints
func (bp *BounceProcessor) ProcessComplaint(email string, campaignID uint) error {
	// Panic recovery (Bug #4 Fix)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[BounceProcessor] PANIC in ProcessComplaint: %v\n%s", r, debug.Stack())
		}
	}()

	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return fmt.Errorf("empty email")
	}

	event := &models.BounceEvent{
		Email:       email,
		BounceType:  "complaint",
		CampaignID:  campaignID,
		ProcessedAt: time.Now(),
	}
	bp.Store.CreateBounceEvent(event)

	source := "fbl"
	if campaignID > 0 {
		source = fmt.Sprintf("fbl:campaign:%d", campaignID)
	}
	// orgID=0: system-level suppression applies across all orgs
	return bp.Store.AddSuppression(0, email, "complaint", source)
}

// GetBounceStats returns bounce statistics
func (bp *BounceProcessor) GetBounceStats(hours int) (map[string]interface{}, error) {
	bounces, err := bp.Store.GetRecentBounces(hours)
	if err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"total":      len(bounces),
		"hard":       0,
		"soft":       0,
		"complaints": 0,
	}

	for _, b := range bounces {
		switch b.BounceType {
		case "hard":
			stats["hard"] = stats["hard"].(int) + 1
		case "soft":
			stats["soft"] = stats["soft"].(int) + 1
		case "complaint":
			stats["complaints"] = stats["complaints"].(int) + 1
		}
	}

	return stats, nil
}
