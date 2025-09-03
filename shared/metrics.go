package shared

import (
	"sync/atomic"
	"time"
)

// MetricsCollector defines the interface for collecting Nova metrics
type MetricsCollector interface {
	// IncEventsEmitted increments events emitted counter
	IncEventsEmitted(eventType string, result string)

	// IncEventsProcessed increments events processed counter
	IncEventsProcessed(listenerID string, result string)

	// ObserveListenerDuration records listener processing duration
	ObserveListenerDuration(listenerID string, duration time.Duration)

	// ObserveSagaDuration records saga step duration
	ObserveSagaDuration(sagaID string, step string, duration time.Duration)

	// ObserveStoreAppendDuration records store append duration
	ObserveStoreAppendDuration(duration time.Duration)

	// ObserveStoreReadDuration records store read duration
	ObserveStoreReadDuration(duration time.Duration)

	// SetQueueSize sets current queue size
	SetQueueSize(component string, size int)

	// SetActiveListeners sets current active listeners count
	SetActiveListeners(emitterID string, count int)
}

// NoOpMetricsCollector provides a no-op implementation for when metrics are disabled
type NoOpMetricsCollector struct{}

// IncEventsEmitted does nothing
func (n NoOpMetricsCollector) IncEventsEmitted(eventType string, result string) {}

// IncEventsProcessed does nothing
func (n NoOpMetricsCollector) IncEventsProcessed(listenerID string, result string) {}

// ObserveListenerDuration does nothing
func (n NoOpMetricsCollector) ObserveListenerDuration(listenerID string, duration time.Duration) {}

// ObserveSagaDuration does nothing
func (n NoOpMetricsCollector) ObserveSagaDuration(sagaID string, step string, duration time.Duration) {}

// ObserveStoreAppendDuration does nothing
func (n NoOpMetricsCollector) ObserveStoreAppendDuration(duration time.Duration) {}

// ObserveStoreReadDuration does nothing
func (n NoOpMetricsCollector) ObserveStoreReadDuration(duration time.Duration) {}

// SetQueueSize does nothing
func (n NoOpMetricsCollector) SetQueueSize(component string, size int) {}

// SetActiveListeners does nothing
func (n NoOpMetricsCollector) SetActiveListeners(emitterID string, count int) {}

// SimpleMetricsCollector provides a basic in-memory metrics collector for testing
type SimpleMetricsCollector struct {
	EventsEmitted     int64
	EventsProcessed   int64
	ListenerDurations int64
	SagaDurations     int64
	StoreAppends      int64
	StoreReads        int64
	QueueSizes        map[string]int64
	ActiveListeners   map[string]int64
}

// NewSimpleMetricsCollector creates a new SimpleMetricsCollector
func NewSimpleMetricsCollector() *SimpleMetricsCollector {
	return &SimpleMetricsCollector{
		QueueSizes:      make(map[string]int64),
		ActiveListeners: make(map[string]int64),
	}
}

// IncEventsEmitted increments events emitted counter
func (s *SimpleMetricsCollector) IncEventsEmitted(eventType string, result string) {
	atomic.AddInt64(&s.EventsEmitted, 1)
}

// IncEventsProcessed increments events processed counter
func (s *SimpleMetricsCollector) IncEventsProcessed(listenerID string, result string) {
	atomic.AddInt64(&s.EventsProcessed, 1)
}

// ObserveListenerDuration records listener processing duration
func (s *SimpleMetricsCollector) ObserveListenerDuration(listenerID string, duration time.Duration) {
	atomic.AddInt64(&s.ListenerDurations, 1)
}

// ObserveSagaDuration records saga step duration
func (s *SimpleMetricsCollector) ObserveSagaDuration(sagaID string, step string, duration time.Duration) {
	atomic.AddInt64(&s.SagaDurations, 1)
}

// ObserveStoreAppendDuration records store append duration
func (s *SimpleMetricsCollector) ObserveStoreAppendDuration(duration time.Duration) {
	atomic.AddInt64(&s.StoreAppends, 1)
}

// ObserveStoreReadDuration records store read duration
func (s *SimpleMetricsCollector) ObserveStoreReadDuration(duration time.Duration) {
	atomic.AddInt64(&s.StoreReads, 1)
}

// SetQueueSize sets current queue size
func (s *SimpleMetricsCollector) SetQueueSize(component string, size int) {
	s.QueueSizes[component] = int64(size)
}

// SetActiveListeners sets current active listeners count
func (s *SimpleMetricsCollector) SetActiveListeners(emitterID string, count int) {
	s.ActiveListeners[emitterID] = int64(count)
}

// GetEventsEmitted returns the current events emitted count
func (s *SimpleMetricsCollector) GetEventsEmitted() int64 {
	return atomic.LoadInt64(&s.EventsEmitted)
}

// GetEventsProcessed returns the current events processed count
func (s *SimpleMetricsCollector) GetEventsProcessed() int64 {
	return atomic.LoadInt64(&s.EventsProcessed)
}
