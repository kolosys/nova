# Quick Start

Build a working event system in minutes. This guide covers the essential patterns for using Nova.

## Basic Event Emission

The simplest way to use Nova is with the Emitter for direct event handling:

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/kolosys/ion/workerpool"
    "github.com/kolosys/nova/emitter"
    "github.com/kolosys/nova/shared"
)

func main() {
    ctx := context.Background()

    // Create an Ion workerpool (required by all Nova components)
    pool := workerpool.New(4, 100)
    defer pool.Close(ctx)

    // Create the emitter
    em := emitter.New(emitter.Config{
        WorkerPool: pool,
        BufferSize: 1000,
    })
    defer em.Shutdown(ctx)

    // Create a listener
    listener := shared.NewBaseListener("user-handler", func(event shared.Event) error {
        data := event.Data().(map[string]any)
        fmt.Printf("User created: %s (%s)\n", data["name"], event.ID())
        return nil
    })

    // Subscribe to events
    em.Subscribe("user.created", listener)

    // Create and emit an event
    event := shared.NewBaseEvent("user-123", "user.created", map[string]any{
        "name":  "Alice",
        "email": "alice@example.com",
    })

    if err := em.Emit(ctx, event); err != nil {
        log.Fatal(err)
    }
}
```

## Async Event Processing

For non-blocking event emission, use `EmitAsync`:

```go
// Configure for async mode
em := emitter.New(emitter.Config{
    WorkerPool: pool,
    AsyncMode:  true,
    BufferSize: 1000,
})

// Events are queued and processed in the background
if err := em.EmitAsync(ctx, event); err != nil {
    log.Printf("Failed to queue event: %v", err)
}

// Batch emit multiple events
events := []shared.Event{event1, event2, event3}
if err := em.EmitBatch(ctx, events); err != nil {
    log.Printf("Batch failed: %v", err)
}
```

## Topic-Based Routing with Bus

The EventBus provides topic-based routing with partitioning and delivery guarantees:

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/kolosys/ion/workerpool"
    "github.com/kolosys/nova/bus"
    "github.com/kolosys/nova/shared"
)

func main() {
    ctx := context.Background()

    pool := workerpool.New(4, 100)
    defer pool.Close(ctx)

    // Create the event bus
    b := bus.New(bus.Config{
        WorkerPool:          pool,
        DefaultPartitions:   4,
        DefaultDeliveryMode: bus.AtLeastOnce,
    })
    defer b.Shutdown(ctx)

    // Create a topic with specific configuration
    b.CreateTopic("orders", bus.TopicConfig{
        BufferSize:   2000,
        Partitions:   8,
        DeliveryMode: bus.ExactlyOnce,
        Retention:    24 * time.Hour,
        OrderingKey:  func(e shared.Event) string {
            return e.Metadata()["customer_id"]
        },
    })

    // Subscribe to topics
    orderHandler := shared.NewBaseListener("order-processor", func(event shared.Event) error {
        fmt.Printf("Processing order: %s\n", event.ID())
        return nil
    })
    b.Subscribe("orders", orderHandler)

    // Pattern-based subscription (regex)
    auditHandler := shared.NewBaseListener("audit-logger", func(event shared.Event) error {
        fmt.Printf("Audit: %s on %s\n", event.Type(), event.ID())
        return nil
    })
    b.SubscribePattern("order\\..*", auditHandler)

    // Publish events
    event := shared.NewBaseEventWithMetadata(
        "order-456",
        "order.created",
        map[string]any{"total": 99.99},
        map[string]string{"customer_id": "cust-789"},
    )

    if err := b.Publish(ctx, "orders", event); err != nil {
        fmt.Printf("Publish failed: %v\n", err)
    }
}
```

## Resilient Listeners

The ListenerManager adds retry policies, circuit breakers, and dead letter queues:

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/kolosys/ion/workerpool"
    "github.com/kolosys/nova/listener"
    "github.com/kolosys/nova/shared"
)

func main() {
    ctx := context.Background()

    pool := workerpool.New(4, 100)
    defer pool.Close(ctx)

    // Create the listener manager
    lm := listener.New(listener.Config{WorkerPool: pool})
    defer lm.Stop(ctx)

    // Create a listener
    handler := shared.NewBaseListener("payment-processor", func(event shared.Event) error {
        // Process payment...
        return nil
    })

    // Register with resilience configuration
    lm.Register(handler, listener.ListenerConfig{
        Concurrency: 10,
        Timeout:     30 * time.Second,
        RetryPolicy: listener.RetryPolicy{
            MaxAttempts:  3,
            InitialDelay: 100 * time.Millisecond,
            MaxDelay:     30 * time.Second,
            Backoff:      listener.ExponentialBackoff,
        },
        Circuit: listener.CircuitConfig{
            Enabled:          true,
            FailureThreshold: 5,
            SuccessThreshold: 3,
            Timeout:          30 * time.Second,
        },
        DeadLetter: listener.DeadLetterConfig{
            Enabled: true,
            Handler: func(event shared.Event, err error) {
                fmt.Printf("Dead letter: %s - %v\n", event.ID(), err)
            },
        },
    })

    // Start processing
    lm.Start(ctx)

    // Check health
    fmt.Printf("Health: %s\n", lm.Health())
}
```

## Event Store with Replay

The memory store enables event persistence, replay, and live subscriptions:

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/kolosys/nova/memory"
    "github.com/kolosys/nova/shared"
)

func main() {
    ctx := context.Background()

    // Create the event store
    store := memory.New(memory.Config{
        MaxEventsPerStream: 100000,
        RetentionDuration:  24 * time.Hour,
    })
    defer store.Close()

    // Append events to a stream
    events := []shared.Event{
        shared.NewBaseEvent("evt-1", "user.created", map[string]any{"name": "Alice"}),
        shared.NewBaseEvent("evt-2", "user.updated", map[string]any{"name": "Alice Smith"}),
    }
    store.Append(ctx, "user-stream", events...)

    // Read events from a stream
    cursor := memory.Cursor{StreamID: "user-stream", Position: 0}
    readEvents, newCursor, _ := store.Read(ctx, "user-stream", cursor, 100)
    fmt.Printf("Read %d events, cursor at position %d\n", len(readEvents), newCursor.Position)

    // Replay historical events
    from := time.Now().Add(-1 * time.Hour)
    to := time.Now()
    replayCh, _ := store.Replay(ctx, from, to)
    for event := range replayCh {
        fmt.Printf("Replayed: %s\n", event.ID())
    }

    // Subscribe to live events
    liveCh, _ := store.Subscribe(ctx, "user-stream", cursor)
    go func() {
        for event := range liveCh {
            fmt.Printf("Live event: %s\n", event.ID())
        }
    }()
}
```

## Adding Middleware

The Emitter supports middleware for cross-cutting concerns:

```go
// Create logging middleware
loggingMiddleware := shared.MiddlewareFunc{
    BeforeFunc: func(event shared.Event) error {
        fmt.Printf("[LOG] Processing event: %s (%s)\n", event.ID(), event.Type())
        return nil
    },
    AfterFunc: func(event shared.Event, err error) error {
        if err != nil {
            fmt.Printf("[LOG] Event %s failed: %v\n", event.ID(), err)
        } else {
            fmt.Printf("[LOG] Event %s completed\n", event.ID())
        }
        return nil
    },
}

em.Middleware(loggingMiddleware)
```

## Graceful Shutdown

Always shut down components gracefully:

```go
// Create a context with timeout for shutdown
shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

// Shutdown in reverse order of creation
if err := em.Shutdown(shutdownCtx); err != nil {
    log.Printf("Emitter shutdown error: %v", err)
}

if err := b.Shutdown(shutdownCtx); err != nil {
    log.Printf("Bus shutdown error: %v", err)
}

if err := lm.Stop(shutdownCtx); err != nil {
    log.Printf("Listener manager shutdown error: %v", err)
}

if err := store.Close(); err != nil {
    log.Printf("Store close error: %v", err)
}

pool.Close(shutdownCtx)
```

## Next Steps

- [Core Concepts](../core-concepts/shared.md) — understand the fundamental types
- [Emitter](../core-concepts/emitter.md) — deep dive into event emission
- [Bus](../core-concepts/bus.md) — topic-based routing details
- [Listener](../core-concepts/listener.md) — resilience patterns
- [Memory Store](../core-concepts/memory.md) — event storage and replay
- [Best Practices](../advanced/best-practices.md) — production recommendations
