package wasm

import (
	"encoding/json"
	"unsafe"
)

// Request matches the Host's Request struct.
type Request struct {
	Action string            `json:"action"`
	Args   map[string]string `json:"args"`
}

// Response matches the Host's Response struct.
type Response struct {
	Status string            `json:"status"` // "success" or "error"
	Error  string            `json:"error,omitempty"`
	Data   map[string]string `json:"data,omitempty"`
}

// Handler is the function signature for plugins.
type Handler func(req Request) Response

var pluginHandler Handler

// Register binds the plugin implementation.
func Register(h Handler) {
	pluginHandler = h
}

// =====================================================================
// WASM ABI for TinyGo
// TinyGo already exports malloc, so we only need to export execute
// =====================================================================

//export execute
func execute(reqPtr uint32, reqSize uint32) uint64 {
	reqBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(reqPtr))), reqSize)

	var req Request
	var resp Response

	if err := json.Unmarshal(reqBytes, &req); err != nil {
		resp = Response{Status: "error", Error: "JSON Unmarshal: " + err.Error()}
	} else if pluginHandler == nil {
		resp = Response{Status: "error", Error: "Plugin not registered"}
	} else {
		resp = pluginHandler(req)
	}

	respBytes, _ := json.Marshal(resp)
	
	// Allocate memory for response
	respPtr := uint32(uintptr(unsafe.Pointer(&respBytes[0])))
	respSize := uint32(len(respBytes))

	// Pack pointer and size into uint64
	return (uint64(respPtr) << 32) | uint64(respSize)
}
