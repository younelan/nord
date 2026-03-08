package wasm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	plugin "observer/base"
	"observer/plugins"
)

// wasmPlugin manages and executes WebAssembly plugins
type wasmPlugin struct {
	plugin.BasePlugin
	engine      *PluginEngine
	ctx         context.Context
	pluginsDir  string
	loadedPlugins map[string]bool
}

func init() {
	plugins.Register(&wasmPlugin{
		loadedPlugins: make(map[string]bool),
	})
}

func (p *wasmPlugin) Name() string {
	return "wasm"
}

func (p *wasmPlugin) Init(controller *plugin.Controller) {
	p.BasePlugin.Init(controller)
	p.ctx = context.Background()
	p.engine = NewEngine(p.ctx)
	
	// Default plugins directory
	p.pluginsDir = "plugins/wasm/modules"
	
	// Load configuration if available
	if cfgData, err := os.ReadFile("data/config.json"); err == nil {
		var cfg struct {
			Wasm struct {
				PluginsDir string `json:"plugins_dir"`
				Enabled    bool   `json:"enabled"`
			} `json:"wasm"`
		}
		if json.Unmarshal(cfgData, &cfg) == nil && cfg.Wasm.PluginsDir != "" {
			p.pluginsDir = cfg.Wasm.PluginsDir
		}
	}
	
	// Auto-load all WASM plugins from directory
	p.loadPluginsFromDirectory()
}

// loadPluginsFromDirectory scans the plugins directory and loads all .wasm files
func (p *wasmPlugin) loadPluginsFromDirectory() {
	if _, err := os.Stat(p.pluginsDir); os.IsNotExist(err) {
		log.Printf("WASM plugins directory does not exist: %s", p.pluginsDir)
		return
	}
	
	entries, err := os.ReadDir(p.pluginsDir)
	if err != nil {
		log.Printf("Failed to read WASM plugins directory: %v", err)
		return
	}
	
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".wasm") {
			continue
		}
		
		pluginPath := filepath.Join(p.pluginsDir, entry.Name())
		pluginName := strings.TrimSuffix(entry.Name(), ".wasm")
		
		wasmBytes, err := os.ReadFile(pluginPath)
		if err != nil {
			log.Printf("Failed to read WASM plugin %s: %v", pluginName, err)
			continue
		}
		
		if err := p.engine.Load(p.ctx, pluginName, wasmBytes); err != nil {
			log.Printf("Failed to load WASM plugin %s: %v", pluginName, err)
			continue
		}
		
		p.loadedPlugins[pluginName] = true
		log.Printf("Loaded WASM plugin: %s", pluginName)
	}
}

func (p *wasmPlugin) OnCommand(args map[string]string) error {
	action := args["action"]
	
	switch action {
	case "list":
		return p.listPlugins()
	case "load":
		pluginPath := args["args"]
		if pluginPath == "" {
			return fmt.Errorf("path argument required")
		}
		return p.loadPlugin(pluginPath)
	case "execute":
		// Parse additional arguments from the args string
		argsStr := args["args"]
		execArgs := parseArgs(argsStr)
		
		pluginName := execArgs["plugin"]
		if pluginName == "" {
			return fmt.Errorf("plugin argument required (use: plugin=name req_action=action)")
		}
		reqAction := execArgs["req_action"]
		if reqAction == "" {
			reqAction = "ping"
		}
		return p.executePlugin(pluginName, reqAction, execArgs)
	case "reload":
		return p.reloadPlugins()
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

// parseArgs parses space-separated key=value pairs
func parseArgs(argsStr string) map[string]string {
	result := make(map[string]string)
	if argsStr == "" {
		return result
	}
	
	parts := strings.Fields(argsStr)
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		}
	}
	return result
}

func (p *wasmPlugin) listPlugins() error {
	fmt.Println("Loaded WASM Plugins:")
	if len(p.loadedPlugins) == 0 {
		fmt.Println("  (none)")
		return nil
	}
	for name := range p.loadedPlugins {
		fmt.Printf("  - %s\n", name)
	}
	return nil
}

func (p *wasmPlugin) loadPlugin(pluginPath string) error {
	wasmBytes, err := os.ReadFile(pluginPath)
	if err != nil {
		return fmt.Errorf("failed to read plugin: %w", err)
	}
	
	pluginName := strings.TrimSuffix(filepath.Base(pluginPath), ".wasm")
	
	if err := p.engine.Load(p.ctx, pluginName, wasmBytes); err != nil {
		return fmt.Errorf("failed to load plugin: %w", err)
	}
	
	p.loadedPlugins[pluginName] = true
	fmt.Printf("Successfully loaded WASM plugin: %s\n", pluginName)
	return nil
}

func (p *wasmPlugin) executePlugin(pluginName, action string, args map[string]string) error {
	if !p.loadedPlugins[pluginName] {
		return fmt.Errorf("plugin %s not loaded", pluginName)
	}
	
	// Build request arguments (exclude internal args)
	reqArgs := make(map[string]string)
	for k, v := range args {
		if k != "action" && k != "plugin" && k != "req_action" {
			reqArgs[k] = v
		}
	}
	
	req := Request{
		Action: action,
		Args:   reqArgs,
	}
	
	resp, err := p.engine.Execute(p.ctx, pluginName, req)
	if err != nil {
		return fmt.Errorf("execution failed: %w", err)
	}
	
	fmt.Printf("Plugin Response:\n")
	fmt.Printf("  Status: %s\n", resp.Status)
	if resp.Error != "" {
		fmt.Printf("  Error: %s\n", resp.Error)
	}
	if len(resp.Data) > 0 {
		fmt.Printf("  Data:\n")
		for k, v := range resp.Data {
			fmt.Printf("    %s: %s\n", k, v)
		}
	}
	
	return nil
}

func (p *wasmPlugin) reloadPlugins() error {
	// Clear loaded plugins
	p.loadedPlugins = make(map[string]bool)
	
	// Reload from directory
	p.loadPluginsFromDirectory()
	
	fmt.Printf("Reloaded %d WASM plugins\n", len(p.loadedPlugins))
	return nil
}

func (p *wasmPlugin) OnUpdate() error {
	return fmt.Errorf("OnUpdate not implemented for WASM plugin")
}

func (p *wasmPlugin) OnCollect(options map[string]interface{}) (map[string]interface{}, error) {
	// Extract plugin name from options
	pluginName, ok := options["wasm_plugin"].(string)
	if !ok {
		return nil, fmt.Errorf("wasm_plugin name not specified in options")
	}
	
	if !p.loadedPlugins[pluginName] {
		return nil, fmt.Errorf("WASM plugin %s not loaded", pluginName)
	}
	
	// Extract action (default to "collect")
	action := "collect"
	if a, ok := options["wasm_action"].(string); ok {
		action = a
	}
	
	// Build request arguments from options
	reqArgs := make(map[string]string)
	for k, v := range options {
		if k != "wasm_plugin" && k != "wasm_action" {
			if str, ok := v.(string); ok {
				reqArgs[k] = str
			}
		}
	}
	
	req := Request{
		Action: action,
		Args:   reqArgs,
	}
	
	resp, err := p.engine.Execute(p.ctx, pluginName, req)
	if err != nil {
		return nil, fmt.Errorf("WASM execution failed: %w", err)
	}
	
	if resp.Status != "success" {
		return nil, fmt.Errorf("WASM plugin error: %s", resp.Error)
	}
	
	// Convert response data to map[string]interface{}
	result := make(map[string]interface{})
	for k, v := range resp.Data {
		result[k] = v
	}
	
	return result, nil
}
