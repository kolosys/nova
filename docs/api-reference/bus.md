# bus API

Complete API documentation for the bus package.

**Import Path:** `github.com/kolosys/nova/bus`

## Package Documentation



## Types

### Config
Config configures the EventBus

#### Example Usage

```go
// Create a new Config
config := Config{
    WorkerPool: &/* value */{},
    DefaultBufferSize: 42,
    DefaultPartitions: 42,
    DefaultDeliveryMode: DeliveryMode{},
    MetricsCollector: /* value */,
    EventValidator: /* value */,
    Name: "example",
}
```

#### Type Definition

```go
type Config struct {
    WorkerPool *workerpool.Pool
    DefaultBufferSize int
    DefaultPartitions int
    DefaultDeliveryMode DeliveryMode
    MetricsCollector shared.MetricsCollector
    EventValidator shared.EventValidator
    Name string
}
```

### Fields

| Field | Type | Description |
| ----- | ---- | ----------- |
| WorkerPool | `*workerpool.Pool` | WorkerPool for async event processing (required) |
| DefaultBufferSize | `int` | DefaultBufferSize sets default buffer size for topics |
| DefaultPartitions | `int` | DefaultPartitions sets default number of partitions |
| DefaultDeliveryMode | `DeliveryMode` | DefaultDeliveryMode sets default delivery mode |
| MetricsCollector | `shared.MetricsCollector` | MetricsCollector for observability (optional) |
| EventValidator | `shared.EventValidator` | EventValidator validates events before processing (optional) |
| Name | `string` | Name identifies this bus instance |

### DeliveryMode
DeliveryMode defines the delivery guarantees for events

#### Example Usage

```go
// Example usage of DeliveryMode
var value DeliveryMode
// Initialize with appropriate value
```

#### Type Definition

```go
type DeliveryMode int
```

## Methods

### String

String returns the string representation of DeliveryMode

```go
func (DeliveryMode) String() string
```

**Parameters:**
  None

**Returns:**
- string

### EventBus
EventBus defines the interface for topic-based event routing

#### Example Usage

```go
// Example implementation of EventBus
type MyEventBus struct {
    // Add your fields here
}

func (m MyEventBus) Publish(param1 context.Context, param2 string, param3 shared.Event) error {
    // Implement your logic here
    return
}

func (m MyEventBus) Subscribe(param1 string, param2 shared.Listener) shared.Subscription {
    // Implement your logic here
    return
}

func (m MyEventBus) SubscribePattern(param1 string, param2 shared.Listener) shared.Subscription {
    // Implement your logic here
    return
}

func (m MyEventBus) CreateTopic(param1 string, param2 TopicConfig) error {
    // Implement your logic here
    return
}

func (m MyEventBus) DeleteTopic(param1 string) error {
    // Implement your logic here
    return
}

func (m MyEventBus) Topics() []string {
    // Implement your logic here
    return
}

func (m MyEventBus) Shutdown(param1 context.Context) error {
    // Implement your logic here
    return
}

func (m MyEventBus) Stats() Stats {
    // Implement your logic here
    return
}


```

#### Type Definition

```go
type EventBus interface {
    Publish(ctx context.Context, topic string, event shared.Event) error
    Subscribe(topic string, listener shared.Listener) shared.Subscription
    SubscribePattern(pattern string, listener shared.Listener) shared.Subscription
    CreateTopic(topic string, config TopicConfig) error
    DeleteTopic(topic string) error
    Topics() []string
    Shutdown(ctx context.Context) error
    Stats() Stats
}
```

## Methods

| Method | Description |
| ------ | ----------- |

### Constructor Functions

### New

New creates a new EventBus

```go
func New(config Config) EventBus
```

**Parameters:**
- `config` (Config)

**Returns:**
- EventBus

### Stats
Stats provides bus statistics

#### Example Usage

```go
// Create a new Stats
stats := Stats{
    EventsPublished: 42,
    EventsProcessed: 42,
    ActiveTopics: 42,
    ActiveSubscribers: 42,
    FailedEvents: 42,
    QueuedEvents: 42,
}
```

#### Type Definition

```go
type Stats struct {
    EventsPublished int64
    EventsProcessed int64
    ActiveTopics int64
    ActiveSubscribers int64
    FailedEvents int64
    QueuedEvents int64
}
```

### Fields

| Field | Type | Description |
| ----- | ---- | ----------- |
| EventsPublished | `int64` |  |
| EventsProcessed | `int64` |  |
| ActiveTopics | `int64` |  |
| ActiveSubscribers | `int64` |  |
| FailedEvents | `int64` |  |
| QueuedEvents | `int64` |  |

### TopicConfig
TopicConfig configures a topic

#### Example Usage

```go
// Create a new TopicConfig
topicconfig := TopicConfig{
    BufferSize: 42,
    Partitions: 42,
    Retention: /* value */,
    DeliveryMode: DeliveryMode{},
    OrderingKey: /* value */,
    MaxConcurrency: 42,
}
```

#### Type Definition

```go
type TopicConfig struct {
    BufferSize int
    Partitions int
    Retention time.Duration
    DeliveryMode DeliveryMode
    OrderingKey func(shared.Event) string
    MaxConcurrency int
}
```

### Fields

| Field | Type | Description |
| ----- | ---- | ----------- |
| BufferSize | `int` | BufferSize sets the buffer size for this topic |
| Partitions | `int` | Partitions sets the number of partitions for parallel processing |
| Retention | `time.Duration` | Retention sets how long events are retained |
| DeliveryMode | `DeliveryMode` | DeliveryMode sets the delivery guarantees |
| OrderingKey | `func(shared.Event) string` | OrderingKey function to determine partition for event ordering |
| MaxConcurrency | `int` | MaxConcurrency limits concurrent processing per partition |

### Constructor Functions

### DefaultTopicConfig

DefaultTopicConfig returns a default topic configuration

```go
func DefaultTopicConfig() TopicConfig
```

**Parameters:**
  None

**Returns:**
- TopicConfig

## External Links

- [Package Overview](../packages/bus.md)
- [pkg.go.dev Documentation](https://pkg.go.dev/github.com/kolosys/nova/bus)
- [Source Code](https://github.com/kolosys/nova/tree/main/bus)
