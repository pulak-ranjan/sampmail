package api

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// SSLHandler handles SSL certificate management
type SSLHandler struct{}

// NewSSLHandler creates a new SSL handler
func NewSSLHandler() *SSLHandler {
	return &SSLHandler{}
}

// SSLStatus represents the current SSL status
type SSLStatus struct {
	Enabled         bool   `json:"enabled"`
	Domain          string `json:"domain,omitempty"`
	ExpiryDate      string `json:"expiry_date,omitempty"`
	DaysUntilExpiry int    `json:"days_until_expiry,omitempty"`
	Issuer          string `json:"issuer,omitempty"`
}

// SSLInstallRequest represents a request to install SSL
type SSLInstallRequest struct {
	Domain string `json:"domain"`
	Email  string `json:"email"`
}

// HandleGetSSLStatus returns the current SSL status
func (h *SSLHandler) HandleGetSSLStatus(w http.ResponseWriter, r *http.Request) {
	status := SSLStatus{Enabled: false}

	// Check nginx config for SSL
	nginxConf := "/etc/nginx/conf.d/sampmail.conf"
	if _, err := os.Stat("/etc/nginx/sites-enabled/sampmail"); err == nil {
		nginxConf = "/etc/nginx/sites-enabled/sampmail"
	}

	confData, err := ioutil.ReadFile(nginxConf)
	if err == nil {
		conf := string(confData)
		// Check if SSL is configured
		if strings.Contains(conf, "ssl_certificate") || strings.Contains(conf, "listen 443") {
			status.Enabled = true

			// Extract domain from server_name
			for _, line := range strings.Split(conf, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "server_name") {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						domain := strings.TrimSuffix(parts[1], ";")
						if domain != "_" && domain != "" {
							status.Domain = domain
							break
						}
					}
				}
			}

			// Try to get certificate info
			if status.Domain != "" {
				certPath := fmt.Sprintf("/etc/letsencrypt/live/%s/fullchain.pem", status.Domain)
				if certInfo, err := getCertificateInfo(certPath); err == nil {
					status.ExpiryDate = certInfo.ExpiryDate
					status.DaysUntilExpiry = certInfo.DaysUntilExpiry
					status.Issuer = certInfo.Issuer
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// HandleInstallSSL installs a Let's Encrypt SSL certificate
func (h *SSLHandler) HandleInstallSSL(w http.ResponseWriter, r *http.Request) {
	var req SSLInstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request"}`, http.StatusBadRequest)
		return
	}

	if req.Domain == "" || req.Email == "" {
		http.Error(w, `{"error": "Domain and email are required"}`, http.StatusBadRequest)
		return
	}

	// Validate domain format
	if strings.Contains(req.Domain, " ") || strings.Contains(req.Domain, "/") {
		http.Error(w, `{"error": "Invalid domain format"}`, http.StatusBadRequest)
		return
	}

	// First, update nginx config with the domain
	if err := updateNginxDomain(req.Domain); err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Failed to update nginx config: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Reload nginx to apply domain change
	exec.Command("systemctl", "reload", "nginx").Run()

	// Run certbot
	cmd := exec.Command("certbot", "--nginx",
		"-d", req.Domain,
		"--non-interactive",
		"--agree-tos",
		"-m", req.Email,
		"--redirect",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try to provide helpful error message
		outputStr := string(output)
		if strings.Contains(outputStr, "DNS problem") {
			http.Error(w, `{"error": "DNS not configured. Make sure your domain points to this server."}`, http.StatusBadRequest)
		} else if strings.Contains(outputStr, "too many certificates") {
			http.Error(w, `{"error": "Rate limit reached. Try again later or use a different domain."}`, http.StatusTooManyRequests)
		} else {
			http.Error(w, fmt.Sprintf(`{"error": "Certbot failed: %s"}`, strings.TrimSpace(outputStr)), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "SSL certificate installed successfully",
	})
}

// HandleRenewSSL renews SSL certificates
func (h *SSLHandler) HandleRenewSSL(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("certbot", "renew", "--nginx")
	output, err := cmd.CombinedOutput()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": "Renewal failed: %s"}`, string(output)), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "SSL certificates renewed",
	})
}

// CertificateInfo contains certificate details
type CertificateInfo struct {
	ExpiryDate      string
	DaysUntilExpiry int
	Issuer          string
}

func getCertificateInfo(certPath string) (*CertificateInfo, error) {
	data, err := ioutil.ReadFile(certPath)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to parse certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}

	daysUntilExpiry := int(time.Until(cert.NotAfter).Hours() / 24)

	return &CertificateInfo{
		ExpiryDate:      cert.NotAfter.Format("2006-01-02"),
		DaysUntilExpiry: daysUntilExpiry,
		Issuer:          cert.Issuer.Organization[0],
	}, nil
}

func updateNginxDomain(domain string) error {
	// Try Rocky/RHEL path first
	confPath := "/etc/nginx/conf.d/sampmail.conf"
	if _, err := os.Stat("/etc/nginx/sites-available/sampmail"); err == nil {
		confPath = "/etc/nginx/sites-available/sampmail"
	}

	data, err := ioutil.ReadFile(confPath)
	if err != nil {
		return err
	}

	conf := string(data)

	// Replace server_name
	lines := strings.Split(conf, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "server_name") {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = indent + "server_name " + domain + ";"
			break
		}
	}

	newConf := strings.Join(lines, "\n")

	// Create backup
	backupPath := confPath + ".bak"
	ioutil.WriteFile(backupPath, data, 0644)

	// Write new config
	return ioutil.WriteFile(confPath, []byte(newConf), 0644)
}
