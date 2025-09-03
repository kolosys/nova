package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/kolosys/nova/memory"
	"github.com/kolosys/nova/shared"
)

func BenchmarkEventStore_Append(b *testing.B) {
	config := memory.DefaultConfig()
	config.MaxEventsPerStream = 0 // Unlimited for benchmark
	store := memory.New(config)
	defer store.Close()

	event := shared.NewBaseEvent("bench-event", "bench.event", "benchmark data")
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		streamID := "bench-stream"
		for pb.Next() {
			if err := store.Append(ctx, streamID, event); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkEventStore_AppendBatch(b *testing.B) {
	config := memory.DefaultConfig()
	config.MaxEventsPerStream = 0 // Unlimited for benchmark
	store := memory.New(config)
	defer store.Close()

	// Create batch of events
	events := make([]shared.Event, 10)
	for i := 0; i < 10; i++ {
		events[i] = shared.NewBaseEvent("bench-event", "bench.event", "benchmark data")
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := store.Append(ctx, "bench-stream", events...); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEventStore_Read(b *testing.B) {
	config := memory.DefaultConfig()
	store := memory.New(config)
	defer store.Close()

	ctx := context.Background()

	// Populate store with events
	events := make([]shared.Event, 1000)
	for i := 0; i < 1000; i++ {
		events[i] = shared.NewBaseEvent("event", "bench.event", "benchmark data")
	}
	if err := store.Append(ctx, "bench-stream", events...); err != nil {
		b.Fatal(err)
	}

	cursor := memory.Cursor{StreamID: "bench-stream", Position: 0}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _, err := store.Read(ctx, "bench-stream", cursor, 100)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkEventStore_ReadStream(b *testing.B) {
	config := memory.DefaultConfig()
	store := memory.New(config)
	defer store.Close()

	ctx := context.Background()

	// Populate store with events
	events := make([]shared.Event, 1000)
	for i := 0; i < 1000; i++ {
		events[i] = shared.NewBaseEvent("event", "bench.event", "benchmark data")
	}
	if err := store.Append(ctx, "bench-stream", events...); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := store.ReadStream(ctx, "bench-stream", 0)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEventStore_ReadTimeRange(b *testing.B) {
	config := memory.DefaultConfig()
	store := memory.New(config)
	defer store.Close()

	ctx := context.Background()

	// Populate store with events across multiple streams
	for streamNum := 0; streamNum < 10; streamNum++ {
		events := make([]shared.Event, 100)
		for i := 0; i < 100; i++ {
			events[i] = shared.NewBaseEvent("event", "bench.event", "benchmark data")
		}
		streamID := "stream-" + string(rune('0'+streamNum))
		if err := store.Append(ctx, streamID, events...); err != nil {
			b.Fatal(err)
		}
	}

	from := time.Now().Add(-time.Hour)
	to := time.Now().Add(time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := store.ReadTimeRange(ctx, from, to)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEventStore_Replay(b *testing.B) {
	config := memory.DefaultConfig()
	store := memory.New(config)
	defer store.Close()

	ctx := context.Background()

	// Populate store with events
	events := make([]shared.Event, 100)
	for i := 0; i < 100; i++ {
		events[i] = shared.NewBaseEvent("event", "bench.event", "benchmark data")
	}
	if err := store.Append(ctx, "bench-stream", events...); err != nil {
		b.Fatal(err)
	}

	from := time.Now().Add(-time.Hour)
	to := time.Now().Add(time.Hour)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		replayCh, err := store.Replay(ctx, from, to)
		if err != nil {
			b.Fatal(err)
		}

		// Consume all events
		for range replayCh {
		}
	}
}

func BenchmarkEventStore_MultipleStreams(b *testing.B) {
	config := memory.DefaultConfig()
	config.MaxStreams = 0 // Unlimited streams
	store := memory.New(config)
	defer store.Close()

	event := shared.NewBaseEvent("bench-event", "bench.event", "benchmark data")
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		streamCount := 0
		for pb.Next() {
			streamID := "stream-" + string(rune('0'+(streamCount%10)))
			streamCount++
			if err := store.Append(ctx, streamID, event); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkEventStore_ConcurrentReadWrite(b *testing.B) {
	config := memory.DefaultConfig()
	config.MaxEventsPerStream = 0 // Unlimited for benchmark
	store := memory.New(config)
	defer store.Close()

	ctx := context.Background()

	// Pre-populate with some events
	events := make([]shared.Event, 100)
	for i := 0; i < 100; i++ {
		events[i] = shared.NewBaseEvent("event", "bench.event", "benchmark data")
	}
	if err := store.Append(ctx, "bench-stream", events...); err != nil {
		b.Fatal(err)
	}

	event := shared.NewBaseEvent("bench-event", "bench.event", "benchmark data")
	cursor := memory.Cursor{StreamID: "bench-stream", Position: 0}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Randomly do either append or read
			if time.Now().UnixNano()%2 == 0 {
				_ = store.Append(ctx, "bench-stream", event)
			} else {
				_, _, _ = store.Read(ctx, "bench-stream", cursor, 10)
			}
		}
	})
}

func BenchmarkEventStore_Subscribe(b *testing.B) {
	config := memory.DefaultConfig()
	store := memory.New(config)
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Pre-populate with events
	events := make([]shared.Event, 10)
	for i := 0; i < 10; i++ {
		events[i] = shared.NewBaseEvent("event", "bench.event", "benchmark data")
	}
	if err := store.Append(ctx, "bench-stream", events...); err != nil {
		b.Fatal(err)
	}

	cursor := memory.Cursor{StreamID: "bench-stream", Position: 0}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eventCh, err := store.Subscribe(ctx, "bench-stream", cursor)
		if err != nil {
			b.Fatal(err)
		}

		// Consume first few events
		eventCount := 0
		for event := range eventCh {
			_ = event
			eventCount++
			if eventCount >= 5 {
				break
			}
		}
	}
}

func BenchmarkEventStore_LargeEvents(b *testing.B) {
	config := memory.DefaultConfig()
	config.MaxEventsPerStream = 0 // Unlimited for benchmark
	store := memory.New(config)
	defer store.Close()

	// Create large event data
	largeData := make([]byte, 10*1024) // 10KB
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	event := shared.NewBaseEvent("large-event", "bench.event", string(largeData))
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := store.Append(ctx, "large-stream", event); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEventStore_StreamInfo(b *testing.B) {
	config := memory.DefaultConfig()
	store := memory.New(config)
	defer store.Close()

	ctx := context.Background()

	// Create multiple streams with events
	for streamNum := 0; streamNum < 50; streamNum++ {
		events := make([]shared.Event, 20)
		for i := 0; i < 20; i++ {
			events[i] = shared.NewBaseEvent("event", "bench.event", "benchmark data")
		}
		streamID := "stream-" + string(rune('0'+(streamNum%10)))
		if err := store.Append(ctx, streamID, events...); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Alternate between GetStreams and GetStreamInfo
			if time.Now().UnixNano()%2 == 0 {
				_ = store.GetStreams()
			} else {
				streamID := "stream-" + string(rune('0'+int(time.Now().UnixNano())%10))
				_, _ = store.GetStreamInfo(streamID)
			}
		}
	})
}
