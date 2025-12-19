# Installation

This guide covers installing Nova and its dependencies.

## Prerequisites

- **Go 1.24** or later
- A Go module initialized in your project

## Install Nova

```bash
go get github.com/kolosys/nova@latest
```

Nova requires [Ion](https://github.com/kolosys/ion) for its workerpool. This dependency is automatically installed.

## Import Packages

Nova is organized into focused packages. Import only what you need:

```go
import (
    "github.com/kolosys/nova/emitter"  // Direct event emission
    "github.com/kolosys/nova/bus"      // Topic-based routing
    "github.com/kolosys/nova/listener" // Lifecycle management
    "github.com/kolosys/nova/memory"   // In-memory event store
    "github.com/kolosys/nova/shared"   // Core types and interfaces
)
```

You'll also need Ion's workerpool:

```go
import "github.com/kolosys/ion/workerpool"
```

## Verify Installation

Create a simple test to verify everything works:

```go
package main

import (
    "context"
    "fmt"

    "github.com/kolosys/ion/workerpool"
    "github.com/kolosys/nova/emitter"
    "github.com/kolosys/nova/shared"
)

func main() {
    pool := workerpool.New(4, 100)
    defer pool.Close(context.Background())

    em := emitter.New(emitter.Config{WorkerPool: pool})
    defer em.Shutdown(context.Background())

    fmt.Println("Nova installed successfully!")
    fmt.Printf("Emitter stats: %+v\n", em.Stats())
}
```

Run it:

```bash
go run main.go
```

## Version Pinning

Install a specific version:

```bash
go get github.com/kolosys/nova@v0.1.0
```

## Updating

Update to the latest version:

```bash
go get -u github.com/kolosys/nova@latest
```

## Development Setup

To contribute or modify Nova:

```bash
git clone https://github.com/kolosys/nova.git
cd nova
go mod download
go test -race ./...
```

## Troubleshooting

### Module Not Found

Ensure your Go environment is configured correctly:

```bash
go env GOPATH
go env GOPROXY
```

Try clearing the module cache:

```bash
go clean -modcache
go get github.com/kolosys/nova@latest
```

### Private Repository Access

For private repositories, configure Git authentication:

```bash
git config --global url."git@github.com:".insteadOf "https://github.com/"
```

Or set GOPRIVATE:

```bash
export GOPRIVATE=github.com/kolosys/*
```

## Next Steps

Continue to the [Quick Start](quick-start.md) guide to build your first event system.
