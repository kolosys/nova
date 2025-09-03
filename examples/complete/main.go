package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/kolosys/ion/workerpool"
	"github.com/kolosys/nova/bus"
	"github.com/kolosys/nova/emitter"
	"github.com/kolosys/nova/listener"
	"github.com/kolosys/nova/memory"
	"github.com/kolosys/nova/shared"
)

// Example demonstrates a complete Nova event system implementation
// This example shows:
// - Event emitters with sync/async processing
// - Event bus with topic routing and pattern matching
// - Listener management with retries and circuit breakers
// - In-memory event store with replay capabilities
// - Middleware for cross-cutting concerns

func main() {
	fmt.Println("🚀 Nova Event System - Complete Example")
	fmt.Println("========================================")

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt)
		<-sigChan
		fmt.Println("\n📝 Shutting down gracefully...")
		cancel()
	}()

	// Create Ion workerpool for event processing
	pool := workerpool.New(8, 100, 
		workerpool.WithName("nova-workers"),
	)
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := pool.Close(shutdownCtx); err != nil {
			log.Printf("Error closing workerpool: %v", err)
		}
	}()

	// Set up metrics collection
	metrics := shared.NewSimpleMetricsCollector()

	// Run the complete example
	if err := runCompleteExample(ctx, pool, metrics); err != nil {
		log.Fatalf("Example failed: %v", err)
	}

	fmt.Println("✅ Example completed successfully!")
}

func runCompleteExample(ctx context.Context, pool *workerpool.Pool, metrics shared.MetricsCollector) error {
	var wg sync.WaitGroup

	// 1. Create Event Store
	fmt.Println("📦 Setting up event store...")
	storeConfig := memory.Config{
		MaxEventsPerStream: 10000,
		MaxStreams:         100,
		RetentionDuration:  24 * time.Hour,
		MetricsCollector:   metrics,
		Name:              "example-store",
	}
	store := memory.New(storeConfig)
	defer store.Close()

	// 2. Create Event Emitter
	fmt.Println("📡 Setting up event emitter...")
	emitterConfig := emitter.Config{
		WorkerPool:       pool,
		AsyncMode:        true,
		BufferSize:       1000,
		MaxConcurrency:   20,
		MetricsCollector: metrics,
		Name:            "example-emitter",
	}
	eventEmitter := emitter.New(emitterConfig)
	defer eventEmitter.Shutdown(ctx)

	// Add middleware for logging and metrics
	loggingMiddleware := shared.MiddlewareFunc{
		BeforeFunc: func(event shared.Event) error {
			fmt.Printf("🔄 Processing event: %s (type: %s)\n", event.ID(), event.Type())
			return nil
		},
		AfterFunc: func(event shared.Event, err error) error {
			if err != nil {
				fmt.Printf("❌ Event %s failed: %v\n", event.ID(), err)
			} else {
				fmt.Printf("✅ Event %s processed successfully\n", event.ID())
			}
			return nil
		},
	}
	eventEmitter.Middleware(loggingMiddleware)

	// 3. Create Event Bus
	fmt.Println("🚌 Setting up event bus...")
	busConfig := bus.Config{
		WorkerPool:          pool,
		DefaultBufferSize:   2000,
		DefaultPartitions:   4,
		DefaultDeliveryMode: bus.AtLeastOnce,
		MetricsCollector:    metrics,
		Name:               "example-bus",
	}
	eventBus := bus.New(busConfig)
	defer eventBus.Shutdown(ctx)

	// Create topics with different configurations
	userTopicConfig := bus.TopicConfig{
		BufferSize:   1000,
		Partitions:   2,
		DeliveryMode: bus.ExactlyOnce,
		OrderingKey: func(e shared.Event) string {
			if userID, ok := e.Metadata()["user_id"]; ok {
				return userID
			}
			return e.ID()
		},
	}
	if err := eventBus.CreateTopic("users", userTopicConfig); err != nil {
		return fmt.Errorf("failed to create users topic: %w", err)
	}

	orderTopicConfig := bus.TopicConfig{
		BufferSize:   2000,
		Partitions:   4,
		DeliveryMode: bus.AtLeastOnce,
		OrderingKey: func(e shared.Event) string {
			if orderID, ok := e.Metadata()["order_id"]; ok {
				return orderID
			}
			return e.ID()
		},
	}
	if err := eventBus.CreateTopic("orders", orderTopicConfig); err != nil {
		return fmt.Errorf("failed to create orders topic: %w", err)
	}

	// 4. Create Listener Manager
	fmt.Println("👂 Setting up listener manager...")
	listenerConfig := listener.Config{
		WorkerPool:       pool,
		MetricsCollector: metrics,
		Name:            "example-listener-manager",
	}
	listenerManager := listener.New(listenerConfig)

	// Create various listeners
	userListener := &UserEventListener{id: "user-processor"}
	orderListener := &OrderEventListener{id: "order-processor"}
	auditListener := &AuditEventListener{id: "audit-logger", store: store}

	// Register listeners with different configurations
	resilientListenerConfig := listener.ListenerConfig{
		Concurrency: 5,
		RetryPolicy: listener.RetryPolicy{
			MaxAttempts:  3,
			InitialDelay: 100 * time.Millisecond,
			MaxDelay:     5 * time.Second,
			Backoff:      listener.ExponentialBackoff,
		},
		Circuit: listener.CircuitConfig{
			Enabled:          true,
			FailureThreshold: 5,
			SuccessThreshold: 3,
			Timeout:          30 * time.Second,
		},
		DeadLetter: listener.DeadLetterConfig{
			Enabled: true,
			Handler: func(event shared.Event, err error) {
				fmt.Printf("💀 Dead letter: Event %s failed permanently: %v\n", event.ID(), err)
			},
		},
		Timeout: 10 * time.Second,
		Name:    "resilient-user-listener",
	}

	if err := listenerManager.Register(userListener, resilientListenerConfig); err != nil {
		return fmt.Errorf("failed to register user listener: %w", err)
	}

	if err := listenerManager.Register(orderListener, listener.DefaultListenerConfig()); err != nil {
		return fmt.Errorf("failed to register order listener: %w", err)
	}

	if err := listenerManager.Register(auditListener, listener.DefaultListenerConfig()); err != nil {
		return fmt.Errorf("failed to register audit listener: %w", err)
	}

	if err := listenerManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start listener manager: %w", err)
	}
	defer listenerManager.Stop(ctx)

	// 5. Set up subscriptions
	fmt.Println("🔗 Setting up subscriptions...")

	// Direct emitter subscriptions
	eventEmitter.Subscribe("user.created", userListener)
	eventEmitter.Subscribe("user.updated", userListener)

	// Bus subscriptions
	eventBus.Subscribe("users", userListener)
	eventBus.Subscribe("orders", orderListener)

	// Pattern subscriptions for audit trail
	eventBus.SubscribePattern(".*", auditListener) // Audit all events

	// 6. Start event simulation
	fmt.Println("🎬 Starting event simulation...")

	wg.Add(3)

	// Emitter events simulation
	go func() {
		defer wg.Done()
		simulateEmitterEvents(ctx, eventEmitter)
	}()

	// Bus events simulation  
	go func() {
		defer wg.Done()
		simulateBusEvents(ctx, eventBus)
	}()

	// Event store operations
	go func() {
		defer wg.Done()
		demonstrateEventStore(ctx, store)
	}()

	// 7. Monitor system health and metrics
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				printSystemStats(eventEmitter, eventBus, listenerManager, store, metrics)
			}
		}
	}()

	// Wait for simulation to complete or context cancellation
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("🏁 Simulation completed")
	case <-ctx.Done():
		fmt.Println("🛑 Simulation cancelled")
	}

	// Give some time for final processing
	time.Sleep(1 * time.Second)

	return nil
}

// Event simulation functions

func simulateEmitterEvents(ctx context.Context, emitter emitter.EventEmitter) {
	fmt.Println("🎭 Starting emitter event simulation...")

	for i := 0; i < 20; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Create user events
		userEvent := shared.NewBaseEventWithMetadata(
			fmt.Sprintf("user-%d", i),
			"user.created",
			map[string]any{
				"name":  fmt.Sprintf("User %d", i),
				"email": fmt.Sprintf("user%d@example.com", i),
			},
			map[string]string{
				"user_id":    fmt.Sprintf("%d", i),
				"source":     "emitter",
				"created_at": time.Now().Format(time.RFC3339),
			},
		)

		if err := emitter.EmitAsync(ctx, userEvent); err != nil {
			fmt.Printf("❌ Failed to emit user event: %v\n", err)
		}

		time.Sleep(500 * time.Millisecond)
	}
}

func simulateBusEvents(ctx context.Context, eventBus bus.EventBus) {
	fmt.Println("🚌 Starting bus event simulation...")

	for i := 0; i < 30; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if i%2 == 0 {
			// User events
			userEvent := shared.NewBaseEventWithMetadata(
				fmt.Sprintf("bus-user-%d", i),
				"user.updated",
				map[string]any{
					"name":   fmt.Sprintf("Updated User %d", i),
					"status": "active",
				},
				map[string]string{
					"user_id": fmt.Sprintf("%d", i),
					"source":  "bus",
				},
			)

			if err := eventBus.Publish(ctx, "users", userEvent); err != nil {
				fmt.Printf("❌ Failed to publish user event: %v\n", err)
			}
		} else {
			// Order events
			orderEvent := shared.NewBaseEventWithMetadata(
				fmt.Sprintf("order-%d", i),
				"order.created",
				map[string]any{
					"amount":    100.50 * float64(i),
					"items":     []string{"item1", "item2"},
					"customer":  fmt.Sprintf("customer-%d", i),
				},
				map[string]string{
					"order_id":  fmt.Sprintf("ord-%d", i),
					"user_id":   fmt.Sprintf("%d", i%10),
					"source":    "bus",
				},
			)

			if err := eventBus.Publish(ctx, "orders", orderEvent); err != nil {
				fmt.Printf("❌ Failed to publish order event: %v\n", err)
			}
		}

		time.Sleep(300 * time.Millisecond)
	}
}

func demonstrateEventStore(ctx context.Context, store memory.EventStore) {
	fmt.Println("📦 Demonstrating event store operations...")

	// Store some events
	events := []shared.Event{
		shared.NewBaseEvent("historical-1", "system.started", "System initialization"),
		shared.NewBaseEvent("historical-2", "config.loaded", "Configuration loaded"),
		shared.NewBaseEvent("historical-3", "services.ready", "All services ready"),
	}

	if err := store.Append(ctx, "system-events", events...); err != nil {
		fmt.Printf("❌ Failed to store system events: %v\n", err)
		return
	}

	// Demonstrate replay
	time.Sleep(2 * time.Second)
	fmt.Println("🔄 Replaying historical events...")

	from := time.Now().Add(-1 * time.Hour)
	to := time.Now().Add(1 * time.Hour)

	replayCh, err := store.Replay(ctx, from, to)
	if err != nil {
		fmt.Printf("❌ Failed to start replay: %v\n", err)
		return
	}

	count := 0
	for event := range replayCh {
		fmt.Printf("📼 Replayed: %s (%s)\n", event.ID(), event.Type())
		count++
		if count >= 10 { // Limit output
			break
		}
	}

	// Demonstrate live subscription
	fmt.Println("📡 Setting up live subscription...")
	cursor := memory.Cursor{StreamID: "live-events", Position: 0}
	liveCh, err := store.Subscribe(ctx, "live-events", cursor)
	if err != nil {
		fmt.Printf("❌ Failed to setup live subscription: %v\n", err)
		return
	}

	// Add some live events
	go func() {
		for i := 0; i < 5; i++ {
			liveEvent := shared.NewBaseEvent(
				fmt.Sprintf("live-%d", i),
				"live.update",
				fmt.Sprintf("Live update %d", i),
			)
			
			if err := store.Append(ctx, "live-events", liveEvent); err != nil {
				fmt.Printf("❌ Failed to append live event: %v\n", err)
				return
			}
			
			time.Sleep(1 * time.Second)
		}
	}()

	// Consume live events
	timeout := time.After(8 * time.Second)
	liveCount := 0
	for {
		select {
		case event, ok := <-liveCh:
			if !ok {
				return
			}
			fmt.Printf("🔴 Live: %s (%s)\n", event.ID(), event.Type())
			liveCount++
			if liveCount >= 5 {
				return
			}
		case <-timeout:
			return
		case <-ctx.Done():
			return
		}
	}
}

func printSystemStats(em emitter.EventEmitter, eb bus.EventBus, lm listener.ListenerManager, store memory.EventStore, metrics shared.MetricsCollector) {
	fmt.Println("\n📊 System Statistics:")
	fmt.Println("=====================")

	// Emitter stats
	emitterStats := em.Stats()
	fmt.Printf("📡 Emitter - Emitted: %d, Processed: %d, Active: %d, Failed: %d\n",
		emitterStats.EventsEmitted, emitterStats.EventsProcessed,
		emitterStats.ActiveListeners, emitterStats.FailedEvents)

	// Bus stats
	busStats := eb.Stats()
	fmt.Printf("🚌 Bus - Published: %d, Processed: %d, Topics: %d, Subscribers: %d\n",
		busStats.EventsPublished, busStats.EventsProcessed,
		busStats.ActiveTopics, busStats.ActiveSubscribers)

	// Listener manager stats
	listenerStats := lm.Stats()
	fmt.Printf("👂 Listeners - Registered: %d, Active: %d, Processed: %d, Retried: %d, Failed: %d\n",
		listenerStats.RegisteredListeners, listenerStats.ActiveListeners,
		listenerStats.EventsProcessed, listenerStats.EventsRetried, listenerStats.EventsFailed)

	// Store stats
	storeStats := store.Stats()
	fmt.Printf("📦 Store - Events: %d, Streams: %d, Appended: %d, Read: %d, Memory: %d KB\n",
		storeStats.TotalEvents, storeStats.TotalStreams,
		storeStats.EventsAppended, storeStats.EventsRead, storeStats.MemoryUsageBytes/1024)

	// Health check
	health := lm.Health()
	fmt.Printf("🏥 System Health: %s\n", health.String())

	fmt.Println()
}
