package shared

import (
	"time"
)

// Event represents a domain event in the Nova system
type Event interface {
	// ID returns the unique identifier for this event
	ID() string

	// Type returns the event type (e.g., "user.created", "order.completed")
	Type() string

	// Timestamp returns when the event occurred
	Timestamp() time.Time

	// Data returns the event payload
	Data() any

	// Metadata returns event metadata for routing, tracing, etc.
	Metadata() map[string]string
}

// BaseEvent provides a default implementation of the Event interface
type BaseEvent struct {
	id        string
	eventType string
	timestamp time.Time
	data      any
	metadata  map[string]string
}

// NewBaseEvent creates a new BaseEvent
func NewBaseEvent(id, eventType string, data any) *BaseEvent {
	return &BaseEvent{
		id:        id,
		eventType: eventType,
		timestamp: time.Now(),
		data:      data,
		metadata:  make(map[string]string),
	}
}

// NewBaseEventWithMetadata creates a new BaseEvent with metadata
func NewBaseEventWithMetadata(id, eventType string, data any, metadata map[string]string) *BaseEvent {
	if metadata == nil {
		metadata = make(map[string]string)
	}
	return &BaseEvent{
		id:        id,
		eventType: eventType,
		timestamp: time.Now(),
		data:      data,
		metadata:  metadata,
	}
}

// ID returns the event ID
func (e *BaseEvent) ID() string {
	return e.id
}

// Type returns the event type
func (e *BaseEvent) Type() string {
	return e.eventType
}

// Timestamp returns the event timestamp
func (e *BaseEvent) Timestamp() time.Time {
	return e.timestamp
}

// Data returns the event data
func (e *BaseEvent) Data() any {
	return e.data
}

// Metadata returns the event metadata
func (e *BaseEvent) Metadata() map[string]string {
	return e.metadata
}

// SetMetadata sets a metadata key-value pair
func (e *BaseEvent) SetMetadata(key, value string) {
	if e.metadata == nil {
		e.metadata = make(map[string]string)
	}
	e.metadata[key] = value
}

// GetMetadata gets a metadata value by key
func (e *BaseEvent) GetMetadata(key string) (string, bool) {
	if e.metadata == nil {
		return "", false
	}
	value, exists := e.metadata[key]
	return value, exists
}

// Listener represents an event listener
type Listener interface {
	// ID returns the unique identifier for this listener
	ID() string

	// Handle processes an event
	Handle(event Event) error

	// OnError is called when Handle returns an error
	OnError(event Event, err error) error
}

// Subscription represents an active subscription to events
type Subscription interface {
	// ID returns the subscription ID
	ID() string

	// Topic returns the topic this subscription is for (if applicable)
	Topic() string

	// Listener returns the listener for this subscription
	Listener() Listener

	// Unsubscribe removes this subscription
	Unsubscribe() error

	// Active returns whether this subscription is active
	Active() bool
}

// Middleware provides hooks for cross-cutting concerns
type Middleware interface {
	// Before is called before event processing
	Before(event Event) error

	// After is called after event processing (err may be nil)
	After(event Event, err error) error
}

// MiddlewareFunc is a function adapter for Middleware
type MiddlewareFunc struct {
	BeforeFunc func(event Event) error
	AfterFunc  func(event Event, err error) error
}

// Before implements Middleware
func (mf MiddlewareFunc) Before(event Event) error {
	if mf.BeforeFunc != nil {
		return mf.BeforeFunc(event)
	}
	return nil
}

// After implements Middleware
func (mf MiddlewareFunc) After(event Event, err error) error {
	if mf.AfterFunc != nil {
		return mf.AfterFunc(event, err)
	}
	return nil
}

// EventValidator validates events before processing
type EventValidator interface {
	// Validate checks if an event is valid
	Validate(event Event) error
}

// EventValidatorFunc is a function adapter for EventValidator
type EventValidatorFunc func(event Event) error

// Validate implements EventValidator
func (f EventValidatorFunc) Validate(event Event) error {
	return f(event)
}

// DefaultEventValidator provides basic event validation
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
