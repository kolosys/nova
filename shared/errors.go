package shared

import (
	"errors"
	"fmt"
)

// Common error types for Nova event system
var (
	// ErrEventNotFound indicates an event was not found in the store
	ErrEventNotFound = errors.New("event not found")

	// ErrInvalidEvent indicates an event failed validation
	ErrInvalidEvent = errors.New("invalid event")

	// ErrListenerNotFound indicates a listener was not found
	ErrListenerNotFound = errors.New("listener not found")

	// ErrTopicNotFound indicates a topic was not found
	ErrTopicNotFound = errors.New("topic not found")

	// ErrSubscriptionNotFound indicates a subscription was not found
	ErrSubscriptionNotFound = errors.New("subscription not found")

	// ErrEmitterClosed indicates the emitter has been closed
	ErrEmitterClosed = errors.New("emitter is closed")

	// ErrBusClosed indicates the bus has been closed
	ErrBusClosed = errors.New("bus is closed")

	// ErrStoreReadOnly indicates the store is in read-only mode
	ErrStoreReadOnly = errors.New("store is read-only")

	// ErrBufferFull indicates a buffer is full and cannot accept more events
	ErrBufferFull = errors.New("buffer is full")

	// ErrRetryLimitExceeded indicates retry attempts have been exhausted
	ErrRetryLimitExceeded = errors.New("retry limit exceeded")

	// ErrTimeout indicates an operation timed out
	ErrTimeout = errors.New("operation timed out")
)

// EventError wraps an error with event context
type EventError struct {
	Event Event
	Err   error
}

func (e *EventError) Error() string {
	return fmt.Sprintf("event error [id=%s, type=%s]: %v", e.Event.ID(), e.Event.Type(), e.Err)
}

func (e *EventError) Unwrap() error {
	return e.Err
}

// NewEventError creates a new EventError
func NewEventError(event Event, err error) *EventError {
	return &EventError{
		Event: event,
		Err:   err,
	}
}

// ListenerError wraps an error with listener context
type ListenerError struct {
	ListenerID string
	Event      Event
	Err        error
}

func (e *ListenerError) Error() string {
	return fmt.Sprintf("listener error [id=%s, event=%s]: %v", e.ListenerID, e.Event.ID(), e.Err)
}

func (e *ListenerError) Unwrap() error {
	return e.Err
}

// NewListenerError creates a new ListenerError
func NewListenerError(listenerID string, event Event, err error) *ListenerError {
	return &ListenerError{
		ListenerID: listenerID,
		Event:      event,
		Err:        err,
	}
}

// ValidationError indicates a validation failure
type ValidationError struct {
	Field   string
	Value   any
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error [field=%s]: %s", e.Field, e.Message)
}

// NewValidationError creates a new ValidationError
func NewValidationError(field string, value any, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}
