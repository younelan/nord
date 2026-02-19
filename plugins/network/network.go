
package network

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net"
	"observer/base"
	"observer/plugins"
	"observer/store"
	"os/exec"
	"strings"
	"time"
)

// --- Structs for Nmap XML Output ---

type NmapRun struct {
	XMLName xml.Name `xml:"nmaprun"`
	Hosts   []Host   `xml:"host"`
}

type Host struct {
	Status    Status    `xml:"status"`
	Addresses []Address `xml:"address"`
	Hostnames []Hostname  `xml:"hostnames>hostname"`
}

type Status struct {
	State string `xml:"state,attr"`
}

type Address struct {
	Addr     string `xml:"addr,attr"`
	AddrType string `xml:"addrtype,attr"`
}

type Hostname struct {
	Name string `xml:"name,attr"`
}

// --- Plugin Implementation ---

// networkPlugin performs network-related checks.
type networkPlugin struct {
	plugin.BasePlugin
}

func init() {
	plugins.Register(&networkPlugin{})
}

// Name returns the plugin's name.
func (p *networkPlugin) Name() string {
	return "Network"
}

// OnCommand handles actions for the network plugin, including perception.
func (p *networkPlugin) OnCommand(args map[string]string) error {
	action := args["action"]
	if action == "perception" {
		return p.runPerception()
	}
	return fmt.Errorf("unknown command for Network plugin: %s", action)
}

// OnCollect handles data collection for the network plugin.
func (p *networkPlugin) OnCollect(options map[string]interface{}) (map[string]interface{}, error) {
	action, _ := options["action"].(string)
	host, _ := options["host"].(map[string]interface{})
	address, _ := host["address"].(string)

	var status bool
	var label, category string

	switch action {
	case "ssh":
		port, _ := host["port"].(string)
		if port == "" {
			port = "22"
		}
		label = fmt.Sprintf("SSH-%s", port)
		category = "network"
		status = p.isPortOpen(address, port)
	case "url":
		label = "URL"
		category = "Web"
		status = p.isPortOpen(address, "80") || p.isPortOpen(address, "443")
	case "ping":
		label = "ping"
		category = "network"
		status = p.isPortOpen(address, "80") || p.isPortOpen(address, "22")
	default:
		return nil, fmt.Errorf("undefined network action: %s", action)
	}

	resultStatus := "down"
	if status {
		resultStatus = "up"
	}

	metric := map[string]interface{}{
		"category": category,
		"name":     label,
		"value":    resultStatus,
		"type":     "status",
	}

	return map[string]interface{}{"metrics": map[string]interface{}{label: metric}}, nil
}

// isPortOpen checks if a TCP port is open at the given host.
func (p *networkPlugin) isPortOpen(host, port string) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 2*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

// runPerception is the main logic for the network discovery feature.
func (p *networkPlugin) runPerception() error {
	fmt.Println("--- Starting Network Perception ---")

	// 1. Load Config
	configFile, err := ioutil.ReadFile("data/config.json")
	if err != nil {
		return fmt.Errorf("could not read config file: %w", err)
	}
	var config plugin.Config
	if err := json.Unmarshal(configFile, &config); err != nil {
		return fmt.Errorf("could not parse config file: %w", err)
	}

	discoveredHosts := make(map[string]interface{})

	// 2. Iterate through perception environments
	for name, env := range config.Perception {
		if !env.Enabled {
			fmt.Printf("    |_ Skipping environment '%s' (disabled)\n", name)
			continue
		}
		fmt.Printf("    |_ Scanning environment: %s\n", name)

		if env.Method == "nmap" {
			// 3. Run Nmap
			fmt.Printf("        |_ Running nmap on ranges: %s\n", strings.Join(env.Ranges, " "))
			nmapArgs := []string{"nmap", "-sn", "-oX", "-"} // -sn: Ping Scan, -oX -: XML output to stdout
			nmapArgs = append(nmapArgs, env.Ranges...)
			cmd := exec.Command("sudo", nmapArgs...)

			var out bytes.Buffer
			cmd.Stdout = &out
			if err := cmd.Run(); err != nil {
				fmt.Printf("          !_ nmap command failed: %v\n", err)
				continue
			}

			// 4. Parse Nmap XML
			var nmapResult NmapRun
			if err := xml.Unmarshal(out.Bytes(), &nmapResult); err != nil {
				fmt.Printf("          !_ Failed to parse nmap XML: %v\n", err)
				continue
			}

			// 5. Test discovered hosts
			for _, host := range nmapResult.Hosts {
				if host.Status.State != "up" {
					continue
				}
				ip := ""
				for _, addr := range host.Addresses {
					if addr.AddrType == "ipv4" {
						ip = addr.Addr
						break
					}
				}
				if ip == "" {
					continue
				}

				fmt.Printf("        |_ Found host: %s\n", ip)
				validServices := p.testHost(ip, env.Detection)
				discoveredHosts[ip] = map[string]interface{}{
					"address": ip,
					"collect": validServices,
				}
			}
		}
	}

	// 6. Save results
	finalOutput := map[string]interface{}{"hosts": discoveredHosts}
	jsonData, err := json.MarshalIndent(finalOutput, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal perception results: %w", err)
	}
	if err := ioutil.WriteFile("data/perception.json", jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write perception.json: %w", err)
	}

	// 7. Persist discovered hosts and their detected services to the store.
	if p.Controller.Store != nil {
		p.writePerceptionToStore(discoveredHosts)
	}

	fmt.Println("--- Network Perception Finished ---")
	return nil
}

// writePerceptionToStore persists each discovered host and its detected services.
// Each detected service (e.g. "network.ping") is recorded as a status=up metric
// under category "discovery" so the hosts table is populated and detection history
// is queryable.
func (p *networkPlugin) writePerceptionToStore(discoveredHosts map[string]interface{}) {
	now := time.Now()
	var records []store.MetricRecord

	for ip, hostAny := range discoveredHosts {
		hostMap, ok := hostAny.(map[string]interface{})
		if !ok {
			continue
		}
		services, _ := hostMap["collect"].([]string)

		for _, svc := range services {
			parts := strings.SplitN(svc, ".", 2)
			pluginName := parts[0]
			action := ""
			if len(parts) == 2 {
				action = parts[1]
			}
			v := 1.0
			records = append(records, store.MetricRecord{
				HostKey:     ip,
				HostName:    ip,
				HostAddress: ip,
				Plugin:      pluginName,
				Name:        action,
				Category:    "discovery",
				MetricType:  "status",
				Value:       "up",
				ValueNum:    &v,
				CollectedAt: now,
			})
		}
	}

	if len(records) == 0 {
		return
	}
	if err := p.Controller.Store.WriteBatch(records); err != nil {
		fmt.Printf("  !_ store: perception WriteBatch error: %v\n", err)
	} else {
		fmt.Printf("  |_ store: wrote %d perception records\n", len(records))
	}
}

// testHost runs detection tests on a given IP.
func (p *networkPlugin) testHost(ip string, tests []string) []string {
	fmt.Printf("            |_ Testing services on %s...\n", ip)
	validServices := []string{}
	for _, test := range tests {
		parts := strings.Split(test, ".")
		if len(parts) < 2 {
			continue
		}
		pluginName, action := parts[0], parts[1]

		targetPlugin, exists := p.Controller.Plugins[pluginName]
		if !exists {
			continue
		}

		pluginOptions := map[string]interface{}{
			"host":   map[string]interface{}{"address": ip},
			"action": action,
		}

		// We only care if the call succeeds and returns a metric with value 'up'.
		result, err := targetPlugin.OnCollect(pluginOptions)
		if err != nil {
			continue
		}

		metrics, _ := result["metrics"].(map[string]interface{})
		for _, metricData := range metrics {
			metric, _ := metricData.(map[string]interface{})
			value, _ := metric["value"].(string)
			if value == "up" {
				validServices = append(validServices, test)
				break // Found one valid metric, no need to check others from this result
			}
		}
	}
	return validServices
}
