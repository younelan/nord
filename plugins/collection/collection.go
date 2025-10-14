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
	config           *plugin.Config
	rawCollect       map[string][]map[string]interface{} // normalized collect per host (fallback by key)
	rawCollectByAddr map[string][]map[string]interface{} // normalized collect per address (fallback by address)
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
	// Read raw config file
	configFile, err := ioutil.ReadFile("data/config.json")
	if err != nil {
		return fmt.Errorf("could not read config file: %w", err)
	}

	// Unmarshal into generic map to normalize hosts.collect
	var rawConfig map[string]interface{}
	if err := json.Unmarshal(configFile, &rawConfig); err != nil {
		return fmt.Errorf("could not parse config file: %w", err)
	}

	// Initialize fallback caches
	p.rawCollect = make(map[string][]map[string]interface{})
	p.rawCollectByAddr = make(map[string][]map[string]interface{})

	// Normalize hosts[].collect to array of objects and cache by host and address
	if hosts, ok := rawConfig["hosts"].(map[string]interface{}); ok {
		for hostName, hv := range hosts {
			hostMap, ok := hv.(map[string]interface{})
			if !ok {
				continue
			}
			coll, exists := hostMap["collect"]
			if !exists || coll == nil {
				continue
			}

			var normalized []interface{}
			switch v := coll.(type) {
			case string:
				for _, item := range strings.Split(v, ",") {
					item = strings.TrimSpace(item)
					if item == "" {
						continue
					}
					fields := strings.Fields(item)
					entry := map[string]interface{}{"metric": strings.TrimSpace(fields[0])}
					if len(fields) >= 2 {
						entry["credentials"] = strings.TrimSpace(fields[1])
					}
					normalized = append(normalized, entry)
				}
			case []interface{}:
				for _, it := range v {
					if s, ok := it.(string); ok {
						fields := strings.Fields(strings.TrimSpace(s))
						if len(fields) == 0 {
							continue
						}
						entry := map[string]interface{}{"metric": fields[0]}
						if len(fields) >= 2 {
							entry["credentials"] = fields[1]
						}
						normalized = append(normalized, entry)
						continue
					}
					if m, ok := it.(map[string]interface{}); ok {
						if ms, ok := m["metric"].(string); ok {
							m["metric"] = strings.TrimSpace(ms)
						}
						normalized = append(normalized, m)
					}
				}
			default:
				// unsupported type -> skip normalization
			}

			if len(normalized) > 0 {
				hostMap["collect"] = normalized

				// cache by host key
				var cached []map[string]interface{}
				for _, it := range normalized {
					if mm, ok := it.(map[string]interface{}); ok {
						cached = append(cached, mm)
					}
				}
				if len(cached) > 0 {
					p.rawCollect[hostName] = cached
					// cache by address too
					if addr, ok := hostMap["address"].(string); ok && strings.TrimSpace(addr) != "" {
						p.rawCollectByAddr[strings.TrimSpace(addr)] = cached
					}
				}
			}
		}
	}

	// Re-marshal normalized raw config and unmarshal into typed config
	normalizedJSON, err := json.Marshal(rawConfig)
	if err != nil {
		return fmt.Errorf("could not re-marshal normalized config: %w", err)
	}

	var config plugin.Config
	if err := json.Unmarshal(normalizedJSON, &config); err != nil {
		return fmt.Errorf("could not parse config file: %w", err)
	}

	p.config = &config
	return nil
}

// collectTask handles a single task (check) for a host
func (p *collectionPlugin) collectTask(hostName string, host plugin.Host, task plugin.CollectTask, taskResultsChan chan<- map[string]interface{}, wg *sync.WaitGroup) {
	defer wg.Done()

	metric := strings.TrimSpace(task.Metric)
	if metric == "" {
		return
	}
	parts := strings.Split(metric, ".")
	var pluginName, action string
	if len(parts) >= 2 {
		pluginName = strings.TrimSpace(parts[0])
		action = strings.TrimSpace(parts[1])
	} else {
		pluginName = strings.TrimSpace(parts[0])
		action = "all" // PHP default
	}

	// Prefix each check with the host name
	fmt.Printf("  |_ %s : %s.%s\n", hostName, pluginName, action)

	// Try lowercase key for lookup robustness
	pluginKey := strings.ToLower(pluginName)
	targetPlugin, exists := p.Controller.Plugins[pluginKey]
	if !exists {
		// Include host for clarity
		fmt.Printf("  !_ %s: Plugin '%s' not found.\n", hostName, pluginName)
		return
	}

	// Build full host map (mirror PHP passing the whole host)
	hostMap := map[string]interface{}{}
	if b, err := json.Marshal(host); err == nil {
		_ = json.Unmarshal(b, &hostMap)
	} else {
		hostMap = map[string]interface{}{"address": host.Address}
	}

	// Prepare options for the plugin call, include collection like PHP
	pluginOptions := map[string]interface{}{
		"host":   hostMap,
		"action": action,
		"collection": map[string]interface{}{
			"metric": metric,
		},
	}

	// Add credentials if specified for the task
	if c := strings.TrimSpace(task.Credentials); c != "" {
		pluginOptions["collection"].(map[string]interface{})["credentials"] = c
		if cred, ok := p.config.Credentials[c]; ok {
			pluginOptions["credentials"] = map[string]interface{}{
				"user": cred.User,
				"pass": cred.Pass,
				"host": cred.Host,
				"port": fmt.Sprintf("%d", cred.Port),
				"type": cred.Type,
			}
		} else {
			// Include host for clarity
			fmt.Printf("          !_ %s | Credentials '%s' not found.\n", hostName, c)
		}
	}

	result, err := targetPlugin.OnCollect(pluginOptions)
	if err != nil {
		// Include host for clarity
		fmt.Printf("          !_ %s | Error: %v\n", hostName, err)
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

	// Build task list; start with typed then merge any missing from rawCollect (by host), then by address
	tasks := make([]plugin.CollectTask, 0, len(host.Collect))
	metricsSet := map[string]struct{}{}

	for _, t := range host.Collect {
		m := strings.TrimSpace(t.Metric)
		if m == "" {
			continue
		}
		t.Metric = m
		tasks = append(tasks, t)
		metricsSet[m] = struct{}{}
	}

	if raw, ok := p.rawCollect[hostName]; ok && len(raw) > 0 {
		for _, mm := range raw {
			m, _ := mm["metric"].(string)
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			if _, seen := metricsSet[m]; seen {
				continue
			}
			ct := plugin.CollectTask{Metric: m}
			if c, ok := mm["credentials"].(string); ok {
				ct.Credentials = strings.TrimSpace(c)
			}
			tasks = append(tasks, ct)
			metricsSet[m] = struct{}{}
		}
	}

	// If still empty or incomplete, merge by address as well
	if raw, ok := p.rawCollectByAddr[strings.TrimSpace(host.Address)]; ok && len(raw) > 0 {
		for _, mm := range raw {
			m, _ := mm["metric"].(string)
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			if _, seen := metricsSet[m]; seen {
				continue
			}
			ct := plugin.CollectTask{Metric: m}
			if c, ok := mm["credentials"].(string); ok {
				ct.Credentials = strings.TrimSpace(c)
			}
			tasks = append(tasks, ct)
			metricsSet[m] = struct{}{}
		}
	}

	// Create WaitGroup and results channel for tasks
	var taskWg sync.WaitGroup
	taskResultsChan := make(chan map[string]interface{}, len(tasks))

	// Start goroutines for each task
	for _, task := range tasks {
		taskWg.Add(1)
		go p.collectTask(hostName, host, task, taskResultsChan, &taskWg)
	}

	// Wait for all task goroutines to complete
	taskWg.Wait()
	close(taskResultsChan)

	// Collect results from tasks (flatten metrics)
	for taskResult := range taskResultsChan {
		// Only merge the inner metrics map entries
		if metricsAny, ok := taskResult["metrics"]; ok {
			if metricsMap, ok := metricsAny.(map[string]interface{}); ok {
				for label, metric := range metricsMap {
					hostMetrics[label] = metric
				}
			}
		}
		// Ignore other keys (e.g., collections) for now to match lincol.json shape
	}

	// Send result to channel (nest metrics under metrics.metrics)
	resultsChan <- map[string]interface{}{
		hostName: map[string]interface{}{
			"metrics": map[string]interface{}{
				"metrics": hostMetrics,
			},
		},
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
			fmt.Println(". |_ Merging hosts from perception.json")
			for ip, host := range perceptionData.Hosts {
				if _, exists := p.config.Hosts[ip]; !exists {
					p.config.Hosts[ip] = host // Add the host if it doesn't already exist
				}
			}
		}
	} else {
		fmt.Println("  |_ perception.json not found, skipping merge.")
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
