package core

import (
	"context"
	"errors"
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	CircuitClosed CircuitState = iota // Normal operation
	CircuitOpen                       // Failing, reject requests
	CircuitHalfOpen                   // Testing if service recovered
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	Name               string
	MaxFailures        int           // Failures before opening
	ResetTimeout       time.Duration // Time before trying half-open
	HalfOpenMaxCalls   int           // Max calls in half-open state
	SuccessThreshold   int           // Successes needed to close from half-open
	OnStateChange      func(name string, from, to CircuitState)
}

// DefaultCircuitBreakerConfig returns production defaults
func DefaultCircuitBreakerConfig(name string) *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		Name:             name,
		MaxFailures:      5,
		ResetTimeout:     30 * time.Second,
		HalfOpenMaxCalls: 3,
		SuccessThreshold: 2,
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config *CircuitBreakerConfig

	mu             sync.RWMutex
	state          CircuitState
	failures       int
	successes      int
	halfOpenCalls  int
	lastFailure    time.Time
	lastStateChange time.Time
}

var (
	ErrCircuitOpen     = errors.New("circuit breaker is open")
	ErrTooManyCalls    = errors.New("too many calls in half-open state")
)

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	if config == nil {
		config = DefaultCircuitBreakerConfig("default")
	}
	return &CircuitBreaker{
		config:          config,
		state:           CircuitClosed,
		lastStateChange: time.Now(),
	}
}

// Execute wraps a function call with circuit breaker protection
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(context.Context) error) error {
	if err := cb.allowRequest(); err != nil {
		return err
	}

	// Execute the function
	err := fn(ctx)

	// Record result
	cb.recordResult(err)

	return err
}

// allowRequest checks if a request should be allowed
func (cb *CircuitBreaker) allowRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return nil

	case CircuitOpen:
		// Check if we should transition to half-open
		if time.Since(cb.lastFailure) > cb.config.ResetTimeout {
			cb.setState(CircuitHalfOpen)
			cb.halfOpenCalls = 0
			cb.successes = 0
			return nil
		}
		return ErrCircuitOpen

	case CircuitHalfOpen:
		if cb.halfOpenCalls >= cb.config.HalfOpenMaxCalls {
			return ErrTooManyCalls
		}
		cb.halfOpenCalls++
		return nil
	}

	return nil
}

// recordResult records the result of a request
func (cb *CircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}
}

func (cb *CircuitBreaker) recordSuccess() {
	switch cb.state {
	case CircuitClosed:
		cb.failures = 0 // Reset failure count on success

	case CircuitHalfOpen:
		cb.successes++
		if cb.successes >= cb.config.SuccessThreshold {
			cb.setState(CircuitClosed)
			cb.failures = 0
		}
	}
}

func (cb *CircuitBreaker) recordFailure() {
	cb.lastFailure = time.Now()

	switch cb.state {
	case CircuitClosed:
		cb.failures++
		if cb.failures >= cb.config.MaxFailures {
			cb.setState(CircuitOpen)
		}

	case CircuitHalfOpen:
		cb.setState(CircuitOpen)
	}
}

func (cb *CircuitBreaker) setState(state CircuitState) {
	if cb.state == state {
		return
	}

	from := cb.state
	cb.state = state
	cb.lastStateChange = time.Now()

	if cb.config.OnStateChange != nil {
		go cb.config.OnStateChange(cb.config.Name, from, state)
	}
}

// State returns the current circuit state
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Stats returns circuit breaker statistics
func (cb *CircuitBreaker) Stats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"name":             cb.config.Name,
		"state":            cb.state.String(),
		"failures":         cb.failures,
		"successes":        cb.successes,
		"last_failure":     cb.lastFailure,
		"last_state_change": cb.lastStateChange,
	}
}

// CircuitBreakerRegistry manages multiple circuit breakers
type CircuitBreakerRegistry struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
}

// Global registry
var globalRegistry = &CircuitBreakerRegistry{
	breakers: make(map[string]*CircuitBreaker),
}

// GetCircuitBreaker returns or creates a circuit breaker by name
func GetCircuitBreaker(name string) *CircuitBreaker {
	globalRegistry.mu.RLock()
	if cb, ok := globalRegistry.breakers[name]; ok {
		globalRegistry.mu.RUnlock()
		return cb
	}
	globalRegistry.mu.RUnlock()

	// Create new circuit breaker
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	// Double-check
	if cb, ok := globalRegistry.breakers[name]; ok {
		return cb
	}

	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig(name))
	globalRegistry.breakers[name] = cb
	return cb
}

// RegisterCircuitBreaker registers a circuit breaker with custom config
func RegisterCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	cb := NewCircuitBreaker(config)

	globalRegistry.mu.Lock()
	globalRegistry.breakers[config.Name] = cb
	globalRegistry.mu.Unlock()

	return cb
}

// AllCircuitBreakerStats returns stats for all circuit breakers
func AllCircuitBreakerStats() map[string]map[string]interface{} {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	stats := make(map[string]map[string]interface{})
	for name, cb := range globalRegistry.breakers {
		stats[name] = cb.Stats()
	}
	return stats
}

// Pre-defined circuit breakers for external services
var (
	SMTPCircuitBreaker    *CircuitBreaker
	ReacherCircuitBreaker *CircuitBreaker
	WebhookCircuitBreaker *CircuitBreaker
)

// InitCircuitBreakers initializes all circuit breakers
func InitCircuitBreakers(onStateChange func(name string, from, to CircuitState)) {
	SMTPCircuitBreaker = RegisterCircuitBreaker(&CircuitBreakerConfig{
		Name:             "smtp",
		MaxFailures:      10,
		ResetTimeout:     60 * time.Second,
		HalfOpenMaxCalls: 5,
		SuccessThreshold: 3,
		OnStateChange:    onStateChange,
	})

	ReacherCircuitBreaker = RegisterCircuitBreaker(&CircuitBreakerConfig{
		Name:             "reacher",
		MaxFailures:      5,
		ResetTimeout:     30 * time.Second,
		HalfOpenMaxCalls: 3,
		SuccessThreshold: 2,
		OnStateChange:    onStateChange,
	})

	WebhookCircuitBreaker = RegisterCircuitBreaker(&CircuitBreakerConfig{
		Name:             "webhook",
		MaxFailures:      3,
		ResetTimeout:     60 * time.Second,
		HalfOpenMaxCalls: 2,
		SuccessThreshold: 2,
		OnStateChange:    onStateChange,
	})
}
