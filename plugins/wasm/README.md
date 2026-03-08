# WASM Plugin System

Nord supports WebAssembly (WASM) plugins for extending monitoring capabilities without recompiling the main application.

## Features

- **Sandboxed Execution**: WASM plugins run in a secure, isolated environment
- **Cross-Platform**: Write once, run anywhere (no platform-specific compilation)
- **Hot Loading**: Load and reload plugins without restarting Nord
- **Performance**: Compiled to native code with Wazero's JIT compiler
- **Simple API**: JSON-based request/response protocol

## Architecture

```
┌─────────────────────────────────────────┐
│         Nord Main Application           │
│  ┌───────────────────────────────────┐  │
│  │     WASM Plugin Manager           │  │
│  │  (plugins/wasm/wasm.go)           │  │
│  └───────────────┬───────────────────┘  │
│                  │                       │
│  ┌───────────────▼───────────────────┐  │
│  │     Wazero Runtime Engine         │  │
│  │  (plugins/wasm/engine.go)         │  │
│  └───────────────┬───────────────────┘  │
└──────────────────┼───────────────────────┘
                   │
        ┌──────────┴──────────┐
        │                     │
   ┌────▼─────┐         ┌────▼─────┐
   │ demo.wasm│         │ custom   │
   │          │         │ .wasm    │
   └──────────┘         └──────────┘
```

## Quick Start

### 1. Build the Demo Plugins

```bash
make build-wasm
```

This creates:
- `plugins/wasm/modules/demo.wasm` - Basic demo plugin
- `plugins/wasm/modules/network-monitor.wasm` - Network monitoring plugin

### 2. List Loaded Plugins

```bash
go run . -p wasm -a list
```

### 3. Execute a Plugin

```bash
# Ping action
go run . -p wasm -a execute plugin=demo req_action=ping target=nord

# Collect metrics
go run . -p wasm -a execute plugin=demo req_action=collect hostname=server1

# Get plugin info
go run . -p wasm -a execute plugin=demo req_action=info
```

### 4. Use in Collection

Add to your `data/config.json`:

```json
{
  "hosts": {
    "my-server": {
      "address": "192.168.1.100",
      "name": "My Server",
      "collect": [
        {
          "metric": "wasm",
          "wasm_plugin": "demo",
          "wasm_action": "collect",
          "hostname": "my-server"
        }
      ]
    }
  }
}
```

## Creating Your Own Plugin

### 1. Create a New Plugin

```go
package main

import (
    "observer/sdk/wasm"
)

func main() {
    wasm.Register(handleRequest)
    select {}
}

func handleRequest(req wasm.Request) wasm.Response {
    switch req.Action {
    case "collect":
        return wasm.Response{
            Status: "success",
            Data: map[string]string{
                "metric": "value",
            },
        }
    default:
        return wasm.Response{
            Status: "error",
            Error:  "unknown action",
        }
    }
}
```

### 2. Build Your Plugin

```bash
GOOS=wasip1 GOARCH=wasm go build -o plugins/wasm/modules/myplugin.wasm myplugin/main.go
```

### 3. Reload Plugins

```bash
go run . -p wasm -a reload
```

## API Reference

### Request Structure

```go
type Request struct {
    Action string            `json:"action"`
    Args   map[string]string `json:"args"`
}
```

### Response Structure

```go
type Response struct {
    Status string            `json:"status"` // "success" or "error"
    Error  string            `json:"error,omitempty"`
    Data   map[string]string `json:"data,omitempty"`
}
```

### Standard Actions

- `collect` - Gather metrics from a host
- `ping` - Health check
- `status` - Plugin status
- `info` - Plugin information

## Configuration

Add to `data/config.json`:

```json
{
  "wasm": {
    "plugins_dir": "plugins/wasm/modules",
    "enabled": true
  }
}
```

## Examples

See the `examples/` directory:
- `demo-plugin/` - Basic plugin demonstrating all features
- `network-monitor-plugin/` - Advanced network monitoring
- `wasm-plugin/` - Low-level example with custom allocator

## Performance

- **Compilation Cache**: Plugins are compiled once and cached
- **Native Speed**: Wazero compiles to native machine code
- **Memory Efficient**: Shared runtime, isolated instances
- **Fast Startup**: Typical plugin load time < 10ms

## Security

WASM plugins run in a sandboxed environment with:
- No direct file system access (except via WASI)
- No network access (unless explicitly granted)
- Memory isolation
- No access to host system calls

## Troubleshooting

### Plugin Not Loading

Check that:
1. The `.wasm` file is in `plugins/wasm/modules/`
2. The file was built with `GOOS=wasip1 GOARCH=wasm`
3. The plugin exports `malloc` and `execute` functions

### Execution Errors

Enable debug logging:
```bash
NORD_DEBUG=1 go run . -p wasm -a execute plugin=demo req_action=ping
```

### Build Errors

Ensure you're using Go 1.21+ with WASI support:
```bash
go version  # Should be 1.21 or higher
```

## Advanced Topics

### Custom Memory Management

See `examples/wasm-plugin/main.go` for a custom allocator implementation.

### Multiple Plugin Instances

Each execution creates a fresh module instance, ensuring isolation.

### Plugin Lifecycle

1. **Load**: Compile WASM to native code (cached)
2. **Instantiate**: Create module instance
3. **Execute**: Call exported function
4. **Cleanup**: Module instance destroyed

## Contributing

To add new example plugins:
1. Create a directory in `examples/`
2. Add build target to `Makefile`
3. Update this README

## License

Same as Nord main project.
