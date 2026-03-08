package wasm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// PluginEngine manages the lifecycle and execution of WebAssembly plugins
type PluginEngine struct {
	runtime wazero.Runtime
	modules map[string]wazero.CompiledModule
	mu      sync.RWMutex
}

// Request matches the Guest's Request struct.
type Request struct {
	Action string            `json:"action"`
	Args   map[string]string `json:"args"`
}

// Response matches the Guest's Response struct.
type Response struct {
	Status string            `json:"status"`
	Error  string            `json:"error,omitempty"`
	Data   map[string]string `json:"data,omitempty"`
}

// NewEngine initializes the Wazero WebAssembly runtime
func NewEngine(ctx context.Context) *PluginEngine {
	cacheDir := filepath.Join(os.TempDir(), "nord-wasm-cache")
	_ = os.MkdirAll(cacheDir, 0755)

	cache, err := wazero.NewCompilationCacheWithDir(cacheDir)
	if err != nil {
		log.Printf("Warning: Could not create WASM compilation cache: %v", err)
	}

	config := wazero.NewRuntimeConfig().WithCompilationCache(cache)
	rt := wazero.NewRuntimeWithConfig(ctx, config)

	// WASI (WebAssembly System Interface) provides standard primitives.
	wasi_snapshot_preview1.MustInstantiate(ctx, rt)

	return &PluginEngine{
		runtime: rt,
		modules: make(map[string]wazero.CompiledModule),
	}
}

// Load compiles a WebAssembly binary (.wasm) into native machine code.
func (e *PluginEngine) Load(ctx context.Context, pluginName string, wasmBytes []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	hash := sha256.Sum256(wasmBytes)
	log.Printf("Loading WASM Plugin: %s (SHA256: %s)", pluginName, hex.EncodeToString(hash[:8]))

	compiled, err := e.runtime.CompileModule(ctx, wasmBytes)
	if err != nil {
		return fmt.Errorf("failed to compile WASM module %s: %w", pluginName, err)
	}

	e.modules[pluginName] = compiled
	return nil
}

// Execute handles the full lifecycle of a standard Go WASM plugin call
func (e *PluginEngine) Execute(ctx context.Context, pluginName string, req Request) (*Response, error) {
	e.mu.RLock()
	compiled, ok := e.modules[pluginName]
	e.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("wasm plugin %q not loaded", pluginName)
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// 1. Instantiate the Module (without calling _start)
	var stdout, stderr bytes.Buffer
	config := wazero.NewModuleConfig().
		WithName(fmt.Sprintf("%s-%d", pluginName, time.Now().UnixNano())).
		WithStdout(&stdout).
		WithStderr(&stderr).
		WithStartFunctions() // Don't auto-call _start

	mod, err := e.runtime.InstantiateModule(ctx, compiled, config)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate WASM module %q: %w", pluginName, err)
	}
	defer mod.Close(ctx)

	// 2. ABI functions
	malloc := mod.ExportedFunction("malloc")
	execute := mod.ExportedFunction("execute")

	if malloc == nil || execute == nil {
		return nil, fmt.Errorf("plugin %q missing ABI exports (malloc or execute)", pluginName)
	}

	// 5. Allocate and write Request
	reqSize := uint64(len(reqBytes))
	results, err := malloc.Call(ctx, reqSize)
	if err != nil {
		return nil, fmt.Errorf("malloc failed: %w", err)
	}
	reqPtr := uint32(results[0])

	if !mod.Memory().Write(reqPtr, reqBytes) {
		return nil, fmt.Errorf("failed to write request bytes")
	}

	// 6. Execute!
	execResults, err := execute.Call(ctx, uint64(reqPtr), reqSize)
	if err != nil {
		return nil, fmt.Errorf("WASM execution failed: %w\nStderr: %s", err, stderr.String())
	}

	packedResult := execResults[0]
	respPtr := uint32(packedResult >> 32)
	respSize := uint32(packedResult)

	// 7. Read and unmarshal Response
	respBytes, ok := mod.Memory().Read(respPtr, respSize)
	if !ok {
		return nil, fmt.Errorf("failed to read response bytes")
	}

	var resp Response
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("invalid guest response JSON: %s", string(respBytes))
	}

	return &resp, nil
}

func (e *PluginEngine) NewModuleConfig() wazero.ModuleConfig { return wazero.NewModuleConfig() }
func (e *PluginEngine) GetRuntime() wazero.Runtime           { return e.runtime }
func (e *PluginEngine) Close(ctx context.Context) error      { return e.runtime.Close(ctx) }

func (e *PluginEngine) InstantiateModule(ctx context.Context, pluginName string, config wazero.ModuleConfig) (api.Module, error) {
	e.mu.RLock()
	compiled, ok := e.modules[pluginName]
	e.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("not loaded")
	}
	return e.runtime.InstantiateModule(ctx, compiled, config)
}
