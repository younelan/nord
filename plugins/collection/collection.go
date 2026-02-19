package collection

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"observer/base"
	"observer/plugins"
	snmpplugin "observer/plugins/snmp"
	"observer/store"
	"strings"
	"sync"
	"time"
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

// collectTask handles a single task (check) for a host.
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
		action = "all"
	}

	fmt.Printf("  |_ %s : %s.%s\n", hostName, pluginName, action)

	pluginKey := strings.ToLower(pluginName)
	targetPlugin, exists := p.Controller.Plugins[pluginKey]
	if !exists {
		fmt.Printf("  !_ %s: Plugin '%s' not found.\n", hostName, pluginName)
		return
	}

	hostMap := map[string]interface{}{}
	if b, err := json.Marshal(host); err == nil {
		_ = json.Unmarshal(b, &hostMap)
	} else {
		hostMap = map[string]interface{}{"address": host.Address}
	}

	pluginOptions := map[string]interface{}{
		"host":   hostMap,
		"action": action,
		"collection": map[string]interface{}{
			"metric": metric,
		},
	}

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
			fmt.Printf("          !_ %s | Credentials '%s' not found.\n", hostName, c)
		}
	}

	result, err := targetPlugin.OnCollect(pluginOptions)
	if err != nil {
		fmt.Printf("          !_ %s | Error: %v\n", hostName, err)
		return
	}

	if result != nil {
		// Tag the result with the plugin name so the store writer can record it.
		result["__plugin"] = pluginName
		taskResultsChan <- result
	}
}

// collectHost handles data collection for a single host.
func (p *collectionPlugin) collectHost(hostName string, host plugin.Host, resultsChan chan<- map[string]interface{}, wg *sync.WaitGroup) {
	defer wg.Done()

	fmt.Printf("  |_ %s (%s)\n", hostName, host.Address)

	hostMetrics := make(map[string]interface{})

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

	var taskWg sync.WaitGroup
	taskResultsChan := make(chan map[string]interface{}, len(tasks))

	for _, task := range tasks {
		taskWg.Add(1)
		go p.collectTask(hostName, host, task, taskResultsChan, &taskWg)
	}

	taskWg.Wait()
	close(taskResultsChan)

	var hostInterfaces []map[string]interface{}

	for taskResult := range taskResultsChan {
		pluginTag, _ := taskResult["__plugin"].(string)

		if metricsAny, ok := taskResult["metrics"]; ok {
			if metricsMap, ok := metricsAny.(map[string]interface{}); ok {
				for label, metric := range metricsMap {
					// Propagate plugin name into each metric map for store writer.
					if pluginTag != "" {
						if m, ok := metric.(map[string]interface{}); ok {
							m["__plugin"] = pluginTag
						}
					}
					hostMetrics[label] = metric
				}
			}
		}

		// Collect interface entity data returned by SNMP table walks.
		if ifacesAny, ok := taskResult["interfaces"]; ok {
			if ifaces, ok := ifacesAny.([]map[string]interface{}); ok {
				hostInterfaces = append(hostInterfaces, ifaces...)
			}
		}
	}

	resultsChan <- map[string]interface{}{
		hostName: map[string]interface{}{
			"metrics": map[string]interface{}{
				"metrics": hostMetrics,
			},
			"__interfaces": hostInterfaces,
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
					p.config.Hosts[ip] = host
				}
			}
		}
	} else {
		fmt.Println("  |_ perception.json not found, skipping merge.")
	}

	finalResults := make(map[string]interface{})

	var wg sync.WaitGroup
	resultsChan := make(chan map[string]interface{}, len(p.config.Hosts))

	for hostName, host := range p.config.Hosts {
		wg.Add(1)
		go p.collectHost(hostName, host, resultsChan, &wg)
	}

	wg.Wait()
	close(resultsChan)

	for hostResult := range resultsChan {
		for hostName, metrics := range hostResult {
			finalResults[hostName] = metrics
		}
	}

	// --- Write to store ---
	if p.Controller.Store != nil {
		p.writeToStore(finalResults)
	}

	// --- Strip internal tags and write JSON ---
	p.stripInternalTags(finalResults)

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

// writeToStore builds MetricRecords and InterfaceRecords from finalResults and persists them.
func (p *collectionPlugin) writeToStore(finalResults map[string]interface{}) {
	now := time.Now()
	var metricRecords []store.MetricRecord
	var ifaceRecords []store.InterfaceRecord

	for hostKey, hostDataAny := range finalResults {
		hostDataMap, ok := hostDataAny.(map[string]interface{})
		if !ok {
			continue
		}

		// Look up host inventory info.
		hostName := hostKey
		hostAddress := ""
		if h, ok := p.config.Hosts[hostKey]; ok {
			if h.Name != "" {
				hostName = h.Name
			}
			hostAddress = h.Address
		}

		// --- Metric records ---
		metricsWrapper, ok := hostDataMap["metrics"].(map[string]interface{})
		if ok {
			metricsMap, ok := metricsWrapper["metrics"].(map[string]interface{})
			if ok {
				for _, metricAny := range metricsMap {
					m, ok := metricAny.(map[string]interface{})
					if !ok {
						continue
					}

					pluginTag, _ := m["__plugin"].(string)
					metricName, _ := m["name"].(string)
					if metricName == "" {
						metricName, _ = m["label"].(string)
					}
					category, _ := m["category"].(string)
					metricType, _ := m["type"].(string)
					instance, _ := m["instance"].(string)
					value := fmt.Sprintf("%v", m["value"])

					// Any non-standard key becomes extra metadata (e.g. "oid").
					var extra map[string]interface{}
					for k, v := range m {
						switch k {
						case "name", "label", "value", "type", "category", "__plugin", "instance":
							// standard keys — skip
						default:
							if extra == nil {
								extra = make(map[string]interface{})
							}
							extra[k] = v
						}
					}

					metricRecords = append(metricRecords, store.MetricRecord{
						HostKey:     hostKey,
						HostName:    hostName,
						HostAddress: hostAddress,
						Plugin:      pluginTag,
						Name:        metricName,
						Category:    category,
						MetricType:  metricType,
						Value:       value,
						ValueNum:    store.ParseValueNum(value),
						Instance:    instance,
						Extra:       extra,
						CollectedAt: now,
					})
				}
			}
		}

		// --- Interface entity records ---
		if ifacesAny, ok := hostDataMap["__interfaces"]; ok {
			if ifaces, ok := ifacesAny.([]map[string]interface{}); ok && len(ifaces) > 0 {
				ifaceRecords = append(ifaceRecords,
					snmpplugin.InterfaceListToRecords(hostKey, hostName, hostAddress, ifaces)...)
			}
		}
	}

	if len(metricRecords) > 0 {
		if err := p.Controller.Store.WriteBatch(metricRecords); err != nil {
			fmt.Printf("  !_ store: WriteBatch error: %v\n", err)
		} else {
			fmt.Printf("  |_ store: wrote %d metric records\n", len(metricRecords))
		}
	}

	if len(ifaceRecords) > 0 {
		if err := p.Controller.Store.UpsertInterfaces(ifaceRecords); err != nil {
			fmt.Printf("  !_ store: UpsertInterfaces error: %v\n", err)
		} else {
			fmt.Printf("  |_ store: upserted %d interface records\n", len(ifaceRecords))
		}
	}
}

// stripInternalTags removes internal keys before JSON marshalling.
func (p *collectionPlugin) stripInternalTags(finalResults map[string]interface{}) {
	for _, hostDataAny := range finalResults {
		hostDataMap, ok := hostDataAny.(map[string]interface{})
		if !ok {
			continue
		}
		// Remove the interfaces slice — it is not part of collection.json output.
		delete(hostDataMap, "__interfaces")

		metricsWrapper, ok := hostDataMap["metrics"].(map[string]interface{})
		if !ok {
			continue
		}
		metricsMap, ok := metricsWrapper["metrics"].(map[string]interface{})
		if !ok {
			continue
		}
		for _, metricAny := range metricsMap {
			if m, ok := metricAny.(map[string]interface{}); ok {
				delete(m, "__plugin")
				delete(m, "instance")
			}
		}
	}
}
