package emitter

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kolosys/ion/workerpool"
	"github.com/kolosys/nova/shared"
)

// EventEmitter defines the interface for event emission and subscription
type EventEmitter interface {
	// Emit sends an event synchronously to all subscribers
	Emit(ctx context.Context, event shared.Event) error

	// EmitAsync sends an event asynchronously to all subscribers
	EmitAsync(ctx context.Context, event shared.Event) error

	// EmitBatch sends multiple events as a batch
	EmitBatch(ctx context.Context, events []shared.Event) error

	// Subscribe adds a listener for a specific event type
	Subscribe(eventType string, listener shared.Listener) shared.Subscription

	// Middleware adds middleware to the emitter
	Middleware(middleware ...shared.Middleware) EventEmitter

	// Shutdown gracefully shuts down the emitter
	Shutdown(ctx context.Context) error

	// Stats returns emitter statistics
	Stats() Stats
}

// Config configures the EventEmitter
type Config struct {
	// WorkerPool for async event processing (required)
	WorkerPool *workerpool.Pool

	// AsyncMode enables async processing by default
	AsyncMode bool

	// BufferSize sets the size of the async event buffer
	BufferSize int

	// MaxConcurrency limits concurrent event processing per subscription
	MaxConcurrency int

	// MetricsCollector for observability (optional)
	MetricsCollector shared.MetricsCollector

	// EventValidator validates events before processing (optional)
	EventValidator shared.EventValidator

	// Name identifies this emitter instance (for metrics/logging)
	Name string
}

// Stats provides emitter statistics
type Stats struct {
	EventsEmitted    int64
	EventsProcessed  int64
	ActiveListeners  int64
	FailedEvents     int64
	QueuedEvents     int64
	MiddlewareErrors int64
}

// emitter implements EventEmitter
type emitter struct {
	config        Config
	workerpool    *workerpool.Pool
	metrics       shared.MetricsCollector
	validator     shared.EventValidator
	middleware    []shared.Middleware
	subscriptions map[string][]*subscription
	subsMu        sync.RWMutex
	closed        int64 // atomic boolean
	eventQueue    chan eventJob
	stats         Stats
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

// subscription represents an active subscription
type subscription struct {
	*shared.BaseSubscription
	emitter     *emitter
	eventType   string
	listenerID  string        // cached to avoid lock contention
	concurrency chan struct{} // semaphore for concurrency control
}

// eventJob represents a queued event
type eventJob struct {
	ctx      context.Context // Parent context for cancellation and deadline propagation
	event    shared.Event
	subs     []*subscription
	resultCh chan error
}

// New creates a new EventEmitter
func New(config Config) EventEmitter {
	if config.WorkerPool == nil {
		panic("WorkerPool is required")
	}
	if config.BufferSize <= 0 {
		config.BufferSize = 1000
	}
	if config.MaxConcurrency <= 0 {
		config.MaxConcurrency = 10
	}
	if config.MetricsCollector == nil {
		config.MetricsCollector = shared.NoOpMetricsCollector{}
	}
	if config.EventValidator == nil {
		config.EventValidator = shared.DefaultEventValidator
	}
	if config.Name == "" {
		config.Name = "emitter"
	}

	e := &emitter{
		config:        config,
		workerpool:    config.WorkerPool,
		metrics:       config.MetricsCollector,
		validator:     config.EventValidator,
		subscriptions: make(map[string][]*subscription),
		eventQueue:    make(chan eventJob, config.BufferSize),
		stopCh:        make(chan struct{}),
	}

	// Start the event processing loop
	e.wg.Add(1)
	go e.processEvents()

	return e
}

// Emit sends an event synchronously
func (e *emitter) Emit(ctx context.Context, event shared.Event) error {
	if atomic.LoadInt64(&e.closed) == 1 {
		return shared.ErrEmitterClosed
	}

	if err := e.validator.Validate(event); err != nil {
		atomic.AddInt64(&e.stats.FailedEvents, 1)
		e.metrics.IncEventsEmitted(event.Type(), "validation_failed")
		return shared.NewEventError(event, err)
	}

	// Apply middleware before processing
	for _, mw := range e.middleware {
		if err := mw.Before(ctx, event); err != nil {
			atomic.AddInt64(&e.stats.MiddlewareErrors, 1)
			return shared.NewEventError(event, err)
		}
	}

	// Get subscribers
	subs := e.getSubscribers(event.Type())
	if len(subs) == 0 {
		atomic.AddInt64(&e.stats.EventsEmitted, 1)
		e.metrics.IncEventsEmitted(event.Type(), "no_subscribers")
		return nil
	}

	// Process synchronously
	var processingErr error
	for _, sub := range subs {
		if err := e.processEventForSubscription(ctx, event, sub); err != nil {
			processingErr = err
			break
		}
	}

	// Apply middleware after processing
	for i := len(e.middleware) - 1; i >= 0; i-- {
		if err := e.middleware[i].After(ctx, event, processingErr); err != nil {
			atomic.AddInt64(&e.stats.MiddlewareErrors, 1)
			return shared.NewEventError(event, err)
		}
	}

	if processingErr != nil {
		atomic.AddInt64(&e.stats.FailedEvents, 1)
		e.metrics.IncEventsEmitted(event.Type(), "failed")
		return processingErr
	}

	atomic.AddInt64(&e.stats.EventsEmitted, 1)
	e.metrics.IncEventsEmitted(event.Type(), "success")
	return nil
}

// EmitAsync sends an event asynchronously
func (e *emitter) EmitAsync(ctx context.Context, event shared.Event) error {
	if atomic.LoadInt64(&e.closed) == 1 {
		return shared.ErrEmitterClosed
	}

	if err := e.validator.Validate(event); err != nil {
		atomic.AddInt64(&e.stats.FailedEvents, 1)
		e.metrics.IncEventsEmitted(event.Type(), "validation_failed")
		return shared.NewEventError(event, err)
	}

	// Apply middleware before processing
	for _, mw := range e.middleware {
		if err := mw.Before(ctx, event); err != nil {
			atomic.AddInt64(&e.stats.MiddlewareErrors, 1)
			return shared.NewEventError(event, err)
		}
	}

	// Get subscribers
	subs := e.getSubscribers(event.Type())
	if len(subs) == 0 {
		atomic.AddInt64(&e.stats.EventsEmitted, 1)
		e.metrics.IncEventsEmitted(event.Type(), "no_subscribers")
		return nil
	}

	// Queue for async processing with context
	job := eventJob{
		ctx:   ctx,
		event: event,
		subs:  subs,
	}

	select {
	case e.eventQueue <- job:
		atomic.AddInt64(&e.stats.QueuedEvents, 1)
		e.metrics.SetQueueSize(e.config.Name, len(e.eventQueue))
		return nil
	case <-ctx.Done():
		atomic.AddInt64(&e.stats.FailedEvents, 1)
		e.metrics.IncEventsEmitted(event.Type(), "timeout")
		return ctx.Err()
	default:
		atomic.AddInt64(&e.stats.FailedEvents, 1)
		e.metrics.IncEventsEmitted(event.Type(), "buffer_full")
		return shared.ErrBufferFull
	}
}

// EmitBatch sends multiple events as a batch
func (e *emitter) EmitBatch(ctx context.Context, events []shared.Event) error {
	if atomic.LoadInt64(&e.closed) == 1 {
		return shared.ErrEmitterClosed
	}

	if len(events) == 0 {
		return nil
	}

	// Process each event
	for i, event := range events {
		var err error
		if e.config.AsyncMode {
			err = e.EmitAsync(ctx, event)
		} else {
			err = e.Emit(ctx, event)
		}

		if err != nil {
			return fmt.Errorf("batch processing failed at event %d: %w", i, err)
		}
	}

	return nil
}

// Subscribe adds a listener for an event type
func (e *emitter) Subscribe(eventType string, listener shared.Listener) shared.Subscription {
	e.subsMu.Lock()
	defer e.subsMu.Unlock()

	// Create subscription with concurrency control
	listenerID := listener.ID()
	sub := &subscription{
		BaseSubscription: shared.NewBaseSubscriptionWithCallback(
			fmt.Sprintf("%s-%s-%d", e.config.Name, listenerID, time.Now().UnixNano()),
			eventType,
			listener,
			func() {
				e.removeSubscription(eventType, listenerID)
			},
		),
		emitter:     e,
		eventType:   eventType,
		listenerID:  listenerID,
		concurrency: make(chan struct{}, e.config.MaxConcurrency),
	}

	// Add to subscriptions
	e.subscriptions[eventType] = append(e.subscriptions[eventType], sub)
	atomic.AddInt64(&e.stats.ActiveListeners, 1)
	e.metrics.SetActiveListeners(e.config.Name, int(atomic.LoadInt64(&e.stats.ActiveListeners)))

	return sub
}

// Middleware adds middleware to the emitter
func (e *emitter) Middleware(middleware ...shared.Middleware) EventEmitter {
	e.middleware = append(e.middleware, middleware...)
	return e
}

// Shutdown gracefully shuts down the emitter
func (e *emitter) Shutdown(ctx context.Context) error {
	if !atomic.CompareAndSwapInt64(&e.closed, 0, 1) {
		return nil // already closed
	}

	// Signal shutdown
	close(e.stopCh)

	// Wait for background goroutines to finish
	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stats returns emitter statistics
func (e *emitter) Stats() Stats {
	return Stats{
		EventsEmitted:    atomic.LoadInt64(&e.stats.EventsEmitted),
		EventsProcessed:  atomic.LoadInt64(&e.stats.EventsProcessed),
		ActiveListeners:  atomic.LoadInt64(&e.stats.ActiveListeners),
		FailedEvents:     atomic.LoadInt64(&e.stats.FailedEvents),
		QueuedEvents:     atomic.LoadInt64(&e.stats.QueuedEvents),
		MiddlewareErrors: atomic.LoadInt64(&e.stats.MiddlewareErrors),
	}
}

// processEvents runs the async event processing loop
func (e *emitter) processEvents() {
	defer e.wg.Done()

	for {
		select {
		case job := <-e.eventQueue:
			atomic.AddInt64(&e.stats.QueuedEvents, -1)
			e.metrics.SetQueueSize(e.config.Name, len(e.eventQueue))
			e.processEventJob(job)

		case <-e.stopCh:
			// Process remaining events in queue
			for {
				select {
				case job := <-e.eventQueue:
					e.processEventJob(job)
				default:
					return
				}
			}
		}
	}
}

// processEventJob processes a single event job
func (e *emitter) processEventJob(job eventJob) {
	var processingErr error

	// Use stored context from job for proper propagation
	ctx := job.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	// Process all subscriptions for this event
	for _, sub := range job.subs {
		if err := e.processEventForSubscription(ctx, job.event, sub); err != nil {
			processingErr = err
			break
		}
	}

	// Apply middleware after processing
	for i := len(e.middleware) - 1; i >= 0; i-- {
		if err := e.middleware[i].After(ctx, job.event, processingErr); err != nil {
			atomic.AddInt64(&e.stats.MiddlewareErrors, 1)
			processingErr = err
		}
	}

	if processingErr != nil {
		atomic.AddInt64(&e.stats.FailedEvents, 1)
		e.metrics.IncEventsEmitted(job.event.Type(), "failed")
	} else {
		atomic.AddInt64(&e.stats.EventsEmitted, 1)
		e.metrics.IncEventsEmitted(job.event.Type(), "success")
	}

	// Send result if requested
	if job.resultCh != nil {
		select {
		case job.resultCh <- processingErr:
		default:
		}
	}
}

// processEventForSubscription processes an event for a specific subscription
func (e *emitter) processEventForSubscription(ctx context.Context, event shared.Event, sub *subscription) error {
	if !sub.Active() {
		return nil
	}

	// Acquire concurrency slot
	select {
	case sub.concurrency <- struct{}{}:
		defer func() { <-sub.concurrency }()
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Would block, submit to worker pool
		return e.workerpool.Submit(ctx, func(ctx context.Context) error {
			select {
			case sub.concurrency <- struct{}{}:
				defer func() { <-sub.concurrency }()
			case <-ctx.Done():
				return ctx.Err()
			}

			return e.handleEventWithListener(ctx, event, sub.Listener())
		})
	}

	return e.handleEventWithListener(ctx, event, sub.Listener())
}

// handleEventWithListener processes an event with a listener
func (e *emitter) handleEventWithListener(ctx context.Context, event shared.Event, listener shared.Listener) error {
	start := time.Now()

	err := listener.Handle(ctx, event)

	duration := time.Since(start)
	e.metrics.ObserveListenerDuration(listener.ID(), duration)

	if err != nil {
		atomic.AddInt64(&e.stats.FailedEvents, 1)
		e.metrics.IncEventsProcessed(listener.ID(), "failed")

		// Call error handler with context
		if handlerErr := listener.OnError(ctx, event, err); handlerErr != nil {
			return shared.NewListenerError(listener.ID(), event, handlerErr)
		}
		return shared.NewListenerError(listener.ID(), event, err)
	}

	atomic.AddInt64(&e.stats.EventsProcessed, 1)
	e.metrics.IncEventsProcessed(listener.ID(), "success")
	return nil
}

// getSubscribers returns all subscribers for an event type
func (e *emitter) getSubscribers(eventType string) []*subscription {
	e.subsMu.RLock()
	defer e.subsMu.RUnlock()

	subs := e.subscriptions[eventType]
	if len(subs) == 0 {
		return nil
	}

	// Return active subscriptions only
	result := make([]*subscription, 0, len(subs))
	for _, sub := range subs {
		if sub.Active() {
			result = append(result, sub)
		}
	}

	return result
}

// removeSubscription removes a subscription
func (e *emitter) removeSubscription(eventType, listenerID string) {
	e.subsMu.Lock()
	defer e.subsMu.Unlock()

	subs := e.subscriptions[eventType]
	for i, sub := range subs {
		if sub.listenerID == listenerID {
			// Remove from slice
			e.subscriptions[eventType] = append(subs[:i], subs[i+1:]...)
			atomic.AddInt64(&e.stats.ActiveListeners, -1)
			e.metrics.SetActiveListeners(e.config.Name, int(atomic.LoadInt64(&e.stats.ActiveListeners)))
			break
		}
	}
}
