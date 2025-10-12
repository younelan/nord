
package mail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"observer/base"
	"observer/plugins"
	"os/exec"
	"strings"
)

// mailPlugin interacts with a local Postfix mail server.
type mailPlugin struct {
	plugin.BasePlugin
}

func init() {
	plugins.Register(&mailPlugin{})
}

// Name returns the plugin's name.
func (p *mailPlugin) Name() string {
	return "Mail"
}

// OnCollect gathers metrics from the Postfix mail server.
func (p *mailPlugin) OnCollect(options map[string]interface{}) (map[string]interface{}, error) {
	metrics := make(map[string]interface{})

	// Get queue size
	queue, err := p.getQueue()
	if err != nil {
		metrics["queue"] = p.errorMetric("Queue", err)
	} else {
		metrics["queue"] = map[string]interface{}{"name": "Queue", "label": "Queue", "value": fmt.Sprintf("%d", len(queue)), "category": "Mail", "type": "text"}
	}

	// Get delivery status
	isPaused, err := p.isDeliveryPaused()
	if err != nil {
		metrics["delivery"] = p.errorMetric("Delivery", err)
	} else {
		deliveryStatus := "On"
		if isPaused {
			deliveryStatus = "Off"
		}
		metrics["delivery"] = map[string]interface{}{"name": "Delivery", "label": "Send", "value": deliveryStatus, "category": "Mail", "type": "text"}
	}

	// Get service status
	isRunning, err := p.isServiceRunning()
	if err != nil {
		metrics["service"] = p.errorMetric("Service", err)
	} else {
		serviceStatus := "Down"
		if isRunning {
			serviceStatus = "Up"
		}
		metrics["service"] = map[string]interface{}{"name": "Service", "label": "Server", "value": serviceStatus, "category": "Mail", "type": "text"}
	}

	return map[string]interface{}{"metrics": metrics}, nil
}

// OnCommand handles control actions for the Postfix service.
func (p *mailPlugin) OnCommand(args map[string]string) error {
	action := args["action"]
	var cmd *exec.Cmd

	switch action {
	case "pause":
		// This requires two commands
		cmd1 := exec.Command("sudo", "postconf", "-e", "defer_transports=smtp")
		cmd2 := exec.Command("sudo", "postfix", "reload")
		return runCommands(cmd1, cmd2)
	case "unpause":
		cmd1 := exec.Command("sudo", "postconf", "-e", "defer_transports=")
		cmd2 := exec.Command("sudo", "postfix", "reload")
		cmd3 := exec.Command("sudo", "postfix", "flush")
		return runCommands(cmd1, cmd2, cmd3)
	case "start":
		cmd = exec.Command("sudo", "systemctl", "start", "postfix")
	case "stop":
		cmd = exec.Command("sudo", "systemctl", "stop", "postfix")
	default:
		return fmt.Errorf("unknown action for mail plugin: %s", action)
	}

	return cmd.Run()
}

// getQueue executes `postqueue -j` and parses the JSON output.
func (p *mailPlugin) getQueue() ([]interface{}, error) {
	cmd := exec.Command("postqueue", "-j")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var queue []interface{}
	// The output is a stream of JSON objects, one per line. We need to decode them one by one.
	decoder := json.NewDecoder(&out)
	for decoder.More() {
		var entry interface{}
		if err := decoder.Decode(&entry); err != nil {
			continue // Or handle error
		}
		queue = append(queue, entry)
	}

	return queue, nil
}

// isDeliveryPaused checks if mail delivery is paused in Postfix.
func (p *mailPlugin) isDeliveryPaused() (bool, error) {
	cmd := exec.Command("postconf", "-h", "defer_transports")
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == "smtp", nil
}

// isServiceRunning checks if the postfix process is active.
func (p *mailPlugin) isServiceRunning() (bool, error) {
	// A simple check, similar to the PHP version.
	cmd := exec.Command("ps", "aux")
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return strings.Contains(string(out), "postfix/"), nil
}

// runCommands executes a series of commands in order.
func runCommands(cmds ...*exec.Cmd) error {
	for _, cmd := range cmds {
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

func (p *mailPlugin) errorMetric(label string, err error) map[string]interface{} {
	return map[string]interface{}{
		"type":     "text",
		"category": "Mail",
		"label":    label,
		"value":    fmt.Sprintf("Error: %v", err),
	}
}
