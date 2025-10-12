
package plugins

import (
	plugin "observer/base"
)

// All is a list of all registered plugins.
var All []plugin.Plugin

// Register adds a plugin to the list of available plugins.
func Register(p plugin.Plugin) {
	All = append(All, p)
}
