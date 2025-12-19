# Best Practices

Production-grade patterns and recommendations for building robust event systems with Nova.

## Event Design

### Event Naming

Use a consistent naming convention for event types:

```go
// Good: domain.action format
"user.created"
"order.completed"
"payment.failed"

// Good: with subdomains
"inventory.stock.depleted"
"notification.email.sent"

// Avoid: inconsistent naming
"CreateUser"
"order_completed"
"PAYMENT-FAILED"
```

### Event Granularity

Prefer fine-grained events over coarse ones:

```go
// Good: specific events
"user.email.changed"
"user.password.changed"
"user.profile.updated"

// Avoid: generic events that require parsing
"user.changed" // What changed?
```

### Event Immutability

Events should be immutable after creation:

```go
// Create event with all data upfront
event := shared.NewBaseEventWithMetadata(
    "order-123",
    "order.created",
    OrderData{
        ID:        "order-123",
        Items:     items,
        Total:     99.99,
        CreatedAt: time.Now(),
    },
    map[string]string{
        "customer_id": "cust-456",
        "source":      "web",
    },
)
```

### Event Versioning

Include version information for schema evolution:

```go
event := shared.NewBaseEventWithMetadata(
    id,
    "order.created.v2",
    OrderDataV2{...},
    map[string]string{
        "schema_version": "2",
    },
)

// Handle multiple versions
listener := shared.NewBaseListener("handler", func(event shared.Event) error {
    version := event.Metadata()["schema_version"]
    switch version {
    case "2":
        return handleOrderV2(event)
    default:
        return handleOrderV1(event)
    }
})
```

## Listener Design

### Idempotent Handlers

Design listeners to handle duplicate events:

```go
listener := shared.NewBaseListener("order-processor", func(event shared.Event) error {
    orderID := event.ID()

    // Check if already processed
    if processed, _ := db.IsProcessed(orderID); processed {
        return nil // Already handled
    }

    // Process the event
    if err := processOrder(event); err != nil {
        return err
    }

    // Mark as processed
    return db.MarkProcessed(orderID)
})
```

### Fast Handlers

Keep handlers fast; offload heavy work:

```go
// Good: quick handler that queues work
listener := shared.NewBaseListener("email-handler", func(event shared.Event) error {
    return emailQueue.Enqueue(event.Data())
})

// Avoid: slow handler that blocks processing
listener := shared.NewBaseListener("slow-handler", func(event shared.Event) error {
    return sendEmailSynchronously(event) // Blocks for seconds
})
```

### Error Classification

Distinguish between retryable and permanent errors:

```go
var (
    ErrTemporary = errors.New("temporary failure")
    ErrPermanent = errors.New("permanent failure")
)

listener := shared.NewBaseListener("handler", func(event shared.Event) error {
    if serviceUnavailable() {
        return fmt.Errorf("service unavailable: %w", ErrTemporary)
    }

    if invalidData(event) {
        return fmt.Errorf("invalid data: %w", ErrPermanent)
    }

    return process(event)
})

// Configure retry policy accordingly
lm.Register(listener, listener.ListenerConfig{
    RetryPolicy: listener.RetryPolicy{
        MaxAttempts: 3,
        RetryCondition: func(err error) bool {
            return errors.Is(err, ErrTemporary)
        },
    },
})
```

## Concurrency

### WorkerPool Sizing

Size workerpools based on workload characteristics:

```go
import "runtime"

// CPU-bound work: match CPU cores
cpuPool := workerpool.New(runtime.NumCPU(), 1000)

// I/O-bound work: higher concurrency
ioPool := workerpool.New(runtime.NumCPU()*4, 5000)

// Mixed workload: balance
mixedPool := workerpool.New(runtime.NumCPU()*2, 2000)
```

### Buffer Sizing

Size buffers for expected throughput:

```go
// Calculate buffer based on rate
eventsPerSecond := 10000
processingTimeMs := 10
safetyFactor := 2

bufferSize := (eventsPerSecond * processingTimeMs / 1000) * safetyFactor

em := emitter.New(emitter.Config{
    WorkerPool: pool,
    BufferSize: bufferSize,
})
```

### Partition Sizing

Use partitions to parallelize while maintaining order:

```go
// Good: partition by customer for order consistency
b.CreateTopic("orders", bus.TopicConfig{
    Partitions: 16,
    OrderingKey: func(e shared.Event) string {
        return e.Metadata()["customer_id"]
    },
})

// Avoid: too many partitions for low-volume topics
b.CreateTopic("rare-events", bus.TopicConfig{
    Partitions: 1, // One partition is fine for low volume
})
```

## Error Handling

### Wrap Errors with Context

Always add context when wrapping errors:

```go
listener := shared.NewBaseListener("order-handler", func(event shared.Event) error {
    order, err := parseOrder(event)
    if err != nil {
        return fmt.Errorf("parsing order %s: %w", event.ID(), err)
    }

    if err := saveOrder(order); err != nil {
        return fmt.Errorf("saving order %s: %w", order.ID, err)
    }

    return nil
})
```

### Use Sentinel Errors

Define and use sentinel errors for known cases:

```go
var (
    ErrOrderNotFound    = errors.New("order not found")
    ErrInsufficientStock = errors.New("insufficient stock")
    ErrPaymentDeclined  = errors.New("payment declined")
)

// Check for specific errors
if errors.Is(err, ErrPaymentDeclined) {
    // Handle payment failure
}
```

### Dead Letter Strategy

Implement a comprehensive dead letter strategy:

```go
lm.Register(handler, listener.ListenerConfig{
    DeadLetter: listener.DeadLetterConfig{
        Enabled: true,
        Handler: func(event shared.Event, err error) {
            // 1. Log with full context
            log.WithFields(log.Fields{
                "event_id":   event.ID(),
                "event_type": event.Type(),
                "error":      err.Error(),
            }).Error("Event sent to dead letter queue")

            // 2. Store for retry/analysis
            db.Insert("dead_letters", DeadLetter{
                EventID:   event.ID(),
                EventType: event.Type(),
                EventData: event.Data(),
                Error:     err.Error(),
                Timestamp: time.Now(),
            })

            // 3. Alert if threshold exceeded
            count := metrics.IncrementDeadLetters()
            if count > threshold {
                alerting.Send("Dead letter threshold exceeded")
            }
        },
    },
})
```

## Observability

### Metrics Collection

Implement comprehensive metrics:

```go
type PrometheusMetrics struct {
    eventsEmitted   *prometheus.CounterVec
    eventsProcessed *prometheus.CounterVec
    eventDuration   *prometheus.HistogramVec
    queueSize       *prometheus.GaugeVec
    deadLetters     *prometheus.CounterVec
}

func NewPrometheusMetrics() *PrometheusMetrics {
    m := &PrometheusMetrics{
        eventsEmitted: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "nova_events_emitted_total",
                Help: "Total events emitted",
            },
            []string{"type", "result"},
        ),
        // ... other metrics
    }
    prometheus.MustRegister(m.eventsEmitted, ...)
    return m
}
```

### Structured Logging

Use structured logging for events:

```go
loggingMW := shared.MiddlewareFunc{
    BeforeFunc: func(event shared.Event) error {
        log.WithFields(log.Fields{
            "event_id":   event.ID(),
            "event_type": event.Type(),
            "metadata":   event.Metadata(),
        }).Info("Processing event")
        return nil
    },
    AfterFunc: func(event shared.Event, err error) error {
        fields := log.Fields{
            "event_id":   event.ID(),
            "event_type": event.Type(),
        }
        if err != nil {
            fields["error"] = err.Error()
            log.WithFields(fields).Error("Event processing failed")
        } else {
            log.WithFields(fields).Info("Event processed")
        }
        return nil
    },
}
```

### Distributed Tracing

Propagate trace context through events:

```go
// Add trace context when emitting
tracingMW := shared.MiddlewareFunc{
    BeforeFunc: func(event shared.Event) error {
        span := trace.SpanFromContext(ctx)
        if span.SpanContext().IsValid() {
            if be, ok := event.(*shared.BaseEvent); ok {
                be.SetMetadata("trace_id", span.SpanContext().TraceID().String())
                be.SetMetadata("span_id", span.SpanContext().SpanID().String())
            }
        }
        return nil
    },
}

// Extract and continue trace in listener
listener := shared.NewBaseListener("traced-handler", func(event shared.Event) error {
    traceID := event.Metadata()["trace_id"]
    parentSpanID := event.Metadata()["span_id"]

    // Create child span
    ctx, span := tracer.Start(ctx, "process-event",
        trace.WithLinks(trace.Link{
            SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
                TraceID: parseTraceID(traceID),
                SpanID:  parseSpanID(parentSpanID),
            }),
        }),
    )
    defer span.End()

    return processEvent(ctx, event)
})
```

## Graceful Shutdown

### Shutdown Order

Shut down components in reverse order of creation:

```go
func shutdown(ctx context.Context) error {
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    // 1. Stop accepting new events
    if err := emitter.Shutdown(ctx); err != nil {
        log.Printf("Emitter shutdown error: %v", err)
    }

    if err := bus.Shutdown(ctx); err != nil {
        log.Printf("Bus shutdown error: %v", err)
    }

    // 2. Wait for in-flight processing
    if err := listenerManager.Stop(ctx); err != nil {
        log.Printf("Listener manager stop error: %v", err)
    }

    // 3. Close storage
    if err := store.Close(); err != nil {
        log.Printf("Store close error: %v", err)
    }

    // 4. Close workerpool last
    pool.Close(ctx)

    return nil
}
```

### Signal Handling

Handle OS signals for graceful shutdown:

```go
func main() {
    ctx, cancel := context.WithCancel(context.Background())

    // Handle shutdown signals
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        <-sigCh
        log.Println("Shutdown signal received")
        cancel()
    }()

    // Start application
    if err := run(ctx); err != nil {
        log.Fatal(err)
    }

    // Graceful shutdown
    shutdownCtx, shutdownCancel := context.WithTimeout(
        context.Background(),
        30*time.Second,
    )
    defer shutdownCancel()

    shutdown(shutdownCtx)
}
```

## Testing

### Unit Testing Listeners

```go
func TestOrderHandler(t *testing.T) {
    tests := []struct {
        name    string
        event   shared.Event
        wantErr bool
    }{
        {
            name: "valid order",
            event: shared.NewBaseEvent("order-1", "order.created", OrderData{
                ID:    "order-1",
                Total: 99.99,
            }),
            wantErr: false,
        },
        {
            name: "invalid order",
            event: shared.NewBaseEvent("order-2", "order.created", OrderData{
                ID:    "",
                Total: -1,
            }),
            wantErr: true,
        },
    }

    handler := createOrderHandler()

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := handler.Handle(tt.event)
            if (err != nil) != tt.wantErr {
                t.Errorf("Handle() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Integration Testing

```go
func TestEventFlow(t *testing.T) {
    ctx := context.Background()

    // Setup
    pool := workerpool.New(2, 100)
    defer pool.Close(ctx)

    em := emitter.New(emitter.Config{WorkerPool: pool})
    defer em.Shutdown(ctx)

    received := make(chan shared.Event, 1)
    listener := shared.NewBaseListener("test", func(event shared.Event) error {
        received <- event
        return nil
    })

    em.Subscribe("test.event", listener)

    // Execute
    event := shared.NewBaseEvent("test-1", "test.event", "data")
    if err := em.Emit(ctx, event); err != nil {
        t.Fatalf("Emit failed: %v", err)
    }

    // Verify
    select {
    case e := <-received:
        if e.ID() != event.ID() {
            t.Errorf("got event ID %s, want %s", e.ID(), event.ID())
        }
    case <-time.After(time.Second):
        t.Error("timeout waiting for event")
    }
}
```

### Race Detection

Always test with race detection:

```bash
go test -race ./...
```

## Further Reading

- [Performance Tuning](performance-tuning.md) — Optimization techniques
- [Core Concepts](../core-concepts/shared.md) — Fundamental types
- [API Reference](../api-reference/) — Complete API documentation
