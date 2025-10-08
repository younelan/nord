
package plugin

import (
	"fmt"
	"strings"
)

// Plugin is the interface that all plugins must implement.
type Plugin interface {
	Name() string
	Init(controller *Controller)
	OnCommand(args map[string]string) error
	OnUpdate() error
	// OnCollect is the new interface for collection plugins
	OnCollect(options map[string]interface{}) (map[string]interface{}, error)
}

// BasePlugin is a helper struct that plugins can embed for default functionality.
type BasePlugin struct {
	Controller *Controller
}

// Name returns the plugin name. This should be overridden by the embedding plugin.
func (p *BasePlugin) Name() string {
	return "BasePlugin"
}

// Init stores a reference to the controller.
func (p *BasePlugin) Init(c *Controller) {
	p.Controller = c
}

// OnCommand is the default command handler.
func (p *BasePlugin) OnCommand(args map[string]string) error {
	return fmt.Errorf("OnCommand not implemented")
}

// OnUpdate is the default update handler.
func (p *BasePlugin) OnUpdate() error {
	return fmt.Errorf("OnUpdate not implemented")
}

// OnCollect is the default collect handler.
func (p *BasePlugin) OnCollect(options map[string]interface{}) (map[string]interface{}, error) {
	return nil, fmt.Errorf("OnCollect not implemented")
}

// Controller manages all the registered plugins.
type Controller struct {
	Plugins map[string]Plugin
}

// NewController creates and returns a new Controller.
func NewController() *Controller {
	return &Controller{
		Plugins: make(map[string]Plugin),
	}
}

// AddPlugin registers a new plugin with the controller.
func (c *Controller) AddPlugin(p Plugin) {
	name := strings.ToLower(p.Name())
	c.Plugins[name] = p
	p.Init(c)
}

// OnCommand dispatches a command to the specified plugin.
func (c *Controller) OnCommand(pluginName string, args map[string]string) error {
	plugin, exists := c.Plugins[strings.ToLower(pluginName)]
	if !exists {
		return fmt.Errorf("plugin '%s' not found", pluginName)
	}
	return plugin.OnCommand(args)
}
