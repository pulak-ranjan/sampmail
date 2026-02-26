package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

// ServiceStatus represents the status of a service
type ServiceStatus struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Status      string `json:"status"` // "running", "stopped", "not_installed"
	Enabled     bool   `json:"enabled"`
	CanInstall  bool   `json:"can_install"`
}

// ServiceHandler handles service management operations
type ServiceHandler struct {
	Store *store.Store
}

// NewServiceHandler creates a new service handler
func NewServiceHandler() *ServiceHandler {
	return &ServiceHandler{}
}

// NewServiceHandlerWithStore creates a new service handler with store access
func NewServiceHandlerWithStore(st *store.Store) *ServiceHandler {
	return &ServiceHandler{Store: st}
}

// Services configuration
var serviceConfigs = map[string]struct {
	DisplayName string
	Description string
	SystemdName string
}{
	"kumomta": {
		DisplayName: "KumoMTA",
		Description: "High-performance Mail Transfer Agent for sending emails",
		SystemdName: "kumomta",
	},
	"dovecot": {
		DisplayName: "Dovecot",
		Description: "IMAP/POP3 server for bounce email handling",
		SystemdName: "dovecot",
	},
	"reacher": {
		DisplayName: "Reacher",
		Description: "Email verification service (Docker container)",
		SystemdName: "docker", // Uses Docker
	},
}

// HandleGetStatus returns status of all managed services
func (h *ServiceHandler) HandleGetStatus(w http.ResponseWriter, r *http.Request) {
	services := []ServiceStatus{}

	for name, cfg := range serviceConfigs {
		status := ServiceStatus{
			Name:        name,
			DisplayName: cfg.DisplayName,
			Description: cfg.Description,
			CanInstall:  true,
		}

		if name == "reacher" {
			// Check Docker container status
			status.Status, status.Enabled = h.getDockerContainerStatus("reacher")
		} else {
			// Check systemd service status
			status.Status, status.Enabled = h.getSystemdStatus(cfg.SystemdName)
		}

		services = append(services, status)
	}

	writeJSON(w, http.StatusOK, services)
}

// HandleInstall installs a service
func (h *ServiceHandler) HandleInstall(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	serviceName := strings.TrimPrefix(r.URL.Path, "/api/services/")
	serviceName = strings.TrimSuffix(serviceName, "/install")

	var result string
	var err error

	switch serviceName {
	case "kumomta":
		result, err = h.installKumoMTA()
	case "dovecot":
		result, err = h.installDovecot()
	case "reacher":
		result, err = h.installReacher()
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Unknown service: " + serviceName})
		return
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":  err.Error(),
			"output": result,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": fmt.Sprintf("%s installed successfully", serviceName),
		"output":  result,
	})
}

// HandleStart starts a service
func (h *ServiceHandler) HandleStart(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	serviceName := strings.TrimPrefix(r.URL.Path, "/api/services/")
	serviceName = strings.TrimSuffix(serviceName, "/start")

	var err error
	if serviceName == "reacher" {
		err = h.startDockerContainer("reacher")
	} else {
		cfg, ok := serviceConfigs[serviceName]
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Unknown service"})
			return
		}
		err = h.controlSystemd(cfg.SystemdName, "start")
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": serviceName + " started"})
}

// HandleStop stops a service
func (h *ServiceHandler) HandleStop(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	serviceName := strings.TrimPrefix(r.URL.Path, "/api/services/")
	serviceName = strings.TrimSuffix(serviceName, "/stop")

	var err error
	if serviceName == "reacher" {
		err = h.stopDockerContainer("reacher")
	} else {
		cfg, ok := serviceConfigs[serviceName]
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Unknown service"})
			return
		}
		err = h.controlSystemd(cfg.SystemdName, "stop")
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": serviceName + " stopped"})
}

// HandleRestart restarts a service
func (h *ServiceHandler) HandleRestart(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireSuperAdmin(w, r); !ok {
		return
	}

	serviceName := strings.TrimPrefix(r.URL.Path, "/api/services/")
	serviceName = strings.TrimSuffix(serviceName, "/restart")

	var err error
	if serviceName == "reacher" {
		_ = h.stopDockerContainer("reacher")
		time.Sleep(2 * time.Second)
		err = h.startDockerContainer("reacher")
	} else {
		cfg, ok := serviceConfigs[serviceName]
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Unknown service"})
			return
		}
		err = h.controlSystemd(cfg.SystemdName, "restart")
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": serviceName + " restarted"})
}

// Helper: Get systemd service status
func (h *ServiceHandler) getSystemdStatus(service string) (status string, enabled bool) {
	// Check if service is installed
	cmd := exec.Command("systemctl", "list-unit-files", service+".service")
	out, err := cmd.Output()
	if err != nil || !strings.Contains(string(out), service) {
		return "not_installed", false
	}

	// Check if running
	cmd = exec.Command("systemctl", "is-active", service)
	out, _ = cmd.Output()
	isActive := strings.TrimSpace(string(out)) == "active"

	// Check if enabled
	cmd = exec.Command("systemctl", "is-enabled", service)
	out, _ = cmd.Output()
	isEnabled := strings.TrimSpace(string(out)) == "enabled"

	if isActive {
		return "running", isEnabled
	}
	return "stopped", isEnabled
}

// Helper: Get Docker container status
func (h *ServiceHandler) getDockerContainerStatus(containerName string) (status string, enabled bool) {
	// Check if docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		return "not_installed", false
	}

	// Check if container exists
	cmd := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", containerName)
	out, err := cmd.Output()
	if err != nil {
		return "not_installed", false
	}

	if strings.TrimSpace(string(out)) == "true" {
		return "running", true
	}
	return "stopped", false
}

// Helper: Control systemd service
func (h *ServiceHandler) controlSystemd(service, action string) error {
	cmd := exec.Command("systemctl", action, service)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err.Error(), string(out))
	}
	return nil
}

// Helper: Start Docker container
func (h *ServiceHandler) startDockerContainer(name string) error {
	// Try to start existing container first
	cmd := exec.Command("docker", "start", name)
	if err := cmd.Run(); err == nil {
		return nil
	}

	// If container doesn't exist, run a new one
	cmd = exec.Command("docker", "run", "-d",
		"--name", name,
		"--restart", "unless-stopped",
		"-p", "8080:8080",
		"reacherhq/backend:latest")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err.Error(), string(out))
	}
	return nil
}

// Helper: Stop Docker container
func (h *ServiceHandler) stopDockerContainer(name string) error {
	cmd := exec.Command("docker", "stop", name)
	return cmd.Run()
}

// Install KumoMTA (Rocky/RHEL)
func (h *ServiceHandler) installKumoMTA() (string, error) {
	script := `#!/bin/bash
set -e
# Disable postfix if running
systemctl disable --now postfix 2>/dev/null || true

# Install dnf-plugins-core if not present
dnf -y install dnf-plugins-core

# Add KumoMTA repo
dnf config-manager --add-repo https://openrepo.kumomta.com/files/kumomta-rocky.repo || \
dnf config-manager --add-repo https://openrepo.kumomta.com/files/kumomta-rhel9.repo

# Install KumoMTA
dnf install -y kumomta

# Enable and start
systemctl enable kumomta
systemctl start kumomta

echo "KumoMTA installed successfully"
`
	return h.runScript(script)
}

// Install Dovecot
func (h *ServiceHandler) installDovecot() (string, error) {
	script := `#!/bin/bash
set -e
dnf install -y dovecot
systemctl enable dovecot
systemctl start dovecot
echo "Dovecot installed successfully"
`
	return h.runScript(script)
}

// Install Reacher (Docker)
func (h *ServiceHandler) installReacher() (string, error) {
	// Check if Docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		return "", fmt.Errorf("Docker is not installed. Please install Docker first")
	}

	// Pull and run Reacher
	script := `#!/bin/bash
set -e
docker pull reacherhq/backend:latest
docker rm -f reacher 2>/dev/null || true
docker run -d --name reacher --restart unless-stopped -p 8080:8080 reacherhq/backend:latest
echo "Reacher installed and running on port 8080"
`
	result, err := h.runScript(script)
	if err != nil {
		return result, err
	}

	// Auto-configure Reacher URL in settings
	if h.Store != nil {
		settings, sErr := h.Store.GetSettings()
		if sErr != nil {
			settings = &models.AppSettings{}
		}

		// Set Reacher URL to local Docker container
		settings.ReacherURL = "http://127.0.0.1:8080"

		if uErr := h.Store.UpsertSettings(settings); uErr != nil {
			result += "\nWarning: Could not auto-configure Reacher URL in settings: " + uErr.Error()
		} else {
			result += "\nAuto-configured Reacher URL: http://127.0.0.1:8080"
		}
	}

	return result, nil
}

// Helper: Run shell script
func (h *ServiceHandler) runScript(script string) (string, error) {
	cmd := exec.Command("bash", "-c", script)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Routes helper for chi router
func (h *ServiceHandler) Routes(r interface{}) {
	// This is called from server.go to set up routes
	// The actual route setup is done in server.go
}

// MarshalJSON helper for ServiceStatus
func (s ServiceStatus) MarshalJSON() ([]byte, error) {
	type Alias ServiceStatus
	return json.Marshal(&struct {
		Alias
	}{
		Alias: (Alias)(s),
	})
}
