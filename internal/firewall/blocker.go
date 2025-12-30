// Package firewall provides an abstraction layer for IP blocking
// This allows the application to work in Docker/K8s where firewall-cmd isn't available
package firewall

import (
	"fmt"
	"net"
	"os/exec"
	"sync"

	"github.com/pulak-ranjan/sampmail/internal/config"
	"github.com/pulak-ranjan/sampmail/internal/logger"
)

// Blocker is the interface for IP blocking implementations
type Blocker interface {
	// Block adds an IP to the blocklist
	Block(ip string) error
	// Unblock removes an IP from the blocklist
	Unblock(ip string) error
	// IsBlocked checks if an IP is blocked
	IsBlocked(ip string) bool
	// ListBlocked returns all blocked IPs
	ListBlocked() []string
	// Available checks if the blocker is usable
	Available() bool
}

var (
	defaultBlocker Blocker
	blockerOnce    sync.Once
)

// GetBlocker returns the appropriate blocker for the environment
func GetBlocker() Blocker {
	blockerOnce.Do(func() {
		cfg := config.Get()

		if !cfg.FirewallEnabled {
			logger.Info("firewall disabled by configuration, using no-op blocker")
			defaultBlocker = &NoOpBlocker{}
			return
		}

		// Try firewalld first
		if fw := NewFirewalldBlocker(); fw.Available() {
			logger.Info("using firewalld for IP blocking")
			defaultBlocker = fw
			return
		}

		// Try iptables
		if ipt := NewIPTablesBlocker(); ipt.Available() {
			logger.Info("using iptables for IP blocking")
			defaultBlocker = ipt
			return
		}

		// Fall back to in-memory blocker (for Docker/K8s)
		logger.Warn("no system firewall available, using in-memory blocker",
			"note", "IP blocks will not persist across restarts")
		defaultBlocker = NewMemoryBlocker()
	})
	return defaultBlocker
}

// BlockIP is a convenience function to block an IP
func BlockIP(ip string) error {
	// Validate IP first
	if err := validateIP(ip); err != nil {
		return err
	}
	return GetBlocker().Block(ip)
}

// validateIP checks that the IP is valid and not protected
func validateIP(ip string) error {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return fmt.Errorf("invalid IP address format: %s", ip)
	}

	// Prevent blocking localhost
	if parsed.IsLoopback() {
		return fmt.Errorf("cannot block localhost")
	}

	// Prevent blocking private IPs
	if parsed.IsPrivate() {
		return fmt.Errorf("cannot block private IP ranges")
	}

	// Prevent blocking unspecified address
	if parsed.IsUnspecified() {
		return fmt.Errorf("cannot block unspecified address")
	}

	return nil
}

// ============================================
// FirewalldBlocker - Uses firewall-cmd
// ============================================

type FirewalldBlocker struct {
	available bool
}

func NewFirewalldBlocker() *FirewalldBlocker {
	fb := &FirewalldBlocker{}
	// Check if firewall-cmd exists and is running
	cmd := exec.Command("firewall-cmd", "--state")
	if err := cmd.Run(); err == nil {
		fb.available = true
	}
	return fb
}

func (fb *FirewalldBlocker) Available() bool {
	return fb.available
}

func (fb *FirewalldBlocker) Block(ip string) error {
	if !fb.available {
		return fmt.Errorf("firewalld not available")
	}

	parsed := net.ParseIP(ip)
	if parsed == nil {
		return fmt.Errorf("invalid IP")
	}
	safeIP := parsed.String()

	// Runtime block
	cmd := exec.Command("firewall-cmd",
		"--add-rich-rule",
		fmt.Sprintf("rule family='ipv4' source address='%s' drop", safeIP))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("runtime block failed: %w", err)
	}

	// Permanent block (best effort)
	cmdPerm := exec.Command("firewall-cmd", "--permanent",
		"--add-rich-rule",
		fmt.Sprintf("rule family='ipv4' source address='%s' drop", safeIP))
	if err := cmdPerm.Run(); err != nil {
		logger.Warn("failed to make block permanent",
			"ip", safeIP,
			"error", err)
	}

	logger.SecurityLogger().Info("IP blocked via firewalld",
		"ip", safeIP)
	return nil
}

func (fb *FirewalldBlocker) Unblock(ip string) error {
	if !fb.available {
		return fmt.Errorf("firewalld not available")
	}

	parsed := net.ParseIP(ip)
	if parsed == nil {
		return fmt.Errorf("invalid IP")
	}
	safeIP := parsed.String()

	rule := fmt.Sprintf("rule family='ipv4' source address='%s' drop", safeIP)

	exec.Command("firewall-cmd", "--remove-rich-rule", rule).Run()
	exec.Command("firewall-cmd", "--permanent", "--remove-rich-rule", rule).Run()

	return nil
}

func (fb *FirewalldBlocker) IsBlocked(ip string) bool {
	// firewalld doesn't have a simple "is blocked" check
	// Would need to parse output of --list-rich-rules
	return false
}

func (fb *FirewalldBlocker) ListBlocked() []string {
	return nil // Not implemented for firewalld
}

// ============================================
// IPTablesBlocker - Uses iptables directly
// ============================================

type IPTablesBlocker struct {
	available bool
}

func NewIPTablesBlocker() *IPTablesBlocker {
	ib := &IPTablesBlocker{}
	cmd := exec.Command("iptables", "--version")
	if err := cmd.Run(); err == nil {
		ib.available = true
	}
	return ib
}

func (ib *IPTablesBlocker) Available() bool {
	return ib.available
}

func (ib *IPTablesBlocker) Block(ip string) error {
	if !ib.available {
		return fmt.Errorf("iptables not available")
	}

	parsed := net.ParseIP(ip)
	if parsed == nil {
		return fmt.Errorf("invalid IP")
	}
	safeIP := parsed.String()

	cmd := exec.Command("iptables", "-A", "INPUT", "-s", safeIP, "-j", "DROP")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("iptables block failed: %w", err)
	}

	logger.SecurityLogger().Info("IP blocked via iptables",
		"ip", safeIP)
	return nil
}

func (ib *IPTablesBlocker) Unblock(ip string) error {
	if !ib.available {
		return fmt.Errorf("iptables not available")
	}

	parsed := net.ParseIP(ip)
	if parsed == nil {
		return fmt.Errorf("invalid IP")
	}

	cmd := exec.Command("iptables", "-D", "INPUT", "-s", parsed.String(), "-j", "DROP")
	return cmd.Run()
}

func (ib *IPTablesBlocker) IsBlocked(ip string) bool {
	return false // Not implemented
}

func (ib *IPTablesBlocker) ListBlocked() []string {
	return nil
}

// ============================================
// MemoryBlocker - In-memory fallback
// ============================================

type MemoryBlocker struct {
	blocked map[string]bool
	mu      sync.RWMutex
}

func NewMemoryBlocker() *MemoryBlocker {
	return &MemoryBlocker{
		blocked: make(map[string]bool),
	}
}

func (mb *MemoryBlocker) Available() bool {
	return true
}

func (mb *MemoryBlocker) Block(ip string) error {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return fmt.Errorf("invalid IP")
	}

	mb.mu.Lock()
	mb.blocked[parsed.String()] = true
	mb.mu.Unlock()

	logger.SecurityLogger().Info("IP blocked in-memory",
		"ip", parsed.String(),
		"warning", "will not persist across restarts")
	return nil
}

func (mb *MemoryBlocker) Unblock(ip string) error {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return fmt.Errorf("invalid IP")
	}

	mb.mu.Lock()
	delete(mb.blocked, parsed.String())
	mb.mu.Unlock()
	return nil
}

func (mb *MemoryBlocker) IsBlocked(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}

	mb.mu.RLock()
	defer mb.mu.RUnlock()
	return mb.blocked[parsed.String()]
}

func (mb *MemoryBlocker) ListBlocked() []string {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	result := make([]string, 0, len(mb.blocked))
	for ip := range mb.blocked {
		result = append(result, ip)
	}
	return result
}

// ============================================
// NoOpBlocker - Does nothing (disabled firewall)
// ============================================

type NoOpBlocker struct{}

func (n *NoOpBlocker) Available() bool       { return true }
func (n *NoOpBlocker) Block(ip string) error { return nil }
func (n *NoOpBlocker) Unblock(ip string) error { return nil }
func (n *NoOpBlocker) IsBlocked(ip string) bool { return false }
func (n *NoOpBlocker) ListBlocked() []string { return nil }
