package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/core"
	"github.com/pulak-ranjan/sampmail/internal/firewall"
	"github.com/pulak-ranjan/sampmail/internal/logger"
	"github.com/pulak-ranjan/sampmail/internal/models"
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Messages []ChatMessage `json:"messages"`
	NewMsg   string        `json:"new_msg"`
}

// Allowed "Safe" Tools for the Agent - NO SHELL EXECUTION
var allowedTools = map[string]string{
	"status":         "Check KumoMTA Service Status",
	"queue":          "Check Queue Summary",
	"logs":           "Get recent logs (from database)",
	"config_bind_ip": "Update SMTP listener IP. Args: IP address",
	"block_ip":       "Ban an IP address. Args: IP",
	"backup_config":  "Create a backup of configuration",
	"dns_mx":         "Perform MX lookup for a domain. Args: domain",
	"dns_txt":        "Perform TXT lookup for a domain. Args: domain",
}

// GET /api/ai/history
func (s *Server) handleGetChatHistory(w http.ResponseWriter, r *http.Request) {
	logs, err := s.Store.GetChatHistory(50)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch history"})
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

// POST /api/ai/chat
func (s *Server) handleAIChat(w http.ResponseWriter, r *http.Request) {
	log := logger.WithComponent("ai_chat")

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	userContent := req.NewMsg
	if userContent == "" && len(req.Messages) > 0 {
		userContent = req.Messages[len(req.Messages)-1].Content
	}

	if userContent == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "empty message"})
		return
	}

	s.Store.SaveChatLog("user", userContent)

	settings, err := s.Store.GetSettings()
	if err != nil || settings == nil || settings.AIAPIKey == "" {
		reply := "AI is not configured. Please add an API Key in Settings."
		s.Store.SaveChatLog("assistant", reply)
		writeJSON(w, http.StatusOK, map[string]string{"reply": reply})
		return
	}

	aiKey, err := core.Decrypt(settings.AIAPIKey)
	if err != nil {
		log.Error("failed to decrypt AI key", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to decrypt AI key"})
		return
	}

	// Gather context using NATIVE Go functions (no shell)
	systemContext := s.gatherSystemContext()

	historyLogs, _ := s.Store.GetChatHistory(10)
	var contextMsgs []ChatMessage
	for _, l := range historyLogs {
		contextMsgs = append(contextMsgs, ChatMessage{Role: l.Role, Content: l.Content})
	}

	toolsDesc := ""
	for k, v := range allowedTools {
		toolsDesc += fmt.Sprintf("- `%s`: %s\n", k, v)
	}

	systemPrompt := fmt.Sprintf(`You are the KumoMTA Guardian. 
Your role is to Secure, Configure, and Monitor this MTA server.

[TOOLS]
You can run system tasks by outputting a command tag at the END of your response.
Format: <<EXEC: command_name args>>
Allowed Commands:
%s

[BEHAVIORAL RULES]
1. **Be Structured:** Use Headers (##) and Bullet points. No conversational fluff.
2. **Be Suspicious:** If logs show "auth failed" or "relay denied" > 5 times from an IP, suggest 'block_ip'.
3. **Be Safe:** ALWAYS run 'backup_config' before 'config_bind_ip'.
4. **Confirmation:** If the user asks for a sensitive action, ASK FOR CONFIRMATION first.

[CURRENT SYSTEM STATUS]
%s
`, toolsDesc, systemContext)

	finalMessages := []ChatMessage{{Role: "system", Content: systemPrompt}}
	finalMessages = append(finalMessages, contextMsgs...)

	rawReply, err := s.sendToAI(settings.AIProvider, aiKey, finalMessages)
	if err != nil {
		log.Error("AI API call failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	reply, toolOutput := s.processToolExecution(rawReply)
	if toolOutput != "" {
		reply += fmt.Sprintf("\n\n**System Output:**\n```\n%s\n```", toolOutput)
	}

	s.Store.SaveChatLog("assistant", reply)

	writeJSON(w, http.StatusOK, map[string]string{"reply": reply})
}

// gatherSystemContext collects system info using NATIVE Go, no shell commands
func (s *Server) gatherSystemContext() string {
	var ctx strings.Builder

	// Queue stats from our Go implementation
	if stats, err := core.GetQueueStats(); err == nil {
		ctx.WriteString(fmt.Sprintf("Queue: Total=%d, Queued=%d, Deferred=%d\n",
			stats.Total, stats.Queued, stats.Deferred))
	}

	// Recent bounce events from database
	if bounces, err := s.Store.GetRecentBounces(24); err == nil && len(bounces) > 0 {
		ctx.WriteString(fmt.Sprintf("Recent Bounces (24h): %d\n", len(bounces)))
		// Show last few
		for i := 0; i < min(5, len(bounces)); i++ {
			ctx.WriteString(fmt.Sprintf("  - %s: %s (%s)\n",
				bounces[i].Email, bounces[i].BounceType, bounces[i].BounceCode))
		}
	}

	// Domain stats from database
	if stats, err := core.GetAllDomainsStats(1); err == nil {
		for domain, days := range stats {
			if len(days) > 0 {
				d := days[len(days)-1]
				ctx.WriteString(fmt.Sprintf("Domain %s: Sent=%d, Delivered=%d, Bounced=%d\n",
					domain, d.Sent, d.Delivered, d.Bounced))
			}
		}
	}

	// Campaign status from database
	var activeCampaigns int64
	s.Store.DB.Model(&models.Campaign{}).Where("status = 'sending'").Count(&activeCampaigns)
	ctx.WriteString(fmt.Sprintf("Active Campaigns: %d\n", activeCampaigns))

	return ctx.String()
}

func (s *Server) processToolExecution(response string) (string, string) {
	re := regexp.MustCompile(`<<EXEC:\s*(\w+)(?:\s+(.+))?>>`)
	matches := re.FindStringSubmatch(response)

	if len(matches) > 0 {
		cmdName := matches[1]
		args := ""
		if len(matches) > 2 {
			args = strings.TrimSpace(matches[2])
		}

		cleanReply := strings.Replace(response, matches[0], "", -1)
		output := s.runSafeTool(cmdName, args)
		return cleanReply, output
	}

	return response, ""
}

// runSafeTool executes only allowlisted logic using NATIVE Go - NO SHELL
func (s *Server) runSafeTool(cmdName, args string) string {
	log := logger.WithComponent("ai_tool")

	switch cmdName {
	case "status":
		// Check if we can connect to KumoMTA API
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get("http://127.0.0.1:8000/api/admin/status")
		if err != nil {
			return "KumoMTA API not responding: " + err.Error()
		}
		defer resp.Body.Close()
		if resp.StatusCode == 200 {
			return "KumoMTA is running and API is responding"
		}
		return fmt.Sprintf("KumoMTA API returned status: %d", resp.StatusCode)

	case "queue":
		stats, err := core.GetQueueStats()
		if err != nil {
			return "Error reading queue stats: " + err.Error()
		}
		return fmt.Sprintf("Total: %d, Queued: %d, Deferred: %d", stats.Total, stats.Queued, stats.Deferred)

	case "logs":
		// Return recent events from database instead of journalctl
		bounces, _ := s.Store.GetRecentBounces(1) // Last hour
		var result strings.Builder
		result.WriteString("Recent Events:\n")
		for i := 0; i < min(20, len(bounces)); i++ {
			b := bounces[i]
			result.WriteString(fmt.Sprintf("[%s] %s - %s (%s)\n",
				b.ProcessedAt.Format("15:04:05"),
				b.Email, b.BounceType, b.DiagnosticCode))
		}
		return result.String()

	case "dns_mx":
		// Use native Go DNS lookup instead of dig
		domain := sanitizeDomain(args)
		if domain == "" {
			return "Invalid domain format"
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		mxRecords, err := net.DefaultResolver.LookupMX(ctx, domain)
		if err != nil {
			return fmt.Sprintf("MX lookup failed: %v", err)
		}

		var result strings.Builder
		result.WriteString(fmt.Sprintf("MX Records for %s:\n", domain))
		// Sort by preference
		sort.Slice(mxRecords, func(i, j int) bool {
			return mxRecords[i].Pref < mxRecords[j].Pref
		})
		for _, mx := range mxRecords {
			result.WriteString(fmt.Sprintf("  %d %s\n", mx.Pref, mx.Host))
		}
		return result.String()

	case "dns_txt":
		domain := sanitizeDomain(args)
		if domain == "" {
			return "Invalid domain format"
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		txtRecords, err := net.DefaultResolver.LookupTXT(ctx, domain)
		if err != nil {
			return fmt.Sprintf("TXT lookup failed: %v", err)
		}

		var result strings.Builder
		result.WriteString(fmt.Sprintf("TXT Records for %s:\n", domain))
		for _, txt := range txtRecords {
			result.WriteString(fmt.Sprintf("  %s\n", txt))
		}
		return result.String()

	case "block_ip":
		if err := firewall.BlockIP(args); err != nil {
			log.Warn("IP block failed", "ip", args, "error", err)
			return "Block Failed: " + err.Error()
		}
		log.Info("IP blocked via AI chat", "ip", args)
		return fmt.Sprintf("IP %s has been blocked.", args)

	case "backup_config":
		if err := core.BackupConfig(); err != nil {
			return "Backup Failed: " + err.Error()
		}
		return "Configuration backed up successfully."

	case "config_bind_ip":
		ip := strings.TrimSpace(args)
		if net.ParseIP(ip) == nil {
			return "Error: Invalid IP address provided."
		}

		settings, err := s.Store.GetSettings()
		if err != nil {
			settings = &models.AppSettings{}
		}
		settings.SMTPListenAddr = ip + ":25"
		if err := s.Store.UpsertSettings(settings); err != nil {
			return "Database Error: " + err.Error()
		}

		snap, err := core.LoadSnapshot(s.Store)
		if err != nil {
			return "Snapshot Error: " + err.Error()
		}

		res, err := core.ApplyKumoConfig(snap)
		if err != nil {
			return fmt.Sprintf("Apply Failed: %v\nValidation Output: %s", err, res.ValidationLog)
		}
		log.Info("SMTP bind IP changed via AI", "ip", ip)
		return fmt.Sprintf("Success! Port 25 Listener updated to %s. Service restarted.", settings.SMTPListenAddr)

	default:
		return "Command not allowed or unknown."
	}
}

// sanitizeDomain validates and cleans a domain name
func sanitizeDomain(domain string) string {
	domain = strings.TrimSpace(domain)
	// Only allow alphanumeric, dots, and hyphens
	validDomain := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.-]*[a-zA-Z0-9]$`)
	if !validDomain.MatchString(domain) || len(domain) > 253 {
		return ""
	}
	return domain
}

func (s *Server) sendToAI(provider, key string, messages []ChatMessage) (string, error) {
	// --- Google Gemini Handler ---
	if provider == "gemini" {
		url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=" + key

		type Part struct {
			Text string `json:"text"`
		}
		type Content struct {
			Role  string `json:"role"`
			Parts []Part `json:"parts"`
		}
		type SystemInstruction struct {
			Parts []Part `json:"parts"`
		}
		type GeminiRequest struct {
			Contents          []Content          `json:"contents"`
			SystemInstruction *SystemInstruction `json:"systemInstruction,omitempty"`
		}

		var geminiReq GeminiRequest
		var contents []Content

		for _, m := range messages {
			if m.Role == "system" {
				geminiReq.SystemInstruction = &SystemInstruction{
					Parts: []Part{{Text: m.Content}},
				}
				continue
			}
			role := "user"
			if m.Role == "assistant" {
				role = "model"
			}
			contents = append(contents, Content{
				Role:  role,
				Parts: []Part{{Text: m.Content}},
			})
		}
		geminiReq.Contents = contents

		reqBody, _ := json.Marshal(geminiReq)
		req, _ := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != 200 {
			return "", fmt.Errorf("Gemini API Error (%d): %s", resp.StatusCode, string(body))
		}

		// Parse Gemini Response
		// Structure: { candidates: [ { content: { parts: [ { text: "..." } ] } } ] }
		var result struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			return "", err
		}
		if len(result.Candidates) > 0 && len(result.Candidates[0].Content.Parts) > 0 {
			return result.Candidates[0].Content.Parts[0].Text, nil
		}
		return "No response from Gemini.", nil
	}

	// --- OpenAI / DeepSeek Handler (Standard) ---
	url := "https://api.openai.com/v1/chat/completions"
	model := "gpt-4o"

	if provider == "deepseek" {
		url = "https://api.deepseek.com/chat/completions"
		model = "deepseek-chat"
	}

	payloadMsgs := make([]map[string]string, len(messages))
	for i, m := range messages {
		payloadMsgs[i] = map[string]string{"role": m.Role, "content": m.Content}
	}

	reqBody, _ := json.Marshal(map[string]interface{}{
		"model":    model,
		"messages": payloadMsgs,
	})

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("AI API Error (%d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	choices, _ := result["choices"].([]interface{})
	if len(choices) > 0 {
		if c, ok := choices[0].(map[string]interface{}); ok {
			if m, ok := c["message"].(map[string]interface{}); ok {
				return m["content"].(string), nil
			}
		}
	}
	return "No response.", nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
