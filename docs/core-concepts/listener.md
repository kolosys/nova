# Listener Manager

The `listener` package provides lifecycle management for event listeners with retry policies, circuit breakers, and dead letter queues.

**Import Path:** `github.com/kolosys/nova/listener`

## Overview

The ListenerManager adds enterprise resilience patterns to event handling. It wraps listeners with retry logic, circuit breakers, and dead letter handling to ensure robust event processing.

```
┌────────────┐     ┌─────────────────────────────────────────────┐
│   Event    │────▶│              ListenerManager                │
└────────────┘     │                                             │
                   │  ┌─────────────────────────────────────┐   │
                   │  │         Managed Listener            │   │
                   │  │  ┌───────────────┐                  │   │
                   │  │  │ Circuit       │──▶ Open?         │   │
                   │  │  │ Breaker       │   └──▶ Reject    │   │
                   │  │  └───────────────┘                  │   │
                   │  │         │                           │   │
                   │  │         ▼                           │   │
                   │  │  ┌───────────────┐                  │   │
                   │  │  │ Concurrency   │──▶ Process       │   │
                   │  │  │ Control       │                  │   │
                   │  │  └───────────────┘                  │   │
                   │  │         │                           │   │
                   │  │    Success? ──▶ Done                │   │
                   │  │         │                           │   │
                   │  │    Failure ──▶ Retry Policy         │   │
                   │  │              └──▶ Dead Letter       │   │
                   │  └─────────────────────────────────────┘   │
                   └─────────────────────────────────────────────┘
```

## Creating a ListenerManager

```go
import (
    "github.com/kolosys/ion/workerpool"
    "github.com/kolosys/nova/listener"
)

pool := workerpool.New(4, 100)
defer pool.Close(ctx)

lm := listener.New(listener.Config{
    WorkerPool: pool,          // Required
    Name:       "app-manager", // For metrics identification
})
defer lm.Stop(ctx)
```

## Registering Listeners

Register listeners with resilience configuration:

```go
handler := shared.NewBaseListener("order-processor", processOrder)

err := lm.Register(handler, listener.ListenerConfig{
    Concurrency: 10,                    // Max concurrent processing
    Timeout:     30 * time.Second,      // Per-event timeout
    Name:        "order-processor",     // For metrics

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
        SlidingWindow:    time.Minute,
    },

    DeadLetter: listener.DeadLetterConfig{
        Enabled: true,
        Handler: func(event shared.Event, err error) {
            log.Printf("Dead letter: %s - %v", event.ID(), err)
        },
    },
})
```

### ListenerConfig Options

| Option        | Default     | Description                             |
| ------------- | ----------- | --------------------------------------- |
| `Concurrency` | `10`        | Max concurrent event processing         |
| `Timeout`     | `30s`       | Timeout for individual event processing |
| `Name`        | listener ID | Identifier for metrics                  |
| `RetryPolicy` | see below   | Retry configuration                     |
| `Circuit`     | see below   | Circuit breaker configuration           |
| `DeadLetter`  | see below   | Dead letter queue configuration         |

## Retry Policies

Retry policies define how failed events are retried.

### Backoff Strategies

```go
// Fixed delay between retries
listener.RetryPolicy{
    MaxAttempts:  5,
    InitialDelay: 1 * time.Second,
    Backoff:      listener.FixedBackoff,
}
// Delays: 1s, 1s, 1s, 1s, 1s

// Linear increase
listener.RetryPolicy{
    MaxAttempts:  5,
    InitialDelay: 1 * time.Second,
    MaxDelay:     30 * time.Second,
    Backoff:      listener.LinearBackoff,
}
// Delays: 1s, 2s, 3s, 4s, 5s

// Exponential increase (default)
listener.RetryPolicy{
    MaxAttempts:  5,
    InitialDelay: 100 * time.Millisecond,
    MaxDelay:     30 * time.Second,
    Backoff:      listener.ExponentialBackoff,
}
// Delays: 100ms, 400ms, 900ms, 1600ms, 2500ms (capped at MaxDelay)
```

### Custom Retry Conditions

Control which errors trigger retries:

```go
listener.RetryPolicy{
    MaxAttempts:  3,
    InitialDelay: 100 * time.Millisecond,
    Backoff:      listener.ExponentialBackoff,

    // Only retry specific error types
    RetryCondition: func(err error) bool {
        // Don't retry validation errors
        var validationErr *shared.ValidationError
        if errors.As(err, &validationErr) {
            return false
        }
        // Retry transient errors
        return errors.Is(err, errTemporaryFailure)
    },
}
```

### Default Retry Policy

```go
policy := listener.DefaultRetryPolicy()
// MaxAttempts:  3
// InitialDelay: 100ms
// MaxDelay:     30s
// Backoff:      ExponentialBackoff
```

## Circuit Breakers

Circuit breakers prevent cascade failures by temporarily blocking requests to failing services.

### Circuit States

```
     ┌─────────────────────────────────────────────┐
     │                                             │
     ▼                                             │
┌─────────┐  failures >= threshold  ┌─────────┐   │
│ CLOSED  │────────────────────────▶│  OPEN   │   │
│ (allow) │                         │ (block) │   │
└─────────┘                         └────┬────┘   │
     ▲                                   │        │
     │                              timeout       │
     │                                   │        │
     │         success >= threshold ┌────▼────┐   │
     └──────────────────────────────│HALF-OPEN│   │
                                    │ (test)  │───┘
                                    └─────────┘  failure
```

### Configuration

```go
listener.CircuitConfig{
    Enabled:          true,
    FailureThreshold: 5,              // Failures before opening
    SuccessThreshold: 3,              // Successes to close
    Timeout:          30 * time.Second, // Time before half-open
    SlidingWindow:    time.Minute,    // Window for counting failures
}
```

### Behavior

1. **Closed** (normal): Events are processed normally
2. **Open** (blocking): Events are rejected immediately with an error
3. **Half-Open** (testing): One event is allowed through to test the service

```go
// Check circuit state via health
health, _ := lm.GetListenerHealth("order-processor")
if health == listener.CircuitOpen {
    log.Println("Circuit breaker is open")
}
```

### Default Circuit Config

```go
config := listener.DefaultCircuitConfig()
// Enabled:          true
// FailureThreshold: 5
// SuccessThreshold: 3
// Timeout:          30s
// SlidingWindow:    1m
```

## Dead Letter Queue

Dead letter queues capture events that fail all retry attempts.

### Configuration

```go
listener.DeadLetterConfig{
    Enabled:    true,
    MaxRetries: 5,  // Override RetryPolicy.MaxAttempts for DLQ
    Handler: func(event shared.Event, err error) {
        // Log for investigation
        log.Printf("Dead letter: id=%s type=%s error=%v",
            event.ID(), event.Type(), err)

        // Store for manual processing
        storeDeadLetter(event, err)

        // Alert operations
        alertOps(event, err)
    },
}
```

### Common Dead Letter Patterns

```go
// Store to database
Handler: func(event shared.Event, err error) {
    db.Insert("dead_letters", map[string]any{
        "event_id":    event.ID(),
        "event_type":  event.Type(),
        "event_data":  event.Data(),
        "error":       err.Error(),
        "timestamp":   time.Now(),
    })
}

// Forward to dead letter topic
Handler: func(event shared.Event, err error) {
    if be, ok := event.(*shared.BaseEvent); ok {
        be.SetMetadata("error", err.Error())
        be.SetMetadata("attempts", fmt.Sprintf("%d", maxAttempts))
    }
    bus.Publish(ctx, "dead-letter", event)
}

// Alert with rate limiting
var alertLimiter = rate.NewLimiter(1, 10) // 1 per second, burst 10

Handler: func(event shared.Event, err error) {
    if alertLimiter.Allow() {
        sendAlert(event, err)
    }
    storeForLater(event, err)
}
```

## Health Monitoring

### Overall Health

```go
health := lm.Health()

switch health {
case listener.Healthy:
    // All systems operational
case listener.Degraded:
    // Some issues but still operational
case listener.Unhealthy:
    // Significant issues
case listener.CircuitOpen:
    // At least one circuit breaker is open
}
```

### Per-Listener Health

```go
health, err := lm.GetListenerHealth("order-processor")
if err != nil {
    log.Printf("Listener not found: %v", err)
    return
}

if health != listener.Healthy {
    log.Printf("Listener health: %s", health)
}
```

### Health Calculation

- **Healthy**: No recent errors, <10% failure rate
- **Degraded**: Errors within the last minute or 10-50% failure rate
- **Unhealthy**: >50% failure rate
- **CircuitOpen**: Circuit breaker is open

## Lifecycle Management

### Starting

```go
if err := lm.Start(ctx); err != nil {
    log.Fatalf("Failed to start: %v", err)
}
```

Starting:

1. Starts retry processors for each listener
2. Starts dead letter processors if enabled
3. Updates active listener count

### Stopping

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

if err := lm.Stop(ctx); err != nil {
    log.Printf("Stop timeout: %v", err)
}
```

Stopping:

1. Signals all processors to stop
2. Waits for in-flight events to complete
3. Cleans up resources

### Unregistering

```go
if err := lm.Unregister("order-processor"); err != nil {
    log.Printf("Unregister failed: %v", err)
}
```

## Processing Events

The manager is typically used with an Emitter or Bus:

```go
// Direct processing
if err := lm.ProcessEvent(ctx, "order-processor", event); err != nil {
    log.Printf("Processing failed: %v", err)
}

// Integration with Emitter
listener := shared.NewBaseListener("order-processor", func(event shared.Event) error {
    return lm.ProcessEvent(ctx, "order-processor", event)
})
emitter.Subscribe("order.created", listener)
```

## Statistics

```go
stats := lm.Stats()

fmt.Printf("Registered listeners: %d\n", stats.RegisteredListeners)
fmt.Printf("Active listeners: %d\n", stats.ActiveListeners)
fmt.Printf("Events processed: %d\n", stats.EventsProcessed)
fmt.Printf("Events retried: %d\n", stats.EventsRetried)
fmt.Printf("Events failed: %d\n", stats.EventsFailed)
fmt.Printf("Dead letter events: %d\n", stats.DeadLetterEvents)
fmt.Printf("Circuit breakers: %d\n", stats.CircuitBreakers)
```

## Common Patterns

### Bulkhead Pattern

Isolate failures between different event types:

```go
// Separate managers for critical vs non-critical
criticalLM := listener.New(listener.Config{
    WorkerPool: criticalPool,
    Name:       "critical",
})

standardLM := listener.New(listener.Config{
    WorkerPool: standardPool,
    Name:       "standard",
})

// Critical events get more resources and stricter config
criticalLM.Register(paymentHandler, listener.ListenerConfig{
    Concurrency: 20,
    RetryPolicy: listener.RetryPolicy{MaxAttempts: 5},
})

standardLM.Register(notificationHandler, listener.ListenerConfig{
    Concurrency: 5,
    RetryPolicy: listener.RetryPolicy{MaxAttempts: 2},
})
```

### Graceful Degradation

```go
handler := shared.NewBaseListener("degraded-handler", func(event shared.Event) error {
    health := externalService.Health()

    if health == Unhealthy {
        // Fall back to simpler processing
        return processSimple(event)
    }

    return processFull(event)
})
```

### Monitoring Dashboard

```go
func healthHandler(w http.ResponseWriter, r *http.Request) {
    health := lm.Health()
    stats := lm.Stats()

    status := http.StatusOK
    if health == listener.Unhealthy {
        status = http.StatusServiceUnavailable
    } else if health == listener.Degraded || health == listener.CircuitOpen {
        status = http.StatusOK // Still serving, but degraded
    }

    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]any{
        "status":    health.String(),
        "listeners": stats.RegisteredListeners,
        "processed": stats.EventsProcessed,
        "failed":    stats.EventsFailed,
        "retried":   stats.EventsRetried,
    })
}
```

## Further Reading

- [Shared Types](shared.md) — Event, Listener interfaces
- [Emitter](emitter.md) — Event emission
- [Bus](bus.md) — Topic-based routing
- [Memory Store](memory.md) — Event persistence
- [Best Practices](../advanced/best-practices.md) — Production recommendations
- [API Reference](../api-reference/listener.md) — Complete API documentation
