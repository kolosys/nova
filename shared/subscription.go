package shared

import (
	"context"
	"sync"
	"sync/atomic"
)

// BaseSubscription provides a default implementation of Subscription
type BaseSubscription struct {
	id       string
	topic    string
	listener Listener
	active   int64 // atomic boolean
	mu       sync.RWMutex
	onClose  func()
}

// NewBaseSubscription creates a new BaseSubscription
func NewBaseSubscription(id, topic string, listener Listener) *BaseSubscription {
	return &BaseSubscription{
		id:       id,
		topic:    topic,
		listener: listener,
		active:   1, // start active
	}
}

// NewBaseSubscriptionWithCallback creates a new BaseSubscription with a close callback
func NewBaseSubscriptionWithCallback(id, topic string, listener Listener, onClose func()) *BaseSubscription {
	return &BaseSubscription{
		id:       id,
		topic:    topic,
		listener: listener,
		active:   1, // start active
		onClose:  onClose,
	}
}

// ID returns the subscription ID
func (s *BaseSubscription) ID() string {
	return s.id
}

// Topic returns the topic
func (s *BaseSubscription) Topic() string {
	return s.topic
}

// Listener returns the listener
func (s *BaseSubscription) Listener() Listener {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.listener
}

// Unsubscribe removes this subscription
func (s *BaseSubscription) Unsubscribe() error {
	if !atomic.CompareAndSwapInt64(&s.active, 1, 0) {
		return ErrSubscriptionNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.onClose != nil {
		s.onClose()
	}

	return nil
}

// Active returns whether this subscription is active
func (s *BaseSubscription) Active() bool {
	return atomic.LoadInt64(&s.active) == 1
}

// BaseListener provides a basic implementation of Listener
type BaseListener struct {
	id          string
	handlerFunc func(ctx context.Context, event Event) error
	errorFunc   func(ctx context.Context, event Event, err error) error
}

// NewBaseListener creates a new BaseListener
func NewBaseListener(id string, handler func(ctx context.Context, event Event) error) *BaseListener {
	return &BaseListener{
		id:          id,
		handlerFunc: handler,
	}
}

// NewBaseListenerWithErrorHandler creates a new BaseListener with error handling
func NewBaseListenerWithErrorHandler(id string, handler func(ctx context.Context, event Event) error, errorHandler func(ctx context.Context, event Event, err error) error) *BaseListener {
	return &BaseListener{
		id:          id,
		handlerFunc: handler,
		errorFunc:   errorHandler,
	}
}

// ID returns the listener ID
func (l *BaseListener) ID() string {
	return l.id
}

// Handle processes an event
func (l *BaseListener) Handle(ctx context.Context, event Event) error {
	if l.handlerFunc == nil {
		return nil
	}
	return l.handlerFunc(ctx, event)
}

// OnError handles errors
func (l *BaseListener) OnError(ctx context.Context, event Event, err error) error {
	if l.errorFunc != nil {
		return l.errorFunc(ctx, event, err)
	}
	return err
}
