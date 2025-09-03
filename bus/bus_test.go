package bus_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/kolosys/ion/workerpool"
	"github.com/kolosys/nova/bus"
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

func TestEventBus_New(t *testing.T) {
	pool := workerpool.New(2, 10, workerpool.WithName("test-pool"))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = pool.Close(ctx)
	}()

	config := bus.Config{
		WorkerPool:          pool,
		DefaultBufferSize:   100,
		DefaultPartitions:   2,
		DefaultDeliveryMode: bus.AtLeastOnce,
		Name:                "test-bus",
	}

	eventBus := bus.New(config)
	if eventBus == nil {
		t.Fatal("Expected event bus to be created")
	}

	// Test shutdown
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := eventBus.Shutdown(ctx); err != nil {
		t.Fatalf("Failed to shutdown event bus: %v", err)
	}
}

func TestEventBus_PublishSubscribe(t *testing.T) {
	pool := workerpool.New(2, 10, workerpool.WithName("test-pool"))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = pool.Close(ctx)
	}()

	metrics := shared.NewSimpleMetricsCollector()
	config := bus.Config{
		WorkerPool:          pool,
		DefaultBufferSize:   100,
		DefaultPartitions:   1,
		DefaultDeliveryMode: bus.AtLeastOnce,
		MetricsCollector:    metrics,
		Name:                "test-bus",
	}

	eventBus := bus.New(config)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = eventBus.Shutdown(ctx)
	}()

	// Create test listener
	listener := newTestListener("test-listener")

	// Subscribe to topic
	sub := eventBus.Subscribe("test.topic", listener)
	if sub == nil {
		t.Fatal("Expected subscription to be created")
	}

	// Create and publish event
	event := shared.NewBaseEvent("test-1", "test.event", "test data")
	ctx := context.Background()

	if err := eventBus.Publish(ctx, "test.topic", event); err != nil {
		t.Fatalf("Failed to publish event: %v", err)
	}

	// Wait for async processing
	time.Sleep(100 * time.Millisecond)

	// Verify event was received
	events := listener.GetEvents()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	if events[0].ID() != "test-1" {
		t.Errorf("Expected event ID 'test-1', got '%s'", events[0].ID())
	}

	// Check stats
	stats := eventBus.Stats()
	if stats.EventsPublished != 1 {
		t.Errorf("Expected 1 event published, got %d", stats.EventsPublished)
	}
}

func TestEventBus_CreateTopic(t *testing.T) {
	pool := workerpool.New(2, 10, workerpool.WithName("test-pool"))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = pool.Close(ctx)
	}()

	config := bus.Config{
		WorkerPool:        pool,
		DefaultBufferSize: 100,
		Name:              "test-bus",
	}

	eventBus := bus.New(config)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = eventBus.Shutdown(ctx)
	}()

	// Create topic with custom config
	topicConfig := bus.TopicConfig{
		BufferSize:     500,
		Partitions:     3,
		Retention:      time.Hour,
		DeliveryMode:   bus.ExactlyOnce,
		MaxConcurrency: 5,
		OrderingKey: func(e shared.Event) string {
			return e.Type()
		},
	}

	if err := eventBus.CreateTopic("custom.topic", topicConfig); err != nil {
		t.Fatalf("Failed to create topic: %v", err)
	}

	// Verify topic exists
	topics := eventBus.Topics()
	found := false
	for _, topic := range topics {
		if topic == "custom.topic" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected topic 'custom.topic' to exist")
	}

	// Try to create the same topic again (should fail)
	if err := eventBus.CreateTopic("custom.topic", topicConfig); err == nil {
		t.Error("Expected error when creating duplicate topic")
	}
}

func TestEventBus_SubscribePattern(t *testing.T) {
	pool := workerpool.New(2, 10, workerpool.WithName("test-pool"))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = pool.Close(ctx)
	}()

	config := bus.Config{
		WorkerPool:        pool,
		DefaultBufferSize: 100,
		DefaultPartitions: 1,
		Name:              "test-bus",
	}

	eventBus := bus.New(config)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = eventBus.Shutdown(ctx)
	}()

	// Create test listener
	listener := newTestListener("pattern-listener")

	// Subscribe to pattern (matches user.* topics)
	sub := eventBus.SubscribePattern("user\\..+", listener)
	if sub == nil {
		t.Fatal("Expected pattern subscription to be created")
	}

	ctx := context.Background()

	// Publish to matching topics
	event1 := shared.NewBaseEvent("test-1", "user.created", "user data")
	if err := eventBus.Publish(ctx, "user.created", event1); err != nil {
		t.Fatalf("Failed to publish to user.created: %v", err)
	}

	event2 := shared.NewBaseEvent("test-2", "user.updated", "user data")
	if err := eventBus.Publish(ctx, "user.updated", event2); err != nil {
		t.Fatalf("Failed to publish to user.updated: %v", err)
	}

	// Publish to non-matching topic
	event3 := shared.NewBaseEvent("test-3", "order.created", "order data")
	if err := eventBus.Publish(ctx, "order.created", event3); err != nil {
		t.Fatalf("Failed to publish to order.created: %v", err)
	}

	// Wait for async processing
	time.Sleep(200 * time.Millisecond)

	// Verify only matching events were received
	events := listener.GetEvents()
	if len(events) != 2 {
		t.Fatalf("Expected 2 events from pattern subscription, got %d", len(events))
	}

	// Verify event IDs
	eventIDs := make(map[string]bool)
	for _, event := range events {
		eventIDs[event.ID()] = true
	}

	if !eventIDs["test-1"] || !eventIDs["test-2"] {
		t.Error("Expected to receive test-1 and test-2 events from pattern subscription")
	}

	if eventIDs["test-3"] {
		t.Error("Should not have received test-3 event from pattern subscription")
	}
}

func TestEventBus_MultiplePartitions(t *testing.T) {
	pool := workerpool.New(4, 20, workerpool.WithName("test-pool"))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = pool.Close(ctx)
	}()

	config := bus.Config{
		WorkerPool:        pool,
		DefaultBufferSize: 100,
		DefaultPartitions: 3,
		Name:              "test-bus",
	}

	eventBus := bus.New(config)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = eventBus.Shutdown(ctx)
	}()

	// Create multiple listeners
	listener1 := newTestListener("listener-1")
	listener2 := newTestListener("listener-2")

	// Subscribe both listeners to the same topic
	sub1 := eventBus.Subscribe("multi.topic", listener1)
	sub2 := eventBus.Subscribe("multi.topic", listener2)

	if sub1 == nil || sub2 == nil {
		t.Fatal("Expected subscriptions to be created")
	}

	ctx := context.Background()

	// Publish multiple events with different ordering keys
	events := []shared.Event{
		shared.NewBaseEventWithMetadata("test-1", "test.event", "data 1", map[string]string{"key": "A"}),
		shared.NewBaseEventWithMetadata("test-2", "test.event", "data 2", map[string]string{"key": "B"}),
		shared.NewBaseEventWithMetadata("test-3", "test.event", "data 3", map[string]string{"key": "A"}),
		shared.NewBaseEventWithMetadata("test-4", "test.event", "data 4", map[string]string{"key": "C"}),
	}

	for _, event := range events {
		if err := eventBus.Publish(ctx, "multi.topic", event); err != nil {
			t.Fatalf("Failed to publish event %s: %v", event.ID(), err)
		}
	}

	// Wait for async processing
	time.Sleep(200 * time.Millisecond)

	// Verify both listeners received all events
	events1 := listener1.GetEvents()
	events2 := listener2.GetEvents()

	if len(events1) != 4 {
		t.Errorf("Expected listener1 to receive 4 events, got %d", len(events1))
	}
	if len(events2) != 4 {
		t.Errorf("Expected listener2 to receive 4 events, got %d", len(events2))
	}

	// Check stats
	stats := eventBus.Stats()
	if stats.EventsPublished != 4 {
		t.Errorf("Expected 4 events published, got %d", stats.EventsPublished)
	}
}

func TestEventBus_DeleteTopic(t *testing.T) {
	pool := workerpool.New(2, 10, workerpool.WithName("test-pool"))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = pool.Close(ctx)
	}()

	config := bus.Config{
		WorkerPool:        pool,
		DefaultBufferSize: 100,
		Name:              "test-bus",
	}

	eventBus := bus.New(config)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = eventBus.Shutdown(ctx)
	}()

	// Create topic
	topicConfig := bus.DefaultTopicConfig()
	if err := eventBus.CreateTopic("delete.topic", topicConfig); err != nil {
		t.Fatalf("Failed to create topic: %v", err)
	}

	// Verify topic exists
	topics := eventBus.Topics()
	if len(topics) != 1 || topics[0] != "delete.topic" {
		t.Errorf("Expected topic 'delete.topic' to exist, got topics: %v", topics)
	}

	// Delete topic
	if err := eventBus.DeleteTopic("delete.topic"); err != nil {
		t.Fatalf("Failed to delete topic: %v", err)
	}

	// Verify topic no longer exists
	topics = eventBus.Topics()
	if len(topics) != 0 {
		t.Errorf("Expected no topics after deletion, got: %v", topics)
	}

	// Try to delete non-existent topic
	if err := eventBus.DeleteTopic("nonexistent.topic"); err == nil {
		t.Error("Expected error when deleting non-existent topic")
	}
}

func TestEventBus_Unsubscribe(t *testing.T) {
	pool := workerpool.New(2, 10, workerpool.WithName("test-pool"))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = pool.Close(ctx)
	}()

	config := bus.Config{
		WorkerPool:        pool,
		DefaultBufferSize: 100,
		DefaultPartitions: 1,
		Name:              "test-bus",
	}

	eventBus := bus.New(config)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = eventBus.Shutdown(ctx)
	}()

	// Create test listener
	listener := newTestListener("test-listener")

	// Subscribe to topic
	sub := eventBus.Subscribe("unsub.topic", listener)
	if sub == nil {
		t.Fatal("Expected subscription to be created")
	}

	ctx := context.Background()

	// Publish first event
	event1 := shared.NewBaseEvent("test-1", "test.event", "data 1")
	if err := eventBus.Publish(ctx, "unsub.topic", event1); err != nil {
		t.Fatalf("Failed to publish first event: %v", err)
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Unsubscribe
	if err := sub.Unsubscribe(); err != nil {
		t.Fatalf("Failed to unsubscribe: %v", err)
	}

	// Publish second event
	event2 := shared.NewBaseEvent("test-2", "test.event", "data 2")
	if err := eventBus.Publish(ctx, "unsub.topic", event2); err != nil {
		t.Fatalf("Failed to publish second event: %v", err)
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Verify only first event was received
	events := listener.GetEvents()
	if len(events) != 1 {
		t.Fatalf("Expected 1 event after unsubscribe, got %d", len(events))
	}

	if events[0].ID() != "test-1" {
		t.Errorf("Expected event ID 'test-1', got '%s'", events[0].ID())
	}
}
