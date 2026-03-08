package main

import (
	"encoding/json"
	"time"
	"unsafe"
)

// =====================================================================
// Custom Persistent Allocator (No Go Runtime Heap dependency)
// =====================================================================

var heap [1024 * 256]byte // 256KB of fixed-size buffer
var heapOffset int

//go:wasmexport malloc
func malloc(size uint64) uint64 {
	if heapOffset+int(size) > len(heap) {
		return 0
	}
	ptr := uint64(uintptr(unsafe.Pointer(&heap[heapOffset])))
	heapOffset += int(size)
	return ptr
}

//go:wasmexport execute
func execute(reqPtr uint64, reqSize uint64) uint64 {
	reqBytes := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(reqPtr))), reqSize)

	var req struct {
		Action string            `json:"action"`
		Args   map[string]string `json:"args"`
	}

	if err := json.Unmarshal(reqBytes, &req); err != nil {
		return 0
	}

	res := "verified_success"
	if req.Action == "ping" {
		res = "pong from " + req.Args["target"]
	}

	resp := struct {
		Status string            `json:"status"`
		Data   map[string]string `json:"data"`
	}{
		Status: "success",
		Data:   map[string]string{"result": res},
	}

	respBytes, _ := json.Marshal(resp)

	// Write response to the start of the heap for the Host to read
	copy(heap[:], respBytes)

	ptr := uint64(uintptr(unsafe.Pointer(&heap[0])))
	size := uint64(len(respBytes))

	return (ptr << 32) | size
}

func main() {
	for {
		time.Sleep(time.Hour)
	}
}
