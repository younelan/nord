package collection

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"observer/base"
	"observer/plugins"
	"strings"
	"sync"
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

// collectTask handles a single task (check) for a host
func (p *collectionPlugin) collectTask(hostName string, host plugin.Host, task struct {
	Metric      string `json:"metric"`
	Credentials string `json:"credentials"`
}, taskResultsChan chan<- map[string]interface{}, wg *sync.WaitGroup) {
	defer wg.Done()

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
		return
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
		return
	}

	if result != nil {
		taskResultsChan <- result
	}
}

// collectHost handles data collection for a single host
func (p *collectionPlugin) collectHost(hostName string, host plugin.Host, resultsChan chan<- map[string]interface{}, wg *sync.WaitGroup) {
	defer wg.Done()

	fmt.Printf("  |_ %s (%s)\n", hostName, host.Address)

	hostMetrics := make(map[string]interface{})

	// Create WaitGroup and results channel for tasks
	var taskWg sync.WaitGroup
	taskResultsChan := make(chan map[string]interface{}, len(host.Collect))

	// Start goroutines for each task
	for _, task := range host.Collect {
		taskWg.Add(1)
		go p.collectTask(hostName, host, task, taskResultsChan, &taskWg)
	}

	// Wait for all task goroutines to complete
	taskWg.Wait()
	close(taskResultsChan)

	// Collect results from tasks
	for taskResult := range taskResultsChan {
		// Merge results
		for k, v := range taskResult {
			hostMetrics[k] = v
		}
	}

	// Send result to channel
	resultsChan <- map[string]interface{}{
		hostName: map[string]interface{}{"metrics": hostMetrics},
	}
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

	// Create WaitGroup and results channel
	var wg sync.WaitGroup
	resultsChan := make(chan map[string]interface{}, len(p.config.Hosts))

	// Start goroutines for each host
	for hostName, host := range p.config.Hosts {
		wg.Add(1)
		go p.collectHost(hostName, host, resultsChan, &wg)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(resultsChan)

	// Collect results from channel
	for hostResult := range resultsChan {
		for hostName, metrics := range hostResult {
			finalResults[hostName] = metrics
		}
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
