
package sshcollect

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"observer/base"
	"observer/plugins"
	"strconv"
	"strings"
)

// --- Device Definition Structs ---
type DeviceDef struct {
	Prelude map[string]CommandDef `json:"prelude"`
	Info    map[string]CommandDef `json:"info"`
	Outro   map[string]CommandDef `json:"outro"`
}

type CommandDef struct {
	Command      string            `json:"command"`
	WaitFor      string            `json:"waitfor"`
	Format       string            `json:"format"`
	Category     string            `json:"category"`
	Replacements map[string]string `json:"replacements"`
	Delimiter    string            `json:"delimiter"`
}

// --- Plugin Implementation ---

type sshCollectPlugin struct {
	plugin.BasePlugin
}

func init() {
	plugins.Register(&sshCollectPlugin{})
}

func (p *sshCollectPlugin) Name() string {
	return "SSHCollect"
}

// OnCollect is the main entry point for the plugin.
func (p *sshCollectPlugin) OnCollect(options map[string]interface{}) (map[string]interface{}, error) {
	// 1. Get Credentials and Device Type
	credsMap, ok := options["credentials"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("credentials not provided or invalid format for sshcollect task")
	}

	user, _ := credsMap["user"].(string)
	pass, _ := credsMap["pass"].(string)
	hostAddr, _ := credsMap["host"].(string)
	portStr, _ := credsMap["port"].(string)

	// Safely get deviceType, defaulting to "nokia2425" if not found or not a string
	var deviceType string
	if dt, ok := credsMap["type"].(string); ok && dt != "" {
		deviceType = dt
	} else {
		deviceType = "nokia2425" // Default as per original PHP behavior
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid port format: %w", err)
	}

	// The original PHP script hardcoded nokia2425 if type was not specified.
	// We will default to nokia2425 if not explicitly set in credentials.
	if deviceType == "" {
		deviceType = "nokia2425"
	}

	// 2. Load Device Definition
	deviceDef, err := p.loadDeviceDef(deviceType)
	if err != nil {
		return nil, err
	}

	// 3. Execute Commands
	sess := &InteractiveSession{}
	if err := sess.Connect(user, pass, hostAddr, port); err != nil {
		return nil, fmt.Errorf("SSH connection failed: %w", err)
	}
	defer sess.Close()

	if err := sess.Shell(); err != nil {
		return nil, fmt.Errorf("failed to start shell: %w", err)
	}

	_, _ = sess.WaitFor("#|>") // Clear banner

	commandResults, err := p.runCommandGroups(sess, deviceDef)
	if err != nil {
		return nil, fmt.Errorf("error during command execution: %w", err)
	}

	return p.parseCollection(commandResults, deviceDef), nil
}

func (p *sshCollectPlugin) runCommandGroups(sess *InteractiveSession, def *DeviceDef) (map[string]string, error) {
	results := make(map[string]string)
	commandGroups := []map[string]CommandDef{def.Prelude, def.Info, def.Outro}

	for _, group := range commandGroups {
		for name, cmd := range group {
			fmt.Printf("        |_ Running SSH command: %s\n", cmd.Command)
			if err := sess.Send(cmd.Command); err != nil {
				return nil, err
			}

			// For exit and logout commands, do not wait for a prompt as the session will close.
			if name == "exit" || name == "logout" {
				continue
			}

			output, err := sess.WaitFor(cmd.WaitFor)
			if err != nil {
				fmt.Printf("            !_ Warning: %v\n", err)
			}
			// Store raw output for parsing later
			if name != "exit" && name != "logout" { // Don't store output of logout commands
				results[name] = output
			}
		}
	}
	return results, nil
}

// parseCollection processes the raw command output into structured metrics.
func (p *sshCollectPlugin) parseCollection(results map[string]string, def *DeviceDef) map[string]interface{} {
	metrics := make(map[string]interface{})
	collections := make(map[string]interface{})

	for name, output := range results {
		cmdDef, ok := def.Info[name]
		if !ok {
			continue // Not a command we need to parse
		}

		lines := strings.Split(output, "\n")
		if len(lines) > 2 {
			lines = lines[1 : len(lines)-1] // Remove first and last lines (command and prompt)
		}

		switch cmdDef.Format {
		case "single-column":
			delimiter := cmdDef.Delimiter
			if delimiter == "" {
				delimiter = ":"
			}
			for _, line := range lines {
				if !strings.Contains(line, delimiter) {
					continue
				}
				parts := strings.SplitN(line, delimiter, 2)
				key := strings.Trim(parts[0], " ([])")
				value := strings.Trim(parts[1], " ([])")

				// Apply replacements
				for old, new := range cmdDef.Replacements {
					key = strings.ReplaceAll(key, old, new)
					value = strings.ReplaceAll(value, old, new)
				}
				key = strings.TrimSpace(key)

				metric := map[string]interface{}{
					"type":     "text",
					"label":    key,
					"value":    value,
					"name":     key,
					"category": cmdDef.Category,
				}
				metrics[key] = metric
			}
		case "hide":
			// Do nothing
		default: // "text"
			collections[name] = strings.Join(lines, "\n")
		}
	}

	return map[string]interface{}{"metrics": metrics, "collections": collections}
}

// --- Helper Functions ---

func (p *sshCollectPlugin) loadAppConfig() (*plugin.Config, error) {
	configFile, err := ioutil.ReadFile("data/config.json")
	if err != nil {
		return nil, err
	}
	var config plugin.Config
	err = json.Unmarshal(configFile, &config)
	return &config, err
}

func (p *sshCollectPlugin) loadDeviceDef(deviceType string) (*DeviceDef, error) {
	defFile, err := ioutil.ReadFile(fmt.Sprintf("plugins/sshcollect/devices/%s.json", deviceType))
	if err != nil {
		return nil, fmt.Errorf("could not read device definition for '%s': %w", deviceType, err)
	}
	var def DeviceDef
	err = json.Unmarshal(defFile, &def)
	return &def, err
}
