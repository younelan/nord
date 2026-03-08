package main

import (
	"fmt"
	"math/rand"
	"time"

	"observer/sdk/wasm"
)

func init() {
	wasm.Register(handleRequest)
}

// main is required but does nothing for TinyGo WASM
func main() {}

func handleRequest(req wasm.Request) wasm.Response {
	switch req.Action {
	case "collect":
		return collectNetworkMetrics(req)
	case "check_latency":
		return checkLatency(req)
	case "check_bandwidth":
		return checkBandwidth(req)
	case "info":
		return pluginInfo()
	default:
		return wasm.Response{
			Status: "error",
			Error:  fmt.Sprintf("unknown action: %s", req.Action),
		}
	}
}

func collectNetworkMetrics(req wasm.Request) wasm.Response {
	host := req.Args["host"]
	if host == "" {
		host = "unknown"
	}
	
	// Simulate network metrics collection
	seed := time.Now().UnixNano()
	rng := rand.New(rand.NewSource(seed))
	
	latency := 10 + rng.Intn(90)  // 10-100ms
	packetLoss := rng.Float64() * 2  // 0-2%
	bandwidth := 80 + rng.Intn(20)  // 80-100 Mbps
	
	status := "up"
	if latency > 80 {
		status = "degraded"
	}
	if packetLoss > 1.5 {
		status = "unstable"
	}
	
	return wasm.Response{
		Status: "success",
		Data: map[string]string{
			"host":        host,
			"status":      status,
			"latency_ms":  fmt.Sprintf("%d", latency),
			"packet_loss": fmt.Sprintf("%.2f%%", packetLoss),
			"bandwidth":   fmt.Sprintf("%d Mbps", bandwidth),
			"jitter":      fmt.Sprintf("%d ms", rng.Intn(10)),
			"timestamp":   time.Now().Format(time.RFC3339),
		},
	}
}

func checkLatency(req wasm.Request) wasm.Response {
	target := req.Args["target"]
	if target == "" {
		return wasm.Response{
			Status: "error",
			Error:  "target parameter required",
		}
	}
	
	seed := time.Now().UnixNano()
	rng := rand.New(rand.NewSource(seed))
	latency := 5 + rng.Intn(50)
	
	return wasm.Response{
		Status: "success",
		Data: map[string]string{
			"target":     target,
			"latency_ms": fmt.Sprintf("%d", latency),
			"status":     "reachable",
		},
	}
}

func checkBandwidth(req wasm.Request) wasm.Response {
	interface_name := req.Args["interface"]
	if interface_name == "" {
		interface_name = "eth0"
	}
	
	seed := time.Now().UnixNano()
	rng := rand.New(rand.NewSource(seed))
	
	rxMbps := 50 + rng.Intn(450)
	txMbps := 20 + rng.Intn(180)
	
	return wasm.Response{
		Status: "success",
		Data: map[string]string{
			"interface":    interface_name,
			"rx_bandwidth": fmt.Sprintf("%d Mbps", rxMbps),
			"tx_bandwidth": fmt.Sprintf("%d Mbps", txMbps),
			"utilization":  fmt.Sprintf("%d%%", (rxMbps+txMbps)/10),
		},
	}
}

func pluginInfo() wasm.Response {
	return wasm.Response{
		Status: "success",
		Data: map[string]string{
			"name":        "Network Monitor WASM Plugin",
			"version":     "1.0.0",
			"description": "Advanced network monitoring via WebAssembly",
			"actions":     "collect, check_latency, check_bandwidth, info",
			"author":      "Nord Team",
		},
	}
}
