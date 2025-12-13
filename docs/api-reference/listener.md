# listener API

Complete API documentation for the listener package.

**Import Path:** `github.com/kolosys/nova/listener`

## Package Documentation



## Types

### BackoffStrategy
BackoffStrategy defines different backoff strategies for retries

#### Example Usage

```go
// Example usage of BackoffStrategy
var value BackoffStrategy
// Initialize with appropriate value
```

#### Type Definition

```go
type BackoffStrategy int
```

## Methods

### String

String returns the string representation of BackoffStrategy

```go
func (HealthStatus) String() string
```

**Parameters:**
  None

**Returns:**
- string

### CircuitConfig
CircuitConfig configures circuit breaker behavior

#### Example Usage

```go
// Create a new CircuitConfig
circuitconfig := CircuitConfig{
    Enabled: true,
    FailureThreshold: 42,
    SuccessThreshold: 42,
    Timeout: /* value */,
    SlidingWindow: /* value */,
}
```

#### Type Definition

```go
type CircuitConfig struct {
    Enabled bool
    FailureThreshold int
    SuccessThreshold int
    Timeout time.Duration
    SlidingWindow time.Duration
}
```

### Fields

| Field | Type | Description |
| ----- | ---- | ----------- |
| Enabled | `bool` | Enabled determines if circuit breaker is active |
| FailureThreshold | `int` | FailureThreshold is the number of failures before opening circuit |
| SuccessThreshold | `int` | SuccessThreshold is the number of successes needed to close circuit |
| Timeout | `time.Duration` | Timeout is how long to wait before trying again after circuit opens |
| SlidingWindow | `time.Duration` | SlidingWindow is the time window for counting failures |

### Constructor Functions

### DefaultCircuitConfig

DefaultCircuitConfig returns a sensible default circuit breaker config

```go
func DefaultCircuitConfig() CircuitConfig
```

**Parameters:**
  None

**Returns:**
- CircuitConfig

### Config
Config configures the ListenerManager

#### Example Usage

```go
// Create a new Config
config := Config{
    WorkerPool: &/* value */{},
    MetricsCollector: /* value */,
    Name: "example",
}
```

#### Type Definition

```go
type Config struct {
    WorkerPool *workerpool.Pool
    MetricsCollector shared.MetricsCollector
    Name string
}
```

### Fields

| Field | Type | Description |
| ----- | ---- | ----------- |
| WorkerPool | `*workerpool.Pool` | WorkerPool for event processing (required) |
| MetricsCollector | `shared.MetricsCollector` | MetricsCollector for observability (optional) |
| Name | `string` | Name identifies this manager instance |

### DeadLetterConfig
DeadLetterConfig configures dead letter queue behavior

#### Example Usage

```go
// Create a new DeadLetterConfig
deadletterconfig := DeadLetterConfig{
    Enabled: true,
    MaxRetries: 42,
    Handler: /* value */,
}
```

#### Type Definition

```go
type DeadLetterConfig struct {
    Enabled bool
    MaxRetries int
    Handler func(event shared.Event, err error)
}
```

### Fields

| Field | Type | Description |
| ----- | ---- | ----------- |
| Enabled | `bool` | Enabled determines if dead letter queue is active |
| MaxRetries | `int` | MaxRetries before sending to dead letter queue |
| Handler | `func(event shared.Event, err error)` | Handler is called for events sent to dead letter queue |

### Constructor Functions

### DefaultDeadLetterConfig

DefaultDeadLetterConfig returns a sensible default dead letter config

```go
func DefaultDeadLetterConfig() DeadLetterConfig
```

**Parameters:**
  None

**Returns:**
- DeadLetterConfig

### HealthStatus
HealthStatus represents listener health

#### Example Usage

```go
// Example usage of HealthStatus
var value HealthStatus
// Initialize with appropriate value
```

#### Type Definition

```go
type HealthStatus int
```

## Methods

### String

String returns the string representation of HealthStatus

```go
func (HealthStatus) String() string
```

**Parameters:**
  None

**Returns:**
- string

### ListenerConfig
ListenerConfig configures listener behavior

#### Example Usage

```go
// Create a new ListenerConfig
listenerconfig := ListenerConfig{
    Concurrency: 42,
    RetryPolicy: RetryPolicy{},
    Circuit: CircuitConfig{},
    DeadLetter: DeadLetterConfig{},
    Timeout: /* value */,
    Name: "example",
}
```

#### Type Definition

```go
type ListenerConfig struct {
    Concurrency int
    RetryPolicy RetryPolicy
    Circuit CircuitConfig
    DeadLetter DeadLetterConfig
    Timeout time.Duration
    Name string
}
```

### Fields

| Field | Type | Description |
| ----- | ---- | ----------- |
| Concurrency | `int` | Concurrency limits concurrent event processing for this listener |
| RetryPolicy | `RetryPolicy` | RetryPolicy defines retry behavior |
| Circuit | `CircuitConfig` | Circuit configures circuit breaker |
| DeadLetter | `DeadLetterConfig` | DeadLetter configures dead letter queue |
| Timeout | `time.Duration` | Timeout for individual event processing |
| Name | `string` | Name identifies this listener (for metrics/logging) |

### Constructor Functions

### DefaultListenerConfig

DefaultListenerConfig returns a sensible default configuration

```go
func DefaultListenerConfig() ListenerConfig
```

**Parameters:**
  None

**Returns:**
- ListenerConfig

### ListenerManager
ListenerManager defines the interface for managing event listeners

#### Example Usage

```go
// Example implementation of ListenerManager
type MyListenerManager struct {
    // Add your fields here
}

func (m MyListenerManager) Register(param1 shared.Listener, param2 ListenerConfig) error {
    // Implement your logic here
    return
}

func (m MyListenerManager) Unregister(param1 string) error {
    // Implement your logic here
    return
}

func (m MyListenerManager) Start(param1 context.Context) error {
    // Implement your logic here
    return
}

func (m MyListenerManager) Stop(param1 context.Context) error {
    // Implement your logic here
    return
}

func (m MyListenerManager) Health() HealthStatus {
    // Implement your logic here
    return
}

func (m MyListenerManager) Stats() Stats {
    // Implement your logic here
    return
}

func (m MyListenerManager) GetListenerHealth(param1 string) HealthStatus {
    // Implement your logic here
    return
}

func (m MyListenerManager) ProcessEvent(param1 context.Context, param2 string, param3 shared.Event) error {
    // Implement your logic here
    return
}


```

#### Type Definition

```go
type ListenerManager interface {
    Register(listener shared.Listener, config ListenerConfig) error
    Unregister(listenerID string) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Health() HealthStatus
    Stats() Stats
    GetListenerHealth(listenerID string) (HealthStatus, error)
    ProcessEvent(ctx context.Context, listenerID string, event shared.Event) error
}
```

## Methods

| Method | Description |
| ------ | ----------- |

### Constructor Functions

### New

New creates a new ListenerManager

```go
func New(config Config) ListenerManager
```

**Parameters:**
- `config` (Config)

**Returns:**
- ListenerManager

### RetryPolicy
RetryPolicy defines retry behavior for failed events

#### Example Usage

```go
// Create a new RetryPolicy
retrypolicy := RetryPolicy{
    MaxAttempts: 42,
    InitialDelay: /* value */,
    MaxDelay: /* value */,
    Backoff: BackoffStrategy{},
    RetryableErrors: [],
    RetryCondition: /* value */,
}
```

#### Type Definition

```go
type RetryPolicy struct {
    MaxAttempts int
    InitialDelay time.Duration
    MaxDelay time.Duration
    Backoff BackoffStrategy
    RetryableErrors []error
    RetryCondition func(error) bool
}
```

### Fields

| Field | Type | Description |
| ----- | ---- | ----------- |
| MaxAttempts | `int` | MaxAttempts is the maximum number of retry attempts (0 = no retries) |
| InitialDelay | `time.Duration` | InitialDelay is the initial delay before first retry |
| MaxDelay | `time.Duration` | MaxDelay is the maximum delay between retries |
| Backoff | `BackoffStrategy` | Backoff strategy to use |
| RetryableErrors | `[]error` | RetryableErrors lists error types that should trigger retries If empty, all errors are considered retryable |
| RetryCondition | `func(error) bool` | RetryCondition is a custom function to determine if an error should be retried |

### Constructor Functions

### DefaultRetryPolicy

DefaultRetryPolicy returns a sensible default retry policy

```go
func DefaultRetryPolicy() RetryPolicy
```

**Parameters:**
  None

**Returns:**
- RetryPolicy

### Stats
Stats provides listener manager statistics

#### Example Usage

```go
// Create a new Stats
stats := Stats{
    RegisteredListeners: 42,
    ActiveListeners: 42,
    EventsProcessed: 42,
    EventsRetried: 42,
    EventsFailed: 42,
    DeadLetterEvents: 42,
    CircuitBreakers: 42,
}
```

#### Type Definition

```go
type Stats struct {
    RegisteredListeners int64
    ActiveListeners int64
    EventsProcessed int64
    EventsRetried int64
    EventsFailed int64
    DeadLetterEvents int64
    CircuitBreakers int64
}
```

### Fields

| Field | Type | Description |
| ----- | ---- | ----------- |
| RegisteredListeners | `int64` |  |
| ActiveListeners | `int64` |  |
| EventsProcessed | `int64` |  |
| EventsRetried | `int64` |  |
| EventsFailed | `int64` |  |
| DeadLetterEvents | `int64` |  |
| CircuitBreakers | `int64` |  |

## External Links

- [Package Overview](../packages/listener.md)
- [pkg.go.dev Documentation](https://pkg.go.dev/github.com/kolosys/nova/listener)
- [Source Code](https://github.com/kolosys/nova/tree/main/listener)
