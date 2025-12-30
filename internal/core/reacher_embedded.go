package core

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// ReacherManager handles the embedded Reacher binary
type ReacherManager struct {
	binaryPath string
	mu         sync.Mutex
	ready      bool
}

var (
	reacherManager *ReacherManager
	reacherOnce    sync.Once
)

// Embedded Reacher binary (placeholder - will be replaced at build time)
// To embed the actual binary, use:
// go build -ldflags="-X main.reacherBinaryPath=/path/to/binary"
// Or include the binary with go:embed

//go:embed reacher_binary_placeholder.txt
var reacherBinaryPlaceholder []byte

// GetReacherManager returns the singleton ReacherManager
func GetReacherManager() *ReacherManager {
	reacherOnce.Do(func() {
		reacherManager = &ReacherManager{}
	})
	return reacherManager
}

// Initialize extracts and prepares the Reacher binary
func (rm *ReacherManager) Initialize() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rm.ready {
		return nil
	}

	// Check for external binary first (user-provided)
	externalPaths := []string{
		"/usr/local/bin/check_if_email_exists",
		"/opt/reacher/bin/check_if_email_exists",
		"./check_if_email_exists",
	}

	for _, path := range externalPaths {
		if _, err := os.Stat(path); err == nil {
			// Verify it's executable
			if rm.verifyBinary(path) {
				rm.binaryPath = path
				rm.ready = true
				log.Printf("[Reacher] Using external binary: %s", path)
				return nil
			}
		}
	}

	// Try to extract embedded binary (if available)
	if len(reacherBinaryPlaceholder) > 100 { // Placeholder is small, real binary is large
		tmpDir := os.TempDir()
		binaryName := "check_if_email_exists"
		if runtime.GOOS == "windows" {
			binaryName += ".exe"
		}

		rm.binaryPath = filepath.Join(tmpDir, "kumomta-reacher", binaryName)

		// Create directory
		os.MkdirAll(filepath.Dir(rm.binaryPath), 0755)

		// Check if already extracted and valid
		if rm.verifyBinary(rm.binaryPath) {
			rm.ready = true
			log.Printf("[Reacher] Using cached embedded binary: %s", rm.binaryPath)
			return nil
		}

		// Extract binary (decompress if gzipped)
		var data []byte
		if isGzipped(reacherBinaryPlaceholder) {
			gz, err := gzip.NewReader(bytes.NewReader(reacherBinaryPlaceholder))
			if err != nil {
				return fmt.Errorf("failed to decompress binary: %w", err)
			}
			data, err = io.ReadAll(gz)
			gz.Close()
			if err != nil {
				return fmt.Errorf("failed to read decompressed binary: %w", err)
			}
		} else {
			data = reacherBinaryPlaceholder
		}

		// Write binary
		if err := os.WriteFile(rm.binaryPath, data, 0755); err != nil {
			return fmt.Errorf("failed to write binary: %w", err)
		}

		if rm.verifyBinary(rm.binaryPath) {
			rm.ready = true
			log.Printf("[Reacher] Extracted embedded binary: %s", rm.binaryPath)
			return nil
		}
	}

	log.Printf("[Reacher] No binary available - email verification will use SMTP-only mode")
	return nil
}

// IsReady returns true if Reacher is available
func (rm *ReacherManager) IsReady() bool {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	return rm.ready
}

// GetBinaryPath returns the path to the Reacher binary
func (rm *ReacherManager) GetBinaryPath() string {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	return rm.binaryPath
}

// verifyBinary checks if the binary exists and is executable
func (rm *ReacherManager) verifyBinary(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	// Check it's executable (on Unix)
	if runtime.GOOS != "windows" && info.Mode()&0111 == 0 {
		return false
	}

	// Try running --version
	cmd := exec.Command(path, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}

	// Check output contains expected text
	return bytes.Contains(output, []byte("check_if_email_exists")) ||
		bytes.Contains(output, []byte("reacher"))
}

// VerifyEmail uses the embedded Reacher binary
func (rm *ReacherManager) VerifyEmail(email string, opts VerifierOptions) (*ReacherResponse, error) {
	if !rm.IsReady() {
		return nil, fmt.Errorf("reacher not available")
	}

	args := []string{email}

	if opts.SenderEmail != "" {
		args = append(args, "--from-email", opts.SenderEmail)
	}
	if opts.HeloHost != "" {
		args = append(args, "--hello-name", opts.HeloHost)
	}

	cmd := exec.Command(rm.binaryPath, args...)

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
			return nil, fmt.Errorf("reacher failed: %v (%s)", err, stderr.String())
		}
	case <-time.After(30 * time.Second):
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return nil, fmt.Errorf("reacher timeout")
	}

	var response ReacherResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// isGzipped checks if data is gzip compressed
func isGzipped(data []byte) bool {
	return len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b
}

// DownloadReacherBinary downloads the Reacher binary from GitHub
func DownloadReacherBinary(destPath string) error {
	// Determine platform
	var assetName string
	switch runtime.GOOS {
	case "linux":
		if runtime.GOARCH == "amd64" {
			assetName = "check-if-email-exists-linux-x64"
		} else if runtime.GOARCH == "arm64" {
			assetName = "check-if-email-exists-linux-arm64"
		}
	case "darwin":
		if runtime.GOARCH == "amd64" {
			assetName = "check-if-email-exists-macos-x64"
		} else if runtime.GOARCH == "arm64" {
			assetName = "check-if-email-exists-macos-arm64"
		}
	case "windows":
		assetName = "check-if-email-exists-windows-x64.exe"
	}

	if assetName == "" {
		return fmt.Errorf("unsupported platform: %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// Get latest release URL
	releaseURL := fmt.Sprintf("https://github.com/reacherhq/check-if-email-exists/releases/latest/download/%s", assetName)

	log.Printf("[Reacher] Downloading from: %s", releaseURL)

	// Use curl or wget to download (simpler than net/http for redirects)
	cmd := exec.Command("curl", "-L", "-o", destPath, releaseURL)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try wget
		cmd = exec.Command("wget", "-O", destPath, releaseURL)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("download failed: %s", string(output))
		}
	}

	// Make executable
	os.Chmod(destPath, 0755)

	log.Printf("[Reacher] Downloaded to: %s", destPath)
	return nil
}

// SetupReacher is called at startup to prepare Reacher
func SetupReacher() {
	rm := GetReacherManager()
	if err := rm.Initialize(); err != nil {
		log.Printf("[Reacher] Initialization warning: %v", err)
	}

	if !rm.IsReady() {
		// Offer to download
		log.Println("[Reacher] Binary not found. For email verification, either:")
		log.Println("  1. Download manually: https://github.com/reacherhq/check-if-email-exists/releases")
		log.Println("  2. Place binary at /usr/local/bin/check_if_email_exists")
		log.Println("  3. Configure reacher_url in settings to use HTTP API")
	}
}
