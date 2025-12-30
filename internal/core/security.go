package core

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/config"
)

const (
	// Directory to store backups
	BackupDir = "/var/lib/sampmail/backups"
)

// BlockIP adds an IP to the firewall drop zone immediately and permanently.
func BlockIP(ip string) error {
	// SECURITY FIX: Use net.ParseIP for proper validation instead of string checks
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return fmt.Errorf("invalid IP address format")
	}
	
	// Prevent blocking localhost or private IPs
	if parsed.IsLoopback() {
		return fmt.Errorf("cannot block localhost")
	}
	if parsed.IsPrivate() {
		return fmt.Errorf("cannot block private IP ranges")
	}
	if parsed.IsUnspecified() {
		return fmt.Errorf("cannot block unspecified address")
	}

	// Use the canonical string representation (safe, validated)
	safeIP := parsed.String()

	// 1. Immediate Block (Runtime)
	cmd := exec.Command("firewall-cmd", "--add-rich-rule", fmt.Sprintf("rule family='ipv4' source address='%s' drop", safeIP))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to apply runtime block: %v", err)
	}

	// 2. Permanent Block (Persist across reboots)
	cmdPerm := exec.Command("firewall-cmd", "--permanent", "--add-rich-rule", fmt.Sprintf("rule family='ipv4' source address='%s' drop", safeIP))
	if err := cmdPerm.Run(); err != nil {
		// Log but don't fail if runtime worked
		fmt.Printf("Warning: failed to make block permanent for %s: %v\n", safeIP, err)
	}

	return nil
}

// BackupConfig creates a timestamped tar.gz of the /opt/kumomta/etc directory.
func BackupConfig() error {
	if err := os.MkdirAll(BackupDir, 0755); err != nil {
		return err
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("config-backup-%s.tar.gz", timestamp)
	destPath := filepath.Join(BackupDir, filename)

	// Create archive
	// tar -czf /path/to/backup.tar.gz -C /opt/kumomta etc
	cmd := exec.Command("tar", "-czf", destPath, "-C", "/opt/kumomta", "etc")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("backup failed: %s", string(output))
	}

	// Retention Policy: Keep last 3 backups
	return pruneBackups()
}

// EnsureRecentBackup creates a backup only if the latest one is older than 24h
// This is safe to call on startup.
func EnsureRecentBackup() error {
	entries, err := os.ReadDir(BackupDir)
	if err != nil {
		// If dir doesn't exist, we should probably backup
		if os.IsNotExist(err) {
			return BackupConfig()
		}
		return err
	}

	var backups []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "config-backup-") {
			backups = append(backups, filepath.Join(BackupDir, e.Name()))
		}
	}

	// No backups? Create one.
	if len(backups) == 0 {
		return BackupConfig()
	}

	// Find the newest backup (by timestamp in filename)
	// Filename format: config-backup-20060102-150405.tar.gz
	// Since format is sortable string, the last one is the newest.
	sort.Strings(backups)
	newestPath := backups[len(backups)-1]
	newestName := filepath.Base(newestPath)

	// Extract timestamp
	// "config-backup-" is 14 chars
	// "20060102-150405" is 15 chars
	if len(newestName) < 29 {
		// weird filename, just backup to be safe
		return BackupConfig()
	}
	tsStr := newestName[14:29]
	ts, err := time.Parse("20060102-150405", tsStr)
	if err != nil {
		return BackupConfig()
	}

	// Check if > 24 hours old
	if time.Since(ts) > 24*time.Hour {
		return BackupConfig()
	}

	// Recent backup exists
	return nil
}

func pruneBackups() error {
	cfg := config.Get()
	entries, err := os.ReadDir(BackupDir)
	if err != nil {
		return err
	}

	var backups []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "config-backup-") {
			backups = append(backups, filepath.Join(BackupDir, e.Name()))
		}
	}

	// Sort (strings with timestamps sort correctly: old -> new)
	sort.Strings(backups)

	// Use configurable retention (default 3)
	retention := cfg.BackupRetention
	if retention < 1 {
		retention = 3
	}

	// If more than retention limit, delete the oldest ones
	if len(backups) > retention {
		toDelete := backups[:len(backups)-retention]
		for _, f := range toDelete {
			if err := os.Remove(f); err != nil {
				fmt.Printf("Warning: failed to prune old backup %s: %v\n", f, err)
			}
		}
	}
	return nil
}

var (
	keyOnce       sync.Once
	encryptionKey []byte
)

// GetEncryptionKey validates and returns the encryption key using proper derivation
func GetEncryptionKey() ([]byte, error) {
	var keyErr error
	keyOnce.Do(func() {
		cfg := config.Get()
		if cfg.AppSecret == "" {
			keyErr = fmt.Errorf("SAMPMAIL_SECRET environment variable is required but not set")
			return
		}
		if len(cfg.AppSecret) < 32 {
			keyErr = fmt.Errorf("SAMPMAIL_SECRET must be at least 32 characters")
			return
		}
		// Use proper key derivation instead of truncation
		encryptionKey = cfg.DeriveEncryptionKey()
	})
	if keyErr != nil {
		return nil, keyErr
	}
	return encryptionKey, nil
}

func Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	key, err := GetEncryptionKey()
	if err != nil {
		return "", fmt.Errorf("encryption unavailable: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	// Backward compatibility: If it's not base64 or decryption fails, return original
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		// Not base64, assume plaintext
		return ciphertext, nil
	}

	key, err := GetEncryptionKey()
	if err != nil {
		// Cannot decrypt without key
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		// Too short to be valid ciphertext, assume plaintext (if valid base64)
		return ciphertext, nil
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		// Decryption failed (auth tag mismatch), so it wasn't encrypted with our key.
		// Assume legacy plaintext.
		return ciphertext, nil
	}

	return string(plaintext), nil
}

// HMAC Signing for Click Tracking
func SignLink(url string) string {
	key, err := GetEncryptionKey()
	if err != nil {
		// Should have checked at startup, but fail safe
		return ""
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(url))
	return hex.EncodeToString(mac.Sum(nil))
}

func VerifyLinkSignature(url, signature string) bool {
	expected := SignLink(url)
	if expected == "" {
		return false
	}
	return hmac.Equal([]byte(expected), []byte(signature))
}

// EncryptPassword is an alias for Encrypt, used for password storage
// This makes the code more readable when encrypting credentials
func EncryptPassword(password string) (string, error) {
	return Encrypt(password)
}

// DecryptPassword is an alias for Decrypt, used for password retrieval
func DecryptPassword(encrypted string) (string, error) {
	return Decrypt(encrypted)
}
