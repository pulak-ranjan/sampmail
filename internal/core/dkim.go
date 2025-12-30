package core

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pulak-ranjan/sampmail/internal/config"
	"github.com/pulak-ranjan/sampmail/internal/logger"
	"github.com/pulak-ranjan/sampmail/internal/models"
)

// dkimKeyPaths returns the paths for DKIM keys using config
func dkimKeyPaths(domain, selector string) (privPath, pubPath, dir string) {
	cfg := config.Get()
	dir = filepath.Join(cfg.DKIMPath(), domain)
	privPath = filepath.Join(dir, selector+".key")
	pubPath = filepath.Join(dir, selector+".pub")
	return
}

// DKIMKeyExists checks if DKIM key exists
func DKIMKeyExists(domain, selector string) bool {
	privPath, _, _ := dkimKeyPaths(domain, selector)
	_, err := os.Stat(privPath)
	return err == nil
}

// GenerateDKIMKey creates a new RSA keypair
// SECURITY FIX: Removed chown - application should run as the correct user
// The application must be started with appropriate permissions to write DKIM keys
func GenerateDKIMKey(domain, selector string) error {
	log := logger.WithComponent("dkim")

	// Validate inputs to prevent path traversal
	if !isValidDomain(domain) {
		return fmt.Errorf("invalid domain name: %s", domain)
	}
	if !isValidSelector(selector) {
		return fmt.Errorf("invalid selector: %s", selector)
	}

	privPath, pubPath, dir := dkimKeyPaths(domain, selector)

	// Create directory with restrictive permissions
	// The process must run as a user with write access to this directory
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("mkdir dkim dir: %w", err)
	}

	// Generate RSA key
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("generate rsa key: %w", err)
	}

	// Encode private key
	privBytes := x509.MarshalPKCS1PrivateKey(privKey)
	privPem := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privBytes,
	})

	// Encode public key
	pubBytes, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		return fmt.Errorf("marshal public key: %w", err)
	}
	pubPem := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})

	// Write private key with secure permissions (owner read/write only)
	if err := os.WriteFile(privPath, privPem, 0600); err != nil {
		return fmt.Errorf("write private key: %w", err)
	}

	// Write public key (readable)
	if err := os.WriteFile(pubPath, pubPem, 0644); err != nil {
		return fmt.Errorf("write public key: %w", err)
	}

	log.Info("DKIM keys generated",
		"domain", domain,
		"selector", selector,
		"key_size", 2048)

	return nil
}

// isValidDomain checks that domain doesn't contain path traversal chars
func isValidDomain(domain string) bool {
	if domain == "" || len(domain) > 253 {
		return false
	}
	// Must not contain path separators or parent directory references
	if strings.Contains(domain, "/") ||
		strings.Contains(domain, "\\") ||
		strings.Contains(domain, "..") {
		return false
	}
	// Basic domain format check
	for _, c := range domain {
		if !((c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '.' || c == '-') {
			return false
		}
	}
	return true
}

// isValidSelector checks that selector is safe
func isValidSelector(selector string) bool {
	if selector == "" || len(selector) > 63 {
		return false
	}
	// Must not contain path separators
	if strings.Contains(selector, "/") ||
		strings.Contains(selector, "\\") ||
		strings.Contains(selector, "..") {
		return false
	}
	// Alphanumeric and hyphens only
	for _, c := range selector {
		if !((c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// DKIMDNSRecord represents a single DNS TXT record for DKIM
type DKIMDNSRecord struct {
	Domain   string `json:"domain"`
	Selector string `json:"selector"`
	DNSName  string `json:"dns_name"`
	DNSValue string `json:"dns_value"`
}

// ListDKIMDNSRecords iterates over all domains+senders
// SECURITY FIX: Validates domain and selector from DB before file operations
// to prevent path traversal if database was polluted with malicious data
func ListDKIMDNSRecords(snap *Snapshot) ([]DKIMDNSRecord, error) {
	var records []DKIMDNSRecord
	log := logger.WithComponent("dkim")

	for _, d := range snap.Domains {
		// SECURITY: Validate domain from database before using in file paths
		if !isValidDomain(d.Name) {
			log.Warn("skipping domain with invalid name from database",
				"domain", d.Name,
				"reason", "failed validation - possible DB pollution")
			continue
		}

		for _, s := range d.Senders {
			selector := s.LocalPart
			if selector == "" {
				continue
			}

			// SECURITY: Validate selector from database before using in file paths
			if !isValidSelector(selector) {
				log.Warn("skipping sender with invalid localpart from database",
					"domain", d.Name,
					"localpart", selector,
					"reason", "failed validation - possible DB pollution")
				continue
			}

			_, pubPath, _ := dkimKeyPaths(d.Name, selector)
			
			// Additional safety: verify the resolved path is under the expected directory
			cfg := config.Get()
			expectedBase := cfg.DKIMPath()
			cleanPath := filepath.Clean(pubPath)
			if !strings.HasPrefix(cleanPath, expectedBase) {
				log.Warn("path traversal attempt blocked",
					"domain", d.Name,
					"selector", selector,
					"resolved_path", cleanPath)
				continue
			}
			
			data, err := os.ReadFile(pubPath)
			if err != nil {
				continue
			}

			pubBase64 := extractPEMBase64(string(data))
			if pubBase64 == "" {
				continue
			}

			name := fmt.Sprintf("%s._domainkey.%s", selector, d.Name)
			value := fmt.Sprintf("v=DKIM1; k=rsa; p=%s", pubBase64)

			records = append(records, DKIMDNSRecord{
				Domain:   d.Name,
				Selector: selector,
				DNSName:  name,
				DNSValue: value,
			})
		}
	}

	return records, nil
}

func extractPEMBase64(pemStr string) string {
	lines := strings.Split(pemStr, "\n")
	var b strings.Builder
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "-----") {
			continue
		}
		b.WriteString(line)
	}
	return b.String()
}

func GenerateDKIMForDomainAllSenders(domain models.Domain) error {
	for _, s := range domain.Senders {
		if s.LocalPart == "" {
			continue
		}
		if err := GenerateDKIMKey(domain.Name, s.LocalPart); err != nil {
			return fmt.Errorf("generate dkim for %s/%s: %w", domain.Name, s.LocalPart, err)
		}
	}
	return nil
}
