# memory API

Complete API documentation for the memory package.

**Import Path:** `github.com/kolosys/nova/memory`

## Package Documentation



## Types

### Config
Config configures the EventStore

#### Example Usage

```go
// Create a new Config
config := Config{
    MaxEventsPerStream: 42,
    MaxStreams: 42,
    RetentionDuration: /* value */,
    SnapshotInterval: /* value */,
    MetricsCollector: /* value */,
    Name: "example",
}
```

#### Type Definition

```go
type Config struct {
    MaxEventsPerStream int64
    MaxStreams int
    RetentionDuration time.Duration
    SnapshotInterval time.Duration
    MetricsCollector shared.MetricsCollector
    Name string
}
```

### Fields

| Field | Type | Description |
| ----- | ---- | ----------- |
| MaxEventsPerStream | `int64` | MaxEventsPerStream limits events per stream (0 = unlimited) |
| MaxStreams | `int` | MaxStreams limits the number of streams (0 = unlimited) |
| RetentionDuration | `time.Duration` | RetentionDuration sets how long to keep events (0 = forever) |
| SnapshotInterval | `time.Duration` | SnapshotInterval sets how often to create snapshots |
| MetricsCollector | `shared.MetricsCollector` | MetricsCollector for observability (optional) |
| Name | `string` | Name identifies this store instance |

### Constructor Functions

### DefaultConfig

DefaultConfig returns a sensible default configuration

```go
func DefaultConfig() Config
```

**Parameters:**
  None

**Returns:**
- Config

### Cursor
Cursor represents a position in the event stream

#### Example Usage

```go
// Create a new Cursor
cursor := Cursor{
    StreamID: "example",
    Position: 42,
    Timestamp: /* value */,
}
```

#### Type Definition

```go
type Cursor struct {
    StreamID string
    Position int64
    Timestamp time.Time
}
```

### Fields

| Field | Type | Description |
| ----- | ---- | ----------- |
| StreamID | `string` | StreamID identifies the stream |
| Position | `int64` | Position is the sequence number within the stream |
| Timestamp | `time.Time` | Timestamp is the time of the event at this position |

## Methods

### String

String returns a string representation of the cursor

```go
func (Cursor) String() string
```

**Parameters:**
  None

**Returns:**
- string

### EventStore
EventStore defines the interface for event storage and replay

#### Example Usage

```go
// Example implementation of EventStore
type MyEventStore struct {
    // Add your fields here
}

func (m MyEventStore) Append(param1 context.Context, param2 string, param3 ...shared.Event) error {
    // Implement your logic here
    return
}

func (m MyEventStore) Read(param1 context.Context, param2 string, param3 Cursor, param4 int) []shared.Event {
    // Implement your logic here
    return
}

func (m MyEventStore) ReadStream(param1 context.Context, param2 string, param3 int64) []shared.Event {
    // Implement your logic here
    return
}

func (m MyEventStore) ReadTimeRange(param1 context.Context, param2 time.Time) []shared.Event {
    // Implement your logic here
    return
}

func (m MyEventStore) Replay(param1 context.Context, param2 time.Time) <-chan shared.Event {
    // Implement your logic here
    return
}

func (m MyEventStore) Subscribe(param1 context.Context, param2 string, param3 Cursor) <-chan shared.Event {
    // Implement your logic here
    return
}

func (m MyEventStore) GetStreams() []StreamInfo {
    // Implement your logic here
    return
}

func (m MyEventStore) GetStreamInfo(param1 string) StreamInfo {
    // Implement your logic here
    return
}

func (m MyEventStore) Snapshot(param1 context.Context) error {
    // Implement your logic here
    return
}

func (m MyEventStore) Close() error {
    // Implement your logic here
    return
}

func (m MyEventStore) Stats() Stats {
    // Implement your logic here
    return
}


```

#### Type Definition

```go
type EventStore interface {
    Append(ctx context.Context, streamID string, events ...shared.Event) error
    Read(ctx context.Context, streamID string, cursor Cursor, limit int) ([]shared.Event, Cursor, error)
    ReadStream(ctx context.Context, streamID string, fromPosition int64) ([]shared.Event, error)
    ReadTimeRange(ctx context.Context, from, to time.Time) ([]shared.Event, error)
    Replay(ctx context.Context, from, to time.Time) (<-chan shared.Event, error)
    Subscribe(ctx context.Context, streamID string, from Cursor) (<-chan shared.Event, error)
    GetStreams() []StreamInfo
    GetStreamInfo(streamID string) (StreamInfo, error)
    Snapshot(ctx context.Context) error
    Close() error
    Stats() Stats
}
```

## Methods

| Method | Description |
| ------ | ----------- |

### Constructor Functions

### New

New creates a new in-memory EventStore

```go
func New(config Config) EventStore
```

**Parameters:**
- `config` (Config)

**Returns:**
- EventStore

### Stats
Stats provides store statistics

#### Example Usage

```go
// Create a new Stats
stats := Stats{
    TotalEvents: 42,
    TotalStreams: 42,
    EventsAppended: 42,
    EventsRead: 42,
    SubscriptionsActive: 42,
    SnapshotsCreated: 42,
    MemoryUsageBytes: 42,
}
```

#### Type Definition

```go
type Stats struct {
    TotalEvents int64
    TotalStreams int64
    EventsAppended int64
    EventsRead int64
    SubscriptionsActive int64
    SnapshotsCreated int64
    MemoryUsageBytes int64
}
```

### Fields

| Field | Type | Description |
| ----- | ---- | ----------- |
| TotalEvents | `int64` |  |
| TotalStreams | `int64` |  |
| EventsAppended | `int64` |  |
| EventsRead | `int64` |  |
| SubscriptionsActive | `int64` |  |
| SnapshotsCreated | `int64` |  |
| MemoryUsageBytes | `int64` |  |

### StreamInfo
StreamInfo provides information about a stream

#### Example Usage

```go
// Create a new StreamInfo
streaminfo := StreamInfo{
    StreamID: "example",
    EventCount: 42,
    FirstEvent: /* value */,
    LastEvent: /* value */,
    LastPosition: 42,
}
```

#### Type Definition

```go
type StreamInfo struct {
    StreamID string
    EventCount int64
    FirstEvent time.Time
    LastEvent time.Time
    LastPosition int64
}
```

### Fields

| Field | Type | Description |
| ----- | ---- | ----------- |
| StreamID | `string` |  |
| EventCount | `int64` |  |
| FirstEvent | `time.Time` |  |
| LastEvent | `time.Time` |  |
| LastPosition | `int64` |  |

## External Links

- [Package Overview](../packages/memory.md)
- [pkg.go.dev Documentation](https://pkg.go.dev/github.com/kolosys/nova/memory)
- [Source Code](https://github.com/kolosys/nova/tree/main/memory)
