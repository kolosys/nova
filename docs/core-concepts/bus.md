# Event Bus

The `bus` package provides topic-based event routing with partitioning, pattern matching, and delivery guarantees.

**Import Path:** `github.com/kolosys/nova/bus`

## Overview

The EventBus adds a topic abstraction layer on top of basic event handling. Events are published to topics, and subscribers receive events from topics they're interested in.

```
┌──────────────┐         ┌──────────────────────────────┐
│  Publisher   │────────▶│           Topic              │
│              │         │  ┌────────┐ ┌────────┐      │
└──────────────┘         │  │ Part 0 │ │ Part 1 │ ...  │
                         │  └───┬────┘ └───┬────┘      │
                         └──────┼──────────┼───────────┘
                                │          │
                         ┌──────▼──────────▼──────┐
                         │     Subscribers        │
                         │  • handler-1           │
                         │  • handler-2           │
                         │  • pattern-subscriber  │
                         └────────────────────────┘
```

## Creating a Bus

```go
import (
    "github.com/kolosys/ion/workerpool"
    "github.com/kolosys/nova/bus"
)

pool := workerpool.New(4, 100)
defer pool.Close(ctx)

b := bus.New(bus.Config{
    WorkerPool:          pool,           // Required
    DefaultBufferSize:   1000,           // Default topic buffer
    DefaultPartitions:   4,              // Default partition count
    DefaultDeliveryMode: bus.AtLeastOnce,// Default delivery guarantee
    Name:                "main-bus",     // For metrics identification
})
defer b.Shutdown(ctx)
```

### Configuration Options

| Option                | Default       | Description                                |
| --------------------- | ------------- | ------------------------------------------ |
| `WorkerPool`          | required      | Ion workerpool for event processing        |
| `DefaultBufferSize`   | `1000`        | Buffer size for auto-created topics        |
| `DefaultPartitions`   | `1`           | Partition count for auto-created topics    |
| `DefaultDeliveryMode` | `AtLeastOnce` | Delivery guarantee for auto-created topics |
| `MetricsCollector`    | no-op         | Custom metrics implementation              |
| `EventValidator`      | default       | Custom event validation                    |
| `Name`                | `"bus"`       | Instance identifier for metrics            |

## Topics

Topics organize events by category. They can be created explicitly or auto-created on first use.

### Explicit Topic Creation

```go
err := b.CreateTopic("orders", bus.TopicConfig{
    BufferSize:     2000,
    Partitions:     8,
    Retention:      24 * time.Hour,
    DeliveryMode:   bus.ExactlyOnce,
    MaxConcurrency: 20,
    OrderingKey: func(e shared.Event) string {
        return e.Metadata()["customer_id"]
    },
})
```

### Topic Configuration

| Option           | Default       | Description                           |
| ---------------- | ------------- | ------------------------------------- |
| `BufferSize`     | `1000`        | Events buffered per partition         |
| `Partitions`     | `1`           | Number of parallel processing lanes   |
| `Retention`      | `24h`         | How long events are kept (for replay) |
| `DeliveryMode`   | `AtLeastOnce` | Delivery guarantee                    |
| `MaxConcurrency` | `10`          | Concurrent handlers per partition     |
| `OrderingKey`    | event ID      | Function to determine partition       |

### Auto-Created Topics

Topics are created automatically when you publish or subscribe:

```go
// Topic "notifications" is created with default config
b.Subscribe("notifications", listener)
b.Publish(ctx, "notifications", event)
```

### Topic Management

```go
// List all topics
topics := b.Topics()
for _, name := range topics {
    fmt.Println(name)
}

// Delete a topic
if err := b.DeleteTopic("old-topic"); err != nil {
    log.Printf("Delete failed: %v", err)
}
```

## Delivery Modes

Nova supports three delivery guarantees:

### AtMostOnce

Fire-and-forget delivery. Events may be lost but are never duplicated.

```go
b.CreateTopic("telemetry", bus.TopicConfig{
    DeliveryMode: bus.AtMostOnce,
})
```

Use for:

- Metrics and telemetry
- Non-critical notifications
- High-volume, low-value events

### AtLeastOnce

Guaranteed delivery with possible duplicates. Failed events are retried.

```go
b.CreateTopic("orders", bus.TopicConfig{
    DeliveryMode: bus.AtLeastOnce,
})
```

Use for:

- Business-critical events
- Events where idempotency is handled downstream
- Default choice for most applications

### ExactlyOnce

Guaranteed delivery without duplicates. Highest overhead.

```go
b.CreateTopic("payments", bus.TopicConfig{
    DeliveryMode: bus.ExactlyOnce,
})
```

Use for:

- Financial transactions
- Events with side effects that cannot be repeated
- When duplicate prevention is critical

## Partitioning

Partitions enable parallel processing while maintaining ordering within a partition.

### How Partitioning Works

```
Event with key "customer-123" ──┐
                                ├──▶ hash("customer-123") % 8 = 3 ──▶ Partition 3
Event with key "customer-123" ──┘

Event with key "customer-456" ─────▶ hash("customer-456") % 8 = 7 ──▶ Partition 7
```

### Ordering Keys

Events with the same ordering key are processed in order:

```go
b.CreateTopic("orders", bus.TopicConfig{
    Partitions: 8,
    OrderingKey: func(e shared.Event) string {
        // All orders from the same customer go to the same partition
        return e.Metadata()["customer_id"]
    },
})
```

### Partition Selection

The default uses the event ID. Customize based on your ordering needs:

```go
// By geographic region
OrderingKey: func(e shared.Event) string {
    return e.Metadata()["region"]
}

// By tenant
OrderingKey: func(e shared.Event) string {
    return e.Metadata()["tenant_id"]
}

// Random distribution (no ordering)
OrderingKey: func(e shared.Event) string {
    return fmt.Sprintf("%d", time.Now().UnixNano())
}
```

## Subscribing

### Direct Subscription

Subscribe to a specific topic:

```go
listener := shared.NewBaseListener("order-handler", processOrder)
sub := b.Subscribe("orders", listener)

// Unsubscribe when done
defer sub.Unsubscribe()
```

### Pattern Subscription

Subscribe to topics matching a regex pattern:

```go
// Subscribe to all user events
auditListener := shared.NewBaseListener("audit", logEvent)
b.SubscribePattern("user\\..*", auditListener)

// Subscribe to all events
allListener := shared.NewBaseListener("monitor", monitorEvent)
b.SubscribePattern(".*", allListener)

// Subscribe to specific patterns
orderListener := shared.NewBaseListener("orders", handleOrders)
b.SubscribePattern("order\\.(created|updated)", orderListener)
```

Pattern subscriptions:

- Apply to existing topics that match
- Automatically apply to new topics that match
- Use Go regex syntax

## Publishing

```go
event := shared.NewBaseEventWithMetadata(
    "order-123",
    "order.created",
    orderData,
    map[string]string{
        "customer_id": "cust-456",
        "region":      "us-west",
    },
)

if err := b.Publish(ctx, "orders", event); err != nil {
    switch {
    case errors.Is(err, shared.ErrBufferFull):
        // Apply backpressure
    case errors.Is(err, shared.ErrBusClosed):
        // Bus is shutting down
    default:
        log.Printf("Publish failed: %v", err)
    }
}
```

### Publish Errors

| Error             | Cause                    | Resolution        |
| ----------------- | ------------------------ | ----------------- |
| `ErrBusClosed`    | Bus is shutting down     | Stop publishing   |
| `ErrBufferFull`   | Partition buffer is full | Slow down or drop |
| `ValidationError` | Event failed validation  | Fix event data    |
| Context cancelled | Timeout or cancellation  | Retry or abort    |

## Statistics

```go
stats := b.Stats()

fmt.Printf("Events published: %d\n", stats.EventsPublished)
fmt.Printf("Events processed: %d\n", stats.EventsProcessed)
fmt.Printf("Active topics: %d\n", stats.ActiveTopics)
fmt.Printf("Active subscribers: %d\n", stats.ActiveSubscribers)
fmt.Printf("Failed events: %d\n", stats.FailedEvents)
fmt.Printf("Queued events: %d\n", stats.QueuedEvents)
```

## Graceful Shutdown

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

if err := b.Shutdown(ctx); err != nil {
    log.Printf("Shutdown timeout: %v", err)
}
```

During shutdown:

1. New publishes are rejected with `ErrBusClosed`
2. All partition queues are closed
3. In-flight events complete or timeout
4. All partition processors finish

## Common Patterns

### Topic Hierarchy

Organize topics in a hierarchy and use pattern subscriptions:

```go
// Topics
"user.created"
"user.updated"
"user.deleted"
"order.created"
"order.shipped"

// Subscribe to all user events
b.SubscribePattern("user\\..*", userHandler)

// Subscribe to all events (for audit)
b.SubscribePattern(".*", auditHandler)
```

### Event Fanout

Multiple consumers process the same event independently:

```go
// Each service subscribes to the same topic
b.Subscribe("order.created", inventoryHandler)
b.Subscribe("order.created", notificationHandler)
b.Subscribe("order.created", analyticsHandler)
```

### Event Filtering

Filter events in the handler:

```go
listener := shared.NewBaseListener("vip-handler", func(event shared.Event) error {
    if event.Metadata()["customer_tier"] != "vip" {
        return nil // Skip non-VIP events
    }
    return processVIPOrder(event)
})
```

### Dead Letter Topics

Handle failed events:

```go
deadLetterHandler := shared.NewBaseListener("dead-letter", func(event shared.Event) error {
    log.Printf("Dead letter: %s - %s", event.ID(), event.Metadata()["error"])
    // Store for manual review
    return nil
})
b.Subscribe("dead-letter", deadLetterHandler)

mainHandler := shared.NewBaseListenerWithErrorHandler(
    "main",
    processEvent,
    func(event shared.Event, err error) error {
        // Forward to dead letter topic
        if be, ok := event.(*shared.BaseEvent); ok {
            be.SetMetadata("error", err.Error())
        }
        return b.Publish(context.Background(), "dead-letter", event)
    },
)
```

## Further Reading

- [Shared Types](shared.md) — Event, Listener interfaces
- [Emitter](emitter.md) — Simpler direct emission
- [Listener Manager](listener.md) — Add resilience with retries and circuits
- [Memory Store](memory.md) — Persist and replay events
- [Best Practices](../advanced/best-practices.md) — Production recommendations
- [API Reference](../api-reference/bus.md) — Complete API documentation
