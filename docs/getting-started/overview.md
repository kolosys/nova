# Overview

Nova is an enterprise-grade event processing library for Go that delivers predictable performance, robust delivery guarantees, and comprehensive observability. Built as a companion to [Ion](https://github.com/kolosys/ion), it leverages proven concurrency primitives for reliable, high-performance event systems.

## Why Nova?

Modern distributed systems require sophisticated event handling with guarantees that basic channels cannot provide. Nova bridges this gap by offering:

- **Production-ready semantics** — at-most-once, at-least-once, and exactly-once delivery modes
- **Resilience patterns** — circuit breakers, retry policies, and dead letter queues
- **Zero external dependencies** — no message brokers or heavyweight frameworks required
- **Context-aware APIs** — full support for cancellation and timeouts

## Core Components

Nova provides four main packages that work independently or together:

| Package      | Purpose                                                         |
| ------------ | --------------------------------------------------------------- |
| **emitter**  | Direct event emission with sync/async processing and middleware |
| **bus**      | Topic-based routing with pattern matching and partitioning      |
| **listener** | Lifecycle management with retries, circuits, and dead letters   |
| **memory**   | In-memory event store with replay and live subscriptions        |

All components share common types from the **shared** package.

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Application   │    │   Application   │    │   Application   │
└────────┬────────┘    └────────┬────────┘    └────────┬────────┘
         │                      │                      │
         ▼                      ▼                      ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│     Emitter     │    │      Bus        │    │  Listener Mgr   │
│  • Sync/Async   │    │  • Topics       │    │  • Retries      │
│  • Middleware   │    │  • Patterns     │    │  • Circuits     │
│  • Concurrency  │    │  • Partitions   │    │  • Dead Letter  │
└────────┬────────┘    └────────┬────────┘    └────────┬────────┘
         │                      │                      │
         └──────────────────────┼──────────────────────┘
                                │
                                ▼
                   ┌─────────────────────┐
                   │    Event Store      │
                   │  • Streams          │
                   │  • Replay           │
                   │  • Subscriptions    │
                   └────────┬────────────┘
                            │
                            ▼
                   ┌─────────────────────┐
                   │  Ion WorkerPool     │
                   │  • Concurrency      │
                   │  • Load Balancing   │
                   └─────────────────────┘
```

## Key Features

### Delivery Guarantees

Nova supports three delivery modes to match your requirements:

- **AtMostOnce** — fire-and-forget for maximum throughput
- **AtLeastOnce** — guaranteed delivery with possible duplicates (default)
- **ExactlyOnce** — guaranteed delivery without duplicates

### Resilience

Built-in fault tolerance patterns protect your system:

- **Retry Policies** — configurable backoff strategies (fixed, linear, exponential)
- **Circuit Breakers** — prevent cascade failures with automatic recovery
- **Dead Letter Queues** — capture failed events for later processing

### Observability

Comprehensive metrics and health monitoring:

- Events emitted, processed, dropped, and failed
- Listener processing durations
- Queue sizes and active subscriptions
- Health status (healthy, degraded, unhealthy, circuit-open)

## Requirements

- Go 1.24 or later
- [github.com/kolosys/ion](https://github.com/kolosys/ion) — provides the workerpool

## Getting Started

Ready to use Nova? Continue to:

1. [Installation](installation.md) — add Nova to your project
2. [Quick Start](quick-start.md) — build your first event system
3. [Core Concepts](../core-concepts/shared.md) — understand the fundamentals

## Links

- **Repository**: [github.com/kolosys/nova](https://github.com/kolosys/nova)
- **Ion Library**: [github.com/kolosys/ion](https://github.com/kolosys/ion)
- **Issues**: [github.com/kolosys/nova/issues](https://github.com/kolosys/nova/issues)
