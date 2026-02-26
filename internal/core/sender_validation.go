package core

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/pulak-ranjan/sampmail/internal/logger"
	"github.com/pulak-ranjan/sampmail/internal/models"
)

// =====================================
// PRE-SEND DNS VALIDATION
// =====================================
// Validates sender's DNS configuration before sending campaigns
// to prevent emails from going to spam due to missing/invalid DNS records

// ValidationResult holds the result of DNS validation
type ValidationResult struct {
	Domain      string             `json:"domain"`
	SenderEmail string             `json:"sender_email"`
	Valid       bool               `json:"valid"`
	Errors      []ValidationError  `json:"errors,omitempty"`
	Warnings    []ValidationError  `json:"warnings,omitempty"`
	CheckedAt   time.Time          `json:"checked_at"`
	Details     ValidationDetails  `json:"details"`
}

// ValidationError represents a validation error or warning
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// ValidationDetails holds detailed DNS record information
type ValidationDetails struct {
	SPF struct {
		Found    bool   `json:"found"`
		Record   string `json:"record,omitempty"`
		Valid    bool   `json:"valid"`
		AllMech  string `json:"all_mech,omitempty"` // -all, ~all, ?all, +all
	} `json:"spf"`
	
	DKIM struct {
		Found    bool   `json:"found"`
		Selector string `json:"selector,omitempty"`
		Record   string `json:"record,omitempty"`
		Valid    bool   `json:"valid"`
		KeySize  int    `json:"key_size,omitempty"`
	} `json:"dkim"`
	
	DMARC struct {
		Found   bool   `json:"found"`
		Record  string `json:"record,omitempty"`
		Policy  string `json:"policy,omitempty"` // none, quarantine, reject
		Pct     int    `json:"pct,omitempty"`
		RUA     string `json:"rua,omitempty"` // Aggregate report URI
		RUF     string `json:"ruf,omitempty"` // Forensic report URI
		Valid   bool   `json:"valid"`
	} `json:"dmarc"`
	
	MX struct {
		Found    bool     `json:"found"`
		Records  []string `json:"records,omitempty"`
		Valid    bool     `json:"valid"`
	} `json:"mx"`
	
	A struct {
		Found   bool     `json:"found"`
		Records []string `json:"records,omitempty"`
	} `json:"a"`
	
	ReverseDNS struct {
		Found    bool   `json:"found"`
		Hostname string `json:"hostname,omitempty"`
		Matches  bool   `json:"matches"`
	} `json:"reverse_dns"`
}

// SenderValidator validates sender DNS configuration
type SenderValidator struct {
	dnsClient *dns.Client
	dnsServer string
	timeout   time.Duration
}

// NewSenderValidator creates a new sender validator
func NewSenderValidator() *SenderValidator {
	return &SenderValidator{
		dnsClient: &dns.Client{
			Timeout: 5 * time.Second,
		},
		dnsServer: "8.8.8.8:53", // Google DNS
		timeout:   10 * time.Second,
	}
}

// SetDNSServer sets a custom DNS server
func (sv *SenderValidator) SetDNSServer(server string) {
	sv.dnsServer = server
}

// ValidateSender performs full DNS validation for a sender
func (sv *SenderValidator) ValidateSender(ctx context.Context, sender *models.Sender) (*ValidationResult, error) {
	if sender == nil {
		return nil, fmt.Errorf("sender is nil")
	}
	
	domain := sender.Domain.Name
	if domain == "" {
		return nil, fmt.Errorf("sender has no domain")
	}
	
	result := &ValidationResult{
		Domain:      domain,
		SenderEmail: sender.Email,
		Valid:       true,
		CheckedAt:   time.Now(),
	}
	
	// Run all validations
	sv.validateSPF(domain, result)
	sv.validateDKIM(domain, "k1", result) // Use default selector k1
	sv.validateDMARC(domain, result)
	sv.validateMX(domain, result)
	sv.validateA(domain, result)
	
	// Determine overall validity
	result.Valid = len(result.Errors) == 0
	
	return result, nil
}

// ValidateDomain performs DNS validation for a domain
func (sv *SenderValidator) ValidateDomain(ctx context.Context, domain string) (*ValidationResult, error) {
	result := &ValidationResult{
		Domain:    domain,
		Valid:     true,
		CheckedAt: time.Now(),
	}
	
	sv.validateSPF(domain, result)
	sv.validateDKIM(domain, "k1", result) // Default selector
	sv.validateDMARC(domain, result)
	sv.validateMX(domain, result)
	sv.validateA(domain, result)
	
	result.Valid = len(result.Errors) == 0
	
	return result, nil
}

// validateSPF checks SPF record
func (sv *SenderValidator) validateSPF(domain string, result *ValidationResult) {
	records, err := sv.queryDNS(domain, dns.TypeTXT)
	if err != nil {
		result.Details.SPF.Found = false
		result.Details.SPF.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "spf",
			Message: "No SPF record found",
			Code:    "SPF_MISSING",
		})
		return
	}
	
	// Find SPF record
	var spfRecord string
	for _, r := range records {
		if strings.HasPrefix(r, "v=spf1") {
			spfRecord = r
			break
		}
	}
	
	if spfRecord == "" {
		result.Details.SPF.Found = false
		result.Details.SPF.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "spf",
			Message: "No SPF record found",
			Code:    "SPF_MISSING",
		})
		return
	}
	
	result.Details.SPF.Found = true
	result.Details.SPF.Record = spfRecord
	
	// Check the "all" mechanism
	if strings.Contains(spfRecord, " -all") {
		result.Details.SPF.AllMech = "-all"
		result.Details.SPF.Valid = true
	} else if strings.Contains(spfRecord, " ~all") {
		result.Details.SPF.AllMech = "~all"
		result.Details.SPF.Valid = true
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "spf",
			Message: "SPF uses ~all (soft fail) instead of -all (hard fail)",
			Code:    "SPF_SOFT_FAIL",
		})
	} else if strings.Contains(spfRecord, " ?all") {
		result.Details.SPF.AllMech = "?all"
		result.Details.SPF.Valid = false
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "spf",
			Message: "SPF uses ?all (neutral) - provides no protection",
			Code:    "SPF_NEUTRAL",
		})
	} else if strings.Contains(spfRecord, " +all") || !strings.Contains(spfRecord, "all") {
		result.Details.SPF.AllMech = "+all"
		result.Details.SPF.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "spf",
			Message: "SPF allows all senders (+all or missing all) - provides no protection",
			Code:    "SPF_ALLOW_ALL",
		})
	} else {
		result.Details.SPF.Valid = true
	}
}

// validateDKIM checks DKIM record
func (sv *SenderValidator) validateDKIM(domain, selector string, result *ValidationResult) {
	if selector == "" {
		selector = "k1" // Default selector
	}
	
	dkimDomain := fmt.Sprintf("%s._domainkey.%s", selector, domain)
	records, err := sv.queryDNS(dkimDomain, dns.TypeTXT)
	if err != nil {
		result.Details.DKIM.Found = false
		result.Details.DKIM.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "dkim",
			Message: fmt.Sprintf("No DKIM record found for selector '%s'", selector),
			Code:    "DKIM_MISSING",
		})
		return
	}
	
	// Find DKIM record
	var dkimRecord string
	for _, r := range records {
		if strings.HasPrefix(r, "v=DKIM1") || strings.Contains(r, "p=") {
			dkimRecord = r
			break
		}
	}
	
	if dkimRecord == "" {
		result.Details.DKIM.Found = false
		result.Details.DKIM.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "dkim",
			Message: fmt.Sprintf("No DKIM record found for selector '%s'", selector),
			Code:    "DKIM_MISSING",
		})
		return
	}
	
	result.Details.DKIM.Found = true
	result.Details.DKIM.Selector = selector
	result.Details.DKIM.Record = dkimRecord
	
	// Check key strength
	if strings.Contains(dkimRecord, "p=") {
		// Extract public key
		parts := strings.Split(dkimRecord, "p=")
		if len(parts) > 1 {
			pubKey := strings.Split(parts[1], ";")[0]
			// Rough estimate: 2048-bit key is ~392 chars base64
			keyLen := len(strings.TrimSpace(pubKey))
			result.Details.DKIM.KeySize = keyLen * 8 // Rough estimate
			
			if keyLen < 100 {
				result.Warnings = append(result.Warnings, ValidationError{
					Field:   "dkim",
					Message: "DKIM key may be too weak (< 1024 bits)",
					Code:    "DKIM_WEAK_KEY",
				})
			}
		}
	}
	
	// Check version
	if !strings.HasPrefix(dkimRecord, "v=DKIM1") {
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "dkim",
			Message: "DKIM record should start with v=DKIM1",
			Code:    "DKIM_VERSION",
		})
	}
	
	result.Details.DKIM.Valid = true
}

// validateDMARC checks DMARC record
func (sv *SenderValidator) validateDMARC(domain string, result *ValidationResult) {
	dmarcDomain := fmt.Sprintf("_dmarc.%s", domain)
	records, err := sv.queryDNS(dmarcDomain, dns.TypeTXT)
	if err != nil {
		result.Details.DMARC.Found = false
		result.Details.DMARC.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "dmarc",
			Message: "No DMARC record found",
			Code:    "DMARC_MISSING",
		})
		return
	}
	
	// Find DMARC record
	var dmarcRecord string
	for _, r := range records {
		if strings.HasPrefix(r, "v=DMARC1") {
			dmarcRecord = r
			break
		}
	}
	
	if dmarcRecord == "" {
		result.Details.DMARC.Found = false
		result.Details.DMARC.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "dmarc",
			Message: "No DMARC record found",
			Code:    "DMARC_MISSING",
		})
		return
	}
	
	result.Details.DMARC.Found = true
	result.Details.DMARC.Record = dmarcRecord
	
	// Parse policy
	if strings.Contains(dmarcRecord, "p=reject") {
		result.Details.DMARC.Policy = "reject"
		result.Details.DMARC.Valid = true
	} else if strings.Contains(dmarcRecord, "p=quarantine") {
		result.Details.DMARC.Policy = "quarantine"
		result.Details.DMARC.Valid = true
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "dmarc",
			Message: "DMARC policy is 'quarantine' - consider upgrading to 'reject'",
			Code:    "DMARC_QUARANTINE",
		})
	} else if strings.Contains(dmarcRecord, "p=none") {
		result.Details.DMARC.Policy = "none"
		result.Details.DMARC.Valid = false
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "dmarc",
			Message: "DMARC policy is 'none' - provides no protection",
			Code:    "DMARC_NONE",
		})
	}
	
	// Parse percentage
	if strings.Contains(dmarcRecord, "pct=") {
		parts := strings.Split(dmarcRecord, "pct=")
		if len(parts) > 1 {
			pctStr := strings.Split(parts[1], ";")[0]
			pctStr = strings.TrimSpace(pctStr)
			fmt.Sscanf(pctStr, "%d", &result.Details.DMARC.Pct)
		}
	}
	
	// Parse reporting URIs
	if strings.Contains(dmarcRecord, "rua=") {
		parts := strings.Split(dmarcRecord, "rua=")
		if len(parts) > 1 {
			result.Details.DMARC.RUA = strings.Split(parts[1], ";")[0]
		}
	}
	if strings.Contains(dmarcRecord, "ruf=") {
		parts := strings.Split(dmarcRecord, "ruf=")
		if len(parts) > 1 {
			result.Details.DMARC.RUF = strings.Split(parts[1], ";")[0]
		}
	}
}

// validateMX checks MX records
func (sv *SenderValidator) validateMX(domain string, result *ValidationResult) {
	records, err := sv.queryDNS(domain, dns.TypeMX)
	if err != nil || len(records) == 0 {
		result.Details.MX.Found = false
		result.Details.MX.Valid = false
		result.Warnings = append(result.Warnings, ValidationError{
			Field:   "mx",
			Message: "No MX records found - domain may not receive email",
			Code:    "MX_MISSING",
		})
		return
	}
	
	result.Details.MX.Found = true
	result.Details.MX.Records = records
	result.Details.MX.Valid = true
}

// validateA checks A records
func (sv *SenderValidator) validateA(domain string, result *ValidationResult) {
	records, err := sv.queryDNS(domain, dns.TypeA)
	if err != nil {
		result.Details.A.Found = false
		return
	}
	
	result.Details.A.Found = true
	result.Details.A.Records = records
}

// queryDNS performs a DNS query
func (sv *SenderValidator) queryDNS(domain string, qtype uint16) ([]string, error) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(domain), qtype)
	m.RecursionDesired = true
	
	r, _, err := sv.dnsClient.Exchange(m, sv.dnsServer)
	if err != nil {
		return nil, err
	}
	
	if r.Rcode != dns.RcodeSuccess {
		return nil, fmt.Errorf("DNS query failed: %s", dns.RcodeToString[r.Rcode])
	}
	
	var results []string
	for _, ans := range r.Answer {
		switch rr := ans.(type) {
		case *dns.TXT:
			results = append(results, strings.Join(rr.Txt, ""))
		case *dns.MX:
			results = append(results, rr.Mx)
		case *dns.A:
			results = append(results, rr.A.String())
		}
	}
	
	return results, nil
}

// ValidateBeforeSend performs quick validation before sending
func (sv *SenderValidator) ValidateBeforeSend(sender *models.Sender) error {
	result, err := sv.ValidateSender(context.Background(), sender)
	if err != nil {
		return err
	}
	
	if !result.Valid {
		// Build error message
		var msgs []string
		for _, e := range result.Errors {
			msgs = append(msgs, e.Message)
		}
		return fmt.Errorf("sender validation failed: %s", strings.Join(msgs, "; "))
	}
	
	return nil
}

// QuickValidate performs a quick validation check
func QuickValidate(domain string) (*ValidationResult, error) {
	sv := NewSenderValidator()
	return sv.ValidateDomain(context.Background(), domain)
}

// =====================================
// REVERSE DNS VALIDATION
// =====================================

// ValidateReverseDNS checks if the IP has valid reverse DNS
func ValidateReverseDNS(ip string) (string, error) {
	names, err := net.LookupAddr(ip)
	if err != nil || len(names) == 0 {
		return "", fmt.Errorf("no reverse DNS for IP %s", ip)
	}
	return names[0], nil
}

// ValidateForwardConfirmedReverseDNS checks FCrDNS
func ValidateForwardConfirmedReverseDNS(ip string) (string, bool, error) {
	// Get reverse DNS
	names, err := net.LookupAddr(ip)
	if err != nil || len(names) == 0 {
		return "", false, fmt.Errorf("no reverse DNS for IP %s", ip)
	}
	
	hostname := strings.TrimSuffix(names[0], ".")
	
	// Forward lookup
	addrs, err := net.LookupHost(hostname)
	if err != nil {
		return hostname, false, nil
	}
	
	// Check if IP is in forward lookup results
	for _, addr := range addrs {
		if addr == ip {
			return hostname, true, nil
		}
	}
	
	return hostname, false, nil
}

// =====================================
// BLACKLIST CHECKING
// =====================================

// BlacklistResult holds blacklist check result
type BlacklistResult struct {
	IP        string   `json:"ip"`
	Listed    bool     `json:"listed"`
	Lists     []string `json:"lists,omitempty"`
	Delisted  []string `json:"delisted,omitempty"`
}

// Common DNS blacklists
var dnsBlacklists = []string{
	"zen.spamhaus.org",
	"bl.spamcop.net",
	"dnsbl.sorbs.net",
	"b.barracudacentral.org",
	"spam.dnsbl.sorbs.net",
}

// CheckBlacklists checks if an IP is listed in DNS blacklists
func CheckBlacklists(ip string) (*BlacklistResult, error) {
	result := &BlacklistResult{
		IP:       ip,
		Listed:   false,
		Lists:    []string{},
		Delisted: []string{},
	}
	
	// Parse IP
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ip)
	}
	
	// Reverse IP for DNSBL query
	reversedIP := reverseIP(parsedIP)
	
	// Check each blacklist
	for _, blacklist := range dnsBlacklists {
		query := fmt.Sprintf("%s.%s", reversedIP, blacklist)
		
		// Quick lookup
		_, err := net.LookupHost(query)
		if err == nil {
			// IP is listed
			result.Listed = true
			result.Lists = append(result.Lists, blacklist)
			logger.WithComponent("blacklist").Warn("IP listed in blacklist",
				"ip", ip,
				"blacklist", blacklist)
		}
	}
	
	return result, nil
}

// reverseIP reverses an IP address for DNSBL queries
func reverseIP(ip net.IP) string {
	if ip.To4() != nil {
		// IPv4
		ip = ip.To4()
		return fmt.Sprintf("%d.%d.%d.%d", ip[3], ip[2], ip[1], ip[0])
	}
	// IPv6 - not typically used for DNSBL
	return ""
}
