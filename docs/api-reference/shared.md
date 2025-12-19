# shared API

Complete API documentation for the shared package.

**Import Path:** `github.com/kolosys/nova/shared`

## Package Documentation



## Variables

**ErrEventNotFound, ErrInvalidEvent, ErrListenerNotFound, ErrTopicNotFound, ErrSubscriptionNotFound, ErrEmitterClosed, ErrBusClosed, ErrStoreReadOnly, ErrBufferFull, ErrRetryLimitExceeded, ErrTimeout**

Common error types for Nova event system


```go
var ErrEventNotFound = errors.New("event not found")	// ErrEventNotFound indicates an event was not found in the store

var ErrInvalidEvent = errors.New("invalid event")	// ErrInvalidEvent indicates an event failed validation

var ErrListenerNotFound = errors.New("listener not found")	// ErrListenerNotFound indicates a listener was not found

var ErrTopicNotFound = errors.New("topic not found")	// ErrTopicNotFound indicates a topic was not found

var ErrSubscriptionNotFound = errors.New("subscription not found")	// ErrSubscriptionNotFound indicates a subscription was not found

var ErrEmitterClosed = errors.New("emitter is closed")	// ErrEmitterClosed indicates the emitter has been closed

var ErrBusClosed = errors.New("bus is closed")	// ErrBusClosed indicates the bus has been closed

var ErrStoreReadOnly = errors.New("store is read-only")	// ErrStoreReadOnly indicates the store is in read-only mode

var ErrBufferFull = errors.New("buffer is full")	// ErrBufferFull indicates a buffer is full and cannot accept more events

var ErrRetryLimitExceeded = errors.New("retry limit exceeded")	// ErrRetryLimitExceeded indicates retry attempts have been exhausted

var ErrTimeout = errors.New("operation timed out")	// ErrTimeout indicates an operation timed out

```

**DefaultEventValidator**

DefaultEventValidator provides basic event validation


```go
var DefaultEventValidator = EventValidatorFunc(func(event Event) error {
	if event == nil {
		return NewValidationError("event", nil, "event cannot be nil")
	}
	if event.ID() == "" {
		return NewValidationError("id", event.ID(), "event ID cannot be empty")
	}
	if event.Type() == "" {
		return NewValidationError("type", event.Type(), "event type cannot be empty")
	}
	if event.Timestamp().IsZero() {
		return NewValidationError("timestamp", event.Timestamp(), "event timestamp cannot be zero")
	}
	return nil
})
```

## Types

### BaseEvent
BaseEvent provides a default implementation of the Event interface

#### Example Usage

```go
// Create a new BaseEvent
baseevent := BaseEvent{

}
```

#### Type Definition

```go
type BaseEvent struct {
}
```

### Constructor Functions

### NewBaseEvent

NewBaseEvent creates a new BaseEvent

```go
func NewBaseEvent(id, eventType string, data any) *BaseEvent
```

**Parameters:**
- `id` (string)
- `eventType` (string)
- `data` (any)

**Returns:**
- *BaseEvent

### NewBaseEventWithMetadata

NewBaseEventWithMetadata creates a new BaseEvent with metadata

```go
func NewBaseEventWithMetadata(id, eventType string, data any, metadata map[string]string) *BaseEvent
```

**Parameters:**
- `id` (string)
- `eventType` (string)
- `data` (any)
- `metadata` (map[string]string)

**Returns:**
- *BaseEvent

## Methods

### Data

Data returns the event data

```go
func (*BaseEvent) Data() any
```

**Parameters:**
  None

**Returns:**
- any

### GetMetadata

GetMetadata gets a metadata value by key

```go
func (*BaseEvent) GetMetadata(key string) (string, bool)
```

**Parameters:**
- `key` (string)

**Returns:**
- string
- bool

### ID

ID returns the event ID

```go
func (*BaseListener) ID() string
```

**Parameters:**
  None

**Returns:**
- string

### Metadata

Metadata returns the event metadata

```go
func (*BaseEvent) Metadata() map[string]string
```

**Parameters:**
  None

**Returns:**
- map[string]string

### SetMetadata

SetMetadata sets a metadata key-value pair

```go
func (*BaseEvent) SetMetadata(key, value string)
```

**Parameters:**
- `key` (string)
- `value` (string)

**Returns:**
  None

### Timestamp

Timestamp returns the event timestamp

```go
func (*BaseEvent) Timestamp() time.Time
```

**Parameters:**
  None

**Returns:**
- time.Time

### Type

Type returns the event type

```go
func (*BaseEvent) Type() string
```

**Parameters:**
  None

**Returns:**
- string

### BaseListener
BaseListener provides a basic implementation of Listener

#### Example Usage

```go
// Create a new BaseListener
baselistener := BaseListener{

}
```

#### Type Definition

```go
type BaseListener struct {
}
```

### Constructor Functions

### NewBaseListener

NewBaseListener creates a new BaseListener

```go
func NewBaseListener(id string, handler func(event Event) error) *BaseListener
```

**Parameters:**
- `id` (string)
- `handler` (func(event Event) error)

**Returns:**
- *BaseListener

### NewBaseListenerWithErrorHandler

NewBaseListenerWithErrorHandler creates a new BaseListener with error handling

```go
func NewBaseListenerWithErrorHandler(id string, handler func(event Event) error, errorHandler func(event Event, err error) error) *BaseListener
```

**Parameters:**
- `id` (string)
- `handler` (func(event Event) error)
- `errorHandler` (func(event Event, err error) error)

**Returns:**
- *BaseListener

## Methods

### Handle

Handle processes an event

```go
func (*BaseListener) Handle(event Event) error
```

**Parameters:**
- `event` (Event)

**Returns:**
- error

### ID

ID returns the listener ID

```go
func (*BaseEvent) ID() string
```

**Parameters:**
  None

**Returns:**
- string

### OnError

OnError handles errors

```go
func (*BaseListener) OnError(event Event, err error) error
```

**Parameters:**
- `event` (Event)
- `err` (error)

**Returns:**
- error

### BaseSubscription
BaseSubscription provides a default implementation of Subscription

#### Example Usage

```go
// Create a new BaseSubscription
basesubscription := BaseSubscription{

}
```

#### Type Definition

```go
type BaseSubscription struct {
}
```

### Constructor Functions

### NewBaseSubscription

NewBaseSubscription creates a new BaseSubscription

```go
func NewBaseSubscription(id, topic string, listener Listener) *BaseSubscription
```

**Parameters:**
- `id` (string)
- `topic` (string)
- `listener` (Listener)

**Returns:**
- *BaseSubscription

### NewBaseSubscriptionWithCallback

NewBaseSubscriptionWithCallback creates a new BaseSubscription with a close callback

```go
func NewBaseSubscriptionWithCallback(id, topic string, listener Listener, onClose func()) *BaseSubscription
```

**Parameters:**
- `id` (string)
- `topic` (string)
- `listener` (Listener)
- `onClose` (func())

**Returns:**
- *BaseSubscription

## Methods

### Active

Active returns whether this subscription is active

```go
func (*BaseSubscription) Active() bool
```

**Parameters:**
  None

**Returns:**
- bool

### ID

ID returns the subscription ID

```go
func (*BaseEvent) ID() string
```

**Parameters:**
  None

**Returns:**
- string

### Listener

Listener returns the listener

```go
func (*BaseSubscription) Listener() Listener
```

**Parameters:**
  None

**Returns:**
- Listener

### Topic

Topic returns the topic

```go
func (*BaseSubscription) Topic() string
```

**Parameters:**
  None

**Returns:**
- string

### Unsubscribe

Unsubscribe removes this subscription

```go
func (*BaseSubscription) Unsubscribe() error
```

**Parameters:**
  None

**Returns:**
- error

### Event
Event represents a domain event in the Nova system

#### Example Usage

```go
// Example implementation of Event
type MyEvent struct {
    // Add your fields here
}

func (m MyEvent) ID() string {
    // Implement your logic here
    return
}

func (m MyEvent) Type() string {
    // Implement your logic here
    return
}

func (m MyEvent) Timestamp() time.Time {
    // Implement your logic here
    return
}

func (m MyEvent) Data() any {
    // Implement your logic here
    return
}

func (m MyEvent) Metadata() map[string]string {
    // Implement your logic here
    return
}


```

#### Type Definition

```go
type Event interface {
    ID() string
    Type() string
    Timestamp() time.Time
    Data() any
    Metadata() map[string]string
}
```

## Methods

| Method | Description |
| ------ | ----------- |

### EventError
EventError wraps an error with event context

#### Example Usage

```go
// Create a new EventError
eventerror := EventError{
    Event: Event{},
    Err: error{},
}
```

#### Type Definition

```go
type EventError struct {
    Event Event
    Err error
}
```

### Fields

| Field | Type | Description |
| ----- | ---- | ----------- |
| Event | `Event` |  |
| Err | `error` |  |

### Constructor Functions

### NewEventError

NewEventError creates a new EventError

```go
func NewEventError(event Event, err error) *EventError
```

**Parameters:**
- `event` (Event)
- `err` (error)

**Returns:**
- *EventError

## Methods

### Error



```go
func (*ValidationError) Error() string
```

**Parameters:**
  None

**Returns:**
- string

### Unwrap



```go
func (*ListenerError) Unwrap() error
```

**Parameters:**
  None

**Returns:**
- error

### EventValidator
EventValidator validates events before processing

#### Example Usage

```go
// Example implementation of EventValidator
type MyEventValidator struct {
    // Add your fields here
}

func (m MyEventValidator) Validate(param1 Event) error {
    // Implement your logic here
    return
}


```

#### Type Definition

```go
type EventValidator interface {
    Validate(event Event) error
}
```

## Methods

| Method | Description |
| ------ | ----------- |

### EventValidatorFunc
EventValidatorFunc is a function adapter for EventValidator

#### Example Usage

```go
// Example usage of EventValidatorFunc
var value EventValidatorFunc
// Initialize with appropriate value
```

#### Type Definition

```go
type EventValidatorFunc func(event Event) error
```

## Methods

### Validate

Validate implements EventValidator

```go
func (EventValidatorFunc) Validate(event Event) error
```

**Parameters:**
- `event` (Event)

**Returns:**
- error

### Listener
Listener represents an event listener

#### Example Usage

```go
// Example implementation of Listener
type MyListener struct {
    // Add your fields here
}

func (m MyListener) ID() string {
    // Implement your logic here
    return
}

func (m MyListener) Handle(param1 Event) error {
    // Implement your logic here
    return
}

func (m MyListener) OnError(param1 Event, param2 error) error {
    // Implement your logic here
    return
}


```

#### Type Definition

```go
type Listener interface {
    ID() string
    Handle(event Event) error
    OnError(event Event, err error) error
}
```

## Methods

| Method | Description |
| ------ | ----------- |

### ListenerError
ListenerError wraps an error with listener context

#### Example Usage

```go
// Create a new ListenerError
listenererror := ListenerError{
    ListenerID: "example",
    Event: Event{},
    Err: error{},
}
```

#### Type Definition

```go
type ListenerError struct {
    ListenerID string
    Event Event
    Err error
}
```

### Fields

| Field | Type | Description |
| ----- | ---- | ----------- |
| ListenerID | `string` |  |
| Event | `Event` |  |
| Err | `error` |  |

### Constructor Functions

### NewListenerError

NewListenerError creates a new ListenerError

```go
func NewListenerError(listenerID string, event Event, err error) *ListenerError
```

**Parameters:**
- `listenerID` (string)
- `event` (Event)
- `err` (error)

**Returns:**
- *ListenerError

## Methods

### Error



```go
func (*ValidationError) Error() string
```

**Parameters:**
  None

**Returns:**
- string

### Unwrap



```go
func (*ListenerError) Unwrap() error
```

**Parameters:**
  None

**Returns:**
- error

### MetricsCollector
MetricsCollector defines the interface for collecting Nova metrics

#### Example Usage

```go
// Example implementation of MetricsCollector
type MyMetricsCollector struct {
    // Add your fields here
}

func (m MyMetricsCollector) IncEventsEmitted(param1 string, param2 string)  {
    // Implement your logic here
    return
}

func (m MyMetricsCollector) IncEventsProcessed(param1 string, param2 string)  {
    // Implement your logic here
    return
}

func (m MyMetricsCollector) ObserveListenerDuration(param1 string, param2 time.Duration)  {
    // Implement your logic here
    return
}

func (m MyMetricsCollector) ObserveSagaDuration(param1 string, param2 string, param3 time.Duration)  {
    // Implement your logic here
    return
}

func (m MyMetricsCollector) ObserveStoreAppendDuration(param1 time.Duration)  {
    // Implement your logic here
    return
}

func (m MyMetricsCollector) ObserveStoreReadDuration(param1 time.Duration)  {
    // Implement your logic here
    return
}

func (m MyMetricsCollector) SetQueueSize(param1 string, param2 int)  {
    // Implement your logic here
    return
}

func (m MyMetricsCollector) SetActiveListeners(param1 string, param2 int)  {
    // Implement your logic here
    return
}

func (m MyMetricsCollector) IncEventsDropped(param1 string, param2 string)  {
    // Implement your logic here
    return
}


```

#### Type Definition

```go
type MetricsCollector interface {
    IncEventsEmitted(eventType string, result string)
    IncEventsProcessed(listenerID string, result string)
    ObserveListenerDuration(listenerID string, duration time.Duration)
    ObserveSagaDuration(sagaID string, step string, duration time.Duration)
    ObserveStoreAppendDuration(duration time.Duration)
    ObserveStoreReadDuration(duration time.Duration)
    SetQueueSize(component string, size int)
    SetActiveListeners(emitterID string, count int)
    IncEventsDropped(component string, reason string)
}
```

## Methods

| Method | Description |
| ------ | ----------- |

### Middleware
Middleware provides hooks for cross-cutting concerns

#### Example Usage

```go
// Example implementation of Middleware
type MyMiddleware struct {
    // Add your fields here
}

func (m MyMiddleware) Before(param1 Event) error {
    // Implement your logic here
    return
}

func (m MyMiddleware) After(param1 Event, param2 error) error {
    // Implement your logic here
    return
}


```

#### Type Definition

```go
type Middleware interface {
    Before(event Event) error
    After(event Event, err error) error
}
```

## Methods

| Method | Description |
| ------ | ----------- |

### MiddlewareFunc
MiddlewareFunc is a function adapter for Middleware

#### Example Usage

```go
// Create a new MiddlewareFunc
middlewarefunc := MiddlewareFunc{
    BeforeFunc: /* value */,
    AfterFunc: /* value */,
}
```

#### Type Definition

```go
type MiddlewareFunc struct {
    BeforeFunc func(event Event) error
    AfterFunc func(event Event, err error) error
}
```

### Fields

| Field | Type | Description |
| ----- | ---- | ----------- |
| BeforeFunc | `func(event Event) error` |  |
| AfterFunc | `func(event Event, err error) error` |  |

## Methods

### After

After implements Middleware

```go
func (MiddlewareFunc) After(event Event, err error) error
```

**Parameters:**
- `event` (Event)
- `err` (error)

**Returns:**
- error

### Before

Before implements Middleware

```go
func (MiddlewareFunc) Before(event Event) error
```

**Parameters:**
- `event` (Event)

**Returns:**
- error

### NoOpMetricsCollector
NoOpMetricsCollector provides a no-op implementation for when metrics are disabled

#### Example Usage

```go
// Create a new NoOpMetricsCollector
noopmetricscollector := NoOpMetricsCollector{

}
```

#### Type Definition

```go
type NoOpMetricsCollector struct {
}
```

## Methods

### IncEventsDropped

IncEventsDropped does nothing

```go
func (*SimpleMetricsCollector) IncEventsDropped(component string, reason string)
```

**Parameters:**
- `component` (string)
- `reason` (string)

**Returns:**
  None

### IncEventsEmitted

IncEventsEmitted does nothing

```go
func (*SimpleMetricsCollector) IncEventsEmitted(eventType string, result string)
```

**Parameters:**
- `eventType` (string)
- `result` (string)

**Returns:**
  None

### IncEventsProcessed

IncEventsProcessed does nothing

```go
func (*SimpleMetricsCollector) IncEventsProcessed(listenerID string, result string)
```

**Parameters:**
- `listenerID` (string)
- `result` (string)

**Returns:**
  None

### ObserveListenerDuration

ObserveListenerDuration does nothing

```go
func (*SimpleMetricsCollector) ObserveListenerDuration(listenerID string, duration time.Duration)
```

**Parameters:**
- `listenerID` (string)
- `duration` (time.Duration)

**Returns:**
  None

### ObserveSagaDuration

ObserveSagaDuration does nothing

```go
func (*SimpleMetricsCollector) ObserveSagaDuration(sagaID string, step string, duration time.Duration)
```

**Parameters:**
- `sagaID` (string)
- `step` (string)
- `duration` (time.Duration)

**Returns:**
  None

### ObserveStoreAppendDuration

ObserveStoreAppendDuration does nothing

```go
func (*SimpleMetricsCollector) ObserveStoreAppendDuration(duration time.Duration)
```

**Parameters:**
- `duration` (time.Duration)

**Returns:**
  None

### ObserveStoreReadDuration

ObserveStoreReadDuration does nothing

```go
func (*SimpleMetricsCollector) ObserveStoreReadDuration(duration time.Duration)
```

**Parameters:**
- `duration` (time.Duration)

**Returns:**
  None

### SetActiveListeners

SetActiveListeners does nothing

```go
func (*SimpleMetricsCollector) SetActiveListeners(emitterID string, count int)
```

**Parameters:**
- `emitterID` (string)
- `count` (int)

**Returns:**
  None

### SetQueueSize

SetQueueSize does nothing

```go
func (*SimpleMetricsCollector) SetQueueSize(component string, size int)
```

**Parameters:**
- `component` (string)
- `size` (int)

**Returns:**
  None

### SimpleMetricsCollector
SimpleMetricsCollector provides a basic in-memory metrics collector for testing

#### Example Usage

```go
// Create a new SimpleMetricsCollector
simplemetricscollector := SimpleMetricsCollector{
    EventsEmitted: 42,
    EventsProcessed: 42,
    EventsDropped: 42,
    ListenerDurations: 42,
    SagaDurations: 42,
    StoreAppends: 42,
    StoreReads: 42,
    QueueSizes: map[],
    ActiveListeners: map[],
}
```

#### Type Definition

```go
type SimpleMetricsCollector struct {
    EventsEmitted int64
    EventsProcessed int64
    EventsDropped int64
    ListenerDurations int64
    SagaDurations int64
    StoreAppends int64
    StoreReads int64
    QueueSizes map[string]int64
    ActiveListeners map[string]int64
}
```

### Fields

| Field | Type | Description |
| ----- | ---- | ----------- |
| EventsEmitted | `int64` |  |
| EventsProcessed | `int64` |  |
| EventsDropped | `int64` |  |
| ListenerDurations | `int64` |  |
| SagaDurations | `int64` |  |
| StoreAppends | `int64` |  |
| StoreReads | `int64` |  |
| QueueSizes | `map[string]int64` |  |
| ActiveListeners | `map[string]int64` |  |

### Constructor Functions

### NewSimpleMetricsCollector

NewSimpleMetricsCollector creates a new SimpleMetricsCollector

```go
func NewSimpleMetricsCollector() *SimpleMetricsCollector
```

**Parameters:**
  None

**Returns:**
- *SimpleMetricsCollector

## Methods

### GetEventsDropped

GetEventsDropped returns the current events dropped count

```go
func (*SimpleMetricsCollector) GetEventsDropped() int64
```

**Parameters:**
  None

**Returns:**
- int64

### GetEventsEmitted

GetEventsEmitted returns the current events emitted count

```go
func (*SimpleMetricsCollector) GetEventsEmitted() int64
```

**Parameters:**
  None

**Returns:**
- int64

### GetEventsProcessed

GetEventsProcessed returns the current events processed count

```go
func (*SimpleMetricsCollector) GetEventsProcessed() int64
```

**Parameters:**
  None

**Returns:**
- int64

### IncEventsDropped

IncEventsDropped increments dropped events counter

```go
func (*SimpleMetricsCollector) IncEventsDropped(component string, reason string)
```

**Parameters:**
- `component` (string)
- `reason` (string)

**Returns:**
  None

### IncEventsEmitted

IncEventsEmitted increments events emitted counter

```go
func (*SimpleMetricsCollector) IncEventsEmitted(eventType string, result string)
```

**Parameters:**
- `eventType` (string)
- `result` (string)

**Returns:**
  None

### IncEventsProcessed

IncEventsProcessed increments events processed counter

```go
func (*SimpleMetricsCollector) IncEventsProcessed(listenerID string, result string)
```

**Parameters:**
- `listenerID` (string)
- `result` (string)

**Returns:**
  None

### ObserveListenerDuration

ObserveListenerDuration records listener processing duration

```go
func (*SimpleMetricsCollector) ObserveListenerDuration(listenerID string, duration time.Duration)
```

**Parameters:**
- `listenerID` (string)
- `duration` (time.Duration)

**Returns:**
  None

### ObserveSagaDuration

ObserveSagaDuration records saga step duration

```go
func (*SimpleMetricsCollector) ObserveSagaDuration(sagaID string, step string, duration time.Duration)
```

**Parameters:**
- `sagaID` (string)
- `step` (string)
- `duration` (time.Duration)

**Returns:**
  None

### ObserveStoreAppendDuration

ObserveStoreAppendDuration records store append duration

```go
func (*SimpleMetricsCollector) ObserveStoreAppendDuration(duration time.Duration)
```

**Parameters:**
- `duration` (time.Duration)

**Returns:**
  None

### ObserveStoreReadDuration

ObserveStoreReadDuration records store read duration

```go
func (*SimpleMetricsCollector) ObserveStoreReadDuration(duration time.Duration)
```

**Parameters:**
- `duration` (time.Duration)

**Returns:**
  None

### SetActiveListeners

SetActiveListeners sets current active listeners count

```go
func (*SimpleMetricsCollector) SetActiveListeners(emitterID string, count int)
```

**Parameters:**
- `emitterID` (string)
- `count` (int)

**Returns:**
  None

### SetQueueSize

SetQueueSize sets current queue size

```go
func (*SimpleMetricsCollector) SetQueueSize(component string, size int)
```

**Parameters:**
- `component` (string)
- `size` (int)

**Returns:**
  None

### Subscription
Subscription represents an active subscription to events

#### Example Usage

```go
// Example implementation of Subscription
type MySubscription struct {
    // Add your fields here
}

func (m MySubscription) ID() string {
    // Implement your logic here
    return
}

func (m MySubscription) Topic() string {
    // Implement your logic here
    return
}

func (m MySubscription) Listener() Listener {
    // Implement your logic here
    return
}

func (m MySubscription) Unsubscribe() error {
    // Implement your logic here
    return
}

func (m MySubscription) Active() bool {
    // Implement your logic here
    return
}


```

#### Type Definition

```go
type Subscription interface {
    ID() string
    Topic() string
    Listener() Listener
    Unsubscribe() error
    Active() bool
}
```

## Methods

| Method | Description |
| ------ | ----------- |

### ValidationError
ValidationError indicates a validation failure

#### Example Usage

```go
// Create a new ValidationError
validationerror := ValidationError{
    Field: "example",
    Value: any{},
    Message: "example",
}
```

#### Type Definition

```go
type ValidationError struct {
    Field string
    Value any
    Message string
}
```

### Fields

| Field | Type | Description |
| ----- | ---- | ----------- |
| Field | `string` |  |
| Value | `any` |  |
| Message | `string` |  |

### Constructor Functions

### NewValidationError

NewValidationError creates a new ValidationError

```go
func NewValidationError(field string, value any, message string) *ValidationError
```

**Parameters:**
- `field` (string)
- `value` (any)
- `message` (string)

**Returns:**
- *ValidationError

## Methods

### Error



```go
func (*ValidationError) Error() string
```

**Parameters:**
  None

**Returns:**
- string

## External Links

- [Package Overview](../packages/shared.md)
- [pkg.go.dev Documentation](https://pkg.go.dev/github.com/kolosys/nova/shared)
- [Source Code](https://github.com/kolosys/nova/tree/main/shared)
