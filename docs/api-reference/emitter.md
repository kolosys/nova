# emitter API

Complete API documentation for the emitter package.

**Import Path:** `github.com/kolosys/nova/emitter`

## Package Documentation



## Types

### Config
Config configures the EventEmitter

#### Example Usage

```go
// Create a new Config
config := Config{
    WorkerPool: &/* value */{},
    AsyncMode: true,
    BufferSize: 42,
    MaxConcurrency: 42,
    MetricsCollector: /* value */,
    EventValidator: /* value */,
    Name: "example",
}
```

#### Type Definition

```go
type Config struct {
    WorkerPool *workerpool.Pool
    AsyncMode bool
    BufferSize int
    MaxConcurrency int
    MetricsCollector shared.MetricsCollector
    EventValidator shared.EventValidator
    Name string
}
```

### Fields

| Field | Type | Description |
| ----- | ---- | ----------- |
| WorkerPool | `*workerpool.Pool` | WorkerPool for async event processing (required) |
| AsyncMode | `bool` | AsyncMode enables async processing by default |
| BufferSize | `int` | BufferSize sets the size of the async event buffer |
| MaxConcurrency | `int` | MaxConcurrency limits concurrent event processing per subscription |
| MetricsCollector | `shared.MetricsCollector` | MetricsCollector for observability (optional) |
| EventValidator | `shared.EventValidator` | EventValidator validates events before processing (optional) |
| Name | `string` | Name identifies this emitter instance (for metrics/logging) |

### EventEmitter
EventEmitter defines the interface for event emission and subscription

#### Example Usage

```go
// Example implementation of EventEmitter
type MyEventEmitter struct {
    // Add your fields here
}

func (m MyEventEmitter) Emit(param1 context.Context, param2 shared.Event) error {
    // Implement your logic here
    return
}

func (m MyEventEmitter) EmitAsync(param1 context.Context, param2 shared.Event) error {
    // Implement your logic here
    return
}

func (m MyEventEmitter) EmitBatch(param1 context.Context, param2 []shared.Event) error {
    // Implement your logic here
    return
}

func (m MyEventEmitter) Subscribe(param1 string, param2 shared.Listener) shared.Subscription {
    // Implement your logic here
    return
}

func (m MyEventEmitter) Middleware(param1 ...shared.Middleware) EventEmitter {
    // Implement your logic here
    return
}

func (m MyEventEmitter) Shutdown(param1 context.Context) error {
    // Implement your logic here
    return
}

func (m MyEventEmitter) Stats() Stats {
    // Implement your logic here
    return
}


```

#### Type Definition

```go
type EventEmitter interface {
    Emit(ctx context.Context, event shared.Event) error
    EmitAsync(ctx context.Context, event shared.Event) error
    EmitBatch(ctx context.Context, events []shared.Event) error
    Subscribe(eventType string, listener shared.Listener) shared.Subscription
    Middleware(middleware ...shared.Middleware) EventEmitter
    Shutdown(ctx context.Context) error
    Stats() Stats
}
```

## Methods

| Method | Description |
| ------ | ----------- |

### Constructor Functions

### New

New creates a new EventEmitter

```go
func New(config Config) EventEmitter
```

**Parameters:**
- `config` (Config)

**Returns:**
- EventEmitter

### Stats
Stats provides emitter statistics

#### Example Usage

```go
// Create a new Stats
stats := Stats{
    EventsEmitted: 42,
    EventsProcessed: 42,
    ActiveListeners: 42,
    FailedEvents: 42,
    QueuedEvents: 42,
    MiddlewareErrors: 42,
}
```

#### Type Definition

```go
type Stats struct {
    EventsEmitted int64
    EventsProcessed int64
    ActiveListeners int64
    FailedEvents int64
    QueuedEvents int64
    MiddlewareErrors int64
}
```

### Fields

| Field | Type | Description |
| ----- | ---- | ----------- |
| EventsEmitted | `int64` |  |
| EventsProcessed | `int64` |  |
| ActiveListeners | `int64` |  |
| FailedEvents | `int64` |  |
| QueuedEvents | `int64` |  |
| MiddlewareErrors | `int64` |  |

## External Links

- [Package Overview](../packages/emitter.md)
- [pkg.go.dev Documentation](https://pkg.go.dev/github.com/kolosys/nova/emitter)
- [Source Code](https://github.com/kolosys/nova/tree/main/emitter)
