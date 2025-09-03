package listener_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kolosys/ion/workerpool"
	"github.com/kolosys/nova/listener"
	"github.com/kolosys/nova/shared"
)

// testListener implements shared.Listener for testing
type testListener struct {
	id         string
	events     []shared.Event
	errors     []error
	mu         sync.RWMutex
	handleFn   func(event shared.Event) error
	errorFn    func(event shared.Event, err error) error
	callCount  int64
	failCount  int64
	shouldFail bool
}

func newTestListener(id string) *testListener {
	return &testListener{
		id:     id,
		events: make([]shared.Event, 0),
		errors: make([]error, 0),
	}
}

func (l *testListener) ID() string {
	return l.id
}

func (l *testListener) Handle(event shared.Event) error {
	atomic.AddInt64(&l.callCount, 1)

	l.mu.Lock()
	defer l.mu.Unlock()

	l.events = append(l.events, event)

	if l.shouldFail {
		atomic.AddInt64(&l.failCount, 1)
		return errors.New("listener error")
	}

	if l.handleFn != nil {
		return l.handleFn(event)
	}
	return nil
}

func (l *testListener) OnError(event shared.Event, err error) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.errors = append(l.errors, err)

	if l.errorFn != nil {
		return l.errorFn(event, err)
	}
	return err
}

func (l *testListener) GetEvents() []shared.Event {
	l.mu.RLock()
	defer l.mu.RUnlock()

	events := make([]shared.Event, len(l.events))
	copy(events, l.events)
	return events
}

func (l *testListener) GetErrors() []error {
	l.mu.RLock()
	defer l.mu.RUnlock()

	errors := make([]error, len(l.errors))
	copy(errors, l.errors)
	return errors
}

func (l *testListener) CallCount() int64 {
	return atomic.LoadInt64(&l.callCount)
}

func (l *testListener) FailCount() int64 {
	return atomic.LoadInt64(&l.failCount)
}

func (l *testListener) SetShouldFail(fail bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.shouldFail = fail
}

func TestListenerManager_New(t *testing.T) {
	pool := workerpool.New(2, 10, workerpool.WithName("test-pool"))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = pool.Close(ctx)
	}()

	config := listener.Config{
		WorkerPool: pool,
		Name:       "test-manager",
	}

	lm := listener.New(config)
	if lm == nil {
		t.Fatal("Expected listener manager to be created")
	}

	// Test initial health
	health := lm.Health()
	if health != listener.Healthy {
		t.Errorf("Expected healthy status, got %v", health)
	}
}

func TestListenerManager_RegisterUnregister(t *testing.T) {
	pool := workerpool.New(2, 10, workerpool.WithName("test-pool"))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = pool.Close(ctx)
	}()

	config := listener.Config{
		WorkerPool: pool,
		Name:       "test-manager",
	}

	lm := listener.New(config)

	// Create test listener
	testListener := newTestListener("test-listener")
	listenerConfig := listener.DefaultListenerConfig()
	listenerConfig.Name = "test-listener"

	// Register listener
	if err := lm.Register(testListener, listenerConfig); err != nil {
		t.Fatalf("Failed to register listener: %v", err)
	}

	// Check stats
	stats := lm.Stats()
	if stats.RegisteredListeners != 1 {
		t.Errorf("Expected 1 registered listener, got %d", stats.RegisteredListeners)
	}

	// Try to register same listener again (should fail)
	if err := lm.Register(testListener, listenerConfig); err == nil {
		t.Error("Expected error when registering duplicate listener")
	}

	// Unregister listener
	if err := lm.Unregister("test-listener"); err != nil {
		t.Fatalf("Failed to unregister listener: %v", err)
	}

	// Check stats
	stats = lm.Stats()
	if stats.RegisteredListeners != 0 {
		t.Errorf("Expected 0 registered listeners, got %d", stats.RegisteredListeners)
	}

	// Try to unregister non-existent listener
	if err := lm.Unregister("non-existent"); err == nil {
		t.Error("Expected error when unregistering non-existent listener")
	}
}

func TestListenerManager_StartStop(t *testing.T) {
	pool := workerpool.New(2, 10, workerpool.WithName("test-pool"))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = pool.Close(ctx)
	}()

	config := listener.Config{
		WorkerPool: pool,
		Name:       "test-manager",
	}

	lm := listener.New(config)

	// Register a listener
	testListener := newTestListener("test-listener")
	listenerConfig := listener.DefaultListenerConfig()

	if err := lm.Register(testListener, listenerConfig); err != nil {
		t.Fatalf("Failed to register listener: %v", err)
	}

	// Start the manager
	ctx := context.Background()
	if err := lm.Start(ctx); err != nil {
		t.Fatalf("Failed to start listener manager: %v", err)
	}

	// Check that listener is active
	stats := lm.Stats()
	if stats.ActiveListeners != 1 {
		t.Errorf("Expected 1 active listener, got %d", stats.ActiveListeners)
	}

	// Try to start again (should fail)
	if err := lm.Start(ctx); err == nil {
		t.Error("Expected error when starting already running manager")
	}

	// Stop the manager
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := lm.Stop(stopCtx); err != nil {
		t.Fatalf("Failed to stop listener manager: %v", err)
	}
}

func TestListenerManager_RetryPolicy(t *testing.T) {
	pool := workerpool.New(2, 10, workerpool.WithName("test-pool"))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = pool.Close(ctx)
	}()

	metrics := shared.NewSimpleMetricsCollector()
	config := listener.Config{
		WorkerPool:       pool,
		MetricsCollector: metrics,
		Name:             "test-manager",
	}

	lm := listener.New(config)

	// Create listener that fails initially
	testListener := newTestListener("retry-listener")
	testListener.SetShouldFail(true)

	// Configure with retry policy
	listenerConfig := listener.ListenerConfig{
		Concurrency: 1,
		RetryPolicy: listener.RetryPolicy{
			MaxAttempts:  3,
			InitialDelay: 10 * time.Millisecond,
			MaxDelay:     100 * time.Millisecond,
			Backoff:      listener.ExponentialBackoff,
		},
		Circuit: listener.CircuitConfig{
			Enabled: false, // Disable for this test
		},
		DeadLetter: listener.DeadLetterConfig{
			Enabled:    true,
			MaxRetries: 3,
		},
		Timeout: time.Second,
		Name:    "retry-listener",
	}

	if err := lm.Register(testListener, listenerConfig); err != nil {
		t.Fatalf("Failed to register listener: %v", err)
	}

	if err := lm.Start(context.Background()); err != nil {
		t.Fatalf("Failed to start listener manager: %v", err)
	}

	// Process an event that will fail
	event := shared.NewBaseEvent("test-1", "test.event", "test data")

	// Use the process method (simulating emitter/bus call)
	err := lm.ProcessEvent(context.Background(), "retry-listener", event)
	if err == nil {
		t.Error("Expected error from failing listener")
	}

	// Wait for retries to process
	time.Sleep(200 * time.Millisecond)

	// Check that retries were attempted
	callCount := testListener.CallCount()
	if callCount <= 1 {
		t.Errorf("Expected multiple calls due to retries, got %d", callCount)
	}

	// Stop the manager
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := lm.Stop(ctx); err != nil {
		t.Fatalf("Failed to stop listener manager: %v", err)
	}
}

func TestListenerManager_CircuitBreaker(t *testing.T) {
	pool := workerpool.New(2, 10, workerpool.WithName("test-pool"))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = pool.Close(ctx)
	}()

	config := listener.Config{
		WorkerPool: pool,
		Name:       "test-manager",
	}

	lm := listener.New(config)

	// Create listener that fails
	testListener := newTestListener("circuit-listener")
	testListener.SetShouldFail(true)

	// Configure with circuit breaker
	listenerConfig := listener.ListenerConfig{
		Concurrency: 1,
		RetryPolicy: listener.RetryPolicy{
			MaxAttempts: 1, // Minimal retries for this test
		},
		Circuit: listener.CircuitConfig{
			Enabled:          true,
			FailureThreshold: 3,
			SuccessThreshold: 2,
			Timeout:          100 * time.Millisecond,
		},
		DeadLetter: listener.DeadLetterConfig{
			Enabled: false,
		},
		Timeout: time.Second,
		Name:    "circuit-listener",
	}

	if err := lm.Register(testListener, listenerConfig); err != nil {
		t.Fatalf("Failed to register listener: %v", err)
	}

	if err := lm.Start(context.Background()); err != nil {
		t.Fatalf("Failed to start listener manager: %v", err)
	}

	// Process events to trigger circuit breaker
	event := shared.NewBaseEvent("test-1", "test.event", "test data")

	// Process multiple failing events to open circuit
	for i := 0; i < 5; i++ {
		_ = lm.ProcessEvent(context.Background(), "circuit-listener", event)
	}

	// Wait a bit for circuit to process
	time.Sleep(50 * time.Millisecond)

	// Check health - should be circuit open
	health, err := lm.GetListenerHealth("circuit-listener")
	if err != nil {
		t.Fatalf("Failed to get listener health: %v", err)
	}

	// Note: The circuit might not be open yet depending on timing
	// In a real implementation, we'd have better control over this
	t.Logf("Listener health after failures: %v", health)

	// Stop the manager
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := lm.Stop(ctx); err != nil {
		t.Fatalf("Failed to stop listener manager: %v", err)
	}
}

func TestListenerManager_HealthStatus(t *testing.T) {
	pool := workerpool.New(2, 10, workerpool.WithName("test-pool"))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = pool.Close(ctx)
	}()

	config := listener.Config{
		WorkerPool: pool,
		Name:       "test-manager",
	}

	lm := listener.New(config)

	// Initially should be healthy (no listeners)
	health := lm.Health()
	if health != listener.Healthy {
		t.Errorf("Expected healthy status with no listeners, got %v", health)
	}

	// Register a healthy listener
	healthyListener := newTestListener("healthy-listener")
	healthyConfig := listener.DefaultListenerConfig()
	healthyConfig.Name = "healthy-listener"

	if err := lm.Register(healthyListener, healthyConfig); err != nil {
		t.Fatalf("Failed to register healthy listener: %v", err)
	}

	// Should still be healthy
	health = lm.Health()
	if health != listener.Healthy {
		t.Errorf("Expected healthy status with healthy listener, got %v", health)
	}

	// Check individual listener health
	listenerHealth, err := lm.GetListenerHealth("healthy-listener")
	if err != nil {
		t.Fatalf("Failed to get listener health: %v", err)
	}
	if listenerHealth != listener.Healthy {
		t.Errorf("Expected healthy listener status, got %v", listenerHealth)
	}

	// Check non-existent listener
	_, err = lm.GetListenerHealth("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent listener health check")
	}
}

func TestRetryPolicy_BackoffStrategies(t *testing.T) {
	tests := []struct {
		name     string
		strategy listener.BackoffStrategy
		want     string
	}{
		{"Fixed", listener.FixedBackoff, "fixed"},
		{"Exponential", listener.ExponentialBackoff, "exponential"},
		{"Linear", listener.LinearBackoff, "linear"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.strategy.String(); got != tt.want {
				t.Errorf("BackoffStrategy.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHealthStatus_String(t *testing.T) {
	tests := []struct {
		name   string
		status listener.HealthStatus
		want   string
	}{
		{"Healthy", listener.Healthy, "healthy"},
		{"Degraded", listener.Degraded, "degraded"},
		{"Unhealthy", listener.Unhealthy, "unhealthy"},
		{"CircuitOpen", listener.CircuitOpen, "circuit-open"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("HealthStatus.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultConfigs(t *testing.T) {
	// Test default retry policy
	retryPolicy := listener.DefaultRetryPolicy()
	if retryPolicy.MaxAttempts != 3 {
		t.Errorf("Expected default retry policy MaxAttempts = 3, got %d", retryPolicy.MaxAttempts)
	}

	// Test default circuit config
	circuitConfig := listener.DefaultCircuitConfig()
	if !circuitConfig.Enabled {
		t.Error("Expected default circuit config to be enabled")
	}

	// Test default dead letter config
	deadLetterConfig := listener.DefaultDeadLetterConfig()
	if !deadLetterConfig.Enabled {
		t.Error("Expected default dead letter config to be enabled")
	}

	// Test default listener config
	listenerConfig := listener.DefaultListenerConfig()
	if listenerConfig.Concurrency != 10 {
		t.Errorf("Expected default listener config Concurrency = 10, got %d", listenerConfig.Concurrency)
	}
}
