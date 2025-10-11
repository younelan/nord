package collection

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"observer/base"
	"observer/plugins"
	"strings"
)

// --- Plugin Implementation ---

// collectionPlugin orchestrates data collection from other plugins.
type collectionPlugin struct {
	plugin.BasePlugin
	config *plugin.Config
}

func init() {
	plugins.Register(&collectionPlugin{})
}

// Name returns the plugin's name.
func (p *collectionPlugin) Name() string {
	return "Collection"
}

// OnCommand handles the primary "collect" action.
func (p *collectionPlugin) OnCommand(args map[string]string) error {
	action, ok := args["action"]
	if !ok || action != "collect" {
		return fmt.Errorf("unknown action for Collection plugin: %v", args)
	}

	fmt.Println("-- Running Data Collection --")
	return p.collectData()
}

// loadConfig reads and parses the config.json file.
func (p *collectionPlugin) loadConfig() error {
	configFile, err := ioutil.ReadFile("data/config.json")
	if err != nil {
		return fmt.Errorf("could not read config file: %w", err)
	}

	var config plugin.Config
	err = json.Unmarshal(configFile, &config)
	if err != nil {
		return fmt.Errorf("could not parse config file: %w", err)
	}
	p.config = &config
	return nil
}

// collectData mimics the logic from the PHP on_collect method.
func (p *collectionPlugin) collectData() error {
	if err := p.loadConfig(); err != nil {
		return err
	}

	// --- Load and merge hosts from perception.json ---
	type PerceptionData struct {
		Hosts map[string]plugin.Host `json:"hosts"`
	}
	perceptionFile, err := ioutil.ReadFile("data/perception.json")
	if err == nil {
		var perceptionData PerceptionData
		if json.Unmarshal(perceptionFile, &perceptionData) == nil {
			fmt.Println("    |_ Merging hosts from perception.json")
			for ip, host := range perceptionData.Hosts {
				if _, exists := p.config.Hosts[ip]; !exists {
					p.config.Hosts[ip] = host // Add the host if it doesn't already exist
				}
			}
		}
	} else {
		fmt.Println("    |_ perception.json not found, skipping merge.")
	}
	// --- End merge ---

	finalResults := make(map[string]interface{})

	for hostName, host := range p.config.Hosts {
		fmt.Printf("  |_ %s (%s)\n", hostName, host.Address)

		hostMetrics := make(map[string]interface{})

		for _, task := range host.Collect {
			parts := strings.Split(task.Metric, ".")
			var pluginName, action string
			if len(parts) >= 2 {
				pluginName = parts[0]
				action = parts[1]
			} else {
				pluginName = parts[0]
				action = "collect" // Default action
			}

			fmt.Printf("     |_ %s : %s\n", pluginName, action)

			targetPlugin, exists := p.Controller.Plugins[pluginName]
			if !exists {
				fmt.Printf("          !_ Plugin '%s' not found.\n", pluginName)
				continue
			}

			// Prepare options for the plugin call
			pluginOptions := map[string]interface{}{
				"host":   map[string]interface{}{"address": host.Address},
				"action": action,
			}

			// Add credentials if specified for the task
			if task.Credentials != "" {
				if cred, ok := p.config.Credentials[task.Credentials]; ok {
					pluginOptions["credentials"] = map[string]interface{}{
						"user": cred.User,
						"pass": cred.Pass,
						"host": cred.Host,
						"port": fmt.Sprintf("%d", cred.Port),
					}
				} else {
					fmt.Printf("          !_ Credentials '%s' not found.\n", task.Credentials)
				}
			}

			result, err := targetPlugin.OnCollect(pluginOptions)
			if err != nil {
				fmt.Printf("          !_ Error: %v\n", err)
				continue
			}

			if result != nil {
				// Merge results
				for k, v := range result {
					hostMetrics[k] = v
				}
			}
		}
		finalResults[hostName] = map[string]interface{}{"metrics": hostMetrics}
	}

	jsonData, err := json.MarshalIndent(finalResults, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal results to JSON: %w", err)
	}

	err = ioutil.WriteFile("data/collection.json", jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write collection.json: %w", err)
	}

	fmt.Println("--- Collection finished, results saved to collection.json ---")
	return nil
}
