package emitter_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/kolosys/ion/workerpool"
	"github.com/kolosys/nova/emitter"
	"github.com/kolosys/nova/shared"
)

// testListener implements shared.Listener for testing
type testListener struct {
	id       string
	events   []shared.Event
	errors   []error
	mu       sync.RWMutex
	handleFn func(event shared.Event) error
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
	l.mu.Lock()
	defer l.mu.Unlock()

	l.events = append(l.events, event)

	if l.handleFn != nil {
		return l.handleFn(event)
	}
	return nil
}

func (l *testListener) OnError(event shared.Event, err error) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.errors = append(l.errors, err)
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

func (l *testListener) EventCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.events)
}

func TestEmitter_New(t *testing.T) {
	pool := workerpool.New(2, 10, workerpool.WithName("test-pool"))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = pool.Close(ctx)
	}()

	config := emitter.Config{
		WorkerPool: pool,
		AsyncMode:  false,
		BufferSize: 100,
		Name:       "test-emitter",
	}

	em := emitter.New(config)
	if em == nil {
		t.Fatal("Expected emitter to be created")
	}

	// Test shutdown
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := em.Shutdown(ctx); err != nil {
		t.Fatalf("Failed to shutdown emitter: %v", err)
	}
}

func TestEmitter_EmitSync(t *testing.T) {
	pool := workerpool.New(2, 10, workerpool.WithName("test-pool"))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = pool.Close(ctx)
	}()

	metrics := shared.NewSimpleMetricsCollector()
	config := emitter.Config{
		WorkerPool:       pool,
		AsyncMode:        false,
		BufferSize:       100,
		Name:             "test-emitter",
		MetricsCollector: metrics,
	}

	em := emitter.New(config)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = em.Shutdown(ctx)
	}()

	// Create test listener
	listener := newTestListener("test-listener")

	// Subscribe to events
	sub := em.Subscribe("test.event", listener)
	if sub == nil {
		t.Fatal("Expected subscription to be created")
	}

	// Create and emit event
	event := shared.NewBaseEvent("test-1", "test.event", "test data")
	ctx := context.Background()

	if err := em.Emit(ctx, event); err != nil {
		t.Fatalf("Failed to emit event: %v", err)
	}

	// Verify event was received
	events := listener.GetEvents()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	if events[0].ID() != "test-1" {
		t.Errorf("Expected event ID 'test-1', got '%s'", events[0].ID())
	}

	// Check stats
	stats := em.Stats()
	if stats.EventsEmitted != 1 {
		t.Errorf("Expected 1 event emitted, got %d", stats.EventsEmitted)
	}
	if stats.EventsProcessed != 1 {
		t.Errorf("Expected 1 event processed, got %d", stats.EventsProcessed)
	}

	// Verify metrics
	if metrics.GetEventsEmitted() != 1 {
		t.Errorf("Expected 1 event in metrics, got %d", metrics.GetEventsEmitted())
	}
	if metrics.GetEventsProcessed() != 1 {
		t.Errorf("Expected 1 processed event in metrics, got %d", metrics.GetEventsProcessed())
	}
}

func TestEmitter_EmitAsync(t *testing.T) {
	pool := workerpool.New(2, 10, workerpool.WithName("test-pool"))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = pool.Close(ctx)
	}()

	metrics := shared.NewSimpleMetricsCollector()
	config := emitter.Config{
		WorkerPool:       pool,
		AsyncMode:        true,
		BufferSize:       100,
		Name:             "test-emitter",
		MetricsCollector: metrics,
	}

	em := emitter.New(config)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = em.Shutdown(ctx)
	}()

	// Create test listener
	listener := newTestListener("test-listener")

	// Subscribe to events
	sub := em.Subscribe("test.event", listener)
	if sub == nil {
		t.Fatal("Expected subscription to be created")
	}

	// Create and emit event asynchronously
	event := shared.NewBaseEvent("test-1", "test.event", "test data")
	ctx := context.Background()

	if err := em.EmitAsync(ctx, event); err != nil {
		t.Fatalf("Failed to emit event: %v", err)
	}

	// Wait a bit for async processing
	time.Sleep(100 * time.Millisecond)

	// Verify event was received
	events := listener.GetEvents()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	if events[0].ID() != "test-1" {
		t.Errorf("Expected event ID 'test-1', got '%s'", events[0].ID())
	}
}

func TestEmitter_MultipleListeners(t *testing.T) {
	pool := workerpool.New(2, 10, workerpool.WithName("test-pool"))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = pool.Close(ctx)
	}()

	config := emitter.Config{
		WorkerPool: pool,
		AsyncMode:  false,
		BufferSize: 100,
		Name:       "test-emitter",
	}

	em := emitter.New(config)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = em.Shutdown(ctx)
	}()

	// Create multiple listeners
	listener1 := newTestListener("listener-1")
	listener2 := newTestListener("listener-2")

	// Subscribe both listeners to the same event type
	sub1 := em.Subscribe("test.event", listener1)
	sub2 := em.Subscribe("test.event", listener2)

	if sub1 == nil || sub2 == nil {
		t.Fatal("Expected subscriptions to be created")
	}

	// Create and emit event
	event := shared.NewBaseEvent("test-1", "test.event", "test data")
	ctx := context.Background()

	if err := em.Emit(ctx, event); err != nil {
		t.Fatalf("Failed to emit event: %v", err)
	}

	// Verify both listeners received the event
	events1 := listener1.GetEvents()
	events2 := listener2.GetEvents()

	if len(events1) != 1 {
		t.Errorf("Expected listener1 to receive 1 event, got %d", len(events1))
	}
	if len(events2) != 1 {
		t.Errorf("Expected listener2 to receive 1 event, got %d", len(events2))
	}

	// Check stats
	stats := em.Stats()
	if stats.EventsEmitted != 1 {
		t.Errorf("Expected 1 event emitted, got %d", stats.EventsEmitted)
	}
	if stats.EventsProcessed != 2 {
		t.Errorf("Expected 2 events processed, got %d", stats.EventsProcessed)
	}
}

func TestEmitter_Unsubscribe(t *testing.T) {
	pool := workerpool.New(2, 10, workerpool.WithName("test-pool"))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = pool.Close(ctx)
	}()

	config := emitter.Config{
		WorkerPool: pool,
		AsyncMode:  false,
		BufferSize: 100,
		Name:       "test-emitter",
	}

	em := emitter.New(config)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = em.Shutdown(ctx)
	}()

	// Create test listener
	listener := newTestListener("test-listener")

	// Subscribe to events
	sub := em.Subscribe("test.event", listener)
	if sub == nil {
		t.Fatal("Expected subscription to be created")
	}

	// Emit first event
	event1 := shared.NewBaseEvent("test-1", "test.event", "test data")
	ctx := context.Background()

	if err := em.Emit(ctx, event1); err != nil {
		t.Fatalf("Failed to emit first event: %v", err)
	}

	// Unsubscribe
	if err := sub.Unsubscribe(); err != nil {
		t.Fatalf("Failed to unsubscribe: %v", err)
	}

	// Emit second event
	event2 := shared.NewBaseEvent("test-2", "test.event", "test data")

	if err := em.Emit(ctx, event2); err != nil {
		t.Fatalf("Failed to emit second event: %v", err)
	}

	// Verify only first event was received
	events := listener.GetEvents()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event after unsubscribe, got %d", len(events))
	}

	if events[0].ID() != "test-1" {
		t.Errorf("Expected event ID 'test-1', got '%s'", events[0].ID())
	}
}

func TestEmitter_EmitBatch(t *testing.T) {
	pool := workerpool.New(2, 10, workerpool.WithName("test-pool"))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = pool.Close(ctx)
	}()

	config := emitter.Config{
		WorkerPool: pool,
		AsyncMode:  false,
		BufferSize: 100,
		Name:       "test-emitter",
	}

	em := emitter.New(config)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = em.Shutdown(ctx)
	}()

	// Create test listener
	listener := newTestListener("test-listener")

	// Subscribe to events
	em.Subscribe("test.event", listener)

	// Create batch of events
	events := []shared.Event{
		shared.NewBaseEvent("test-1", "test.event", "data 1"),
		shared.NewBaseEvent("test-2", "test.event", "data 2"),
		shared.NewBaseEvent("test-3", "test.event", "data 3"),
	}

	ctx := context.Background()

	if err := em.EmitBatch(ctx, events); err != nil {
		t.Fatalf("Failed to emit batch: %v", err)
	}

	// Verify all events were received
	receivedEvents := listener.GetEvents()
	if len(receivedEvents) != 3 {
		t.Fatalf("Expected 3 events, got %d", len(receivedEvents))
	}

	// Verify event order
	expectedIDs := []string{"test-1", "test-2", "test-3"}
	for i, event := range receivedEvents {
		if event.ID() != expectedIDs[i] {
			t.Errorf("Expected event %d to have ID '%s', got '%s'", i, expectedIDs[i], event.ID())
		}
	}
}
