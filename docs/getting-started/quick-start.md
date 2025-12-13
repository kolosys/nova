# Quick Start

This guide will help you get started with nova quickly with a basic example.

## Basic Usage

Here's a simple example to get you started:

```go
package main

import (
    "fmt"
    "log"
    "github.com/kolosys/nova/bus"
    "github.com/kolosys/nova/emitter"
    "github.com/kolosys/nova/listener"
    "github.com/kolosys/nova/memory"
    "github.com/kolosys/nova/shared"
)

func main() {
    // Basic usage example
    fmt.Println("Welcome to nova!")
    
    // TODO: Add your code here
}
```

## Common Use Cases

### Using bus

**Import Path:** `github.com/kolosys/nova/bus`



```go
package main

import (
    "fmt"
    "github.com/kolosys/nova/bus"
)

func main() {
    // Example usage of bus
    fmt.Println("Using bus package")
}
```

#### Available Types
- **Config** - Config configures the EventBus
- **DeliveryMode** - DeliveryMode defines the delivery guarantees for events
- **EventBus** - EventBus defines the interface for topic-based event routing
- **Stats** - Stats provides bus statistics
- **TopicConfig** - TopicConfig configures a topic

For detailed API documentation, see the [bus API Reference](../api-reference/bus.md).

### Using emitter

**Import Path:** `github.com/kolosys/nova/emitter`



```go
package main

import (
    "fmt"
    "github.com/kolosys/nova/emitter"
)

func main() {
    // Example usage of emitter
    fmt.Println("Using emitter package")
}
```

#### Available Types
- **Config** - Config configures the EventEmitter
- **EventEmitter** - EventEmitter defines the interface for event emission and subscription
- **Stats** - Stats provides emitter statistics

For detailed API documentation, see the [emitter API Reference](../api-reference/emitter.md).

### Using listener

**Import Path:** `github.com/kolosys/nova/listener`



```go
package main

import (
    "fmt"
    "github.com/kolosys/nova/listener"
)

func main() {
    // Example usage of listener
    fmt.Println("Using listener package")
}
```

#### Available Types
- **BackoffStrategy** - BackoffStrategy defines different backoff strategies for retries
- **CircuitConfig** - CircuitConfig configures circuit breaker behavior
- **Config** - Config configures the ListenerManager
- **DeadLetterConfig** - DeadLetterConfig configures dead letter queue behavior
- **HealthStatus** - HealthStatus represents listener health
- **ListenerConfig** - ListenerConfig configures listener behavior
- **ListenerManager** - ListenerManager defines the interface for managing event listeners
- **RetryPolicy** - RetryPolicy defines retry behavior for failed events
- **Stats** - Stats provides listener manager statistics

For detailed API documentation, see the [listener API Reference](../api-reference/listener.md).

### Using memory

**Import Path:** `github.com/kolosys/nova/memory`



```go
package main

import (
    "fmt"
    "github.com/kolosys/nova/memory"
)

func main() {
    // Example usage of memory
    fmt.Println("Using memory package")
}
```

#### Available Types
- **Config** - Config configures the EventStore
- **Cursor** - Cursor represents a position in the event stream
- **EventStore** - EventStore defines the interface for event storage and replay
- **Stats** - Stats provides store statistics
- **StreamInfo** - StreamInfo provides information about a stream

For detailed API documentation, see the [memory API Reference](../api-reference/memory.md).

### Using shared

**Import Path:** `github.com/kolosys/nova/shared`



```go
package main

import (
    "fmt"
    "github.com/kolosys/nova/shared"
)

func main() {
    // Example usage of shared
    fmt.Println("Using shared package")
}
```

#### Available Types
- **BaseEvent** - BaseEvent provides a default implementation of the Event interface
- **BaseListener** - BaseListener provides a basic implementation of Listener
- **BaseSubscription** - BaseSubscription provides a default implementation of Subscription
- **Event** - Event represents a domain event in the Nova system
- **EventError** - EventError wraps an error with event context
- **EventValidator** - EventValidator validates events before processing
- **EventValidatorFunc** - EventValidatorFunc is a function adapter for EventValidator
- **Listener** - Listener represents an event listener
- **ListenerError** - ListenerError wraps an error with listener context
- **MetricsCollector** - MetricsCollector defines the interface for collecting Nova metrics
- **Middleware** - Middleware provides hooks for cross-cutting concerns
- **MiddlewareFunc** - MiddlewareFunc is a function adapter for Middleware
- **NoOpMetricsCollector** - NoOpMetricsCollector provides a no-op implementation for when metrics are disabled
- **SimpleMetricsCollector** - SimpleMetricsCollector provides a basic in-memory metrics collector for testing
- **Subscription** - Subscription represents an active subscription to events
- **ValidationError** - ValidationError indicates a validation failure

For detailed API documentation, see the [shared API Reference](../api-reference/shared.md).

## Step-by-Step Tutorial

### Step 1: Import the Package

First, import the necessary packages in your Go file:

```go
import (
    "fmt"
    "github.com/kolosys/nova/bus"
    "github.com/kolosys/nova/emitter"
    "github.com/kolosys/nova/listener"
    "github.com/kolosys/nova/memory"
    "github.com/kolosys/nova/shared"
)
```

### Step 2: Initialize

Set up the basic configuration:

```go
func main() {
    // Initialize your application
    fmt.Println("Initializing nova...")
}
```

### Step 3: Use the Library

Implement your specific use case:

```go
func main() {
    // Your implementation here
}
```

## Running Your Code

To run your Go program:

```bash
go run main.go
```

To build an executable:

```bash
go build -o myapp
./myapp
```

## Configuration Options

nova can be configured to suit your needs. Check the [Core Concepts](../core-concepts/) section for detailed information about configuration options.

## Error Handling

Always handle errors appropriately:

```go
result, err := someFunction()
if err != nil {
    log.Fatalf("Error: %v", err)
}
```

## Best Practices

- Always handle errors returned by library functions
- Check the API documentation for detailed parameter information
- Use meaningful variable and function names
- Add comments to document your code

## Complete Example

Here's a complete working example:

```go
package main

import (
    "fmt"
    "log"
    "github.com/kolosys/nova/bus"
    "github.com/kolosys/nova/emitter"
    "github.com/kolosys/nova/listener"
    "github.com/kolosys/nova/memory"
    "github.com/kolosys/nova/shared"
)

func main() {
    fmt.Println("Starting nova application...")
    
    // Add your implementation here
    
    fmt.Println("Application completed successfully!")
}
```

## Next Steps

Now that you've seen the basics, explore:

- **[Core Concepts](../core-concepts/)** - Understanding the library architecture
- **[API Reference](../api-reference/)** - Complete API documentation
- **[Examples](../examples/README.md)** - More detailed examples
- **[Advanced Topics](../advanced/)** - Performance tuning and advanced patterns

## Getting Help

If you run into issues:

1. Check the [API Reference](../api-reference/)
2. Browse the [Examples](../examples/README.md)
3. Visit the [GitHub Issues](https://github.com/kolosys/nova/issues) page

