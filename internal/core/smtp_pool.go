package core

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"sync"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/logger"
)

// SMTPPoolConfig holds SMTP connection pool configuration
type SMTPPoolConfig struct {
	Addr            string
	MaxConnections  int
	MinConnections  int
	MaxIdleTime     time.Duration
	ConnectTimeout  time.Duration
	HealthCheckInterval time.Duration
}

// DefaultSMTPPoolConfig returns production-ready defaults
func DefaultSMTPPoolConfig(addr string) *SMTPPoolConfig {
	return &SMTPPoolConfig{
		Addr:            addr,
		MaxConnections:  100,
		MinConnections:  5,
		MaxIdleTime:     5 * time.Minute,
		ConnectTimeout:  10 * time.Second,
		HealthCheckInterval: 30 * time.Second,
	}
}

// SMTPConnection wraps an SMTP client with metadata
type SMTPConnection struct {
	client    *smtp.Client
	conn      net.Conn
	createdAt time.Time
	lastUsed  time.Time
	inUse     bool
}

// SMTPPool manages a pool of SMTP connections
type SMTPPool struct {
	config      *SMTPPoolConfig
	mu          sync.Mutex
	connections []*SMTPConnection
	available   chan *SMTPConnection
	closed      bool
	wg          sync.WaitGroup
	stopCh      chan struct{}
}

var (
	ErrPoolClosed      = errors.New("connection pool is closed")
	ErrPoolExhausted   = errors.New("connection pool exhausted")
	ErrConnectionBad   = errors.New("connection is bad")
)

// NewSMTPPool creates a new SMTP connection pool
func NewSMTPPool(config *SMTPPoolConfig) (*SMTPPool, error) {
	pool := &SMTPPool{
		config:    config,
		available: make(chan *SMTPConnection, config.MaxConnections),
		stopCh:    make(chan struct{}),
	}

	// Create minimum connections
	for i := 0; i < config.MinConnections; i++ {
		conn, err := pool.createConnection()
		if err != nil {
			// Log but continue - pool can grow later
			logger.WithComponent("smtp_pool").Warn("failed to create initial connection", "error", err)
			continue
		}
		pool.connections = append(pool.connections, conn)
		pool.available <- conn
	}

	// Start health checker
	pool.wg.Add(1)
	go pool.healthChecker()

	return pool, nil
}

// createConnection creates a new SMTP connection
func (p *SMTPPool) createConnection() (*SMTPConnection, error) {
	// Use circuit breaker
	var smtpConn *SMTPConnection

	err := SMTPCircuitBreaker.Execute(context.Background(), func(ctx context.Context) error {
		conn, err := net.DialTimeout("tcp", p.config.Addr, p.config.ConnectTimeout)
		if err != nil {
			return fmt.Errorf("dial failed: %w", err)
		}

		client, err := smtp.NewClient(conn, "localhost")
		if err != nil {
			conn.Close()
			return fmt.Errorf("SMTP client creation failed: %w", err)
		}

		smtpConn = &SMTPConnection{
			client:    client,
			conn:      conn,
			createdAt: time.Now(),
			lastUsed:  time.Now(),
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return smtpConn, nil
}

// Get retrieves a connection from the pool
func (p *SMTPPool) Get(ctx context.Context) (*SMTPConnection, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, ErrPoolClosed
	}
	p.mu.Unlock()

	// Try to get from available connections
	select {
	case conn := <-p.available:
		if p.isHealthy(conn) {
			conn.inUse = true
			conn.lastUsed = time.Now()
			return conn, nil
		}
		// Connection is bad, close and create new
		p.closeConnection(conn)
		return p.createAndReturn()

	case <-ctx.Done():
		return nil, ctx.Err()

	default:
		// No available connections, try to create new
		return p.createAndReturn()
	}
}

// createAndReturn creates a new connection if pool isn't at max
// FIXED: Releases mutex before network I/O to prevent global lockup
func (p *SMTPPool) createAndReturn() (*SMTPConnection, error) {
	p.mu.Lock()
	
	// Check if at capacity
	if len(p.connections) >= p.config.MaxConnections {
		// At capacity - wait for available connection
		p.mu.Unlock()
		
		select {
		case conn := <-p.available:
			if p.isHealthy(conn) {
				conn.inUse = true
				conn.lastUsed = time.Now()
				return conn, nil
			}
			p.closeConnection(conn)
			// Recursively try again
			return p.createAndReturn()
		case <-time.After(5 * time.Second):
			return nil, ErrPoolExhausted
		}
	}
	
	// Not at capacity - release lock BEFORE dialing
	currentCount := len(p.connections)
	p.mu.Unlock()
	
	// Create connection WITHOUT holding the lock
	conn, err := p.createConnection()
	if err != nil {
		return nil, err
	}
	
	// Re-acquire lock to add to pool
	p.mu.Lock()
	// Double-check we're still under limit (another goroutine may have added)
	if len(p.connections) >= p.config.MaxConnections {
		p.mu.Unlock()
		// Lost the race - close this connection and wait for available
		if conn.client != nil {
			conn.client.Quit()
		}
		if conn.conn != nil {
			conn.conn.Close()
		}
		// Wait for an available connection instead
		select {
		case available := <-p.available:
			if p.isHealthy(available) {
				available.inUse = true
				available.lastUsed = time.Now()
				return available, nil
			}
			p.closeConnection(available)
			return p.createAndReturn()
		case <-time.After(5 * time.Second):
			return nil, ErrPoolExhausted
		}
	}
	
	conn.inUse = true
	p.connections = append(p.connections, conn)
	p.mu.Unlock()
	
	logger.WithComponent("smtp_pool").Debug("created new connection",
		"total", currentCount+1,
		"max", p.config.MaxConnections)
	
	return conn, nil
}

// Put returns a connection to the pool
func (p *SMTPPool) Put(conn *SMTPConnection) {
	if conn == nil {
		return
	}

	p.mu.Lock()
	if p.closed {
		p.closeConnectionLocked(conn)
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	conn.inUse = false
	conn.lastUsed = time.Now()

	select {
	case p.available <- conn:
	default:
		// Pool is full, close connection
		p.closeConnection(conn)
	}
}

// MarkBad marks a connection as bad and removes it from pool
func (p *SMTPPool) MarkBad(conn *SMTPConnection) {
	p.closeConnection(conn)
}

// isHealthy checks if a connection is still valid
func (p *SMTPPool) isHealthy(conn *SMTPConnection) bool {
	// Check if idle too long
	if time.Since(conn.lastUsed) > p.config.MaxIdleTime {
		return false
	}

	// Try NOOP command
	if err := conn.client.Noop(); err != nil {
		return false
	}

	return true
}

// closeConnection closes a connection and removes from pool
func (p *SMTPPool) closeConnection(conn *SMTPConnection) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closeConnectionLocked(conn)
}

func (p *SMTPPool) closeConnectionLocked(conn *SMTPConnection) {
	if conn.client != nil {
		conn.client.Quit()
	}
	if conn.conn != nil {
		conn.conn.Close()
	}

	// Remove from connections slice
	for i, c := range p.connections {
		if c == conn {
			p.connections = append(p.connections[:i], p.connections[i+1:]...)
			break
		}
	}
}

// healthChecker periodically checks connection health
func (p *SMTPPool) healthChecker() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.checkConnections()
		}
	}
}

// checkConnections checks and removes unhealthy connections
func (p *SMTPPool) checkConnections() {
	p.mu.Lock()
	defer p.mu.Unlock()

	var healthy []*SMTPConnection
	for _, conn := range p.connections {
		if conn.inUse {
			healthy = append(healthy, conn)
			continue
		}

		if p.isHealthy(conn) {
			healthy = append(healthy, conn)
		} else {
			p.closeConnectionLocked(conn)
		}
	}
	p.connections = healthy

	// Ensure minimum connections
	for len(p.connections) < p.config.MinConnections {
		conn, err := p.createConnection()
		if err != nil {
			break
		}
		p.connections = append(p.connections, conn)
		select {
		case p.available <- conn:
		default:
		}
	}
}

// Close closes all connections and the pool
func (p *SMTPPool) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()

	close(p.stopCh)
	p.wg.Wait()

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, conn := range p.connections {
		if conn.client != nil {
			conn.client.Quit()
		}
		if conn.conn != nil {
			conn.conn.Close()
		}
	}
	p.connections = nil

	close(p.available)

	return nil
}

// Stats returns pool statistics
func (p *SMTPPool) Stats() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	inUse := 0
	for _, conn := range p.connections {
		if conn.inUse {
			inUse++
		}
	}

	return map[string]interface{}{
		"total":     len(p.connections),
		"available": len(p.available),
		"in_use":    inUse,
		"max":       p.config.MaxConnections,
	}
}

// Global SMTP pool
var globalSMTPPool *SMTPPool

// InitSMTPPool initializes the global SMTP pool
func InitSMTPPool(config *SMTPPoolConfig) error {
	var err error
	globalSMTPPool, err = NewSMTPPool(config)
	return err
}

// GetSMTPPool returns the global SMTP pool
func GetSMTPPool() *SMTPPool {
	return globalSMTPPool
}

// SendEmail is a helper that uses the pool to send an email
func SendEmail(ctx context.Context, from, to, subject, body string) error {
	if globalSMTPPool == nil {
		return errors.New("SMTP pool not initialized")
	}

	conn, err := globalSMTPPool.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get SMTP connection: %w", err)
	}
	defer globalSMTPPool.Put(conn)

	// Reset connection state
	if err := conn.client.Reset(); err != nil {
		globalSMTPPool.MarkBad(conn)
		return fmt.Errorf("failed to reset connection: %w", err)
	}

	if err := conn.client.Mail(from); err != nil {
		return fmt.Errorf("MAIL command failed: %w", err)
	}

	if err := conn.client.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT command failed: %w", err)
	}

	wc, err := conn.client.Data()
	if err != nil {
		return fmt.Errorf("DATA command failed: %w", err)
	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		from, to, subject, body)

	if _, err := wc.Write([]byte(msg)); err != nil {
		wc.Close()
		return fmt.Errorf("failed to write message: %w", err)
	}

	if err := wc.Close(); err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	return nil
}

// SMTPEmailSender implements EmailSender interface using the SMTP pool
type SMTPEmailSender struct {
	DefaultFrom string
}

// NewSMTPEmailSender creates a new SMTPEmailSender
func NewSMTPEmailSender(defaultFrom string) *SMTPEmailSender {
	return &SMTPEmailSender{DefaultFrom: defaultFrom}
}

// SendEmail implements EmailSender interface
func (s *SMTPEmailSender) SendEmail(ctx context.Context, to, subject, body string) error {
	return SendEmail(ctx, s.DefaultFrom, to, subject, body)
}

