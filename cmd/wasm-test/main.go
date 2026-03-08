package main

import (
	"context"
	"fmt"
	"log"
	"observer/plugins/wasm"
	"os"
)

func main() {
	ctx := context.Background()
	engine := wasm.NewEngine(ctx)
	defer engine.Close(ctx)

	wasmPath := "examples/wasm-plugin/main.wasm"
	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		log.Fatalf("Failed to read wasm file: %v", err)
	}

	err = engine.Load(ctx, "test-plugin", wasmBytes)
	if err != nil {
		log.Fatalf("Failed to load plugin: %v", err)
	}

	fmt.Println("WASM Plugin loaded correctly!")

	// 1. Manually instantiate but prevent the auto-run of _start
	config := engine.NewModuleConfig().WithStartFunctions()
	mod, err := engine.InstantiateModule(ctx, "test-plugin", config)
	if err != nil {
		log.Fatalf("Instantiate failed: %v", err)
	}
	defer mod.Close(ctx)

	// 2. NO START CALL!
	fmt.Println("Bypassing _start...")

	// 3. Now try to call add
	add := mod.ExportedFunction("add")
	if add == nil {
		log.Fatalf("add not found")
	}

	results, err := add.Call(ctx, 10, 20)
	if err != nil {
		log.Fatalf("add failed: %v", err)
	}

	fmt.Printf("ABI Success! 10 + 20 = %v\n", results[0])
}
