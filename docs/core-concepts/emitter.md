# Event Emitter

The `emitter` package provides direct event emission with synchronous and asynchronous processing, middleware support, and concurrency control.

**Import Path:** `github.com/kolosys/nova/emitter`

## Overview

The EventEmitter is the simplest way to publish and subscribe to events in Nova. It maps event types to listeners directly, without the topic abstraction of the Bus.

```
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│    Event     │───▶│   Emitter    │───▶│  Listeners   │
│  user.created│    │  middleware  │    │  handler-1   │
└──────────────┘    │  validation  │    │  handler-2   │
                    │  concurrency │    └──────────────┘
                    └──────────────┘
```

## Creating an Emitter

Every emitter requires an Ion workerpool:

```go
import (
    "github.com/kolosys/ion/workerpool"
    "github.com/kolosys/nova/emitter"
)

pool := workerpool.New(4, 100)
defer pool.Close(ctx)

em := emitter.New(emitter.Config{
    WorkerPool:     pool,           // Required
    AsyncMode:      true,           // Use async by default for batches
    BufferSize:     1000,           // Async queue size
    MaxConcurrency: 10,             // Max concurrent handlers per subscription
    Name:           "main-emitter", // For metrics identification
})
defer em.Shutdown(ctx)
```

### Configuration Options

| Option             | Default     | Description                                |
| ------------------ | ----------- | ------------------------------------------ |
| `WorkerPool`       | required    | Ion workerpool for async processing        |
| `AsyncMode`        | `false`     | Default mode for batch operations          |
| `BufferSize`       | `1000`      | Size of the async event queue              |
| `MaxConcurrency`   | `10`        | Max concurrent processing per subscription |
| `MetricsCollector` | no-op       | Custom metrics implementation              |
| `EventValidator`   | default     | Custom event validation                    |
| `Name`             | `"emitter"` | Instance identifier for metrics            |

## Subscribing to Events

Subscribe listeners to specific event types:

```go
// Create a listener
listener := shared.NewBaseListener("user-handler", func(event shared.Event) error {
    userData := event.Data().(map[string]any)
    fmt.Printf("User: %s\n", userData["name"])
    return nil
})

// Subscribe returns a Subscription
sub := em.Subscribe("user.created", listener)

// Multiple listeners can subscribe to the same event type
auditListener := shared.NewBaseListener("audit-logger", logEvent)
em.Subscribe("user.created", auditListener)

// Unsubscribe when done
defer sub.Unsubscribe()
```

## Emitting Events

### Synchronous Emission

`Emit` processes the event immediately and blocks until all listeners complete:

```go
event := shared.NewBaseEvent("user-123", "user.created", userData)

if err := em.Emit(ctx, event); err != nil {
    log.Printf("Event processing failed: %v", err)
}
```

Use synchronous emission when:

- You need to know immediately if processing succeeded
- Ordering guarantees are important
- The processing is fast

### Asynchronous Emission

`EmitAsync` queues the event and returns immediately:

```go
if err := em.EmitAsync(ctx, event); err != nil {
    if errors.Is(err, shared.ErrBufferFull) {
        // Apply backpressure
    }
    log.Printf("Failed to queue event: %v", err)
}
```

Use asynchronous emission when:

- High throughput is needed
- Processing latency doesn't affect the caller
- You can tolerate eventual consistency

### Batch Emission

`EmitBatch` processes multiple events efficiently:

```go
events := []shared.Event{event1, event2, event3}

if err := em.EmitBatch(ctx, events); err != nil {
    log.Printf("Batch failed at: %v", err)
}
```

The batch respects the `AsyncMode` configuration.

## Middleware

Middleware intercepts events before and after processing:

```go
// Logging middleware
loggingMW := shared.MiddlewareFunc{
    BeforeFunc: func(event shared.Event) error {
        log.Printf("[→] %s (%s)", event.ID(), event.Type())
        return nil
    },
    AfterFunc: func(event shared.Event, err error) error {
        if err != nil {
            log.Printf("[✗] %s failed: %v", event.ID(), err)
        } else {
            log.Printf("[✓] %s completed", event.ID())
        }
        return nil
    },
}

// Tracing middleware
tracingMW := shared.MiddlewareFunc{
    BeforeFunc: func(event shared.Event) error {
        if be, ok := event.(*shared.BaseEvent); ok {
            be.SetMetadata("trace_start", time.Now().Format(time.RFC3339Nano))
        }
        return nil
    },
}

// Validation middleware
validationMW := shared.MiddlewareFunc{
    BeforeFunc: func(event shared.Event) error {
        if event.Data() == nil {
            return errors.New("event data required")
        }
        return nil
    },
}

// Add middleware (order matters - first added runs first)
em.Middleware(tracingMW, loggingMW, validationMW)
```

Middleware execution order:

1. `Before` runs in order of addition
2. Event is processed by listeners
3. `After` runs in reverse order (last added runs first)

## Concurrency Control

Each subscription has independent concurrency control:

```go
// Configure via emitter
em := emitter.New(emitter.Config{
    WorkerPool:     pool,
    MaxConcurrency: 20, // Each subscription can process 20 events concurrently
})
```

When concurrency is saturated:

- Synchronous emit blocks until a slot is available
- Asynchronous emit submits to the workerpool

## Statistics

Monitor emitter health and performance:

```go
stats := em.Stats()

fmt.Printf("Events emitted: %d\n", stats.EventsEmitted)
fmt.Printf("Events processed: %d\n", stats.EventsProcessed)
fmt.Printf("Active listeners: %d\n", stats.ActiveListeners)
fmt.Printf("Failed events: %d\n", stats.FailedEvents)
fmt.Printf("Queued events: %d\n", stats.QueuedEvents)
fmt.Printf("Middleware errors: %d\n", stats.MiddlewareErrors)
```

## Graceful Shutdown

Always shut down emitters properly:

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

if err := em.Shutdown(ctx); err != nil {
    log.Printf("Shutdown error: %v", err)
}
```

During shutdown:

1. New emissions are rejected with `ErrEmitterClosed`
2. Queued async events are processed
3. In-flight events complete or timeout

## Error Handling

### Validation Errors

Events are validated before processing:

```go
event := shared.NewBaseEvent("", "user.created", nil) // Empty ID

err := em.Emit(ctx, event)
// Returns: EventError containing ValidationError
```

### Listener Errors

Errors from listeners are wrapped with context:

```go
listener := shared.NewBaseListener("failing", func(event shared.Event) error {
    return errors.New("processing failed")
})

err := em.Emit(ctx, event)
// Returns: ListenerError with listener ID and event context
```

### Buffer Full

Async emission fails when the buffer is full:

```go
err := em.EmitAsync(ctx, event)
if errors.Is(err, shared.ErrBufferFull) {
    // Apply backpressure: slow down, retry later, or drop
}
```

## Common Patterns

### Fire and Forget

For non-critical events:

```go
go func() {
    _ = em.EmitAsync(ctx, event)
}()
```

### Request-Response Pattern

For synchronous workflows:

```go
resultCh := make(chan Result, 1)

listener := shared.NewBaseListener("responder", func(event shared.Event) error {
    result := processEvent(event)
    resultCh <- result
    return nil
})

sub := em.Subscribe(event.Type(), listener)
defer sub.Unsubscribe()

if err := em.Emit(ctx, event); err != nil {
    return nil, err
}

select {
case result := <-resultCh:
    return result, nil
case <-ctx.Done():
    return nil, ctx.Err()
}
```

### Fan-Out

Multiple listeners process the same event:

```go
for i := 0; i < 3; i++ {
    listener := shared.NewBaseListener(
        fmt.Sprintf("worker-%d", i),
        processEvent,
    )
    em.Subscribe("work.item", listener)
}
```

## Further Reading

- [Shared Types](shared.md) — Event, Listener, and Middleware interfaces
- [Bus](bus.md) — Topic-based routing for more complex scenarios
- [Listener Manager](listener.md) — Add resilience with retries and circuits
- [Best Practices](../advanced/best-practices.md) — Production recommendations
- [API Reference](../api-reference/emitter.md) — Complete API documentation
