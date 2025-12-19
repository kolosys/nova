# Performance Tuning

Optimization techniques for high-throughput, low-latency event processing with Nova.

## Benchmarking

### Running Benchmarks

```bash
# Run all benchmarks
go test -bench=. -benchmem ./...

# Run specific package benchmarks
go test -bench=. -benchmem ./emitter/
go test -bench=. -benchmem ./bus/

# Run with more iterations for accuracy
go test -bench=. -benchmem -count=5 ./...

# Profile CPU and memory
go test -bench=BenchmarkEmitter -cpuprofile=cpu.prof -memprofile=mem.prof ./emitter/
```

### Baseline Performance

Typical Nova benchmarks on modern hardware:

| Benchmark       | ops/sec | ns/op | allocs/op |
| --------------- | ------- | ----- | --------- |
| Emitter (sync)  | ~4M     | ~250  | 2-3       |
| Emitter (async) | ~8M     | ~125  | 1-2       |
| Bus (publish)   | ~3M     | ~300  | 3-4       |
| Store (append)  | ~3M     | ~350  | 2-3       |

### Custom Benchmarks

Write benchmarks for your specific use cases:

```go
func BenchmarkMyHandler(b *testing.B) {
    ctx := context.Background()
    pool := workerpool.New(4, 1000)
    defer pool.Close(ctx)

    em := emitter.New(emitter.Config{WorkerPool: pool})
    defer em.Shutdown(ctx)

    handler := shared.NewBaseListener("bench", func(e shared.Event) error {
        return nil
    })
    em.Subscribe("bench.event", handler)

    event := shared.NewBaseEvent("id", "bench.event", nil)

    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            em.Emit(ctx, event)
        }
    })
}
```

## WorkerPool Tuning

### Sizing Guidelines

```go
import "runtime"

// Formula: workers = cores * multiplier
// Multiplier depends on workload:
//   CPU-bound: 1x
//   Mixed: 2x
//   I/O-bound: 4x+

func optimalWorkers(workloadType string) int {
    cores := runtime.NumCPU()
    switch workloadType {
    case "cpu":
        return cores
    case "mixed":
        return cores * 2
    case "io":
        return cores * 4
    default:
        return cores * 2
    }
}
```

### Queue Size

Size the queue to absorb bursts:

```go
// Calculate based on expected burst
burstDurationMs := 100
eventsPerSecond := 50000
queueSize := (eventsPerSecond * burstDurationMs) / 1000

pool := workerpool.New(workers, queueSize)
```

### Separate Pools

Use separate pools for different workloads:

```go
// High-priority, fast handlers
fastPool := workerpool.New(runtime.NumCPU(), 5000)

// Low-priority, slow handlers
slowPool := workerpool.New(runtime.NumCPU()*4, 10000)

fastEmitter := emitter.New(emitter.Config{WorkerPool: fastPool})
slowEmitter := emitter.New(emitter.Config{WorkerPool: slowPool})
```

## Buffer Optimization

### Emitter Buffer

Size for expected async load:

```go
em := emitter.New(emitter.Config{
    WorkerPool: pool,
    BufferSize: 10000, // Handle bursts without blocking
    AsyncMode:  true,
})
```

### Bus Partition Buffers

Size per partition based on event rate:

```go
// Calculate per-partition buffer
totalEventsPerSecond := 100000
partitions := 16
processingLatencyMs := 10
safetyFactor := 3

bufferPerPartition := (totalEventsPerSecond / partitions) *
                      (processingLatencyMs / 1000) *
                      safetyFactor

b.CreateTopic("high-volume", bus.TopicConfig{
    BufferSize: bufferPerPartition,
    Partitions: partitions,
})
```

### Store Subscription Buffers

Balance memory vs responsiveness:

```go
// Small buffer: lower memory, may drop if slow consumer
// Large buffer: higher memory, handles slow consumers

store := memory.New(memory.Config{
    // Internal subscription channels use 100-event buffers
    // Adjust based on consumer speed
})
```

## Concurrency Tuning

### Listener Concurrency

Match concurrency to downstream capacity:

```go
// If downstream handles 100 concurrent requests
lm.Register(handler, listener.ListenerConfig{
    Concurrency: 100,
})

// Conservative approach: start low, increase based on metrics
lm.Register(handler, listener.ListenerConfig{
    Concurrency: 10, // Start here
})
```

### Partition Concurrency

Configure per-partition limits:

```go
b.CreateTopic("orders", bus.TopicConfig{
    Partitions:     8,
    MaxConcurrency: 10, // 10 concurrent per partition = 80 total
})
```

## Memory Optimization

### Event Reuse

For high-throughput scenarios, consider event pooling:

```go
var eventPool = sync.Pool{
    New: func() any {
        return &shared.BaseEvent{}
    },
}

func getEvent() *shared.BaseEvent {
    return eventPool.Get().(*shared.BaseEvent)
}

func putEvent(e *shared.BaseEvent) {
    // Reset the event
    *e = shared.BaseEvent{}
    eventPool.Put(e)
}
```

### Reduce Allocations

Avoid allocations in hot paths:

```go
// Avoid: allocates map every call
func (e *MyEvent) Metadata() map[string]string {
    return map[string]string{"key": "value"}
}

// Better: pre-allocate
type MyEvent struct {
    metadata map[string]string
}

func NewMyEvent() *MyEvent {
    return &MyEvent{
        metadata: make(map[string]string),
    }
}
```

### Store Retention

Configure retention to control memory:

```go
store := memory.New(memory.Config{
    MaxEventsPerStream: 10000,        // Limit per stream
    MaxStreams:         1000,         // Limit stream count
    RetentionDuration:  time.Hour,    // Auto-cleanup
})
```

## Latency Optimization

### Sync vs Async

Choose based on latency requirements:

```go
// Sync: predictable latency, immediate feedback
em.Emit(ctx, event)

// Async: lower apparent latency, eventual processing
em.EmitAsync(ctx, event)
```

### Delivery Mode Impact

| Mode        | Latency | Throughput |
| ----------- | ------- | ---------- |
| AtMostOnce  | Lowest  | Highest    |
| AtLeastOnce | Medium  | Medium     |
| ExactlyOnce | Highest | Lowest     |

```go
// High throughput, tolerate loss
b.CreateTopic("telemetry", bus.TopicConfig{
    DeliveryMode: bus.AtMostOnce,
})

// Balanced
b.CreateTopic("notifications", bus.TopicConfig{
    DeliveryMode: bus.AtLeastOnce,
})

// Low throughput, guaranteed
b.CreateTopic("payments", bus.TopicConfig{
    DeliveryMode: bus.ExactlyOnce,
})
```

### Reduce Lock Contention

Nova uses fine-grained locking, but you can help:

```go
// Use per-partition ordering to spread load
b.CreateTopic("events", bus.TopicConfig{
    Partitions: runtime.NumCPU() * 2,
    OrderingKey: func(e shared.Event) string {
        return e.Metadata()["partition_key"]
    },
})
```

## Monitoring Performance

### Key Metrics

Monitor these metrics for performance issues:

```go
stats := em.Stats()

// Queue depth (should stay low)
if stats.QueuedEvents > bufferSize*0.8 {
    log.Warn("Queue nearing capacity")
}

// Failed events (should be rare)
failureRate := float64(stats.FailedEvents) / float64(stats.EventsEmitted)
if failureRate > 0.01 {
    log.Warn("High failure rate: %.2f%%", failureRate*100)
}
```

### Profiling

Profile when optimizing:

```go
import (
    "runtime/pprof"
    "os"
)

// CPU profiling
f, _ := os.Create("cpu.prof")
pprof.StartCPUProfile(f)
defer pprof.StopCPUProfile()

// Memory profiling
f, _ := os.Create("mem.prof")
pprof.WriteHeapProfile(f)
```

Analyze with:

```bash
go tool pprof cpu.prof
go tool pprof -alloc_space mem.prof
```

### Continuous Profiling

Enable pprof endpoint for production:

```go
import _ "net/http/pprof"

go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

Access at:

- CPU: http://localhost:6060/debug/pprof/profile
- Memory: http://localhost:6060/debug/pprof/heap
- Goroutines: http://localhost:6060/debug/pprof/goroutine

## Tuning Checklist

### Before Going Live

- [ ] Run benchmarks with production-like data
- [ ] Test with race detection: `go test -race ./...`
- [ ] Profile CPU and memory under load
- [ ] Size workerpools based on workload type
- [ ] Size buffers for expected burst capacity
- [ ] Configure appropriate delivery modes per topic
- [ ] Set retention policies to control memory
- [ ] Enable metrics collection
- [ ] Configure alerting for queue depth and failure rates

### Under Load Issues

| Symptom            | Possible Cause    | Solution                            |
| ------------------ | ----------------- | ----------------------------------- |
| High latency       | Queue backup      | Increase workers or buffer          |
| Buffer full errors | Burst too large   | Increase buffer or add backpressure |
| Memory growth      | No retention      | Configure retention duration        |
| CPU saturation     | Too many workers  | Reduce to match cores               |
| Many retries       | Downstream issues | Check circuit breaker config        |

### Performance Regression

If performance degrades after changes:

1. Compare benchmarks before/after
2. Profile to identify hot spots
3. Check for new allocations in hot paths
4. Verify configuration hasn't changed
5. Look for increased lock contention

## Further Reading

- [Best Practices](best-practices.md) — Production patterns
- [Core Concepts](../core-concepts/shared.md) — Understanding the architecture
- [Ion Documentation](https://github.com/kolosys/ion) — WorkerPool tuning
