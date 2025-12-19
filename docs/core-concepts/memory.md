# Memory Store

The `memory` package provides an in-memory event store with stream management, replay capabilities, and live subscriptions.

**Import Path:** `github.com/kolosys/nova/memory`

## Overview

The EventStore persists events in memory, organized into streams. It supports reading historical events, replaying time ranges, and subscribing to live updates.

```
┌─────────────────────────────────────────────────────────────┐
│                       EventStore                            │
│                                                             │
│  ┌─────────────────┐  ┌─────────────────┐                  │
│  │   user-stream   │  │  order-stream   │  ...             │
│  │  ┌───┬───┬───┐  │  │  ┌───┬───┬───┐  │                  │
│  │  │ 1 │ 2 │ 3 │  │  │  │ 1 │ 2 │ 3 │  │                  │
│  │  └───┴───┴───┘  │  │  └───┴───┴───┘  │                  │
│  │      events     │  │      events     │                  │
│  └─────────────────┘  └─────────────────┘                  │
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              Subscriptions (live updates)            │   │
│  │  subscriber-1 ──▶ user-stream                       │   │
│  │  subscriber-2 ──▶ order-stream                      │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

## Creating a Store

```go
import "github.com/kolosys/nova/memory"

store := memory.New(memory.Config{
    MaxEventsPerStream: 100000,         // Events per stream before rejection
    MaxStreams:         10000,          // Maximum streams
    RetentionDuration:  24 * time.Hour, // Auto-cleanup old events
    SnapshotInterval:   time.Hour,      // Periodic snapshots
    Name:               "main-store",   // For metrics identification
})
defer store.Close()
```

### Configuration Options

| Option               | Default          | Description                           |
| -------------------- | ---------------- | ------------------------------------- |
| `MaxEventsPerStream` | `100000`         | Max events per stream (0 = unlimited) |
| `MaxStreams`         | `10000`          | Max number of streams (0 = unlimited) |
| `RetentionDuration`  | `0` (forever)    | Auto-cleanup events older than this   |
| `SnapshotInterval`   | `1h`             | How often to create snapshots         |
| `MetricsCollector`   | no-op            | Custom metrics implementation         |
| `Name`               | `"memory-store"` | Instance identifier for metrics       |

### Default Configuration

```go
config := memory.DefaultConfig()
// MaxEventsPerStream: 100000
// MaxStreams:         10000
// RetentionDuration:  0 (keep forever)
// SnapshotInterval:   1h
```

## Streams

Streams organize related events together. Each event belongs to exactly one stream.

### Stream Naming

Choose stream names based on your domain:

```go
// By entity
"user-123"          // Events for user 123
"order-456"         // Events for order 456

// By aggregate
"user-stream"       // All user events
"order-stream"      // All order events

// By category and ID
"users/123/events"
"orders/456/history"
```

### Stream Information

```go
// Get info for a specific stream
info, err := store.GetStreamInfo("user-stream")
if err != nil {
    log.Printf("Stream not found: %v", err)
}

fmt.Printf("Stream: %s\n", info.StreamID)
fmt.Printf("Events: %d\n", info.EventCount)
fmt.Printf("First event: %s\n", info.FirstEvent)
fmt.Printf("Last event: %s\n", info.LastEvent)
fmt.Printf("Last position: %d\n", info.LastPosition)

// List all streams
streams := store.GetStreams()
for _, s := range streams {
    fmt.Printf("%s: %d events\n", s.StreamID, s.EventCount)
}
```

## Appending Events

Add events to a stream:

```go
events := []shared.Event{
    shared.NewBaseEvent("evt-1", "user.created", userData1),
    shared.NewBaseEvent("evt-2", "user.updated", userData2),
}

if err := store.Append(ctx, "user-stream", events...); err != nil {
    log.Printf("Append failed: %v", err)
}
```

### Append Errors

| Error                | Cause                         | Resolution                           |
| -------------------- | ----------------------------- | ------------------------------------ |
| `ValidationError`    | Event failed validation       | Fix event data                       |
| Stream limit reached | `MaxEventsPerStream` exceeded | Archive old events or increase limit |
| Store closed         | Store has been closed         | Stop appending                       |

## Reading Events

### Read with Cursor

Read events sequentially using a cursor:

```go
// Start from the beginning
cursor := memory.Cursor{
    StreamID: "user-stream",
    Position: 0,
}

// Read 100 events
events, newCursor, err := store.Read(ctx, "user-stream", cursor, 100)
if err != nil {
    log.Printf("Read failed: %v", err)
}

fmt.Printf("Read %d events\n", len(events))
fmt.Printf("New position: %d\n", newCursor.Position)

// Continue reading from new position
moreEvents, finalCursor, _ := store.Read(ctx, "user-stream", newCursor, 100)
```

### Read Entire Stream

Read all events from a position:

```go
events, err := store.ReadStream(ctx, "user-stream", 0) // From position 0
if err != nil {
    log.Printf("Read failed: %v", err)
}

for _, event := range events {
    fmt.Printf("Event: %s (%s)\n", event.ID(), event.Type())
}
```

### Read Time Range

Read events across all streams within a time range:

```go
from := time.Now().Add(-24 * time.Hour)
to := time.Now()

events, err := store.ReadTimeRange(ctx, from, to)
if err != nil {
    log.Printf("Read failed: %v", err)
}

fmt.Printf("Found %d events in the last 24 hours\n", len(events))
```

## Cursors

Cursors track reading position in a stream:

```go
type Cursor struct {
    StreamID  string    // Stream identifier
    Position  int64     // Sequence number
    Timestamp time.Time // Timestamp at this position
}

// Create a cursor
cursor := memory.Cursor{
    StreamID: "user-stream",
    Position: 0,
}

// String representation
fmt.Println(cursor.String())
// "stream:user-stream pos:0 time:2024-01-01T00:00:00Z"
```

### Cursor Persistence

For durable subscriptions, persist cursors:

```go
// Save cursor position
func saveCursor(consumerID string, cursor memory.Cursor) error {
    return db.Update("cursors", consumerID, map[string]any{
        "stream_id": cursor.StreamID,
        "position":  cursor.Position,
    })
}

// Load cursor position
func loadCursor(consumerID, streamID string) memory.Cursor {
    row := db.Get("cursors", consumerID)
    if row == nil {
        return memory.Cursor{StreamID: streamID, Position: 0}
    }
    return memory.Cursor{
        StreamID: row["stream_id"].(string),
        Position: row["position"].(int64),
    }
}
```

## Replay

Replay historical events from a time range:

```go
from := time.Now().Add(-1 * time.Hour)
to := time.Now()

replayCh, err := store.Replay(ctx, from, to)
if err != nil {
    log.Printf("Replay failed: %v", err)
}

for event := range replayCh {
    fmt.Printf("Replaying: %s at %s\n", event.ID(), event.Timestamp())
    processEvent(event)
}
```

### Replay Use Cases

```go
// Rebuild read model after deployment
func rebuildReadModel() {
    from := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
    to := time.Now()

    replayCh, _ := store.Replay(ctx, from, to)
    for event := range replayCh {
        updateReadModel(event)
    }
}

// Debug specific time range
func debugTimeRange(from, to time.Time) {
    replayCh, _ := store.Replay(ctx, from, to)
    for event := range replayCh {
        log.Printf("DEBUG: %s %s %+v", event.ID(), event.Type(), event.Data())
    }
}

// Sync new service
func syncNewService(service Service) {
    replayCh, _ := store.Replay(ctx, time.Time{}, time.Now())
    for event := range replayCh {
        service.ApplyEvent(event)
    }
}
```

## Live Subscriptions

Subscribe to receive new events in real-time:

```go
cursor := memory.Cursor{StreamID: "user-stream", Position: 0}

liveCh, err := store.Subscribe(ctx, "user-stream", cursor)
if err != nil {
    log.Printf("Subscribe failed: %v", err)
}

// Process events as they arrive
go func() {
    for event := range liveCh {
        fmt.Printf("New event: %s\n", event.ID())
        processEvent(event)
    }
}()
```

### Subscription Behavior

1. Sends historical events from cursor position
2. Continues with live events as they're appended
3. Closes when context is cancelled or store closes

### Multiple Subscribers

Each subscriber gets its own channel:

```go
// Multiple consumers with independent cursors
sub1, _ := store.Subscribe(ctx, "orders", cursor1)
sub2, _ := store.Subscribe(ctx, "orders", cursor2)

go processConsumer("consumer-1", sub1)
go processConsumer("consumer-2", sub2)
```

## Retention

Configure automatic cleanup of old events:

```go
store := memory.New(memory.Config{
    RetentionDuration: 7 * 24 * time.Hour, // Keep 7 days
})
```

### Retention Behavior

- Events older than `RetentionDuration` are removed
- Cleanup runs periodically (10 times per retention period)
- Empty streams are removed automatically

### Manual Cleanup

For more control, implement manual cleanup:

```go
// Archive old events before cleanup
func archiveAndCleanup(streamID string, olderThan time.Time) error {
    events, _ := store.ReadTimeRange(ctx, time.Time{}, olderThan)

    if err := archiveToS3(events); err != nil {
        return err
    }

    // Note: memory store doesn't support deletion
    // Use retention or recreate the store
    return nil
}
```

## Snapshots

Snapshots capture store state for recovery:

```go
if err := store.Snapshot(ctx); err != nil {
    log.Printf("Snapshot failed: %v", err)
}
```

Automatic snapshots run at `SnapshotInterval`.

> **Note**: The current implementation increments a counter. Production implementations would serialize state to disk.

## Statistics

```go
stats := store.Stats()

fmt.Printf("Total events: %d\n", stats.TotalEvents)
fmt.Printf("Total streams: %d\n", stats.TotalStreams)
fmt.Printf("Events appended: %d\n", stats.EventsAppended)
fmt.Printf("Events read: %d\n", stats.EventsRead)
fmt.Printf("Active subscriptions: %d\n", stats.SubscriptionsActive)
fmt.Printf("Snapshots created: %d\n", stats.SnapshotsCreated)
fmt.Printf("Memory usage (est.): %d bytes\n", stats.MemoryUsageBytes)
```

## Graceful Shutdown

```go
if err := store.Close(); err != nil {
    log.Printf("Close failed: %v", err)
}
```

Closing:

1. Signals background workers to stop
2. Closes all active subscriptions
3. Waits for cleanup to complete

## Common Patterns

### Event Sourcing

```go
type UserAggregate struct {
    ID      string
    Name    string
    Email   string
    Version int64
}

func LoadUser(store memory.EventStore, userID string) (*UserAggregate, error) {
    streamID := fmt.Sprintf("user-%s", userID)
    events, err := store.ReadStream(ctx, streamID, 0)
    if err != nil {
        return nil, err
    }

    user := &UserAggregate{ID: userID}
    for _, event := range events {
        user.Apply(event)
    }
    return user, nil
}

func (u *UserAggregate) Apply(event shared.Event) {
    switch event.Type() {
    case "user.created":
        data := event.Data().(map[string]any)
        u.Name = data["name"].(string)
        u.Email = data["email"].(string)
    case "user.updated":
        data := event.Data().(map[string]any)
        if name, ok := data["name"]; ok {
            u.Name = name.(string)
        }
    }
    u.Version++
}
```

### CQRS Read Model

```go
type OrderSummary struct {
    OrderID     string
    CustomerID  string
    Total       float64
    Status      string
}

var orderSummaries = make(map[string]*OrderSummary)

func UpdateReadModel(event shared.Event) {
    switch event.Type() {
    case "order.created":
        data := event.Data().(map[string]any)
        orderSummaries[event.ID()] = &OrderSummary{
            OrderID:    event.ID(),
            CustomerID: data["customer_id"].(string),
            Total:      data["total"].(float64),
            Status:     "pending",
        }
    case "order.completed":
        if order, ok := orderSummaries[event.ID()]; ok {
            order.Status = "completed"
        }
    }
}

// Rebuild on startup
func RebuildReadModel() {
    replayCh, _ := store.Replay(ctx, time.Time{}, time.Now())
    for event := range replayCh {
        UpdateReadModel(event)
    }
}
```

### Audit Trail

```go
func CreateAuditListener(store memory.EventStore) shared.Listener {
    return shared.NewBaseListener("audit", func(event shared.Event) error {
        streamID := fmt.Sprintf("audit-%s", time.Now().Format("2006-01-02"))
        return store.Append(ctx, streamID, event)
    })
}

// Query audit trail
func GetAuditTrail(date time.Time) ([]shared.Event, error) {
    streamID := fmt.Sprintf("audit-%s", date.Format("2006-01-02"))
    return store.ReadStream(ctx, streamID, 0)
}
```

## Further Reading

- [Shared Types](shared.md) — Event interface
- [Emitter](emitter.md) — Event emission
- [Bus](bus.md) — Topic-based routing
- [Listener Manager](listener.md) — Resilience patterns
- [Best Practices](../advanced/best-practices.md) — Production recommendations
- [API Reference](../api-reference/memory.md) — Complete API documentation
