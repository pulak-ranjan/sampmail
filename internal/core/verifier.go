package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

// Reacher-compatible Result Structure
type EmailVerificationResult struct {
	Input       string `json:"input"`
	IsReachable string `json:"is_reachable"` // "safe", "risky", "invalid", "unknown"

	Misc   MiscDetails   `json:"misc"`
	MX     MXDetails     `json:"mx"`
	SMTP   SMTPDetails   `json:"smtp"`
	Syntax SyntaxDetails `json:"syntax"`

	Error     string `json:"error,omitempty"`
	RiskScore int    `json:"risk_score"`
	Log       string `json:"log"`
	Source    string `json:"source,omitempty"` // "reacher_api", "reacher_bin", "local"
}

type MiscDetails struct {
	IsDisposable  bool `json:"is_disposable"`
	IsRoleAccount bool `json:"is_role_account"`
}

type MXDetails struct {
	AcceptsMail bool     `json:"accepts_mail"`
	Records     []string `json:"records"`
}

type SMTPDetails struct {
	CanConnect    bool `json:"can_connect_smtp"`
	IsCatchAll    bool `json:"is_catch_all"`
	IsDeliverable bool `json:"is_deliverable"`
}

type SyntaxDetails struct {
	Domain        string `json:"domain"`
	Username      string `json:"username"`
	IsValidSyntax bool   `json:"is_valid_syntax"`
}

// Common disposable domains (extended list)
var disposableDomains = map[string]bool{
	"mailinator.com": true, "yopmail.com": true, "guerrillamail.com": true,
	"temp-mail.org": true, "10minutemail.com": true, "sharklasers.com": true,
	"tempmail.com": true, "throwaway.email": true, "fakeinbox.com": true,
	"trashmail.com": true, "mailnesia.com": true, "dispostable.com": true,
	"getnada.com": true, "mohmal.com": true, "emailondeck.com": true,
}

// Common role accounts
var roleAccounts = map[string]bool{
	"admin": true, "support": true, "info": true, "sales": true, "contact": true,
	"webmaster": true, "postmaster": true, "hostmaster": true, "abuse": true,
	"noreply": true, "no-reply": true, "mailer-daemon": true, "help": true,
	"billing": true, "marketing": true, "hr": true, "jobs": true, "careers": true,
}

// Domains that block direct SMTP verification (use Reacher for these)
var hardToVerifyDomains = map[string]bool{
	// Google
	"gmail.com": true, "googlemail.com": true, "google.com": true,
	// Microsoft
	"outlook.com": true, "hotmail.com": true, "live.com": true, "msn.com": true,
	"outlook.co.uk": true, "hotmail.co.uk": true, "live.co.uk": true,
	// Yahoo
	"yahoo.com": true, "yahoo.co.uk": true, "yahoo.fr": true, "yahoo.de": true,
	"ymail.com": true, "rocketmail.com": true,
	// Apple
	"icloud.com": true, "me.com": true, "mac.com": true,
	// AOL
	"aol.com": true, "aim.com": true,
	// Proton
	"protonmail.com": true, "proton.me": true, "pm.me": true,
	// Others that commonly block
	"comcast.net": true, "verizon.net": true, "att.net": true,
}

// VerifierOptions configures the check
type VerifierOptions struct {
	SenderEmail string
	HeloHost    string
	SourceIPs   []string // List of local IPs to rotate (WARNING: these should NOT be your sending IPs!)
	ProxyURLs   []string // List of proxies to rotate (RECOMMENDED for verification)

	// Reacher Configuration (choose one method)
	ReacherURL     string // HTTP API URL (e.g., "http://localhost:8080")
	ReacherAPIKey  string // API key for hosted Reacher
	ReacherBinPath string // Path to check_if_email_exists binary (e.g., "/usr/local/bin/check_if_email_exists")

	UseReacherOnly   bool // If true, skip local SMTP checks entirely
	SkipCatchAllTest bool // If true, skip the catch-all probe (safer for reputation)
	RequireProxy     bool // If true, only verify via proxy (protects sending IP reputation)
}

// ReacherRequest is the request body for Reacher HTTP API
type ReacherRequest struct {
	ToEmail     string `json:"to_email"`
	FromEmail   string `json:"from_email,omitempty"`
	HelloName   string `json:"hello_name,omitempty"`
	ProxyHost   string `json:"proxy_host,omitempty"`
	ProxyPort   int    `json:"proxy_port,omitempty"`
	SmtpTimeout string `json:"smtp_timeout,omitempty"`
}

// ReacherResponse is the response from Reacher (both API and binary output the same JSON)
type ReacherResponse struct {
	Input       string `json:"input"`
	IsReachable string `json:"is_reachable"`
	Misc        struct {
		IsDisposable  bool   `json:"is_disposable"`
		IsRoleAccount bool   `json:"is_role_account"`
		GravatarUrl   string `json:"gravatar_url,omitempty"`
	} `json:"misc"`
	MX struct {
		AcceptsMail bool     `json:"accepts_mail"`
		Records     []string `json:"records"`
	} `json:"mx"`
	SMTP struct {
		CanConnectSmtp bool `json:"can_connect_smtp"`
		HasFullInbox   bool `json:"has_full_inbox"`
		IsCatchAll     bool `json:"is_catch_all"`
		IsDeliverable  bool `json:"is_deliverable"`
		IsDisabled     bool `json:"is_disabled"`
	} `json:"smtp"`
	Syntax struct {
		Domain        string `json:"domain"`
		IsValidSyntax bool   `json:"is_valid_syntax"`
		Username      string `json:"username"`
	} `json:"syntax"`
}

// VerifyEmail performs robust checks with Reacher fallback for hard-to-verify domains
func VerifyEmail(email string, opts VerifierOptions) EmailVerificationResult {
	res := EmailVerificationResult{Input: email}

	// 1. Syntax Check
	parts := strings.Split(email, "@")
	if len(parts) != 2 || !strings.Contains(parts[1], ".") {
		res.IsReachable = "invalid"
		res.Syntax.IsValidSyntax = false
		res.Error = "Invalid syntax"
		return res
	}
	res.Syntax.IsValidSyntax = true
	res.Syntax.Username = parts[0]
	res.Syntax.Domain = strings.ToLower(parts[1])

	// Misc Checks
	res.Misc.IsDisposable = disposableDomains[res.Syntax.Domain]
	res.Misc.IsRoleAccount = roleAccounts[strings.ToLower(parts[0])]

	// If disposable, mark as risky immediately
	if res.Misc.IsDisposable {
		res.IsReachable = "risky"
		res.RiskScore = 80
		res.Log = "Disposable email domain detected. "
	}

	// 2. MX Record Lookup
	mxs, err := net.LookupMX(res.Syntax.Domain)
	if err != nil || len(mxs) == 0 {
		res.IsReachable = "invalid"
		res.MX.AcceptsMail = false
		res.Error = "No MX records found"
		return res
	}

	res.MX.AcceptsMail = true
	for _, mx := range mxs {
		res.MX.Records = append(res.MX.Records, mx.Host)
	}

	// 3. Decide verification method
	hasReacher := opts.ReacherURL != "" || opts.ReacherBinPath != ""
	useReacher := opts.UseReacherOnly || (hasReacher && hardToVerifyDomains[res.Syntax.Domain])

	if useReacher && hasReacher {
		res.Log += "Using Reacher for verification. "
		return verifyWithReacher(email, opts, res)
	}

	// 4. Local SMTP Verification
	res.Log += "Using local SMTP verification. "
	return verifyWithLocalSMTP(email, opts, res, mxs[0].Host)
}

// verifyWithReacher uses either the binary or HTTP API
func verifyWithReacher(email string, opts VerifierOptions, res EmailVerificationResult) EmailVerificationResult {
	// Prefer binary if available (no port conflicts, faster)
	if opts.ReacherBinPath != "" {
		return verifyWithReacherBinary(email, opts, res)
	}

	// Fall back to HTTP API
	return verifyWithReacherAPI(email, opts, res)
}

// verifyWithReacherBinary calls the check_if_email_exists binary directly
func verifyWithReacherBinary(email string, opts VerifierOptions, res EmailVerificationResult) EmailVerificationResult {
	res.Source = "reacher_bin"

	// Build command arguments
	args := []string{email}

	if opts.SenderEmail != "" {
		args = append(args, "--from-email", opts.SenderEmail)
	}
	if opts.HeloHost != "" {
		args = append(args, "--hello-name", opts.HeloHost)
	}

	// Execute binary with timeout
	cmd := exec.Command(opts.ReacherBinPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run with timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		if err != nil {
			res.Log += fmt.Sprintf("Reacher binary error: %v (%s). ", err, stderr.String())
			// Fallback to local SMTP
			if !opts.UseReacherOnly {
				mxs, _ := net.LookupMX(res.Syntax.Domain)
				if len(mxs) > 0 {
					return verifyWithLocalSMTP(email, opts, res, mxs[0].Host)
				}
			}
			res.IsReachable = "unknown"
			res.Error = "Reacher binary failed"
			return res
		}
	case <-time.After(30 * time.Second):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		res.Log += "Reacher binary timed out. "
		res.IsReachable = "unknown"
		res.Error = "Reacher timeout"
		return res
	}

	// Parse JSON output
	var reacherResp ReacherResponse
	if err := json.Unmarshal(stdout.Bytes(), &reacherResp); err != nil {
		res.Log += fmt.Sprintf("Failed to parse Reacher output: %v. ", err)
		res.IsReachable = "unknown"
		res.Error = "Invalid Reacher response"
		return res
	}

	return mapReacherResponse(reacherResp, res)
}

// verifyWithReacherAPI calls the Reacher HTTP backend
func verifyWithReacherAPI(email string, opts VerifierOptions, res EmailVerificationResult) EmailVerificationResult {
	res.Source = "reacher_api"

	reqBody := ReacherRequest{
		ToEmail:     email,
		FromEmail:   opts.SenderEmail,
		HelloName:   opts.HeloHost,
		SmtpTimeout: "10s",
	}

	if reqBody.FromEmail == "" {
		reqBody.FromEmail = "verify@sampmail.local"
	}
	if reqBody.HelloName == "" {
		reqBody.HelloName = "sampmail.local"
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		res.Error = fmt.Sprintf("Failed to create request: %v", err)
		res.IsReachable = "unknown"
		return res
	}

	// Determine endpoint
	endpoint := strings.TrimSuffix(opts.ReacherURL, "/") + "/v0/check_email"

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		res.Error = fmt.Sprintf("Failed to create HTTP request: %v", err)
		res.IsReachable = "unknown"
		return res
	}

	req.Header.Set("Content-Type", "application/json")

	// Add API key if provided (for hosted Reacher)
	if opts.ReacherAPIKey != "" {
		req.Header.Set("Authorization", opts.ReacherAPIKey)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		res.Log += fmt.Sprintf("Reacher API request failed: %v. ", err)
		// Fallback to local SMTP if API fails
		if !opts.UseReacherOnly {
			res.Log += "Falling back to local SMTP. "
			mxs, _ := net.LookupMX(res.Syntax.Domain)
			if len(mxs) > 0 {
				return verifyWithLocalSMTP(email, opts, res, mxs[0].Host)
			}
		}
		res.Error = "Reacher API request failed"
		res.IsReachable = "unknown"
		return res
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		res.Error = fmt.Sprintf("Failed to read response: %v", err)
		res.IsReachable = "unknown"
		return res
	}

	if resp.StatusCode != 200 {
		res.Log += fmt.Sprintf("Reacher API returned status %d. ", resp.StatusCode)
		if !opts.UseReacherOnly {
			mxs, _ := net.LookupMX(res.Syntax.Domain)
			if len(mxs) > 0 {
				return verifyWithLocalSMTP(email, opts, res, mxs[0].Host)
			}
		}
		res.Error = fmt.Sprintf("Reacher API returned status %d", resp.StatusCode)
		res.IsReachable = "unknown"
		return res
	}

	var reacherResp ReacherResponse
	if err := json.Unmarshal(body, &reacherResp); err != nil {
		res.Error = fmt.Sprintf("Failed to parse Reacher response: %v", err)
		res.IsReachable = "unknown"
		return res
	}

	return mapReacherResponse(reacherResp, res)
}

// mapReacherResponse converts Reacher output to our result format
func mapReacherResponse(reacherResp ReacherResponse, res EmailVerificationResult) EmailVerificationResult {
	res.IsReachable = reacherResp.IsReachable
	res.Misc.IsDisposable = reacherResp.Misc.IsDisposable
	res.Misc.IsRoleAccount = reacherResp.Misc.IsRoleAccount
	res.SMTP.CanConnect = reacherResp.SMTP.CanConnectSmtp
	res.SMTP.IsCatchAll = reacherResp.SMTP.IsCatchAll
	res.SMTP.IsDeliverable = reacherResp.SMTP.IsDeliverable

	// Update MX if Reacher provided them
	if len(reacherResp.MX.Records) > 0 {
		res.MX.Records = reacherResp.MX.Records
		res.MX.AcceptsMail = reacherResp.MX.AcceptsMail
	}

	// Calculate risk score
	switch res.IsReachable {
	case "safe":
		res.RiskScore = 0
	case "risky":
		res.RiskScore = 50
	case "invalid":
		res.RiskScore = 100
	case "unknown":
		res.RiskScore = 50
	}

	if res.Misc.IsDisposable {
		res.RiskScore = max(res.RiskScore, 80)
	}
	if res.SMTP.IsCatchAll {
		res.RiskScore = max(res.RiskScore, 40)
	}

	res.Log += fmt.Sprintf("Reacher result: %s. ", res.IsReachable)
	return res
}

// verifyWithLocalSMTP performs direct SMTP verification
// IMPORTANT: For reputation safety, we prefer proxies over source IPs
func verifyWithLocalSMTP(email string, opts VerifierOptions, res EmailVerificationResult, mxHost string) EmailVerificationResult {
	res.Source = "local"
	mxHost = strings.TrimSuffix(mxHost, ".")

	// Prepare list of Dialers - PROXIES FIRST (safer for reputation!)
	dialers := make([]func(network, addr string) (net.Conn, error), 0)
	usingProxy := make([]bool, 0) // Track which dialers are proxies

	// A. Add Proxies FIRST (preferred for reputation protection)
	for _, pURL := range opts.ProxyURLs {
		proxyURL := pURL // capture closure
		dialers = append(dialers, func(network, addr string) (net.Conn, error) {
			u, err := url.Parse(proxyURL)
			if err != nil {
				return nil, err
			}
			d, err := proxy.FromURL(u, proxy.Direct)
			if err != nil {
				return nil, err
			}
			return d.Dial(network, addr)
		})
		usingProxy = append(usingProxy, true)
	}

	// B. If RequireProxy is set and no proxies available, fail safely
	if opts.RequireProxy && len(opts.ProxyURLs) == 0 {
		res.Log += "RequireProxy set but no proxies configured. "
		res.IsReachable = "unknown"
		res.RiskScore = 50
		res.Error = "No proxy available for verification"
		return res
	}

	// C. Only add Source IPs if RequireProxy is NOT set
	if !opts.RequireProxy {
		for _, ip := range opts.SourceIPs {
			localIP := ip // capture closure
			dialers = append(dialers, func(network, addr string) (net.Conn, error) {
				localAddr, err := net.ResolveTCPAddr("tcp", localIP+":0")
				if err != nil {
					return nil, err
				}
				d := net.Dialer{LocalAddr: localAddr, Timeout: 10 * time.Second}
				return d.Dial(network, addr)
			})
			usingProxy = append(usingProxy, false)
		}

		// D. Add Default Interface as last resort
		if len(dialers) == 0 {
			dialers = append(dialers, func(network, addr string) (net.Conn, error) {
				return net.DialTimeout(network, addr, 10*time.Second)
			})
			usingProxy = append(usingProxy, false)
		}
	}

	// Try each dialer
	for i, dial := range dialers {
		res.Log += fmt.Sprintf("[Attempt %d] ", i+1)

		// Only do catch-all check via proxy to protect sending IP reputation
		allowCatchAll := false
		if i < len(usingProxy) && usingProxy[i] && !opts.SkipCatchAllTest {
			allowCatchAll = true
		}

		result := performSMTPCheck(dial, mxHost, email, opts, allowCatchAll)

		res.SMTP.CanConnect = (result.Error == "" || strings.Contains(result.Error, "550") || strings.Contains(result.Error, "RCPT"))
		res.SMTP.IsCatchAll = result.IsCatchAll

		if result.Error == "" {
			if result.IsCatchAll {
				res.IsReachable = "risky"
				res.SMTP.IsDeliverable = true
				res.RiskScore = 50
			} else {
				res.IsReachable = "safe"
				res.SMTP.IsDeliverable = true
				res.RiskScore = 0
			}
			return res
		}

		if strings.Contains(result.Error, "550") || strings.Contains(result.Error, "User unknown") ||
			strings.Contains(result.Error, "does not exist") || strings.Contains(result.Error, "Recipient rejected") {
			res.IsReachable = "invalid"
			res.SMTP.IsDeliverable = false
			res.RiskScore = 100
			return res
		}

		res.Log += fmt.Sprintf("Failed (%s). ", result.Error)
	}

	// If all local attempts failed and we have Reacher, try it
	hasReacher := opts.ReacherURL != "" || opts.ReacherBinPath != ""
	if hasReacher && !opts.UseReacherOnly {
		res.Log += "Local SMTP failed, trying Reacher. "
		return verifyWithReacher(email, opts, res)
	}

	res.IsReachable = "unknown"
	res.RiskScore = 50
	res.Error = "All connection attempts failed"
	return res
}

// VerifyEmailBatch verifies multiple emails efficiently
func VerifyEmailBatch(emails []string, opts VerifierOptions, concurrency int) []EmailVerificationResult {
	if concurrency <= 0 {
		concurrency = 5
	}

	results := make([]EmailVerificationResult, len(emails))
	sem := make(chan struct{}, concurrency)
	done := make(chan struct{})

	for i, email := range emails {
		go func(idx int, e string) {
			sem <- struct{}{} // Acquire
			results[idx] = VerifyEmail(e, opts)
			<-sem // Release
			done <- struct{}{}
		}(i, email)
	}

	for range emails {
		<-done
	}

	return results
}

type smtpCheckResult struct {
	IsCatchAll bool
	Error      string
}

func performSMTPCheck(dial func(network, addr string) (net.Conn, error), host, email string, opts VerifierOptions, allowCatchAllCheck bool) smtpCheckResult {
	conn, err := dial("tcp", fmt.Sprintf("%s:25", host))
	if err != nil {
		return smtpCheckResult{Error: fmt.Sprintf("Connect error: %v", err)}
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return smtpCheckResult{Error: fmt.Sprintf("Client error: %v", err)}
	}
	defer client.Quit()

	helo := opts.HeloHost
	if helo == "" {
		helo = "check.kumomta.local"
	}

	if err := client.Hello(helo); err != nil {
		return smtpCheckResult{Error: fmt.Sprintf("HELO error: %v", err)}
	}

	sender := opts.SenderEmail
	if sender == "" {
		sender = fmt.Sprintf("verifier@%s", helo)
	}

	if err := client.Mail(sender); err != nil {
		return smtpCheckResult{Error: fmt.Sprintf("MAIL FROM error: %v", err)}
	}

	// Catch-All Check - ONLY if allowed (via proxy) to protect sending IP reputation
	// Sending random emails from your marketing IPs is a Directory Harvesting Attack signature!
	if allowCatchAllCheck {
		randomLocal := fmt.Sprintf("random-%d", time.Now().UnixNano())
		domain := strings.Split(email, "@")[1]
		randomEmail := fmt.Sprintf("%s@%s", randomLocal, domain)

		err = client.Rcpt(randomEmail)
		if err == nil {
			return smtpCheckResult{IsCatchAll: true, Error: ""}
		}
	}

	// Real Email Check
	if err := client.Rcpt(email); err != nil {
		return smtpCheckResult{Error: fmt.Sprintf("RCPT TO error: %v", err)}
	}

	return smtpCheckResult{IsCatchAll: false, Error: ""}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
