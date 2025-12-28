package listener

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kolosys/ion/workerpool"
	"github.com/kolosys/nova/shared"
)

// BackoffStrategy defines different backoff strategies for retries
type BackoffStrategy int

const (
	// FixedBackoff uses a fixed delay between retries
	FixedBackoff BackoffStrategy = iota
	// ExponentialBackoff increases delay exponentially
	ExponentialBackoff
	// LinearBackoff increases delay linearly
	LinearBackoff
)

// String returns the string representation of BackoffStrategy
func (b BackoffStrategy) String() string {
	switch b {
	case FixedBackoff:
		return "fixed"
	case ExponentialBackoff:
		return "exponential"
	case LinearBackoff:
		return "linear"
	default:
		return "unknown"
	}
}

// RetryPolicy defines retry behavior for failed events
type RetryPolicy struct {
	// MaxAttempts is the maximum number of retry attempts (0 = no retries)
	MaxAttempts int

	// InitialDelay is the initial delay before first retry
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration

	// Backoff strategy to use
	Backoff BackoffStrategy

	// RetryableErrors lists error types that should trigger retries
	// If empty, all errors are considered retryable
	RetryableErrors []error

	// RetryCondition is a custom function to determine if an error should be retried
	RetryCondition func(error) bool
}

// DefaultRetryPolicy returns a sensible default retry policy
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     30 * time.Second,
		Backoff:      ExponentialBackoff,
	}
}

// CircuitConfig configures circuit breaker behavior
type CircuitConfig struct {
	// Enabled determines if circuit breaker is active
	Enabled bool

	// FailureThreshold is the number of failures before opening circuit
	FailureThreshold int

	// SuccessThreshold is the number of successes needed to close circuit
	SuccessThreshold int

	// Timeout is how long to wait before trying again after circuit opens
	Timeout time.Duration

	// SlidingWindow is the time window for counting failures
	SlidingWindow time.Duration
}

// DefaultCircuitConfig returns a sensible default circuit breaker config
func DefaultCircuitConfig() CircuitConfig {
	return CircuitConfig{
		Enabled:          true,
		FailureThreshold: 5,
		SuccessThreshold: 3,
		Timeout:          30 * time.Second,
		SlidingWindow:    time.Minute,
	}
}

// DeadLetterConfig configures dead letter queue behavior
type DeadLetterConfig struct {
	// Enabled determines if dead letter queue is active
	Enabled bool

	// MaxRetries before sending to dead letter queue
	MaxRetries int

	// Handler is called for events sent to dead letter queue
	Handler func(event shared.Event, err error)
}

// DefaultDeadLetterConfig returns a sensible default dead letter config
func DefaultDeadLetterConfig() DeadLetterConfig {
	return DeadLetterConfig{
		Enabled:    true,
		MaxRetries: 5,
		Handler:    func(event shared.Event, err error) {}, // no-op by default
	}
}

// ListenerConfig configures listener behavior
type ListenerConfig struct {
	// Concurrency limits concurrent event processing for this listener
	Concurrency int

	// RetryPolicy defines retry behavior
	RetryPolicy RetryPolicy

	// Circuit configures circuit breaker
	Circuit CircuitConfig

	// DeadLetter configures dead letter queue
	DeadLetter DeadLetterConfig

	// Timeout for individual event processing
	Timeout time.Duration

	// Name identifies this listener (for metrics/logging)
	Name string
}

// DefaultListenerConfig returns a sensible default configuration
func DefaultListenerConfig() ListenerConfig {
	return ListenerConfig{
		Concurrency: 10,
		RetryPolicy: DefaultRetryPolicy(),
		Circuit:     DefaultCircuitConfig(),
		DeadLetter:  DefaultDeadLetterConfig(),
		Timeout:     30 * time.Second,
		Name:        "listener",
	}
}

// HealthStatus represents listener health
type HealthStatus int

const (
	// Healthy indicates normal operation
	Healthy HealthStatus = iota
	// Degraded indicates some issues but still operational
	Degraded
	// Unhealthy indicates significant issues
	Unhealthy
	// CircuitOpen indicates circuit breaker is open
	CircuitOpen
)

// String returns the string representation of HealthStatus
func (h HealthStatus) String() string {
	switch h {
	case Healthy:
		return "healthy"
	case Degraded:
		return "degraded"
	case Unhealthy:
		return "unhealthy"
	case CircuitOpen:
		return "circuit-open"
	default:
		return "unknown"
	}
}

// ListenerManager defines the interface for managing event listeners
type ListenerManager interface {
	// Register adds a listener with configuration
	Register(listener shared.Listener, config ListenerConfig) error

	// Unregister removes a listener
	Unregister(listenerID string) error

	// Start begins processing events for all registered listeners
	Start(ctx context.Context) error

	// Stop gracefully stops processing events
	Stop(ctx context.Context) error

	// Health returns the overall health status
	Health() HealthStatus

	// Stats returns listener statistics
	Stats() Stats

	// GetListenerHealth returns health for a specific listener
	GetListenerHealth(listenerID string) (HealthStatus, error)

	// ProcessEvent processes an event with a specific listener (called by emitter/bus)
	ProcessEvent(ctx context.Context, listenerID string, event shared.Event) error
}

// Config configures the ListenerManager
type Config struct {
	// WorkerPool for event processing (required)
	WorkerPool *workerpool.Pool

	// MetricsCollector for observability (optional)
	MetricsCollector shared.MetricsCollector

	// Name identifies this manager instance
	Name string
}

// Stats provides listener manager statistics
type Stats struct {
	RegisteredListeners int64
	ActiveListeners     int64
	EventsProcessed     int64
	EventsRetried       int64
	EventsFailed        int64
	DeadLetterEvents    int64
	CircuitBreakers     int64
}

// listenerManager implements ListenerManager
type listenerManager struct {
	config      Config
	workerpool  *workerpool.Pool
	metrics     shared.MetricsCollector
	listeners   map[string]*managedListener
	listenersMu sync.RWMutex
	running     int64 // atomic boolean
	stopCh      chan struct{}
	wg          sync.WaitGroup
	stats       Stats
}

// managedListener wraps a listener with management capabilities
type managedListener struct {
	listener     shared.Listener
	config       ListenerConfig
	manager      *listenerManager
	concurrency  chan struct{} // semaphore for concurrency control
	circuit      *circuitBreaker
	retryQueue   chan retryJob
	deadLetterCh chan deadLetterEvent
	stats        listenerStats
	running      int64 // atomic boolean
	stopCh       chan struct{}
	wg           sync.WaitGroup
}

// listenerStats tracks per-listener statistics
type listenerStats struct {
	EventsProcessed   int64
	EventsSucceeded   int64
	EventsFailed      int64
	EventsRetried     int64
	DeadLetterEvents  int64
	CircuitBreaks     int64
	LastProcessedTime int64 // Unix nano timestamp
	LastErrorTime     int64 // Unix nano timestamp
	LastError         error
}

// retryJob represents a queued retry
type retryJob struct {
	ctx         context.Context // Parent context for propagation
	event       shared.Event
	attempt     int
	lastError   error
	scheduledAt time.Time
}

// deadLetterEvent represents an event sent to dead letter queue
type deadLetterEvent struct {
	event     shared.Event
	finalErr  error
	attempts  int
	timestamp time.Time
}

// circuitBreaker implements circuit breaker pattern
type circuitBreaker struct {
	config         CircuitConfig
	state          int64 // atomic: 0=closed, 1=open, 2=half-open
	failures       int64
	successes      int64
	lastFailure    int64 // timestamp
	lastTransition int64 // timestamp
	mu             sync.RWMutex
}

// New creates a new ListenerManager
func New(config Config) ListenerManager {
	if config.WorkerPool == nil {
		panic("WorkerPool is required")
	}
	if config.MetricsCollector == nil {
		config.MetricsCollector = shared.NoOpMetricsCollector{}
	}
	if config.Name == "" {
		config.Name = "listener-manager"
	}

	return &listenerManager{
		config:     config,
		workerpool: config.WorkerPool,
		metrics:    config.MetricsCollector,
		listeners:  make(map[string]*managedListener),
		stopCh:     make(chan struct{}),
	}
}

// Register adds a listener with configuration
func (lm *listenerManager) Register(listener shared.Listener, config ListenerConfig) error {
	lm.listenersMu.Lock()
	defer lm.listenersMu.Unlock()

	listenerID := listener.ID()
	if _, exists := lm.listeners[listenerID]; exists {
		return fmt.Errorf("listener %s already registered", listenerID)
	}

	// Apply defaults
	if config.Concurrency <= 0 {
		config.Concurrency = 10
	}
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}
	if config.Name == "" {
		config.Name = listenerID
	}

	ml := &managedListener{
		listener:     listener,
		config:       config,
		manager:      lm,
		concurrency:  make(chan struct{}, config.Concurrency),
		retryQueue:   make(chan retryJob, 1000), // Buffer for retries
		deadLetterCh: make(chan deadLetterEvent, 100),
		stopCh:       make(chan struct{}),
	}

	// Initialize circuit breaker if enabled
	if config.Circuit.Enabled {
		ml.circuit = &circuitBreaker{
			config: config.Circuit,
		}
	}

	lm.listeners[listenerID] = ml
	atomic.AddInt64(&lm.stats.RegisteredListeners, 1)

	return nil
}

// Unregister removes a listener
func (lm *listenerManager) Unregister(listenerID string) error {
	lm.listenersMu.Lock()
	defer lm.listenersMu.Unlock()

	ml, exists := lm.listeners[listenerID]
	if !exists {
		return shared.ErrListenerNotFound
	}

	// Stop the managed listener
	if atomic.LoadInt64(&ml.running) == 1 {
		close(ml.stopCh)
		ml.wg.Wait()
	}

	delete(lm.listeners, listenerID)
	atomic.AddInt64(&lm.stats.RegisteredListeners, -1)

	return nil
}

// Start begins processing events for all registered listeners
func (lm *listenerManager) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt64(&lm.running, 0, 1) {
		return fmt.Errorf("listener manager already running")
	}

	lm.listenersMu.RLock()
	listeners := make([]*managedListener, 0, len(lm.listeners))
	for _, ml := range lm.listeners {
		listeners = append(listeners, ml)
	}
	lm.listenersMu.RUnlock()

	// Start all managed listeners
	for _, ml := range listeners {
		if err := lm.startManagedListener(ml); err != nil {
			return fmt.Errorf("failed to start listener %s: %w", ml.listener.ID(), err)
		}
	}

	return nil
}

// Stop gracefully stops processing events
func (lm *listenerManager) Stop(ctx context.Context) error {
	if !atomic.CompareAndSwapInt64(&lm.running, 1, 0) {
		return nil // already stopped
	}

	// Signal all listeners to stop
	close(lm.stopCh)

	lm.listenersMu.RLock()
	var listeners []*managedListener
	for _, ml := range lm.listeners {
		listeners = append(listeners, ml)
	}
	lm.listenersMu.RUnlock()

	// Stop all managed listeners
	for _, ml := range listeners {
		close(ml.stopCh)
	}

	// Wait for all listeners to stop
	done := make(chan struct{})
	go func() {
		lm.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Health returns the overall health status
func (lm *listenerManager) Health() HealthStatus {
	lm.listenersMu.RLock()
	defer lm.listenersMu.RUnlock()

	if len(lm.listeners) == 0 {
		return Healthy // No listeners to manage
	}

	healthyCount := 0
	circuitOpenCount := 0

	for _, ml := range lm.listeners {
		health := lm.getListenerHealth(ml)
		switch health {
		case Healthy:
			healthyCount++
		case CircuitOpen:
			circuitOpenCount++
		}
	}

	totalListeners := len(lm.listeners)
	healthyRatio := float64(healthyCount) / float64(totalListeners)

	if circuitOpenCount > 0 {
		return CircuitOpen
	}
	if healthyRatio >= 0.8 {
		return Healthy
	}
	if healthyRatio >= 0.5 {
		return Degraded
	}
	return Unhealthy
}

// Stats returns listener statistics
func (lm *listenerManager) Stats() Stats {
	return Stats{
		RegisteredListeners: atomic.LoadInt64(&lm.stats.RegisteredListeners),
		ActiveListeners:     atomic.LoadInt64(&lm.stats.ActiveListeners),
		EventsProcessed:     atomic.LoadInt64(&lm.stats.EventsProcessed),
		EventsRetried:       atomic.LoadInt64(&lm.stats.EventsRetried),
		EventsFailed:        atomic.LoadInt64(&lm.stats.EventsFailed),
		DeadLetterEvents:    atomic.LoadInt64(&lm.stats.DeadLetterEvents),
		CircuitBreakers:     atomic.LoadInt64(&lm.stats.CircuitBreakers),
	}
}

// GetListenerHealth returns health for a specific listener
func (lm *listenerManager) GetListenerHealth(listenerID string) (HealthStatus, error) {
	lm.listenersMu.RLock()
	ml, exists := lm.listeners[listenerID]
	lm.listenersMu.RUnlock()

	if !exists {
		return Unhealthy, shared.ErrListenerNotFound
	}

	return lm.getListenerHealth(ml), nil
}

// ProcessEvent processes an event with a specific listener (called by emitter/bus)
func (lm *listenerManager) ProcessEvent(ctx context.Context, listenerID string, event shared.Event) error {
	lm.listenersMu.RLock()
	ml, exists := lm.listeners[listenerID]
	lm.listenersMu.RUnlock()

	if !exists {
		return shared.ErrListenerNotFound
	}

	return lm.processEventWithListener(ctx, ml, event)
}

// Helper methods

func (lm *listenerManager) startManagedListener(ml *managedListener) error {
	if !atomic.CompareAndSwapInt64(&ml.running, 0, 1) {
		return fmt.Errorf("listener already running")
	}

	// Start retry processor
	ml.wg.Add(1)
	go lm.processRetries(ml)

	// Start dead letter processor
	if ml.config.DeadLetter.Enabled {
		ml.wg.Add(1)
		go lm.processDeadLetters(ml)
	}

	atomic.AddInt64(&lm.stats.ActiveListeners, 1)
	return nil
}

func (lm *listenerManager) getListenerHealth(ml *managedListener) HealthStatus {
	// Check circuit breaker state
	if ml.circuit != nil && ml.circuit.isOpen() {
		return CircuitOpen
	}

	// Check recent activity
	now := time.Now()
	lastErrorTimeNano := atomic.LoadInt64(&ml.stats.LastErrorTime)
	if lastErrorTimeNano > 0 {
		timeSinceLastError := now.Sub(time.Unix(0, lastErrorTimeNano))
		if timeSinceLastError < time.Minute {
			return Degraded
		}
	}

	failures := atomic.LoadInt64(&ml.stats.EventsFailed)
	successes := atomic.LoadInt64(&ml.stats.EventsSucceeded)

	if failures == 0 || (successes > 0 && float64(failures)/float64(successes+failures) < 0.1) {
		return Healthy
	}

	return Degraded
}

func (lm *listenerManager) processEventWithListener(ctx context.Context, ml *managedListener, event shared.Event) error {
	// Check if circuit breaker allows the call
	if ml.circuit != nil && !ml.circuit.allowRequest() {
		return fmt.Errorf("circuit breaker open for listener %s", ml.listener.ID())
	}

	// Acquire concurrency slot
	select {
	case ml.concurrency <- struct{}{}:
		defer func() { <-ml.concurrency }()
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Submit to worker pool if would block
		return lm.workerpool.Submit(ctx, func(ctx context.Context) error {
			select {
			case ml.concurrency <- struct{}{}:
				defer func() { <-ml.concurrency }()
			case <-ctx.Done():
				return ctx.Err()
			}
			return lm.handleEventWithListener(ctx, ml, event)
		})
	}

	return lm.handleEventWithListener(ctx, ml, event)
}

func (lm *listenerManager) handleEventWithListener(ctx context.Context, ml *managedListener, event shared.Event) error {
	start := time.Now()
	err := ml.listener.Handle(ctx, event)
	duration := time.Since(start)

	// Update metrics
	lm.metrics.ObserveListenerDuration(ml.listener.ID(), duration)
	nowNano := time.Now().UnixNano()
	atomic.StoreInt64(&ml.stats.LastProcessedTime, nowNano)
	atomic.AddInt64(&ml.stats.EventsProcessed, 1)
	atomic.AddInt64(&lm.stats.EventsProcessed, 1)

	if err != nil {
		atomic.AddInt64(&ml.stats.EventsFailed, 1)
		atomic.AddInt64(&lm.stats.EventsFailed, 1)
		atomic.StoreInt64(&ml.stats.LastErrorTime, nowNano)
		ml.stats.LastError = err

		// Record failure in circuit breaker
		if ml.circuit != nil {
			ml.circuit.recordFailure()
		}

		// Check if error is retryable
		if lm.shouldRetry(ml, err, 1) {
			retryJob := retryJob{
				ctx:         ctx,
				event:       event,
				attempt:     1,
				lastError:   err,
				scheduledAt: time.Now().Add(lm.calculateDelay(ml.config.RetryPolicy, 1)),
			}

			select {
			case ml.retryQueue <- retryJob:
				atomic.AddInt64(&ml.stats.EventsRetried, 1)
				atomic.AddInt64(&lm.stats.EventsRetried, 1)
			default:
				// Retry queue full, send to dead letter
				lm.sendToDeadLetter(ml, event, err, 1)
			}
		} else {
			lm.sendToDeadLetter(ml, event, err, 1)
		}

		lm.metrics.IncEventsProcessed(ml.listener.ID(), "failed")
		return shared.NewListenerError(ml.listener.ID(), event, err)
	}

	// Success
	atomic.AddInt64(&ml.stats.EventsSucceeded, 1)
	if ml.circuit != nil {
		ml.circuit.recordSuccess()
	}

	lm.metrics.IncEventsProcessed(ml.listener.ID(), "success")
	return nil
}

func (lm *listenerManager) processRetries(ml *managedListener) {
	defer ml.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ml.stopCh:
			return
		case <-lm.stopCh:
			return
		case <-ticker.C:
			lm.processRetryQueue(ml)
		}
	}
}

func (lm *listenerManager) processRetryQueue(ml *managedListener) {
	now := time.Now()

	// Process ready retries
	for {
		select {
		case job := <-ml.retryQueue:
			if now.Before(job.scheduledAt) {
				// Not ready yet, put it back
				select {
				case ml.retryQueue <- job:
				default:
					// Queue full, send to dead letter
					lm.sendToDeadLetter(ml, job.event, job.lastError, job.attempt)
				}
				return
			}

			// Process retry
			lm.processRetryJob(ml, job)
		default:
			return
		}
	}
}

func (lm *listenerManager) processRetryJob(ml *managedListener, job retryJob) {
	start := time.Now()
	ctx := job.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	err := ml.listener.Handle(ctx, job.event)
	duration := time.Since(start)

	lm.metrics.ObserveListenerDuration(ml.listener.ID(), duration)
	atomic.AddInt64(&ml.stats.EventsProcessed, 1)

	if err != nil {
		atomic.AddInt64(&ml.stats.EventsFailed, 1)

		if ml.circuit != nil {
			ml.circuit.recordFailure()
		}

		// Check if we should retry again
		if lm.shouldRetry(ml, err, job.attempt+1) {
			nextJob := retryJob{
				ctx:         ctx,
				event:       job.event,
				attempt:     job.attempt + 1,
				lastError:   err,
				scheduledAt: time.Now().Add(lm.calculateDelay(ml.config.RetryPolicy, job.attempt+1)),
			}

			select {
			case ml.retryQueue <- nextJob:
				atomic.AddInt64(&ml.stats.EventsRetried, 1)
			default:
				lm.sendToDeadLetter(ml, job.event, err, job.attempt+1)
			}
		} else {
			lm.sendToDeadLetter(ml, job.event, err, job.attempt+1)
		}
	} else {
		// Success
		atomic.AddInt64(&ml.stats.EventsSucceeded, 1)
		if ml.circuit != nil {
			ml.circuit.recordSuccess()
		}
	}
}

func (lm *listenerManager) processDeadLetters(ml *managedListener) {
	defer ml.wg.Done()

	for {
		select {
		case <-ml.stopCh:
			return
		case <-lm.stopCh:
			return
		case dlEvent := <-ml.deadLetterCh:
			// Call dead letter handler
			if ml.config.DeadLetter.Handler != nil {
				ml.config.DeadLetter.Handler(dlEvent.event, dlEvent.finalErr)
			}
			atomic.AddInt64(&ml.stats.DeadLetterEvents, 1)
			atomic.AddInt64(&lm.stats.DeadLetterEvents, 1)
		}
	}
}

func (lm *listenerManager) shouldRetry(ml *managedListener, err error, attempt int) bool {
	policy := ml.config.RetryPolicy

	if attempt > policy.MaxAttempts {
		return false
	}

	// Check custom retry condition
	if policy.RetryCondition != nil {
		return policy.RetryCondition(err)
	}

	// Check retryable errors list
	if len(policy.RetryableErrors) > 0 {
		for _, retryableErr := range policy.RetryableErrors {
			if err == retryableErr {
				return true
			}
		}
		return false
	}

	// Default: retry all errors
	return true
}

func (lm *listenerManager) calculateDelay(policy RetryPolicy, attempt int) time.Duration {
	var delay time.Duration

	switch policy.Backoff {
	case FixedBackoff:
		delay = policy.InitialDelay
	case ExponentialBackoff:
		delay = time.Duration(float64(policy.InitialDelay) * float64(attempt*attempt))
	case LinearBackoff:
		delay = time.Duration(float64(policy.InitialDelay) * float64(attempt))
	default:
		delay = policy.InitialDelay
	}

	if delay > policy.MaxDelay {
		delay = policy.MaxDelay
	}

	return delay
}

func (lm *listenerManager) sendToDeadLetter(ml *managedListener, event shared.Event, err error, attempts int) {
	if !ml.config.DeadLetter.Enabled {
		return
	}

	dlEvent := deadLetterEvent{
		event:     event,
		finalErr:  err,
		attempts:  attempts,
		timestamp: time.Now(),
	}

	select {
	case ml.deadLetterCh <- dlEvent:
	default:
		// Dead letter queue full, drop event to prevent blocking
		lm.metrics.IncEventsDropped("listener", "dead_letter_queue_full")
	}
}

// Circuit breaker implementation

func (cb *circuitBreaker) isOpen() bool {
	state := atomic.LoadInt64(&cb.state)
	if state == 0 { // closed
		return false
	}
	if state == 1 { // open
		// Check if timeout has passed
		lastTransition := atomic.LoadInt64(&cb.lastTransition)
		if time.Since(time.Unix(0, lastTransition)) >= cb.config.Timeout {
			// Transition to half-open
			atomic.CompareAndSwapInt64(&cb.state, 1, 2)
		}
		return state == 1
	}
	// half-open
	return false
}

func (cb *circuitBreaker) allowRequest() bool {
	if cb == nil {
		return true
	}
	return !cb.isOpen()
}

func (cb *circuitBreaker) recordSuccess() {
	if cb == nil {
		return
	}

	successes := atomic.AddInt64(&cb.successes, 1)
	state := atomic.LoadInt64(&cb.state)

	if state == 2 && int(successes) >= cb.config.SuccessThreshold { // half-open -> closed
		atomic.StoreInt64(&cb.state, 0)
		atomic.StoreInt64(&cb.failures, 0)
		atomic.StoreInt64(&cb.successes, 0)
		atomic.StoreInt64(&cb.lastTransition, time.Now().UnixNano())
	}
}

func (cb *circuitBreaker) recordFailure() {
	if cb == nil {
		return
	}

	failures := atomic.AddInt64(&cb.failures, 1)
	atomic.StoreInt64(&cb.lastFailure, time.Now().UnixNano())

	state := atomic.LoadInt64(&cb.state)
	if state == 0 && int(failures) >= cb.config.FailureThreshold { // closed -> open
		atomic.StoreInt64(&cb.state, 1)
		atomic.StoreInt64(&cb.lastTransition, time.Now().UnixNano())
	} else if state == 2 { // half-open -> open
		atomic.StoreInt64(&cb.state, 1)
		atomic.StoreInt64(&cb.lastTransition, time.Now().UnixNano())
	}
}
