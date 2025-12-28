package bus

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kolosys/ion/workerpool"
	"github.com/kolosys/nova/shared"
)

// DeliveryMode defines the delivery guarantees for events
type DeliveryMode int

const (
	// AtMostOnce provides fire-and-forget delivery with no guarantees
	AtMostOnce DeliveryMode = iota
	// AtLeastOnce guarantees delivery but may deliver duplicates
	AtLeastOnce
	// ExactlyOnce guarantees delivery without duplicates (highest overhead)
	ExactlyOnce
)

// String returns the string representation of DeliveryMode
func (d DeliveryMode) String() string {
	switch d {
	case AtMostOnce:
		return "at-most-once"
	case AtLeastOnce:
		return "at-least-once"
	case ExactlyOnce:
		return "exactly-once"
	default:
		return "unknown"
	}
}

// TopicConfig configures a topic
type TopicConfig struct {
	// BufferSize sets the buffer size for this topic
	BufferSize int

	// Partitions sets the number of partitions for parallel processing
	Partitions int

	// Retention sets how long events are retained
	Retention time.Duration

	// DeliveryMode sets the delivery guarantees
	DeliveryMode DeliveryMode

	// OrderingKey function to determine partition for event ordering
	OrderingKey func(shared.Event) string

	// MaxConcurrency limits concurrent processing per partition
	MaxConcurrency int
}

// DefaultTopicConfig returns a default topic configuration
func DefaultTopicConfig() TopicConfig {
	return TopicConfig{
		BufferSize:     1000,
		Partitions:     1,
		Retention:      24 * time.Hour,
		DeliveryMode:   AtLeastOnce,
		MaxConcurrency: 10,
		OrderingKey:    func(e shared.Event) string { return e.ID() },
	}
}

// EventBus defines the interface for topic-based event routing
type EventBus interface {
	// Publish sends an event to a topic
	Publish(ctx context.Context, topic string, event shared.Event) error

	// Subscribe adds a listener to a topic
	Subscribe(topic string, listener shared.Listener) shared.Subscription

	// SubscribePattern adds a listener to topics matching a pattern
	SubscribePattern(pattern string, listener shared.Listener) shared.Subscription

	// CreateTopic creates a topic with specific configuration
	CreateTopic(topic string, config TopicConfig) error

	// DeleteTopic removes a topic
	DeleteTopic(topic string) error

	// Topics returns all available topics
	Topics() []string

	// Shutdown gracefully shuts down the bus
	Shutdown(ctx context.Context) error

	// Stats returns bus statistics
	Stats() Stats
}

// Config configures the EventBus
type Config struct {
	// WorkerPool for async event processing (required)
	WorkerPool *workerpool.Pool

	// DefaultBufferSize sets default buffer size for topics
	DefaultBufferSize int

	// DefaultPartitions sets default number of partitions
	DefaultPartitions int

	// DefaultDeliveryMode sets default delivery mode
	DefaultDeliveryMode DeliveryMode

	// MetricsCollector for observability (optional)
	MetricsCollector shared.MetricsCollector

	// EventValidator validates events before processing (optional)
	EventValidator shared.EventValidator

	// Name identifies this bus instance
	Name string
}

// Stats provides bus statistics
type Stats struct {
	EventsPublished   int64
	EventsProcessed   int64
	ActiveTopics      int64
	ActiveSubscribers int64
	FailedEvents      int64
	QueuedEvents      int64
}

// eventBus implements EventBus
type eventBus struct {
	config        Config
	workerpool    *workerpool.Pool
	metrics       shared.MetricsCollector
	validator     shared.EventValidator
	topics        map[string]*topic
	topicsMu      sync.RWMutex
	subscriptions map[string][]*subscription
	subsMu        sync.RWMutex
	patterns      []*patternSubscription
	patternsMu    sync.RWMutex
	closed        int64 // atomic boolean
	stats         Stats
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

// topic represents a topic with its configuration and state
type topic struct {
	name       string
	config     TopicConfig
	partitions []*partition
	created    time.Time
}

// partition represents a single partition within a topic
type partition struct {
	id          int
	eventQueue  chan eventJob
	subscribers []*subscription
	subsMu      sync.RWMutex
	processed   int64
	failed      int64
}

// subscription represents an active topic subscription
type subscription struct {
	*shared.BaseSubscription
	bus        *eventBus
	topicName  string
	partition  int    // -1 for all partitions
	listenerID string // cached to avoid lock contention
}

// patternSubscription represents a pattern-based subscription
type patternSubscription struct {
	*subscription
	pattern    *regexp.Regexp
	rawPattern string
}

// eventJob represents a queued event for processing
type eventJob struct {
	ctx         context.Context // Parent context for cancellation and deadline propagation
	event       shared.Event
	topic       string
	partition   int
	delivery    DeliveryMode
	attempts    int
	maxAttempts int
}

// New creates a new EventBus
func New(config Config) EventBus {
	if config.WorkerPool == nil {
		panic("WorkerPool is required")
	}
	if config.DefaultBufferSize <= 0 {
		config.DefaultBufferSize = 1000
	}
	if config.DefaultPartitions <= 0 {
		config.DefaultPartitions = 1
	}
	if config.MetricsCollector == nil {
		config.MetricsCollector = shared.NoOpMetricsCollector{}
	}
	if config.EventValidator == nil {
		config.EventValidator = shared.DefaultEventValidator
	}
	if config.Name == "" {
		config.Name = "bus"
	}

	b := &eventBus{
		config:        config,
		workerpool:    config.WorkerPool,
		metrics:       config.MetricsCollector,
		validator:     config.EventValidator,
		topics:        make(map[string]*topic),
		subscriptions: make(map[string][]*subscription),
		patterns:      make([]*patternSubscription, 0),
		stopCh:        make(chan struct{}),
	}

	return b
}

// Publish sends an event to a topic
func (b *eventBus) Publish(ctx context.Context, topicName string, event shared.Event) error {
	if atomic.LoadInt64(&b.closed) == 1 {
		return shared.ErrBusClosed
	}

	if err := b.validator.Validate(event); err != nil {
		atomic.AddInt64(&b.stats.FailedEvents, 1)
		b.metrics.IncEventsEmitted(event.Type(), "validation_failed")
		return shared.NewEventError(event, err)
	}

	// Get or create topic
	t, err := b.getOrCreateTopic(topicName)
	if err != nil {
		atomic.AddInt64(&b.stats.FailedEvents, 1)
		return err
	}

	// Determine partition
	partitionID := b.selectPartition(t, event)
	partition := t.partitions[partitionID]

	// Create event job with context for propagation
	job := eventJob{
		ctx:         ctx,
		event:       event,
		topic:       topicName,
		partition:   partitionID,
		delivery:    t.config.DeliveryMode,
		attempts:    0,
		maxAttempts: 3, // Default retry attempts
	}

	// Queue event for processing
	select {
	case partition.eventQueue <- job:
		atomic.AddInt64(&b.stats.QueuedEvents, 1)
		atomic.AddInt64(&b.stats.EventsPublished, 1)
		b.metrics.IncEventsEmitted(event.Type(), "queued")
		b.metrics.SetQueueSize(fmt.Sprintf("%s-topic-%s-partition-%d", b.config.Name, topicName, partitionID), len(partition.eventQueue))
		return nil
	case <-ctx.Done():
		atomic.AddInt64(&b.stats.FailedEvents, 1)
		b.metrics.IncEventsEmitted(event.Type(), "timeout")
		return ctx.Err()
	default:
		atomic.AddInt64(&b.stats.FailedEvents, 1)
		b.metrics.IncEventsEmitted(event.Type(), "buffer_full")
		return shared.ErrBufferFull
	}
}

// Subscribe adds a listener to a topic
func (b *eventBus) Subscribe(topicName string, listener shared.Listener) shared.Subscription {
	b.subsMu.Lock()
	defer b.subsMu.Unlock()

	// Create subscription
	listenerID := listener.ID()
	sub := &subscription{
		BaseSubscription: shared.NewBaseSubscriptionWithCallback(
			fmt.Sprintf("%s-sub-%s-%d", b.config.Name, listenerID, time.Now().UnixNano()),
			topicName,
			listener,
			func() {
				b.removeSubscription(topicName, listenerID)
			},
		),
		bus:        b,
		topicName:  topicName,
		partition:  -1, // Subscribe to all partitions
		listenerID: listenerID,
	}

	// Add to subscriptions
	b.subscriptions[topicName] = append(b.subscriptions[topicName], sub)
	atomic.AddInt64(&b.stats.ActiveSubscribers, 1)

	// Ensure topic exists and add subscriber to all partitions
	b.ensureTopicAndAddSubscriber(topicName, sub)

	return sub
}

// SubscribePattern adds a listener to topics matching a pattern
func (b *eventBus) SubscribePattern(pattern string, listener shared.Listener) shared.Subscription {
	b.patternsMu.Lock()
	defer b.patternsMu.Unlock()

	// Compile regex pattern
	regex, err := regexp.Compile(pattern)
	if err != nil {
		// Return a no-op subscription that's already unsubscribed
		sub := shared.NewBaseSubscription("invalid", pattern, listener)
		_ = sub.Unsubscribe()
		return sub
	}

	// Create pattern subscription
	listenerID := listener.ID()
	sub := &subscription{
		BaseSubscription: shared.NewBaseSubscriptionWithCallback(
			fmt.Sprintf("%s-pattern-%s-%d", b.config.Name, listenerID, time.Now().UnixNano()),
			pattern,
			listener,
			func() {
				b.removePatternSubscription(pattern, listenerID)
			},
		),
		bus:        b,
		topicName:  pattern,
		partition:  -1,
		listenerID: listenerID,
	}

	patternSub := &patternSubscription{
		subscription: sub,
		pattern:      regex,
		rawPattern:   pattern,
	}

	b.patterns = append(b.patterns, patternSub)
	atomic.AddInt64(&b.stats.ActiveSubscribers, 1)

	// Add to existing topics that match the pattern
	b.addPatternSubscriberToMatchingTopics(patternSub)

	return sub
}

// CreateTopic creates a topic with specific configuration
func (b *eventBus) CreateTopic(topicName string, config TopicConfig) error {
	b.topicsMu.Lock()
	defer b.topicsMu.Unlock()

	if _, exists := b.topics[topicName]; exists {
		return fmt.Errorf("topic %s already exists", topicName)
	}

	t := &topic{
		name:       topicName,
		config:     config,
		partitions: make([]*partition, config.Partitions),
		created:    time.Now(),
	}

	// Create partitions
	for i := 0; i < config.Partitions; i++ {
		p := &partition{
			id:          i,
			eventQueue:  make(chan eventJob, config.BufferSize),
			subscribers: make([]*subscription, 0),
		}
		t.partitions[i] = p

		// Start partition processor
		b.wg.Add(1)
		go b.processPartition(t, p)
	}

	b.topics[topicName] = t
	atomic.AddInt64(&b.stats.ActiveTopics, 1)

	// Add existing pattern subscribers to this new topic
	b.addPatternSubscribersToTopic(topicName, t)

	return nil
}

// DeleteTopic removes a topic
func (b *eventBus) DeleteTopic(topicName string) error {
	b.topicsMu.Lock()
	defer b.topicsMu.Unlock()

	t, exists := b.topics[topicName]
	if !exists {
		return shared.ErrTopicNotFound
	}

	// Close all partition queues
	for _, p := range t.partitions {
		close(p.eventQueue)
	}

	delete(b.topics, topicName)
	atomic.AddInt64(&b.stats.ActiveTopics, -1)

	return nil
}

// Topics returns all available topics
func (b *eventBus) Topics() []string {
	b.topicsMu.RLock()
	defer b.topicsMu.RUnlock()

	topics := make([]string, 0, len(b.topics))
	for name := range b.topics {
		topics = append(topics, name)
	}
	return topics
}

// Shutdown gracefully shuts down the bus
func (b *eventBus) Shutdown(ctx context.Context) error {
	if !atomic.CompareAndSwapInt64(&b.closed, 0, 1) {
		return nil // already closed
	}

	// Close all topics
	b.topicsMu.Lock()
	for _, t := range b.topics {
		for _, p := range t.partitions {
			close(p.eventQueue)
		}
	}
	b.topicsMu.Unlock()

	// Signal shutdown
	close(b.stopCh)

	// Wait for all partition processors to finish
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stats returns bus statistics
func (b *eventBus) Stats() Stats {
	return Stats{
		EventsPublished:   atomic.LoadInt64(&b.stats.EventsPublished),
		EventsProcessed:   atomic.LoadInt64(&b.stats.EventsProcessed),
		ActiveTopics:      atomic.LoadInt64(&b.stats.ActiveTopics),
		ActiveSubscribers: atomic.LoadInt64(&b.stats.ActiveSubscribers),
		FailedEvents:      atomic.LoadInt64(&b.stats.FailedEvents),
		QueuedEvents:      atomic.LoadInt64(&b.stats.QueuedEvents),
	}
}

// Helper methods

func (b *eventBus) getOrCreateTopic(topicName string) (*topic, error) {
	b.topicsMu.RLock()
	t, exists := b.topics[topicName]
	b.topicsMu.RUnlock()

	if exists {
		return t, nil
	}

	// Create topic with default config
	config := DefaultTopicConfig()
	config.BufferSize = b.config.DefaultBufferSize
	config.Partitions = b.config.DefaultPartitions
	config.DeliveryMode = b.config.DefaultDeliveryMode

	if err := b.CreateTopic(topicName, config); err != nil {
		return nil, err
	}

	// Get the topic after creation
	b.topicsMu.RLock()
	t = b.topics[topicName]
	b.topicsMu.RUnlock()

	return t, nil
}

func (b *eventBus) selectPartition(t *topic, event shared.Event) int {
	if t.config.Partitions == 1 {
		return 0
	}

	key := t.config.OrderingKey(event)
	// Simple hash-based partitioning
	hash := 0
	for _, c := range key {
		hash = hash*31 + int(c)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash % t.config.Partitions
}

func (b *eventBus) ensureTopicAndAddSubscriber(topicName string, sub *subscription) {
	// Get or create topic
	t, err := b.getOrCreateTopic(topicName)
	if err != nil {
		return
	}

	// Add subscriber to all partitions
	for _, p := range t.partitions {
		p.subsMu.Lock()
		p.subscribers = append(p.subscribers, sub)
		p.subsMu.Unlock()
	}
}

func (b *eventBus) addPatternSubscriberToMatchingTopics(patternSub *patternSubscription) {
	b.topicsMu.RLock()
	defer b.topicsMu.RUnlock()

	for topicName, t := range b.topics {
		if patternSub.pattern.MatchString(topicName) {
			// Add to all partitions of matching topics
			for _, p := range t.partitions {
				p.subsMu.Lock()
				p.subscribers = append(p.subscribers, patternSub.subscription)
				p.subsMu.Unlock()
			}
		}
	}
}

func (b *eventBus) addPatternSubscribersToTopic(topicName string, t *topic) {
	b.patternsMu.RLock()
	defer b.patternsMu.RUnlock()

	for _, patternSub := range b.patterns {
		if patternSub.pattern.MatchString(topicName) {
			// Add pattern subscriber to all partitions of this topic
			for _, p := range t.partitions {
				p.subsMu.Lock()
				p.subscribers = append(p.subscribers, patternSub.subscription)
				p.subsMu.Unlock()
			}
		}
	}
}

func (b *eventBus) removeSubscription(topicName, listenerID string) {
	// Remove from subscriptions map
	b.subsMu.Lock()
	subs := b.subscriptions[topicName]
	var foundIndex = -1
	for i, sub := range subs {
		// Use cached listener ID to avoid lock contention
		if sub.listenerID == listenerID {
			foundIndex = i
			break
		}
	}
	if foundIndex >= 0 {
		b.subscriptions[topicName] = append(subs[:foundIndex], subs[foundIndex+1:]...)
		atomic.AddInt64(&b.stats.ActiveSubscribers, -1)
	}
	b.subsMu.Unlock()

	// Remove from topic partitions
	b.removeSubscriberFromTopic(topicName, listenerID)
}

func (b *eventBus) removePatternSubscription(pattern, listenerID string) {
	b.patternsMu.Lock()
	defer b.patternsMu.Unlock()

	for i, patternSub := range b.patterns {
		if patternSub.rawPattern == pattern && patternSub.listenerID == listenerID {
			// Remove from slice
			b.patterns = append(b.patterns[:i], b.patterns[i+1:]...)
			atomic.AddInt64(&b.stats.ActiveSubscribers, -1)
			break
		}
	}

	// Remove from all matching topics
	b.removePatternSubscriberFromAllTopics(pattern, listenerID)
}

func (b *eventBus) removeSubscriberFromTopic(topicName, listenerID string) {
	b.topicsMu.RLock()
	t, exists := b.topics[topicName]
	b.topicsMu.RUnlock()

	if !exists {
		return
	}

	for _, p := range t.partitions {
		p.subsMu.Lock()
		for i, sub := range p.subscribers {
			if sub.listenerID == listenerID {
				p.subscribers = append(p.subscribers[:i], p.subscribers[i+1:]...)
				break
			}
		}
		p.subsMu.Unlock()
	}
}

func (b *eventBus) removePatternSubscriberFromAllTopics(pattern, listenerID string) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return
	}

	b.topicsMu.RLock()
	defer b.topicsMu.RUnlock()

	for topicName, t := range b.topics {
		if regex.MatchString(topicName) {
			for _, p := range t.partitions {
				p.subsMu.Lock()
				for i, sub := range p.subscribers {
					if sub.listenerID == listenerID {
						p.subscribers = append(p.subscribers[:i], p.subscribers[i+1:]...)
						break
					}
				}
				p.subsMu.Unlock()
			}
		}
	}
}

func (b *eventBus) processPartition(t *topic, p *partition) {
	defer b.wg.Done()

	for job := range p.eventQueue {
		atomic.AddInt64(&b.stats.QueuedEvents, -1)
		b.processEventJob(t, p, job)
	}
}

func (b *eventBus) processEventJob(t *topic, p *partition, job eventJob) {
	p.subsMu.RLock()
	subscribers := make([]*subscription, len(p.subscribers))
	copy(subscribers, p.subscribers)
	p.subsMu.RUnlock()

	if len(subscribers) == 0 {
		atomic.AddInt64(&b.stats.EventsProcessed, 1)
		b.metrics.IncEventsProcessed("no-subscribers", "success")
		return
	}

	// Process event for each subscriber
	var processingErrors []error
	for _, sub := range subscribers {
		if !sub.Active() {
			continue
		}

		// Submit to worker pool for concurrent processing with propagated context
		err := b.workerpool.Submit(job.ctx, func(ctx context.Context) error {
			return b.handleEventForSubscriber(ctx, job.event, sub)
		})

		if err != nil {
			processingErrors = append(processingErrors, err)
		}
	}

	if len(processingErrors) > 0 {
		atomic.AddInt64(&p.failed, 1)
		atomic.AddInt64(&b.stats.FailedEvents, 1)
		b.metrics.IncEventsProcessed(fmt.Sprintf("topic-%s-partition-%d", t.name, p.id), "failed")

		// Handle retries for at-least-once and exactly-once delivery
		if (t.config.DeliveryMode == AtLeastOnce || t.config.DeliveryMode == ExactlyOnce) &&
			job.attempts < job.maxAttempts {
			// Retry the job
			job.attempts++
			select {
			case p.eventQueue <- job:
				atomic.AddInt64(&b.stats.QueuedEvents, 1)
			default:
				// Retry queue full, drop retry attempt to prevent blocking
				b.metrics.IncEventsDropped("bus", "retry_queue_full")
			}
		}
	} else {
		atomic.AddInt64(&p.processed, 1)
		atomic.AddInt64(&b.stats.EventsProcessed, 1)
		b.metrics.IncEventsProcessed(fmt.Sprintf("topic-%s-partition-%d", t.name, p.id), "success")
	}
}

func (b *eventBus) handleEventForSubscriber(ctx context.Context, event shared.Event, sub *subscription) error {
	start := time.Now()

	err := sub.Listener().Handle(ctx, event)

	duration := time.Since(start)
	b.metrics.ObserveListenerDuration(sub.Listener().ID(), duration)

	if err != nil {
		// Call error handler with context
		if handlerErr := sub.Listener().OnError(ctx, event, err); handlerErr != nil {
			return shared.NewListenerError(sub.Listener().ID(), event, handlerErr)
		}
		return shared.NewListenerError(sub.Listener().ID(), event, err)
	}

	return nil
}
