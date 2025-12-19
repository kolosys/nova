# Shared Types

The `shared` package provides the foundational types and interfaces used throughout Nova. All other packages depend on these core abstractions.

**Import Path:** `github.com/kolosys/nova/shared`

## Event Interface

Events are the fundamental unit of data in Nova. The `Event` interface defines what every event must provide:

```go
type Event interface {
    ID() string                    // Unique identifier
    Type() string                  // Event type (e.g., "user.created")
    Timestamp() time.Time          // When the event occurred
    Data() any                     // Event payload
    Metadata() map[string]string   // Routing, tracing, and context
}
```

### BaseEvent

Nova provides `BaseEvent` as a ready-to-use implementation:

```go
// Create a simple event
event := shared.NewBaseEvent("user-123", "user.created", map[string]any{
    "name":  "Alice",
    "email": "alice@example.com",
})

// Create an event with metadata
event := shared.NewBaseEventWithMetadata(
    "order-456",
    "order.completed",
    orderData,
    map[string]string{
        "customer_id": "cust-789",
        "trace_id":    "abc123",
    },
)

// Add metadata after creation
event.SetMetadata("partition_key", "us-west-2")

// Read metadata
if traceID, ok := event.GetMetadata("trace_id"); ok {
    // Use trace ID
}
```

### Custom Events

Implement the `Event` interface for domain-specific events:

```go
type OrderEvent struct {
    id        string
    orderID   string
    items     []OrderItem
    total     float64
    createdAt time.Time
}

func (e *OrderEvent) ID() string                  { return e.id }
func (e *OrderEvent) Type() string                { return "order.created" }
func (e *OrderEvent) Timestamp() time.Time        { return e.createdAt }
func (e *OrderEvent) Data() any                   { return e }
func (e *OrderEvent) Metadata() map[string]string { return nil }
```

## Listener Interface

Listeners process events. The interface is intentionally minimal:

```go
type Listener interface {
    ID() string                           // Unique identifier
    Handle(event Event) error             // Process an event
    OnError(event Event, err error) error // Handle errors from Handle()
}
```

### BaseListener

Use `BaseListener` for quick listener creation:

```go
// Simple listener
listener := shared.NewBaseListener("order-processor", func(event shared.Event) error {
    order := event.Data().(map[string]any)
    fmt.Printf("Processing order: %v\n", order)
    return nil
})

// Listener with error handler
listener := shared.NewBaseListenerWithErrorHandler(
    "payment-processor",
    func(event shared.Event) error {
        return processPayment(event)
    },
    func(event shared.Event, err error) error {
        log.Printf("Payment failed for %s: %v", event.ID(), err)
        return nil // Suppress error propagation
    },
)
```

## Subscription Interface

Subscriptions represent active connections between listeners and event sources:

```go
type Subscription interface {
    ID() string             // Subscription identifier
    Topic() string          // Topic or event type subscribed to
    Listener() Listener     // The subscribed listener
    Unsubscribe() error     // Remove subscription
    Active() bool           // Check if subscription is active
}
```

Subscriptions are returned by `Subscribe` methods and can be used to unsubscribe:

```go
sub := emitter.Subscribe("user.created", listener)

// Later, to unsubscribe
if err := sub.Unsubscribe(); err != nil {
    log.Printf("Unsubscribe failed: %v", err)
}

// Check if still active
if sub.Active() {
    // Still receiving events
}
```

## Middleware Interface

Middleware provides hooks for cross-cutting concerns like logging, tracing, and validation:

```go
type Middleware interface {
    Before(event Event) error              // Called before processing
    After(event Event, err error) error    // Called after processing
}
```

### MiddlewareFunc

Use `MiddlewareFunc` for inline middleware:

```go
// Logging middleware
loggingMW := shared.MiddlewareFunc{
    BeforeFunc: func(event shared.Event) error {
        log.Printf("Processing: %s (%s)", event.ID(), event.Type())
        return nil
    },
    AfterFunc: func(event shared.Event, err error) error {
        if err != nil {
            log.Printf("Failed: %s - %v", event.ID(), err)
        }
        return nil
    },
}

// Validation middleware
validationMW := shared.MiddlewareFunc{
    BeforeFunc: func(event shared.Event) error {
        if event.Data() == nil {
            return errors.New("event data cannot be nil")
        }
        return nil
    },
}

emitter.Middleware(loggingMW, validationMW)
```

## Event Validation

Nova validates events before processing. The default validator checks for:

- Non-nil event
- Non-empty ID
- Non-empty Type
- Non-zero Timestamp

```go
// Default validation
err := shared.DefaultEventValidator.Validate(event)

// Custom validator
customValidator := shared.EventValidatorFunc(func(event shared.Event) error {
    if event.Type() != "user.created" && event.Type() != "user.updated" {
        return shared.NewValidationError("type", event.Type(), "unsupported event type")
    }
    return nil
})
```

## Error Types

Nova provides structured error types for debugging and handling:

### Sentinel Errors

```go
var (
    ErrEventNotFound        // Event not found in store
    ErrInvalidEvent         // Event failed validation
    ErrListenerNotFound     // Listener not registered
    ErrTopicNotFound        // Topic does not exist
    ErrSubscriptionNotFound // Subscription not found
    ErrEmitterClosed        // Emitter has been shut down
    ErrBusClosed            // Bus has been shut down
    ErrBufferFull           // Buffer cannot accept more events
    ErrRetryLimitExceeded   // All retry attempts failed
    ErrTimeout              // Operation timed out
)
```

### Wrapped Errors

```go
// EventError includes event context
eventErr := shared.NewEventError(event, originalErr)
fmt.Println(eventErr) // "event error [id=123, type=user.created]: original error"

// ListenerError includes listener context
listenerErr := shared.NewListenerError("my-listener", event, originalErr)
fmt.Println(listenerErr) // "listener error [id=my-listener, event=123]: original error"

// ValidationError includes field details
validErr := shared.NewValidationError("email", "invalid@", "invalid email format")
fmt.Println(validErr) // "validation error [field=email]: invalid email format"

// Unwrap to get original error
if errors.Is(eventErr, originalErr) {
    // Handle specific error type
}
```

## Metrics Collection

Nova emits metrics through the `MetricsCollector` interface:

```go
type MetricsCollector interface {
    IncEventsEmitted(eventType, result string)
    IncEventsProcessed(listenerID, result string)
    ObserveListenerDuration(listenerID string, duration time.Duration)
    ObserveStoreAppendDuration(duration time.Duration)
    ObserveStoreReadDuration(duration time.Duration)
    SetQueueSize(component string, size int)
    SetActiveListeners(emitterID string, count int)
    IncEventsDropped(component, reason string)
}
```

### SimpleMetricsCollector

For testing and development:

```go
metrics := shared.NewSimpleMetricsCollector()

emitter := emitter.New(emitter.Config{
    WorkerPool:       pool,
    MetricsCollector: metrics,
})

// Check metrics
fmt.Printf("Events emitted: %d\n", metrics.GetEventsEmitted())
fmt.Printf("Events processed: %d\n", metrics.GetEventsProcessed())
```

### NoOpMetricsCollector

Used by default when no collector is provided:

```go
// Silently ignores all metrics
var _ MetricsCollector = NoOpMetricsCollector{}
```

### Integrating with Prometheus

Create a custom collector that wraps Prometheus metrics:

```go
type PrometheusMetrics struct {
    eventsEmitted   *prometheus.CounterVec
    eventDuration   *prometheus.HistogramVec
    // ... other metrics
}

func (p *PrometheusMetrics) IncEventsEmitted(eventType, result string) {
    p.eventsEmitted.WithLabelValues(eventType, result).Inc()
}

// Implement remaining interface methods...
```

## Design Principles

The shared package follows these principles:

1. **Interface-first** — All types are interfaces, allowing custom implementations
2. **Composable** — Small interfaces that can be combined
3. **Zero-allocation defaults** — Built-in implementations avoid allocations in hot paths
4. **Thread-safe** — All implementations are safe for concurrent use

## Further Reading

- [Emitter](emitter.md) — Event emission using shared types
- [Bus](bus.md) — Topic-based routing
- [Listener](listener.md) — Listener management
- [Memory Store](memory.md) — Event persistence
- [API Reference](../api-reference/shared.md) — Complete API documentation
