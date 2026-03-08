package device

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	plugin "observer/base"
	"observer/plugins"
)

//go:embed templates/*
var templates embed.FS

type devicePlugin struct {
	plugin.BasePlugin
}

func init() {
	plugins.Register(&devicePlugin{})
}

func (p *devicePlugin) Name() string {
	return "Device"
}

func (p *devicePlugin) GetMenus() map[string]plugin.MenuItem {
	return map[string]plugin.MenuItem{
		"devices": {
			Text:   "Devices",
			Weight: 0,
			Plugin: "device",
			Page:   "list",
		},
	}
}

func (p *devicePlugin) ShowPage(params map[string]string) (string, error) {
	page := params["page"]
	if page == "" {
		page = "list"
	}

	switch page {
	case "details":
		return p.showDevicePage(params)
	case "list":
		fallthrough
	default:
		return p.hostListPage()
	}
}

func (p *devicePlugin) hostListPage() (string, error) {
	// Load config
	configData, err := os.ReadFile("data/config.json")
	if err != nil {
		return "", err
	}
	var config map[string]interface{}
	json.Unmarshal(configData, &config)

	// Load collections
	collectionsData, err := os.ReadFile("data/collection.json")
	if err != nil {
		return "", err
	}
	var collections map[string]interface{}
	json.Unmarshal(collectionsData, &collections)

	// Load perception
	perceptionData, _ := os.ReadFile("data/perception.json")
	var perception map[string]interface{}
	json.Unmarshal(perceptionData, &perception)

	// Get hosts from config
	hosts := make(map[string]interface{})
	if configHosts, ok := config["hosts"].(map[string]interface{}); ok {
		hosts = configHosts
	}

	// Merge perception hosts
	if perceptionHosts, ok := perception["hosts"].(map[string]interface{}); ok {
		for idx, host := range perceptionHosts {
			if _, exists := hosts[idx]; !exists {
				hosts[idx] = host
			}
		}
	}

	// Load remote data
	if remote, ok := config["remote"].(map[string]interface{}); ok {
		if tokens, ok := remote["tokens"].(map[string]interface{}); ok {
			for idx, token := range tokens {
				remoteData, _ := os.ReadFile(fmt.Sprintf("data/remote_%s.json", idx))
				var remoteJSON map[string]interface{}
				if json.Unmarshal(remoteData, &remoteJSON) == nil {
					remoteGroup := ""
					if tokenMap, ok := token.(map[string]interface{}); ok {
						if group, ok := tokenMap["group"].(string); ok {
							remoteGroup = group
						}
					}

					if remoteCollection, ok := remoteJSON["collection"].(map[string]interface{}); ok {
						for hostIdx, hostData := range remoteCollection {
							key := remoteGroup + hostIdx
							if _, exists := hosts[key]; !exists {
								if remoteGroup != "" {
									if hostMap, ok := hostData.(map[string]interface{}); ok {
										hostMap["group"] = remoteGroup
										hosts[key] = hostMap
									}
								} else {
									hosts[key] = hostData
								}
							}
						}
					}
				}
			}
		}
	}

	// Build JSON hosts array
	var jsonHosts []map[string]interface{}
	for idx, hostData := range hosts {
		hostMap, ok := hostData.(map[string]interface{})
		if !ok {
			continue
		}

		host := map[string]interface{}{
			"id":             idx,
			"name":           getStringFromMap(hostMap, "name", idx),
			"status":         "up",
			"extendedStatus": getStringFromMap(hostMap, "summary", ""),
			"collapsed":      getBoolFromMap(hostMap, "collapsed", true),
			"selected":       false,
			"metrics":        make(map[string]interface{}),
		}

		// Get group
		if group, ok := hostMap["group"].(string); ok {
			host["group"] = group
		}

		// Get metrics from collections
		if collectionData, ok := collections[idx].(map[string]interface{}); ok {
			if metricsData, ok := collectionData["metrics"].(map[string]interface{}); ok {
				host["metrics"] = p.convertMetricsToArrays(metricsData)
			}
		}

		// Build actions
		actions := []map[string]interface{}{
			{"name": "Details", "plugin": "device", "page": "details"},
		}
		if hostActions, ok := hostMap["actions"].([]interface{}); ok {
			for _, action := range hostActions {
				if actionMap, ok := action.(map[string]interface{}); ok {
					actions = append(actions, actionMap)
				}
			}
		}
		host["actions"] = actions

		jsonHosts = append(jsonHosts, host)
	}

	// Build page with templates - order matters! JS must load before HTML
	output := p.buildPageOrdered([]struct {
		name string
		typ  string
	}{
		{"device.js", "script"},
		{"device.css", "style"},
		{"device_list.html", "html"},
	})

	// Replace hostlist placeholder
	hostsJSON, _ := json.MarshalIndent(jsonHosts, "", "  ")
	output = strings.Replace(output, "{{$hostlist}}", string(hostsJSON), 1)

	return output, nil
}

func (p *devicePlugin) showDevicePage(params map[string]string) (string, error) {
	deviceID := params["device_id"]
	if deviceID == "" {
		return "Invalid Device Name", nil
	}

	// Load collections
	collectionsData, err := os.ReadFile("data/collection.json")
	if err != nil {
		return "", err
	}
	var collections map[string]interface{}
	json.Unmarshal(collectionsData, &collections)

	deviceData, ok := collections[deviceID].(map[string]interface{})
	if !ok {
		return "Invalid Device Name", nil
	}

	// Load config
	configData, _ := os.ReadFile("data/config.json")
	var config map[string]interface{}
	json.Unmarshal(configData, &config)

	// Build device data
	result := make(map[string]interface{})
	if configHosts, ok := config["hosts"].(map[string]interface{}); ok {
		if hostConfig, ok := configHosts[deviceID].(map[string]interface{}); ok {
			result = hostConfig
		}
	}

	// Convert metrics to array format for JavaScript
	metricsConverted := p.convertMetricsToArrays(deviceData["metrics"])

	// Add metrics section
	sections := []map[string]interface{}{
		{
			"title":   "Info",
			"type":    "metrics",
			"icon":    "fa-chart-line",
			"metrics": metricsConverted,
		},
	}

	// Add collections sections
	if deviceCollections, ok := deviceData["collections"].(map[string]interface{}); ok {
		for idx, collection := range deviceCollections {
			if collMap, ok := collection.(map[string]interface{}); ok {
				collType := getStringFromMap(collMap, "collection-type", "text")
				if collType == "text" {
					sections = append(sections, map[string]interface{}{
						"title": idx,
						"type":  "text",
						"icon":  "fa-chart-line",
						"text":  fmt.Sprintf("<pre>%s</pre>", getStringFromMap(collMap, "data", "")),
					})
				}
			}
		}
	}

	result["sections"] = sections
	result["actions"] = []interface{}{}

	// Build output
	deviceJSON, _ := json.MarshalIndent(result, "", "  ")
	output := fmt.Sprintf("<script>\nconst deviceData = %s;\n</script>\n\n", deviceJSON)

	// Add templates - order matters! JS must load before HTML
	output += p.buildPageOrdered([]struct {
		name string
		typ  string
	}{
		{"device.js", "script"},
		{"device.css", "style"},
		{"device_details.html", "html"},
	})

	return output, nil
}

// convertMetricsToArrays converts metrics from map[category]map[name]metric to map[category][]metric
func (p *devicePlugin) convertMetricsToArrays(metricsInterface interface{}) map[string][]interface{} {
	result := make(map[string][]interface{})
	
	metrics, ok := metricsInterface.(map[string]interface{})
	if !ok {
		return result
	}

	for category, categoryData := range metrics {
		categoryMap, ok := categoryData.(map[string]interface{})
		if !ok {
			continue
		}

		var metricsArray []interface{}
		for _, metric := range categoryMap {
			metricsArray = append(metricsArray, metric)
		}
		result[category] = metricsArray
	}

	return result
}

func (p *devicePlugin) buildPage(widgets map[string]string) string {
	var output strings.Builder

	for fname, ftype := range widgets {
		content, err := templates.ReadFile("templates/" + fname)
		if err != nil {
			continue
		}

		contentStr := string(content)

		// Substitute template variables
		contentStr = p.substituteVars(contentStr)

		switch ftype {
		case "script":
			output.WriteString("<script>\n")
			output.WriteString(contentStr)
			output.WriteString("\n</script>\n")
		case "style":
			output.WriteString("<style>\n")
			output.WriteString(contentStr)
			output.WriteString("\n</style>\n")
		case "html":
			fallthrough
		default:
			output.WriteString(contentStr)
			output.WriteString("\n")
		}
	}

	return output.String()
}

func (p *devicePlugin) buildPageOrdered(widgets []struct {
	name string
	typ  string
}) string {
	var output strings.Builder

	for _, widget := range widgets {
		content, err := templates.ReadFile("templates/" + widget.name)
		if err != nil {
			continue
		}

		contentStr := string(content)

		// Substitute template variables
		contentStr = p.substituteVars(contentStr)

		switch widget.typ {
		case "script":
			output.WriteString("<script>\n")
			output.WriteString(contentStr)
			output.WriteString("\n</script>\n")
		case "style":
			output.WriteString("<style>\n")
			output.WriteString(contentStr)
			output.WriteString("\n</style>\n")
		case "html":
			fallthrough
		default:
			output.WriteString(contentStr)
			output.WriteString("\n")
		}
	}

	return output.String()
}

func (p *devicePlugin) substituteVars(content string) string {
	// Substitute common template variables
	replacements := map[string]string{
		"{{$Ops Wellness}}":    "Network Health",
		"{{$Collapse}}":        "Collapse",
		"{{$Network Health}}":  "Network Health",
		"{{$Details}}":         "Details",
		"{{$Queue}}":           "Queue",
		"{{$Stats}}":           "Stats",
		"{{$Info}}":            "Info",
		"{{$Status}}":          "Status",
		"{{$Metrics}}":         "Metrics",
		"{{$Actions}}":         "Actions",
		"{{$Name}}":            "Name",
		"{{$Address}}":         "Address",
		"{{$Group}}":           "Group",
		"{{$Refresh Data}}":    "Refresh Data",
	}

	result := content
	for placeholder, value := range replacements {
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result
}

func getStringFromMap(m map[string]interface{}, key, defaultValue string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return defaultValue
}

func getBoolFromMap(m map[string]interface{}, key string, defaultValue bool) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return defaultValue
}
