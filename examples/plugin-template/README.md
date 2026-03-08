# WASM Plugin Template

This is a template for creating custom WASM plugins for Nord.

## Quick Start

### 1. Copy the Template

```bash
cp -r examples/plugin-template my-plugin
cd my-plugin
```

### 2. Customize the Plugin

Edit `main.go`:

```go
const (
    PluginName    = "My Custom Plugin"  // Change this
    PluginVersion = "1.0.0"
    PluginAuthor  = "Your Name"         // Change this
)
```

### 3. Implement Your Logic

Add your custom logic in the `collect()` function:

```go
func collect(req wasm.Request) wasm.Response {
    hostname := req.Args["hostname"]
    
    // Your custom logic here
    // Example: query an API, read metrics, etc.
    
    return wasm.Response{
        Status: "success",
        Data: map[string]string{
            "hostname": hostname,
            "my_metric": "value",
        },
    }
}
```

### 4. Build Your Plugin

```bash
tinygo build -o ../../plugins/wasm/modules/my-plugin.wasm -target=wasi main.go
```

### 5. Test Your Plugin

```bash
cd ../..
go run . -p wasm -a reload
go run . -p wasm -a execute "plugin=my-plugin req_action=info"
go run . -p wasm -a execute "plugin=my-plugin req_action=collect hostname=test"
```

## Plugin Structure

### Required Functions

#### `init()`
Registers your handler with the SDK:
```go
func init() {
    wasm.Register(handleRequest)
}
```

#### `main()`
Required but empty for TinyGo:
```go
func main() {}
```

#### `handleRequest()`
Routes actions to handlers:
```go
func handleRequest(req wasm.Request) wasm.Response {
    switch req.Action {
    case "collect":
        return collect(req)
    // ... other actions
    }
}
```

### Standard Actions

Implement these for Nord integration:

- **`collect`** - Gather metrics (called by collection system)
- **`ping`** - Health check
- **`status`** - Plugin status
- **`info`** - Plugin metadata

### Custom Actions

Add your own actions:

```go
case "my_custom_action":
    return myCustomAction(req)
```

## Request/Response API

### Request Structure

```go
type Request struct {
    Action string            // The action to perform
    Args   map[string]string // Action arguments
}
```

Access arguments:
```go
value := req.Args["key"]
```

### Response Structure

```go
type Response struct {
    Status string            // "success" or "error"
    Error  string            // Error message (if status is "error")
    Data   map[string]string // Response data
}
```

Success response:
```go
return wasm.Response{
    Status: "success",
    Data: map[string]string{
        "key": "value",
    },
}
```

Error response:
```go
return wasm.Response{
    Status: "error",
    Error:  "something went wrong",
}
```

## Best Practices

### 1. Error Handling

Always handle errors gracefully:

```go
if value == "" {
    return wasm.Response{
        Status: "error",
        Error:  "required parameter missing",
    }
}
```

### 2. Default Values

Provide sensible defaults:

```go
hostname := req.Args["hostname"]
if hostname == "" {
    hostname = "unknown"
}
```

### 3. Validation

Validate input parameters:

```go
if !isValidHostname(hostname) {
    return errorResponse("invalid hostname")
}
```

### 4. Documentation

Document your actions in the `info` response:

```go
Data: map[string]string{
    "actions": "collect, ping, my_action",
    "description": "What your plugin does",
}
```

### 5. Consistent Naming

Use consistent key names in responses:
- `hostname` not `host` or `server`
- `status` not `state`
- `timestamp` not `time`

## Integration with Nord

### Configuration

Add to `data/config.json`:

```json
{
  "hosts": {
    "my-server": {
      "address": "192.168.1.100",
      "name": "My Server",
      "collect": [
        {
          "metric": "wasm",
          "wasm_plugin": "my-plugin",
          "wasm_action": "collect",
          "hostname": "my-server"
        }
      ]
    }
  }
}
```

### Run Collection

```bash
go run . --collect
```

## Examples

### Query an External API

```go
func collect(req wasm.Request) wasm.Response {
    // Note: WASM has limited network access
    // You may need to pass data via Args
    
    apiData := req.Args["api_data"]
    
    return wasm.Response{
        Status: "success",
        Data: map[string]string{
            "result": apiData,
        },
    }
}
```

### Calculate Metrics

```go
func collect(req wasm.Request) wasm.Response {
    value1 := req.Args["value1"]
    value2 := req.Args["value2"]
    
    // Perform calculations
    result := calculateSomething(value1, value2)
    
    return wasm.Response{
        Status: "success",
        Data: map[string]string{
            "calculated": result,
        },
    }
}
```

### Multiple Metrics

```go
func collect(req wasm.Request) wasm.Response {
    return wasm.Response{
        Status: "success",
        Data: map[string]string{
            "cpu_usage":    "45%",
            "memory_free":  "2.5 GB",
            "disk_usage":   "67%",
            "network_rx":   "1.2 Mbps",
            "network_tx":   "0.8 Mbps",
            "status":       "healthy",
        },
    }
}
```

## Debugging

### Print to Stderr

```go
import "fmt"

func collect(req wasm.Request) wasm.Response {
    fmt.Fprintf(os.Stderr, "Debug: hostname=%s\n", req.Args["hostname"])
    // ...
}
```

### Return Debug Info

```go
return wasm.Response{
    Status: "success",
    Data: map[string]string{
        "result": "value",
        "debug_info": "additional context",
    },
}
```

## Limitations

1. **No Network Access**: WASM plugins can't make HTTP requests directly
2. **No File System**: Limited file system access via WASI
3. **Synchronous Only**: No async/await support
4. **JSON Only**: All data must be strings in the Data map

## Workarounds

### Network Access

Pass data via Args:
```bash
go run . -p wasm -a execute "plugin=my-plugin req_action=collect api_data=$(curl -s https://api.example.com)"
```

### Complex Data

Serialize to JSON string:
```go
Data: map[string]string{
    "metrics": `{"cpu": 45, "memory": 2048}`,
}
```

## Testing

### Unit Test Your Logic

Create `main_test.go`:

```go
package main

import "testing"

func TestCollect(t *testing.T) {
    req := wasm.Request{
        Action: "collect",
        Args: map[string]string{
            "hostname": "test",
        },
    }
    
    resp := collect(req)
    
    if resp.Status != "success" {
        t.Errorf("expected success, got %s", resp.Status)
    }
}
```

Run tests:
```bash
go test
```

### Integration Test

```bash
# Build
tinygo build -o test.wasm -target=wasi main.go

# Test
go run ../../. -p wasm -a load test.wasm
go run ../../. -p wasm -a execute "plugin=test req_action=collect"
```

## Resources

- [TinyGo Documentation](https://tinygo.org/docs/)
- [WASM Specification](https://webassembly.org/)
- [Wazero Runtime](https://wazero.io/)
- [Nord WASM Quick Start](../../WASM_QUICKSTART.md)
- [Nord WASM README](../../plugins/wasm/README.md)

## Support

For issues or questions:
1. Check the main WASM documentation
2. Review example plugins in `examples/`
3. Run the test suite: `./test-wasm.sh`

## License

Same as Nord main project.
