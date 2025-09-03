package bus_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/kolosys/ion/workerpool"
	"github.com/kolosys/nova/bus"
	"github.com/kolosys/nova/shared"
)

// benchmarkListener is a minimal listener for benchmarking
type benchmarkListener struct {
	id    string
	count int64
}

func newBenchmarkListener(id string) *benchmarkListener {
	return &benchmarkListener{id: id}
}

func (l *benchmarkListener) ID() string {
	return l.id
}

func (l *benchmarkListener) Handle(shared.Event) error {
	l.count++
	return nil
}

func (l *benchmarkListener) OnError(shared.Event, error) error {
	return nil
}

func BenchmarkEventBus_Publish(b *testing.B) {
	pool := workerpool.New(4, 100, workerpool.WithName("bench-pool"))
	defer pool.Close(context.Background())

	config := bus.Config{
		WorkerPool:          pool,
		DefaultBufferSize:   1000,
		DefaultPartitions:   1,
		DefaultDeliveryMode: bus.AtMostOnce,
		Name:                "bench-bus",
	}

	eventBus := bus.New(config)
	defer eventBus.Shutdown(context.Background())

	listener := newBenchmarkListener("bench-listener")
	eventBus.Subscribe("bench.topic", listener)

	event := shared.NewBaseEvent("bench-event", "bench.event", "benchmark data")
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := eventBus.Publish(ctx, "bench.topic", event); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkEventBus_MultipleTopics(b *testing.B) {
	pool := workerpool.New(8, 200, workerpool.WithName("bench-pool"))
	defer pool.Close(context.Background())

	config := bus.Config{
		WorkerPool:          pool,
		DefaultBufferSize:   1000,
		DefaultPartitions:   1,
		DefaultDeliveryMode: bus.AtMostOnce,
		Name:                "bench-bus",
	}

	eventBus := bus.New(config)
	defer eventBus.Shutdown(context.Background())

	// Create multiple topics with listeners
	topics := []string{"topic1", "topic2", "topic3", "topic4", "topic5"}
	for i, topic := range topics {
		listener := newBenchmarkListener("bench-listener-" + string(rune('0'+i)))
		eventBus.Subscribe(topic, listener)
	}

	events := make([]shared.Event, len(topics))
	for i := range events {
		events[i] = shared.NewBaseEvent("bench-event", "bench.event", "benchmark data")
	}

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			topic := topics[int(time.Now().UnixNano())%len(topics)]
			event := events[int(time.Now().UnixNano())%len(events)]
			if err := eventBus.Publish(ctx, topic, event); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkEventBus_Partitions(b *testing.B) {
	pool := workerpool.New(8, 200, workerpool.WithName("bench-pool"))
	defer pool.Close(context.Background())

	config := bus.Config{
		WorkerPool:          pool,
		DefaultBufferSize:   1000,
		DefaultPartitions:   8, // Multiple partitions for parallel processing
		DefaultDeliveryMode: bus.AtMostOnce,
		Name:                "bench-bus",
	}

	eventBus := bus.New(config)
	defer eventBus.Shutdown(context.Background())

	// Create topic with multiple partitions
	topicConfig := bus.TopicConfig{
		BufferSize:   2000,
		Partitions:   8,
		DeliveryMode: bus.AtMostOnce,
		OrderingKey: func(e shared.Event) string {
			return e.ID() // Use event ID for partitioning
		},
	}
	if err := eventBus.CreateTopic("partitioned.topic", topicConfig); err != nil {
		b.Fatal(err)
	}

	listener := newBenchmarkListener("bench-listener")
	eventBus.Subscribe("partitioned.topic", listener)

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Create event with unique ID for partitioning
			eventID := "event-" + string(rune('0'+int(time.Now().UnixNano())%10))
			event := shared.NewBaseEvent(eventID, "bench.event", "benchmark data")
			if err := eventBus.Publish(ctx, "partitioned.topic", event); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkEventBus_PatternSubscription(b *testing.B) {
	pool := workerpool.New(4, 100, workerpool.WithName("bench-pool"))
	defer pool.Close(context.Background())

	config := bus.Config{
		WorkerPool:          pool,
		DefaultBufferSize:   1000,
		DefaultPartitions:   1,
		DefaultDeliveryMode: bus.AtMostOnce,
		Name:                "bench-bus",
	}

	eventBus := bus.New(config)
	defer eventBus.Shutdown(context.Background())

	// Pattern subscription
	listener := newBenchmarkListener("pattern-listener")
	eventBus.SubscribePattern("bench\\..+", listener)

	event := shared.NewBaseEvent("bench-event", "bench.event", "benchmark data")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := eventBus.Publish(ctx, "bench.topic", event); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEventBus_DeliveryModes(b *testing.B) {
	pool := workerpool.New(4, 100, workerpool.WithName("bench-pool"))
	defer pool.Close(context.Background())

	modes := []struct {
		name string
		mode bus.DeliveryMode
	}{
		{"AtMostOnce", bus.AtMostOnce},
		{"AtLeastOnce", bus.AtLeastOnce},
		{"ExactlyOnce", bus.ExactlyOnce},
	}

	for _, mode := range modes {
		b.Run(mode.name, func(b *testing.B) {
			config := bus.Config{
				WorkerPool:          pool,
				DefaultBufferSize:   1000,
				DefaultPartitions:   1,
				DefaultDeliveryMode: mode.mode,
				Name:                "bench-bus",
			}

			eventBus := bus.New(config)
			defer eventBus.Shutdown(context.Background())

			listener := newBenchmarkListener("bench-listener")
			eventBus.Subscribe("bench.topic", listener)

			event := shared.NewBaseEvent("bench-event", "bench.event", "benchmark data")
			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := eventBus.Publish(ctx, "bench.topic", event); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkEventBus_HighConcurrency(b *testing.B) {
	pool := workerpool.New(16, 1000, workerpool.WithName("bench-pool"))
	defer pool.Close(context.Background())

	config := bus.Config{
		WorkerPool:          pool,
		DefaultBufferSize:   10000,
		DefaultPartitions:   16,
		DefaultDeliveryMode: bus.AtMostOnce,
		Name:                "bench-bus",
	}

	eventBus := bus.New(config)
	defer eventBus.Shutdown(context.Background())

	// Create multiple listeners
	for i := 0; i < 5; i++ {
		listener := newBenchmarkListener("bench-listener-" + string(rune('0'+i)))
		eventBus.Subscribe("high.concurrency.topic", listener)
	}

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			eventID := "event-" + string(rune('0'+int(time.Now().UnixNano())%100))
			event := shared.NewBaseEvent(eventID, "bench.event", "benchmark data")
			if err := eventBus.Publish(ctx, "high.concurrency.topic", event); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkEventBus_Routing(b *testing.B) {
	pool := workerpool.New(4, 100, workerpool.WithName("bench-pool"))
	defer pool.Close(context.Background())

	config := bus.Config{
		WorkerPool:          pool,
		DefaultBufferSize:   1000,
		DefaultPartitions:   1,
		DefaultDeliveryMode: bus.AtMostOnce,
		Name:                "bench-bus",
	}

	eventBus := bus.New(config)
	defer eventBus.Shutdown(context.Background())

	// Create many topics to test routing performance
	for i := 0; i < 100; i++ {
		topicName := "topic" + string(rune('0'+(i%10)))
		listener := newBenchmarkListener("listener-" + string(rune('0'+(i%10))))
		eventBus.Subscribe(topicName, listener)
	}

	topics := make([]string, 10)
	for i := 0; i < 10; i++ {
		topics[i] = "topic" + string(rune('0'+i))
	}

	event := shared.NewBaseEvent("bench-event", "bench.event", "benchmark data")
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			topic := topics[int(time.Now().UnixNano())%len(topics)]
			if err := eventBus.Publish(ctx, topic, event); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// heavyBenchmarkListener simulates a listener that takes processing time
type heavyBenchmarkListener struct {
	id    string
	count int64
	mu    sync.Mutex
}

func (l *heavyBenchmarkListener) ID() string {
	return l.id
}

func (l *heavyBenchmarkListener) Handle(shared.Event) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.count++
	// Simulate some processing time
	time.Sleep(50 * time.Microsecond)
	return nil
}

func (l *heavyBenchmarkListener) OnError(shared.Event, error) error {
	return nil
}

func BenchmarkEventBus_HeavyListeners(b *testing.B) {
	pool := workerpool.New(16, 1000, workerpool.WithName("bench-pool"))
	defer pool.Close(context.Background())

	config := bus.Config{
		WorkerPool:          pool,
		DefaultBufferSize:   5000,
		DefaultPartitions:   8,
		DefaultDeliveryMode: bus.AtMostOnce,
		Name:                "bench-bus",
	}

	eventBus := bus.New(config)
	defer eventBus.Shutdown(context.Background())

	// Heavy listeners
	for i := 0; i < 3; i++ {
		listener := &heavyBenchmarkListener{id: "heavy-listener-" + string(rune('0'+i))}
		eventBus.Subscribe("heavy.topic", listener)
	}

	event := shared.NewBaseEvent("bench-event", "bench.event", "benchmark data")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := eventBus.Publish(ctx, "heavy.topic", event); err != nil {
			b.Fatal(err)
		}
	}
}
