package main

import (
	"fmt"

	"observer/sdk/wasm"
)

// Plugin metadata
const (
	PluginName    = "My Custom Plugin"
	PluginVersion = "1.0.0"
	PluginAuthor  = "Your Name"
)

func init() {
	// Register the plugin handler
	wasm.Register(handleRequest)
}

// Required empty main for TinyGo
func main() {}

// handleRequest is the main entry point for all plugin actions
func handleRequest(req wasm.Request) wasm.Response {
	switch req.Action {
	case "collect":
		return collect(req)
	case "ping":
		return ping(req)
	case "status":
		return status(req)
	case "info":
		return info(req)
	default:
		return wasm.Response{
			Status: "error",
			Error:  fmt.Sprintf("unknown action: %s", req.Action),
		}
	}
}

// collect gathers metrics from the target system
// This is called by Nord's collection system
func collect(req wasm.Request) wasm.Response {
	// Get parameters from request
	hostname := req.Args["hostname"]
	if hostname == "" {
		hostname = "unknown"
	}

	// TODO: Implement your metric collection logic here
	// Example: query an API, read a file, calculate values, etc.

	return wasm.Response{
		Status: "success",
		Data: map[string]string{
			"hostname":      hostname,
			"custom_metric": "42",
			"status":        "healthy",
			// Add your metrics here
		},
	}
}

// ping performs a health check
func ping(req wasm.Request) wasm.Response {
	target := req.Args["target"]
	if target == "" {
		target = "default"
	}

	// TODO: Implement your health check logic

	return wasm.Response{
		Status: "success",
		Data: map[string]string{
			"message": fmt.Sprintf("pong from %s", target),
			"healthy": "true",
		},
	}
}

// status returns the current plugin status
func status(req wasm.Request) wasm.Response {
	return wasm.Response{
		Status: "success",
		Data: map[string]string{
			"plugin_status": "running",
			"version":       PluginVersion,
			"capabilities":  "collect,ping,status,info",
		},
	}
}

// info returns plugin metadata
func info(req wasm.Request) wasm.Response {
	return wasm.Response{
		Status: "success",
		Data: map[string]string{
			"name":        PluginName,
			"version":     PluginVersion,
			"author":      PluginAuthor,
			"description": "A template for creating custom WASM plugins",
			"actions":     "collect, ping, status, info",
		},
	}
}

// Helper function example: parse a value with error handling
func parseValue(value string, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

// Helper function example: create an error response
func errorResponse(message string) wasm.Response {
	return wasm.Response{
		Status: "error",
		Error:  message,
	}
}
