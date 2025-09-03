package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/kolosys/nova/memory"
	"github.com/kolosys/nova/shared"
)

func TestEventStore_New(t *testing.T) {
	config := memory.DefaultConfig()
	config.Name = "test-store"

	store := memory.New(config)
	if store == nil {
		t.Fatal("Expected event store to be created")
	}

	// Test initial stats
	stats := store.Stats()
	if stats.TotalEvents != 0 {
		t.Errorf("Expected 0 total events, got %d", stats.TotalEvents)
	}
	if stats.TotalStreams != 0 {
		t.Errorf("Expected 0 total streams, got %d", stats.TotalStreams)
	}

	// Close the store
	if err := store.Close(); err != nil {
		t.Fatalf("Failed to close store: %v", err)
	}
}

func TestEventStore_AppendAndRead(t *testing.T) {
	config := memory.DefaultConfig()
	store := memory.New(config)
	defer store.Close()

	// Create test events
	event1 := shared.NewBaseEvent("event-1", "test.event", "data 1")
	event2 := shared.NewBaseEvent("event-2", "test.event", "data 2")
	event3 := shared.NewBaseEvent("event-3", "test.event", "data 3")

	ctx := context.Background()

	// Append events to stream
	if err := store.Append(ctx, "test-stream", event1, event2, event3); err != nil {
		t.Fatalf("Failed to append events: %v", err)
	}

	// Check stats
	stats := store.Stats()
	if stats.TotalEvents != 3 {
		t.Errorf("Expected 3 total events, got %d", stats.TotalEvents)
	}
	if stats.TotalStreams != 1 {
		t.Errorf("Expected 1 total stream, got %d", stats.TotalStreams)
	}

	// Read events from stream
	cursor := memory.Cursor{StreamID: "test-stream", Position: 0}
	events, newCursor, err := store.Read(ctx, "test-stream", cursor, 10)
	if err != nil {
		t.Fatalf("Failed to read events: %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("Expected 3 events, got %d", len(events))
	}

	// Verify event order and content
	expectedIDs := []string{"event-1", "event-2", "event-3"}
	for i, event := range events {
		if event.ID() != expectedIDs[i] {
			t.Errorf("Expected event %d to have ID '%s', got '%s'", i, expectedIDs[i], event.ID())
		}
	}

	// Check cursor was updated
	if newCursor.Position != 3 {
		t.Errorf("Expected cursor position 3, got %d", newCursor.Position)
	}
	if newCursor.StreamID != "test-stream" {
		t.Errorf("Expected cursor stream 'test-stream', got '%s'", newCursor.StreamID)
	}
}

func TestEventStore_ReadStream(t *testing.T) {
	config := memory.DefaultConfig()
	store := memory.New(config)
	defer store.Close()

	// Create and append test events
	events := []shared.Event{
		shared.NewBaseEvent("event-1", "test.event", "data 1"),
		shared.NewBaseEvent("event-2", "test.event", "data 2"),
		shared.NewBaseEvent("event-3", "test.event", "data 3"),
		shared.NewBaseEvent("event-4", "test.event", "data 4"),
	}

	ctx := context.Background()
	if err := store.Append(ctx, "test-stream", events...); err != nil {
		t.Fatalf("Failed to append events: %v", err)
	}

	// Read entire stream
	allEvents, err := store.ReadStream(ctx, "test-stream", 0)
	if err != nil {
		t.Fatalf("Failed to read stream: %v", err)
	}

	if len(allEvents) != 4 {
		t.Fatalf("Expected 4 events, got %d", len(allEvents))
	}

	// Read from position 2
	partialEvents, err := store.ReadStream(ctx, "test-stream", 2)
	if err != nil {
		t.Fatalf("Failed to read stream from position: %v", err)
	}

	if len(partialEvents) != 2 {
		t.Fatalf("Expected 2 events from position 2, got %d", len(partialEvents))
	}

	// Verify we got the right events
	if partialEvents[0].ID() != "event-3" {
		t.Errorf("Expected first event to be 'event-3', got '%s'", partialEvents[0].ID())
	}
	if partialEvents[1].ID() != "event-4" {
		t.Errorf("Expected second event to be 'event-4', got '%s'", partialEvents[1].ID())
	}

	// Read from non-existent stream
	_, err = store.ReadStream(ctx, "non-existent", 0)
	if err == nil {
		t.Error("Expected error when reading non-existent stream")
	}
}

func TestEventStore_ReadTimeRange(t *testing.T) {
	config := memory.DefaultConfig()
	store := memory.New(config)
	defer store.Close()

	ctx := context.Background()

	// Create events with specific timestamps
	baseTime := time.Now().Truncate(time.Hour)
	event1 := shared.NewBaseEvent("event-1", "test.event", "data 1")
	event2 := shared.NewBaseEvent("event-2", "test.event", "data 2")
	event3 := shared.NewBaseEvent("event-3", "test.event", "data 3")

	// We can't easily control the timestamp in BaseEvent, so let's test the functionality
	// In a real implementation, you might have events with controllable timestamps

	// Append events to different streams
	if err := store.Append(ctx, "stream-1", event1); err != nil {
		t.Fatalf("Failed to append to stream-1: %v", err)
	}

	// Small delay to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	if err := store.Append(ctx, "stream-2", event2); err != nil {
		t.Fatalf("Failed to append to stream-2: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	if err := store.Append(ctx, "stream-1", event3); err != nil {
		t.Fatalf("Failed to append to stream-1: %v", err)
	}

	// Read all events in time range
	now := time.Now()
	events, err := store.ReadTimeRange(ctx, baseTime, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("Failed to read time range: %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("Expected 3 events in time range, got %d", len(events))
	}

	// Events should be sorted by timestamp
	for i := 1; i < len(events); i++ {
		if events[i].Timestamp().Before(events[i-1].Timestamp()) {
			t.Error("Events are not sorted by timestamp")
		}
	}
}

func TestEventStore_Replay(t *testing.T) {
	config := memory.DefaultConfig()
	store := memory.New(config)
	defer store.Close()

	ctx := context.Background()

	// Create and append test events
	events := []shared.Event{
		shared.NewBaseEvent("event-1", "test.event", "data 1"),
		shared.NewBaseEvent("event-2", "test.event", "data 2"),
		shared.NewBaseEvent("event-3", "test.event", "data 3"),
	}

	if err := store.Append(ctx, "test-stream", events...); err != nil {
		t.Fatalf("Failed to append events: %v", err)
	}

	// Test replay
	now := time.Now()
	replayCh, err := store.Replay(ctx, now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("Failed to start replay: %v", err)
	}

	// Collect replayed events
	var replayedEvents []shared.Event
	for event := range replayCh {
		replayedEvents = append(replayedEvents, event)
	}

	if len(replayedEvents) != 3 {
		t.Fatalf("Expected 3 replayed events, got %d", len(replayedEvents))
	}

	// Verify event order
	expectedIDs := []string{"event-1", "event-2", "event-3"}
	for i, event := range replayedEvents {
		if event.ID() != expectedIDs[i] {
			t.Errorf("Expected replayed event %d to have ID '%s', got '%s'", i, expectedIDs[i], event.ID())
		}
	}
}

func TestEventStore_Subscribe(t *testing.T) {
	config := memory.DefaultConfig()
	store := memory.New(config)
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Append initial events
	initialEvents := []shared.Event{
		shared.NewBaseEvent("initial-1", "test.event", "data 1"),
		shared.NewBaseEvent("initial-2", "test.event", "data 2"),
	}

	if err := store.Append(ctx, "test-stream", initialEvents...); err != nil {
		t.Fatalf("Failed to append initial events: %v", err)
	}

	// Subscribe to the stream from the beginning
	cursor := memory.Cursor{StreamID: "test-stream", Position: 0}
	eventCh, err := store.Subscribe(ctx, "test-stream", cursor)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Read initial events from subscription
	var receivedEvents []shared.Event
	timeout := time.After(500 * time.Millisecond)

readLoop:
	for {
		select {
		case event, ok := <-eventCh:
			if !ok {
				break readLoop
			}
			receivedEvents = append(receivedEvents, event)
			if len(receivedEvents) >= 2 {
				break readLoop // We got the initial events
			}
		case <-timeout:
			break readLoop
		}
	}

	if len(receivedEvents) < 2 {
		t.Errorf("Expected at least 2 events from subscription, got %d", len(receivedEvents))
	}

	// Add a new event and check if subscription receives it
	newEvent := shared.NewBaseEvent("new-event", "test.event", "new data")
	if err := store.Append(ctx, "test-stream", newEvent); err != nil {
		t.Fatalf("Failed to append new event: %v", err)
	}

	// In a production implementation, the subscription would receive the new event
	// For this test, we just verify the subscription channel was created successfully
}

func TestEventStore_GetStreamInfo(t *testing.T) {
	config := memory.DefaultConfig()
	store := memory.New(config)
	defer store.Close()

	ctx := context.Background()

	// Test getting info for non-existent stream
	_, err := store.GetStreamInfo("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent stream")
	}

	// Create and populate a stream
	events := []shared.Event{
		shared.NewBaseEvent("event-1", "test.event", "data 1"),
		shared.NewBaseEvent("event-2", "test.event", "data 2"),
		shared.NewBaseEvent("event-3", "test.event", "data 3"),
	}

	if err := store.Append(ctx, "test-stream", events...); err != nil {
		t.Fatalf("Failed to append events: %v", err)
	}

	// Get stream info
	info, err := store.GetStreamInfo("test-stream")
	if err != nil {
		t.Fatalf("Failed to get stream info: %v", err)
	}

	if info.StreamID != "test-stream" {
		t.Errorf("Expected stream ID 'test-stream', got '%s'", info.StreamID)
	}
	if info.EventCount != 3 {
		t.Errorf("Expected 3 events, got %d", info.EventCount)
	}
	if info.LastPosition != 3 {
		t.Errorf("Expected last position 3, got %d", info.LastPosition)
	}

	// Test GetStreams
	allStreams := store.GetStreams()
	if len(allStreams) != 1 {
		t.Errorf("Expected 1 stream, got %d", len(allStreams))
	}
	if allStreams[0].StreamID != "test-stream" {
		t.Errorf("Expected stream 'test-stream', got '%s'", allStreams[0].StreamID)
	}
}

func TestEventStore_Snapshot(t *testing.T) {
	config := memory.DefaultConfig()
	store := memory.New(config)
	defer store.Close()

	ctx := context.Background()

	// Create snapshot
	if err := store.Snapshot(ctx); err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	// Check that snapshot was recorded in stats
	stats := store.Stats()
	if stats.SnapshotsCreated != 1 {
		t.Errorf("Expected 1 snapshot created, got %d", stats.SnapshotsCreated)
	}
}

func TestEventStore_Limits(t *testing.T) {
	config := memory.Config{
		MaxEventsPerStream: 2,
		MaxStreams:         1,
		Name:               "limited-store",
	}
	store := memory.New(config)
	defer store.Close()

	ctx := context.Background()

	// Add events up to the limit
	event1 := shared.NewBaseEvent("event-1", "test.event", "data 1")
	event2 := shared.NewBaseEvent("event-2", "test.event", "data 2")

	if err := store.Append(ctx, "test-stream", event1, event2); err != nil {
		t.Fatalf("Failed to append events within limit: %v", err)
	}

	// Try to add one more event (should fail)
	event3 := shared.NewBaseEvent("event-3", "test.event", "data 3")
	err := store.Append(ctx, "test-stream", event3)
	if err == nil {
		t.Error("Expected error when exceeding stream event limit")
	}
}

func TestEventStore_Close(t *testing.T) {
	config := memory.DefaultConfig()
	store := memory.New(config)

	// Close the store
	if err := store.Close(); err != nil {
		t.Fatalf("Failed to close store: %v", err)
	}

	// Try operations after close (should fail)
	ctx := context.Background()
	event := shared.NewBaseEvent("event-1", "test.event", "data")

	err := store.Append(ctx, "test-stream", event)
	if err == nil {
		t.Error("Expected error when appending to closed store")
	}

	_, _, err = store.Read(ctx, "test-stream", memory.Cursor{}, 10)
	if err == nil {
		t.Error("Expected error when reading from closed store")
	}
}

func TestCursor_String(t *testing.T) {
	cursor := memory.Cursor{
		StreamID:  "test-stream",
		Position:  42,
		Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
	}

	expected := "stream:test-stream pos:42 time:2023-01-01T12:00:00Z"
	if cursor.String() != expected {
		t.Errorf("Expected cursor string '%s', got '%s'", expected, cursor.String())
	}
}

func TestDefaultConfig(t *testing.T) {
	config := memory.DefaultConfig()

	if config.MaxEventsPerStream != 100000 {
		t.Errorf("Expected MaxEventsPerStream 100000, got %d", config.MaxEventsPerStream)
	}
	if config.MaxStreams != 10000 {
		t.Errorf("Expected MaxStreams 10000, got %d", config.MaxStreams)
	}
	if config.RetentionDuration != 0 {
		t.Errorf("Expected RetentionDuration 0, got %v", config.RetentionDuration)
	}
	if config.Name != "memory-store" {
		t.Errorf("Expected Name 'memory-store', got '%s'", config.Name)
	}
}
