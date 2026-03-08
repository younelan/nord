package main

import (
	"fmt"
	"time"

	"observer/sdk/wasm"
)

func init() {
	// Register our plugin handler
	wasm.Register(handleRequest)
}

// main is required but does nothing for TinyGo WASM
func main() {}

func handleRequest(req wasm.Request) wasm.Response {
	switch req.Action {
	case "ping":
		return handlePing(req)
	case "collect":
		return handleCollect(req)
	case "status":
		return handleStatus(req)
	case "info":
		return handleInfo(req)
	default:
		return wasm.Response{
			Status: "error",
			Error:  fmt.Sprintf("unknown action: %s", req.Action),
		}
	}
}

func handlePing(req wasm.Request) wasm.Response {
	target := req.Args["target"]
	if target == "" {
		target = "world"
	}
	
	return wasm.Response{
		Status: "success",
		Data: map[string]string{
			"message": fmt.Sprintf("pong from %s", target),
			"time":    time.Now().Format(time.RFC3339),
		},
	}
}

func handleCollect(req wasm.Request) wasm.Response {
	// Simulate collecting metrics
	hostname := req.Args["hostname"]
	if hostname == "" {
		hostname = "unknown"
	}
	
	return wasm.Response{
		Status: "success",
		Data: map[string]string{
			"plugin":      "demo-wasm-plugin",
			"hostname":    hostname,
			"metric_type": "custom",
			"cpu_usage":   "42%",
			"memory_free": "2.5 GB",
			"uptime":      "7d 3h 15m",
			"status":      "healthy",
			"collected":   time.Now().Format(time.RFC3339),
		},
	}
}

func handleStatus(req wasm.Request) wasm.Response {
	return wasm.Response{
		Status: "success",
		Data: map[string]string{
			"plugin_status": "running",
			"version":       "1.0.0",
			"capabilities":  "ping,collect,status,info",
		},
	}
}

func handleInfo(req wasm.Request) wasm.Response {
	return wasm.Response{
		Status: "success",
		Data: map[string]string{
			"name":        "Demo WASM Plugin",
			"version":     "1.0.0",
			"author":      "Nord Team",
			"description": "A demonstration WebAssembly plugin for Nord monitoring",
			"actions":     "ping, collect, status, info",
		},
	}
}
