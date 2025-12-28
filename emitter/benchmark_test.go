package emitter_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/kolosys/ion/workerpool"
	"github.com/kolosys/nova/emitter"
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

func (l *benchmarkListener) Handle(_ context.Context, _ shared.Event) error {
	l.count++
	return nil
}

func (l *benchmarkListener) OnError(_ context.Context, _ shared.Event, _ error) error {
	return nil
}

func BenchmarkEmitter_EmitSync(b *testing.B) {
	pool := workerpool.New(4, 100, workerpool.WithName("bench-pool"))
	defer pool.Close(context.Background())

	config := emitter.Config{
		WorkerPool: pool,
		AsyncMode:  false,
		BufferSize: 1000,
		Name:       "bench-emitter",
	}

	em := emitter.New(config)
	defer em.Shutdown(context.Background())

	listener := newBenchmarkListener("bench-listener")
	em.Subscribe("bench.event", listener)

	event := shared.NewBaseEvent("bench-event", "bench.event", "benchmark data")
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := em.Emit(ctx, event); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkEmitter_EmitAsync(b *testing.B) {
	pool := workerpool.New(4, 100, workerpool.WithName("bench-pool"))
	defer pool.Close(context.Background())

	config := emitter.Config{
		WorkerPool: pool,
		AsyncMode:  true,
		BufferSize: 10000, // Large buffer for async benchmarks
		Name:       "bench-emitter",
	}

	em := emitter.New(config)
	defer em.Shutdown(context.Background())

	listener := newBenchmarkListener("bench-listener")
	em.Subscribe("bench.event", listener)

	event := shared.NewBaseEvent("bench-event", "bench.event", "benchmark data")
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := em.EmitAsync(ctx, event); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkEmitter_MultipleListeners(b *testing.B) {
	pool := workerpool.New(8, 200, workerpool.WithName("bench-pool"))
	defer pool.Close(context.Background())

	config := emitter.Config{
		WorkerPool: pool,
		AsyncMode:  false,
		BufferSize: 1000,
		Name:       "bench-emitter",
	}

	em := emitter.New(config)
	defer em.Shutdown(context.Background())

	// Create multiple listeners
	for i := 0; i < 10; i++ {
		listener := newBenchmarkListener("bench-listener-" + string(rune('0'+i)))
		em.Subscribe("bench.event", listener)
	}

	event := shared.NewBaseEvent("bench-event", "bench.event", "benchmark data")
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := em.Emit(ctx, event); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkEmitter_EmitBatch(b *testing.B) {
	pool := workerpool.New(4, 100, workerpool.WithName("bench-pool"))
	defer pool.Close(context.Background())

	config := emitter.Config{
		WorkerPool: pool,
		AsyncMode:  false,
		BufferSize: 1000,
		Name:       "bench-emitter",
	}

	em := emitter.New(config)
	defer em.Shutdown(context.Background())

	listener := newBenchmarkListener("bench-listener")
	em.Subscribe("bench.event", listener)

	// Create batch of events
	events := make([]shared.Event, 10)
	for i := 0; i < 10; i++ {
		events[i] = shared.NewBaseEvent("bench-event", "bench.event", "benchmark data")
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := em.EmitBatch(ctx, events); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEmitter_Concurrency(b *testing.B) {
	pool := workerpool.New(16, 1000, workerpool.WithName("bench-pool"))
	defer pool.Close(context.Background())

	config := emitter.Config{
		WorkerPool:     pool,
		AsyncMode:      true,
		BufferSize:     50000,
		MaxConcurrency: 100,
		Name:           "bench-emitter",
	}

	em := emitter.New(config)
	defer em.Shutdown(context.Background())

	// Heavy listener that takes some time
	heavyListener := &heavyBenchmarkListener{id: "heavy-listener"}
	em.Subscribe("bench.event", heavyListener)

	event := shared.NewBaseEvent("bench-event", "bench.event", "benchmark data")
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := em.EmitAsync(ctx, event); err != nil {
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

func (l *heavyBenchmarkListener) Handle(_ context.Context, _ shared.Event) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.count++
	// Simulate some processing time
	time.Sleep(100 * time.Microsecond)
	return nil
}

func (l *heavyBenchmarkListener) OnError(_ context.Context, _ shared.Event, _ error) error {
	return nil
}

func BenchmarkEmitter_Middleware(b *testing.B) {
	pool := workerpool.New(4, 100, workerpool.WithName("bench-pool"))
	defer pool.Close(context.Background())

	config := emitter.Config{
		WorkerPool: pool,
		AsyncMode:  false,
		BufferSize: 1000,
		Name:       "bench-emitter",
	}

	em := emitter.New(config)
	defer em.Shutdown(context.Background())

	// Add middleware
	middleware := shared.MiddlewareFunc{
		BeforeFunc: func(_ context.Context, _ shared.Event) error { return nil },
		AfterFunc:  func(_ context.Context, _ shared.Event, _ error) error { return nil },
	}
	em.Middleware(middleware, middleware, middleware) // Stack 3 middleware

	listener := newBenchmarkListener("bench-listener")
	em.Subscribe("bench.event", listener)

	event := shared.NewBaseEvent("bench-event", "bench.event", "benchmark data")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := em.Emit(ctx, event); err != nil {
			b.Fatal(err)
		}
	}
}
