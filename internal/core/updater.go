package core

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/logger"
)

// =====================================
// VERSION MANAGEMENT & AUTO-UPDATE
// SampMail v2.0.0 - Self-Updating System
// =====================================

// Current version (set at build time via ldflags)
var (
	Version   = "0.1.12"
	BuildTime = ""
	GitCommit = ""
	Channel   = "stable" // stable, beta, dev
)

// UpdateConfig holds update server configuration
type UpdateConfig struct {
	UpdateServerURL string        // URL to check for updates
	CheckInterval   time.Duration // How often to check
	AutoCheck       bool          // Enable automatic checking
	AutoDownload    bool          // Auto-download updates (not auto-install)
}

// DefaultUpdateConfig returns default update configuration
func DefaultUpdateConfig() *UpdateConfig {
	return &UpdateConfig{
		UpdateServerURL: "https://api.github.com/repos/pulak-ranjan/sampmail/releases",
		CheckInterval:   24 * time.Hour,
		AutoCheck:       true,
		AutoDownload:    false,
	}
}

// VersionInfo holds version details
type VersionInfo struct {
	Version     string    `json:"version"`
	Channel     string    `json:"channel"`
	BuildTime   string    `json:"build_time"`
	GitCommit   string    `json:"git_commit"`
	GoVersion   string    `json:"go_version"`
	OS          string    `json:"os"`
	Arch        string    `json:"arch"`
	Uptime      string    `json:"uptime"`
	StartedAt   time.Time `json:"started_at"`
}

// ReleaseInfo holds information about an available release
type ReleaseInfo struct {
	Version      string    `json:"version"`
	TagName      string    `json:"tag_name"`
	Name         string    `json:"name"`
	Channel      string    `json:"channel"`
	ReleaseDate  time.Time `json:"release_date"`
	ReleaseNotes string    `json:"release_notes"`
	DownloadURL  string    `json:"download_url"`
	Checksum     string    `json:"checksum"`
	Size         int64     `json:"size"`
	IsLTS        bool      `json:"is_lts"`
	MinVersion   string    `json:"min_version"` // Minimum version to upgrade from
}

// UpdateStatus represents the current update state
type UpdateStatus struct {
	Available        bool         `json:"available"`
	CurrentVersion   string       `json:"current_version"`
	LatestVersion    string       `json:"latest_version,omitempty"`
	LatestLTS        string       `json:"latest_lts,omitempty"`
	ReleaseInfo      *ReleaseInfo `json:"release_info,omitempty"`
	LastChecked      time.Time    `json:"last_checked"`
	DownloadedPath   string       `json:"downloaded_path,omitempty"`
	DownloadReady    bool         `json:"download_ready"`
	DownloadProgress int          `json:"download_progress"` // 0-100
	UpdateInProgress bool         `json:"update_in_progress"`
	Error            string       `json:"error,omitempty"`
}

// Updater manages version checking and updates
type Updater struct {
	config     *UpdateConfig
	status     *UpdateStatus
	httpClient *http.Client
	mu         sync.RWMutex
	startedAt  time.Time
	stopChan   chan struct{}
	log        *slog.Logger
}

// Global updater instance
var (
	globalUpdater *Updater
	updaterOnce   sync.Once
)

// InitUpdater initializes the global updater
func InitUpdater(config *UpdateConfig) *Updater {
	updaterOnce.Do(func() {
		if config == nil {
			config = DefaultUpdateConfig()
		}
		globalUpdater = &Updater{
			config: config,
			status: &UpdateStatus{
				CurrentVersion: Version,
				LastChecked:    time.Time{},
			},
			httpClient: &http.Client{
				Timeout: 30 * time.Second,
			},
			startedAt: time.Now(),
			stopChan:  make(chan struct{}),
			log:       logger.WithComponent("updater"),
		}
	})
	return globalUpdater
}

// GetUpdater returns the global updater instance
func GetUpdater() *Updater {
	if globalUpdater == nil {
		return InitUpdater(nil)
	}
	return globalUpdater
}

// GetVersionInfo returns current version information
func (u *Updater) GetVersionInfo() *VersionInfo {
	return &VersionInfo{
		Version:   Version,
		Channel:   Channel,
		BuildTime: BuildTime,
		GitCommit: GitCommit,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Uptime:    time.Since(u.startedAt).Round(time.Second).String(),
		StartedAt: u.startedAt,
	}
}

// GetStatus returns current update status
func (u *Updater) GetStatus() *UpdateStatus {
	u.mu.RLock()
	defer u.mu.RUnlock()
	
	// Return a copy
	status := *u.status
	return &status
}

// Start begins automatic update checking
func (u *Updater) Start() {
	if !u.config.AutoCheck {
		return
	}

	go func() {
		// Initial check after 1 minute
		time.Sleep(1 * time.Minute)
		u.CheckForUpdates(context.Background())

		ticker := time.NewTicker(u.config.CheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				u.CheckForUpdates(context.Background())
			case <-u.stopChan:
				return
			}
		}
	}()

	u.log.Info("update checker started",
		"interval", u.config.CheckInterval,
		"current_version", Version)
}

// Stop stops automatic update checking
func (u *Updater) Stop() {
	close(u.stopChan)
}

// CheckForUpdates checks if a new version is available
func (u *Updater) CheckForUpdates(ctx context.Context) (*UpdateStatus, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.log.Info("checking for updates", "current_version", Version)

	// Fetch releases from GitHub API
	releases, err := u.fetchReleases(ctx)
	if err != nil {
		u.status.Error = err.Error()
		u.status.LastChecked = time.Now()
		return u.status, err
	}

	// Find latest stable and LTS versions
	var latestStable, latestLTS *ReleaseInfo
	for _, release := range releases {
		if release.Channel == "stable" || release.Channel == "" {
			if latestStable == nil || compareVersions(release.Version, latestStable.Version) > 0 {
				latestStable = release
			}
		}
		if release.IsLTS {
			if latestLTS == nil || compareVersions(release.Version, latestLTS.Version) > 0 {
				latestLTS = release
			}
		}
	}

	// Update status
	u.status.LastChecked = time.Now()
	u.status.Error = ""

	if latestStable != nil && compareVersions(latestStable.Version, Version) > 0 {
		u.status.Available = true
		u.status.LatestVersion = latestStable.Version
		u.status.ReleaseInfo = latestStable
	} else {
		u.status.Available = false
		u.status.LatestVersion = Version
	}

	if latestLTS != nil {
		u.status.LatestLTS = latestLTS.Version
	}

	if u.status.Available {
		u.log.Info("update available",
			"current", Version,
			"latest", u.status.LatestVersion,
			"lts", u.status.LatestLTS)
	} else {
		u.log.Info("system is up to date", "version", Version)
	}

	return u.status, nil
}

// fetchReleases fetches release information from GitHub
func (u *Updater) fetchReleases(ctx context.Context) ([]*ReleaseInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", u.config.UpdateServerURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", fmt.Sprintf("SampMail/%s", Version))

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned %d", resp.StatusCode)
	}

	var ghReleases []struct {
		TagName     string    `json:"tag_name"`
		Name        string    `json:"name"`
		Body        string    `json:"body"`
		Prerelease  bool      `json:"prerelease"`
		Draft       bool      `json:"draft"`
		PublishedAt time.Time `json:"published_at"`
		Assets      []struct {
			Name               string `json:"name"`
			Size               int64  `json:"size"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ghReleases); err != nil {
		return nil, fmt.Errorf("decode releases: %w", err)
	}

	var releases []*ReleaseInfo
	for _, gh := range ghReleases {
		if gh.Draft {
			continue
		}

		version := strings.TrimPrefix(gh.TagName, "v")
		channel := "stable"
		if gh.Prerelease {
			channel = "beta"
		}

		isLTS := strings.Contains(gh.Name, "LTS") || strings.Contains(gh.Body, "[LTS]")

		// Find the appropriate asset for current OS/arch
		var downloadURL string
		var size int64
		assetName := fmt.Sprintf("sampmail-%s-%s", runtime.GOOS, runtime.GOARCH)
		for _, asset := range gh.Assets {
			if strings.Contains(asset.Name, assetName) {
				downloadURL = asset.BrowserDownloadURL
				size = asset.Size
				break
			}
		}

		releases = append(releases, &ReleaseInfo{
			Version:      version,
			TagName:      gh.TagName,
			Name:         gh.Name,
			Channel:      channel,
			ReleaseDate:  gh.PublishedAt,
			ReleaseNotes: gh.Body,
			DownloadURL:  downloadURL,
			Size:         size,
			IsLTS:        isLTS,
		})
	}

	return releases, nil
}

// DownloadUpdate downloads the update package with progress tracking
func (u *Updater) DownloadUpdate(ctx context.Context) (string, error) {
	u.mu.Lock()
	if !u.status.Available || u.status.ReleaseInfo == nil {
		u.mu.Unlock()
		return "", fmt.Errorf("no update available")
	}

	if u.status.ReleaseInfo.DownloadURL == "" {
		u.mu.Unlock()
		return "", fmt.Errorf("no download URL for current platform")
	}

	releaseInfo := u.status.ReleaseInfo
	u.status.DownloadProgress = 0
	u.mu.Unlock()

	u.log.Info("downloading update",
		"version", releaseInfo.Version,
		"url", releaseInfo.DownloadURL)

	// Create temp directory for download
	tmpDir := filepath.Join(os.TempDir(), "sampmail-update")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	// Download file
	req, err := http.NewRequestWithContext(ctx, "GET", releaseInfo.DownloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: %d", resp.StatusCode)
	}

	// Save to file with progress tracking
	filename := filepath.Join(tmpDir, fmt.Sprintf("sampmail-%s.zip", releaseInfo.Version))
	out, err := os.Create(filename)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}

	// Copy with hash calculation and progress
	hasher := sha256.New()
	totalSize := resp.ContentLength
	var downloaded int64

	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			out.Write(buf[:n])
			hasher.Write(buf[:n])
			downloaded += int64(n)

			// Update progress
			if totalSize > 0 {
				u.mu.Lock()
				u.status.DownloadProgress = int(downloaded * 100 / totalSize)
				u.mu.Unlock()
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			out.Close()
			os.Remove(filename)
			return "", fmt.Errorf("download error: %w", err)
		}
	}
	out.Close()

	// Verify checksum if provided
	if releaseInfo.Checksum != "" {
		actualSum := hex.EncodeToString(hasher.Sum(nil))
		if actualSum != releaseInfo.Checksum {
			os.Remove(filename)
			return "", fmt.Errorf("checksum mismatch: expected %s, got %s",
				releaseInfo.Checksum, actualSum)
		}
	}

	u.mu.Lock()
	u.status.DownloadedPath = filename
	u.status.DownloadReady = true
	u.status.DownloadProgress = 100
	u.mu.Unlock()

	u.log.Info("update downloaded",
		"version", releaseInfo.Version,
		"size", downloaded,
		"path", filename)

	return filename, nil
}

// ApplyUpdate applies the downloaded update
func (u *Updater) ApplyUpdate(ctx context.Context) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if !u.status.DownloadReady || u.status.DownloadedPath == "" {
		return fmt.Errorf("no update ready to apply")
	}

	u.status.UpdateInProgress = true
	u.log.Info("applying update", "version", u.status.ReleaseInfo.Version)

	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		u.status.UpdateInProgress = false
		return fmt.Errorf("get executable path: %w", err)
	}

	// Create backup of current executable
	backupPath := execPath + ".backup"
	if err := copyFile(execPath, backupPath); err != nil {
		u.status.UpdateInProgress = false
		return fmt.Errorf("backup current version: %w", err)
	}

	// Extract update
	extractDir := filepath.Join(filepath.Dir(u.status.DownloadedPath), "extract")
	if err := u.extractZip(u.status.DownloadedPath, extractDir); err != nil {
		u.status.UpdateInProgress = false
		return fmt.Errorf("extract update: %w", err)
	}

	// Find new executable in extracted files
	newExec := filepath.Join(extractDir, "sampmail")
	if runtime.GOOS == "windows" {
		newExec += ".exe"
	}

	if _, err := os.Stat(newExec); os.IsNotExist(err) {
		u.status.UpdateInProgress = false
		return fmt.Errorf("new executable not found in update package")
	}

	// Replace executable
	if err := os.Rename(newExec, execPath); err != nil {
		// Try to restore backup
		os.Rename(backupPath, execPath)
		u.status.UpdateInProgress = false
		return fmt.Errorf("replace executable: %w", err)
	}

	// Make executable
	if runtime.GOOS != "windows" {
		os.Chmod(execPath, 0755)
	}

	u.log.Info("update applied successfully",
		"version", u.status.ReleaseInfo.Version,
		"restart_required", true)

	// Schedule restart (gives time for response to be sent)
	go func() {
		time.Sleep(2 * time.Second)
		u.restartService()
	}()

	return nil
}

// extractZip extracts a zip file to a directory
func (u *Updater) extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	os.MkdirAll(dest, 0755)

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)

		// Security: prevent zip slip
		if !strings.HasPrefix(filepath.Clean(fpath), filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, f.Mode())
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

// restartService restarts the service
func (u *Updater) restartService() {
	u.log.Info("restarting service...")

	// Try systemctl first
	if err := exec.Command("systemctl", "restart", "sampmail").Run(); err != nil {
		// Fallback: just exit and let process manager restart
		u.log.Info("systemctl restart failed, exiting for manual restart")
		os.Exit(0)
	}
}

// Rollback reverts to the previous version
func (u *Updater) Rollback() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	backupPath := execPath + ".backup"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("no backup found")
	}

	if err := os.Rename(backupPath, execPath); err != nil {
		return fmt.Errorf("restore backup: %w", err)
	}

	u.log.Info("rollback completed", "restart_required", true)

	// Schedule restart
	go func() {
		time.Sleep(2 * time.Second)
		u.restartService()
	}()

	return nil
}

// compareVersions compares two semantic versions
// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func compareVersions(v1, v2 string) int {
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	for i := 0; i < 3; i++ {
		var n1, n2 int
		if i < len(parts1) {
			fmt.Sscanf(parts1[i], "%d", &n1)
		}
		if i < len(parts2) {
			fmt.Sscanf(parts2[i], "%d", &n2)
		}

		if n1 < n2 {
			return -1
		}
		if n1 > n2 {
			return 1
		}
	}

	return 0
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
