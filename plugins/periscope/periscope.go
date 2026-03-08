package periscope

import (
	plugin "observer/base"
	"observer/plugins"
)

type periscopePlugin struct {
	plugin.BasePlugin
}

func init() {
	plugins.Register(&periscopePlugin{})
}

func (p *periscopePlugin) Name() string {
	return "Periscope"
}

func (p *periscopePlugin) GetMenus() map[string]plugin.MenuItem {
	return map[string]plugin.MenuItem{
		"periscope": {
			Text:   "Periscope",
			Weight: -5, // Negative weight to appear first
			Children: map[string]plugin.MenuItem{
				"logoff": {
					Plugin: "periscope",
					Action: "logoff",
					Text:   "Logoff",
				},
				"security": {
					Plugin: "periscope",
					Page:   "security",
					URL:    "editpasswd.php",
					Text:   "Security",
				},
			},
		},
	}
}
