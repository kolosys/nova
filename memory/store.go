package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kolosys/nova/shared"
)

// Cursor represents a position in the event stream
type Cursor struct {
	// StreamID identifies the stream
	StreamID string

	// Position is the sequence number within the stream
	Position int64

	// Timestamp is the time of the event at this position
	Timestamp time.Time
}

// String returns a string representation of the cursor
func (c Cursor) String() string {
	return fmt.Sprintf("stream:%s pos:%d time:%v", c.StreamID, c.Position, c.Timestamp.Format(time.RFC3339))
}

// StreamInfo provides information about a stream
type StreamInfo struct {
	StreamID     string
	EventCount   int64
	FirstEvent   time.Time
	LastEvent    time.Time
	LastPosition int64
}

// EventStore defines the interface for event storage and replay
type EventStore interface {
	// Append adds events to a stream
	Append(ctx context.Context, streamID string, events ...shared.Event) error

	// Read reads events from a stream starting at a cursor
	Read(ctx context.Context, streamID string, cursor Cursor, limit int) ([]shared.Event, Cursor, error)

	// ReadStream reads all events from a stream starting at a position
	ReadStream(ctx context.Context, streamID string, fromPosition int64) ([]shared.Event, error)

	// ReadTimeRange reads events from a time range across all streams
	ReadTimeRange(ctx context.Context, from, to time.Time) ([]shared.Event, error)

	// Replay creates a channel for replaying events from a time range
	Replay(ctx context.Context, from, to time.Time) (<-chan shared.Event, error)

	// Subscribe creates a channel for live events starting from a cursor
	Subscribe(ctx context.Context, streamID string, from Cursor) (<-chan shared.Event, error)

	// GetStreams returns information about all streams
	GetStreams() []StreamInfo

	// GetStreamInfo returns information about a specific stream
	GetStreamInfo(streamID string) (StreamInfo, error)

	// Snapshot creates a snapshot of the current state
	Snapshot(ctx context.Context) error

	// Close gracefully closes the event store
	Close() error

	// Stats returns store statistics
	Stats() Stats
}

// Config configures the EventStore
type Config struct {
	// MaxEventsPerStream limits events per stream (0 = unlimited)
	MaxEventsPerStream int64

	// MaxStreams limits the number of streams (0 = unlimited)
	MaxStreams int

	// RetentionDuration sets how long to keep events (0 = forever)
	RetentionDuration time.Duration

	// SnapshotInterval sets how often to create snapshots
	SnapshotInterval time.Duration

	// MetricsCollector for observability (optional)
	MetricsCollector shared.MetricsCollector

	// Name identifies this store instance
	Name string
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() Config {
	return Config{
		MaxEventsPerStream: 100000, // 100k events per stream
		MaxStreams:         10000,  // 10k streams
		RetentionDuration:  0,      // Keep forever by default
		SnapshotInterval:   time.Hour,
		Name:               "memory-store",
	}
}

// Stats provides store statistics
type Stats struct {
	TotalEvents         int64
	TotalStreams        int64
	EventsAppended      int64
	EventsRead          int64
	SubscriptionsActive int64
	SnapshotsCreated    int64
	MemoryUsageBytes    int64
}

// eventStore implements EventStore using in-memory storage
type eventStore struct {
	config         Config
	metrics        shared.MetricsCollector
	streams        map[string]*stream
	streamsmu      sync.RWMutex
	globalSequence int64 // atomic global sequence number
	subscriptions  map[string]*subscription
	subscmu        sync.RWMutex
	closed         int64 // atomic boolean
	stats          Stats
	stopCh         chan struct{}
	wg             sync.WaitGroup
}

// stream represents a single event stream
type stream struct {
	id       string
	events   []storedEvent
	mu       sync.RWMutex
	position int64 // last position in stream
	created  time.Time
	updated  time.Time
}

// storedEvent wraps an event with storage metadata
type storedEvent struct {
	Event          shared.Event
	StreamID       string
	Position       int64 // position within stream
	GlobalSequence int64 // global sequence across all streams
	StoredAt       time.Time
}

// subscription represents an active subscription
type subscription struct {
	id       string
	streamID string
	cursor   Cursor
	ch       chan shared.Event
	stopCh   chan struct{}
	active   int64 // atomic boolean
}

// New creates a new in-memory EventStore
func New(config Config) EventStore {
	if config.MetricsCollector == nil {
		config.MetricsCollector = shared.NoOpMetricsCollector{}
	}
	if config.Name == "" {
		config.Name = "memory-store"
	}

	es := &eventStore{
		config:        config,
		metrics:       config.MetricsCollector,
		streams:       make(map[string]*stream),
		subscriptions: make(map[string]*subscription),
		stopCh:        make(chan struct{}),
	}

	// Start background tasks
	if config.RetentionDuration > 0 {
		es.wg.Add(1)
		go es.retentionWorker()
	}

	if config.SnapshotInterval > 0 {
		es.wg.Add(1)
		go es.snapshotWorker()
	}

	return es
}

// Append adds events to a stream
func (es *eventStore) Append(ctx context.Context, streamID string, events ...shared.Event) error {
	if atomic.LoadInt64(&es.closed) == 1 {
		return fmt.Errorf("event store is closed")
	}

	if len(events) == 0 {
		return nil
	}

	// Validate events
	for _, event := range events {
		if err := shared.DefaultEventValidator.Validate(event); err != nil {
			return shared.NewEventError(event, err)
		}
	}

	start := time.Now()
	defer func() {
		es.metrics.ObserveStoreAppendDuration(time.Since(start))
	}()

	// Get or create stream
	s := es.getOrCreateStream(streamID)

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check stream limits
	if es.config.MaxEventsPerStream > 0 && int64(len(s.events)) >= es.config.MaxEventsPerStream {
		return fmt.Errorf("stream %s has reached maximum events limit (%d)", streamID, es.config.MaxEventsPerStream)
	}

	// Append events
	for _, event := range events {
		s.position++
		globalSeq := atomic.AddInt64(&es.globalSequence, 1)

		storedEvent := storedEvent{
			Event:          event,
			StreamID:       streamID,
			Position:       s.position,
			GlobalSequence: globalSeq,
			StoredAt:       time.Now(),
		}

		s.events = append(s.events, storedEvent)
		s.updated = time.Now()

		// Update stats
		atomic.AddInt64(&es.stats.TotalEvents, 1)
		atomic.AddInt64(&es.stats.EventsAppended, 1)
	}

	// Notify subscribers
	es.notifySubscribers(streamID, s.events[len(s.events)-len(events):])

	return nil
}

// Read reads events from a stream starting at a cursor
func (es *eventStore) Read(ctx context.Context, streamID string, cursor Cursor, limit int) ([]shared.Event, Cursor, error) {
	if atomic.LoadInt64(&es.closed) == 1 {
		return nil, cursor, fmt.Errorf("event store is closed")
	}

	start := time.Now()
	defer func() {
		es.metrics.ObserveStoreReadDuration(time.Since(start))
	}()

	es.streamsmu.RLock()
	s, exists := es.streams[streamID]
	es.streamsmu.RUnlock()

	if !exists {
		return nil, cursor, shared.ErrEventNotFound
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Find starting position
	startPos := cursor.Position
	if startPos < 0 {
		startPos = 0
	}

	var result []shared.Event
	newCursor := cursor

	for _, storedEvent := range s.events {
		if storedEvent.Position <= startPos {
			continue
		}

		result = append(result, storedEvent.Event)
		newCursor = Cursor{
			StreamID:  streamID,
			Position:  storedEvent.Position,
			Timestamp: storedEvent.Event.Timestamp(),
		}

		if limit > 0 && len(result) >= limit {
			break
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return result, newCursor, ctx.Err()
		default:
		}
	}

	atomic.AddInt64(&es.stats.EventsRead, int64(len(result)))
	return result, newCursor, nil
}

// ReadStream reads all events from a stream starting at a position
func (es *eventStore) ReadStream(ctx context.Context, streamID string, fromPosition int64) ([]shared.Event, error) {
	if atomic.LoadInt64(&es.closed) == 1 {
		return nil, fmt.Errorf("event store is closed")
	}

	es.streamsmu.RLock()
	s, exists := es.streams[streamID]
	es.streamsmu.RUnlock()

	if !exists {
		return nil, shared.ErrEventNotFound
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []shared.Event
	for _, storedEvent := range s.events {
		if storedEvent.Position <= fromPosition {
			continue
		}

		result = append(result, storedEvent.Event)

		// Check context cancellation
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}
	}

	atomic.AddInt64(&es.stats.EventsRead, int64(len(result)))
	return result, nil
}

// ReadTimeRange reads events from a time range across all streams
func (es *eventStore) ReadTimeRange(ctx context.Context, from, to time.Time) ([]shared.Event, error) {
	if atomic.LoadInt64(&es.closed) == 1 {
		return nil, fmt.Errorf("event store is closed")
	}

	es.streamsmu.RLock()
	streams := make([]*stream, 0, len(es.streams))
	for _, s := range es.streams {
		streams = append(streams, s)
	}
	es.streamsmu.RUnlock()

	var allEvents []storedEvent
	for _, s := range streams {
		s.mu.RLock()
		for _, storedEvent := range s.events {
			eventTime := storedEvent.Event.Timestamp()
			if (eventTime.Equal(from) || eventTime.After(from)) &&
				(to.IsZero() || eventTime.Before(to) || eventTime.Equal(to)) {
				allEvents = append(allEvents, storedEvent)
			}
		}
		s.mu.RUnlock()

		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	// Sort by timestamp
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Event.Timestamp().Before(allEvents[j].Event.Timestamp())
	})

	result := make([]shared.Event, len(allEvents))
	for i, storedEvent := range allEvents {
		result[i] = storedEvent.Event
	}

	atomic.AddInt64(&es.stats.EventsRead, int64(len(result)))
	return result, nil
}

// Replay creates a channel for replaying events from a time range
func (es *eventStore) Replay(ctx context.Context, from, to time.Time) (<-chan shared.Event, error) {
	if atomic.LoadInt64(&es.closed) == 1 {
		return nil, fmt.Errorf("event store is closed")
	}

	events, err := es.ReadTimeRange(ctx, from, to)
	if err != nil {
		return nil, err
	}

	ch := make(chan shared.Event, 1000) // Buffered channel
	go func() {
		defer close(ch)
		for _, event := range events {
			select {
			case ch <- event:
			case <-ctx.Done():
				return
			case <-es.stopCh:
				return
			}
		}
	}()

	return ch, nil
}

// Subscribe creates a channel for live events starting from a cursor
func (es *eventStore) Subscribe(ctx context.Context, streamID string, from Cursor) (<-chan shared.Event, error) {
	if atomic.LoadInt64(&es.closed) == 1 {
		return nil, fmt.Errorf("event store is closed")
	}

	sub := &subscription{
		id:       fmt.Sprintf("sub-%d", time.Now().UnixNano()),
		streamID: streamID,
		cursor:   from,
		ch:       make(chan shared.Event, 100),
		stopCh:   make(chan struct{}),
		active:   1,
	}

	// Send historical events first
	go func() {
		defer func() {
			// Mark subscription as inactive before closing channel
			atomic.StoreInt64(&sub.active, 0)
			close(sub.ch)
		}()

		// Read historical events
		events, err := es.ReadStream(ctx, streamID, from.Position)
		if err != nil && err != shared.ErrEventNotFound {
			return
		}

		// Send historical events
		for _, event := range events {
			select {
			case sub.ch <- event:
			case <-ctx.Done():
				return
			case <-sub.stopCh:
				return
			case <-es.stopCh:
				return
			}
		}

		// Keep subscription active for live events
		for {
			select {
			case <-ctx.Done():
				return
			case <-sub.stopCh:
				return
			case <-es.stopCh:
				return
			case <-time.After(100 * time.Millisecond):
				// Periodic check for new events (polling-based approach)
				// Note: Event-driven notifications could be added for lower latency
			}
		}
	}()

	// Register subscription
	es.subscmu.Lock()
	es.subscriptions[sub.id] = sub
	atomic.AddInt64(&es.stats.SubscriptionsActive, 1)
	es.subscmu.Unlock()

	// Clean up subscription when done
	go func() {
		select {
		case <-ctx.Done():
		case <-es.stopCh:
		}
		es.unsubscribe(sub.id)
	}()

	return sub.ch, nil
}

// GetStreams returns information about all streams
func (es *eventStore) GetStreams() []StreamInfo {
	es.streamsmu.RLock()
	defer es.streamsmu.RUnlock()

	result := make([]StreamInfo, 0, len(es.streams))
	for _, s := range es.streams {
		s.mu.RLock()
		info := StreamInfo{
			StreamID:     s.id,
			EventCount:   int64(len(s.events)),
			LastPosition: s.position,
		}
		if len(s.events) > 0 {
			info.FirstEvent = s.events[0].Event.Timestamp()
			info.LastEvent = s.events[len(s.events)-1].Event.Timestamp()
		}
		s.mu.RUnlock()
		result = append(result, info)
	}

	return result
}

// GetStreamInfo returns information about a specific stream
func (es *eventStore) GetStreamInfo(streamID string) (StreamInfo, error) {
	es.streamsmu.RLock()
	s, exists := es.streams[streamID]
	es.streamsmu.RUnlock()

	if !exists {
		return StreamInfo{}, shared.ErrEventNotFound
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	info := StreamInfo{
		StreamID:     s.id,
		EventCount:   int64(len(s.events)),
		LastPosition: s.position,
	}
	if len(s.events) > 0 {
		info.FirstEvent = s.events[0].Event.Timestamp()
		info.LastEvent = s.events[len(s.events)-1].Event.Timestamp()
	}

	return info, nil
}

// Snapshot creates a snapshot of the current state
func (es *eventStore) Snapshot(ctx context.Context) error {
	if atomic.LoadInt64(&es.closed) == 1 {
		return fmt.Errorf("event store is closed")
	}

	// In a real implementation, this would serialize state to disk
	// For now, we just update the snapshot counter
	atomic.AddInt64(&es.stats.SnapshotsCreated, 1)
	return nil
}

// Close gracefully closes the event store
func (es *eventStore) Close() error {
	if !atomic.CompareAndSwapInt64(&es.closed, 0, 1) {
		return nil // already closed
	}

	// Signal all background workers to stop
	close(es.stopCh)

	// Close all active subscriptions
	es.subscmu.Lock()
	for _, sub := range es.subscriptions {
		close(sub.stopCh)
	}
	es.subscmu.Unlock()

	// Wait for background workers
	es.wg.Wait()

	return nil
}

// Stats returns store statistics
func (es *eventStore) Stats() Stats {
	// Calculate memory usage estimate
	memoryUsage := atomic.LoadInt64(&es.stats.TotalEvents) * 1024 // Rough estimate: 1KB per event

	return Stats{
		TotalEvents:         atomic.LoadInt64(&es.stats.TotalEvents),
		TotalStreams:        atomic.LoadInt64(&es.stats.TotalStreams),
		EventsAppended:      atomic.LoadInt64(&es.stats.EventsAppended),
		EventsRead:          atomic.LoadInt64(&es.stats.EventsRead),
		SubscriptionsActive: atomic.LoadInt64(&es.stats.SubscriptionsActive),
		SnapshotsCreated:    atomic.LoadInt64(&es.stats.SnapshotsCreated),
		MemoryUsageBytes:    memoryUsage,
	}
}

// Helper methods

func (es *eventStore) getOrCreateStream(streamID string) *stream {
	es.streamsmu.RLock()
	s, exists := es.streams[streamID]
	es.streamsmu.RUnlock()

	if exists {
		return s
	}

	es.streamsmu.Lock()
	defer es.streamsmu.Unlock()

	// Check again after acquiring write lock
	if s, exists := es.streams[streamID]; exists {
		return s
	}

	// Check stream limits
	if es.config.MaxStreams > 0 && len(es.streams) >= es.config.MaxStreams {
		// Stream limit reached - use first available stream (round-robin)
		// Alternative strategies: LRU eviction, oldest-first, or error return
		for _, existingStream := range es.streams {
			return existingStream
		}
	}

	// Create new stream
	s = &stream{
		id:      streamID,
		events:  make([]storedEvent, 0),
		created: time.Now(),
		updated: time.Now(),
	}

	es.streams[streamID] = s
	atomic.AddInt64(&es.stats.TotalStreams, 1)

	return s
}

func (es *eventStore) notifySubscribers(streamID string, events []storedEvent) {
	es.subscmu.RLock()
	defer es.subscmu.RUnlock()

	for _, sub := range es.subscriptions {
		if sub.streamID != streamID || atomic.LoadInt64(&sub.active) == 0 {
			continue
		}

		// Send events to subscriber
		go func(sub *subscription, events []storedEvent) {
			for _, storedEvent := range events {
				if storedEvent.Position <= sub.cursor.Position {
					continue
				}

				// Check if subscription is still active before sending
				if atomic.LoadInt64(&sub.active) == 0 {
					return
				}

				select {
				case sub.ch <- storedEvent.Event:
					sub.cursor.Position = storedEvent.Position
					sub.cursor.Timestamp = storedEvent.Event.Timestamp()
				case <-sub.stopCh:
					return
				default:
					// Subscription channel full, skip event to prevent blocking
					es.metrics.IncEventsDropped("subscription", "channel_full")
				}
			}
		}(sub, events)
	}
}

func (es *eventStore) unsubscribe(subscriptionID string) {
	es.subscmu.Lock()
	defer es.subscmu.Unlock()

	if sub, exists := es.subscriptions[subscriptionID]; exists {
		atomic.StoreInt64(&sub.active, 0)
		delete(es.subscriptions, subscriptionID)
		atomic.AddInt64(&es.stats.SubscriptionsActive, -1)
	}
}

func (es *eventStore) retentionWorker() {
	defer es.wg.Done()

	ticker := time.NewTicker(es.config.RetentionDuration / 10) // Check 10 times per retention period
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			es.cleanupExpiredEvents()
		case <-es.stopCh:
			return
		}
	}
}

func (es *eventStore) cleanupExpiredEvents() {
	if es.config.RetentionDuration <= 0 {
		return
	}

	cutoff := time.Now().Add(-es.config.RetentionDuration)

	es.streamsmu.Lock()
	defer es.streamsmu.Unlock()

	for streamID, s := range es.streams {
		s.mu.Lock()

		// Find events to keep (newer than cutoff)
		var keepEvents []storedEvent
		removedCount := 0

		for _, event := range s.events {
			if event.Event.Timestamp().After(cutoff) {
				keepEvents = append(keepEvents, event)
			} else {
				removedCount++
			}
		}

		// Update stream if events were removed
		if removedCount > 0 {
			s.events = keepEvents
			atomic.AddInt64(&es.stats.TotalEvents, -int64(removedCount))
		}

		// Remove empty streams
		if len(s.events) == 0 {
			delete(es.streams, streamID)
			atomic.AddInt64(&es.stats.TotalStreams, -1)
		}

		s.mu.Unlock()
	}
}

func (es *eventStore) snapshotWorker() {
	defer es.wg.Done()

	ticker := time.NewTicker(es.config.SnapshotInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = es.Snapshot(context.Background())
		case <-es.stopCh:
			return
		}
	}
}
