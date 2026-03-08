
package mail

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	
	plugin "observer/base"
	"observer/plugins"
	"os/exec"
)

//go:embed templates/*
var templates embed.FS

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

// GetMenus returns menu items for the mail plugin
func (p *mailPlugin) GetMenus() map[string]plugin.MenuItem {
	return map[string]plugin.MenuItem{
		"mail": {
			Text:   "Mail Server",
			Weight: 1,
			Children: map[string]plugin.MenuItem{
				"viewqueue": {
					Plugin: "mail",
					Page:   "viewqueue",
					Text:   "View Queue",
				},
				"mailsummary": {
					Plugin: "mail",
					Page:   "mailsummary",
					Text:   "Mail Summary",
				},
			},
		},
	}
}

// ShowPage renders the mail queue UI
func (p *mailPlugin) ShowPage(params map[string]string) (string, error) {
	page := params["page"]
	if page == "" {
		page = "viewqueue"
	}

	switch page {
	case "viewsummary", "mailsummary":
		return p.showQueueSummary()
	default:
		return p.showQueue()
	}
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
	// Check if fake_queue is enabled in config
	if p.Controller != nil {
		configData, err := os.ReadFile("data/config.json")
		if err == nil {
			var config map[string]interface{}
			if json.Unmarshal(configData, &config) == nil {
				if fakeQueue, ok := config["fake_queue"].(bool); ok && fakeQueue {
					return p.getFakeQueue()
				}
			}
		}
	}

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


func (p *mailPlugin) showQueue() (string, error) {
	queue, err := p.getQueue()
	if err != nil || len(queue) == 0 {
		return `<div class='widgetheader'><h1 class='widgetheader'>Mail Queue Contents</h1></div>
		<p>Queue is Empty, Back to <a href='?plugin=device'>Device</a></p>`, nil
	}

	// Load template
	templateHTML, err := templates.ReadFile("templates/mail.html")
	if err != nil {
		return "", err
	}

	// Substitute i18n variables
	templateStr := p.substituteI18n(string(templateHTML))

	// Define fields
	fields := map[string]interface{}{
		"recipients": map[string]interface{}{
			"label":    "Recipient",
			"visible":  true,
			"type":     "array_index",
			"subfield": "address",
		},
		"arrival_time": map[string]interface{}{
			"label":   "Date",
			"visible": true,
			"type":    "date",
			"align":   "right",
		},
		"queue_id": map[string]interface{}{
			"label":   "ID",
			"visible": true,
		},
		"sender": map[string]interface{}{
			"label":   "Sender",
			"visible": false,
		},
		"message_size": map[string]interface{}{
			"label":   "Size",
			"visible": false,
		},
		"queue_name": map[string]interface{}{
			"label":   "Queue",
			"visible": false,
		},
	}

	// Load JS
	jsContent, _ := templates.ReadFile("templates/mail.js")

	// Build output
	var output strings.Builder
	output.WriteString(templateStr)
	output.WriteString("\n<script>\n")
	
	fieldsJSON, _ := json.MarshalIndent(fields, "", "  ")
	output.WriteString(fmt.Sprintf("const fields=%s;\n", fieldsJSON))
	
	queueJSON, _ := json.MarshalIndent(queue, "", "  ")
	output.WriteString(fmt.Sprintf("const mailQueue=%s;\n", queueJSON))
	
	output.WriteString("dateFormat='d-m';\n")
	output.Write(jsContent)
	output.WriteString("\n</script>\n")

	return output.String(), nil
}

// substituteI18n replaces {{$Variable}} with English text (or translated if lang is set)
func (p *mailPlugin) substituteI18n(content string) string {
	// For now, use English translations (keys = values)
	// In the future, load from config based on lang setting
	translations := map[string]string{
		"{{$Mail Queue}}":                          "Mail Queue",
		"{{$Show/Hide}}":                           "Show/Hide",
		"{{$Drag a field to Move}}":                "Drag a field to Move",
		"{{$Click a field to Sort that Column}}":   "Click a field to Sort that Column",
		"{{$Click Hide/Show to Hide/Show Fields}}": "Click Hide/Show to Hide/Show Fields",
		"{{$Mail Queue Contents}}":                 "Mail Queue Contents",
		"{{$Queue is Empty, Back to }}":            "Queue is Empty, Back to ",
		"{{$Mail Statistics}}":                     "Mail Statistics",
		"{{$Queue Name}}":                          "Queue Name",
		"{{$Sender}}":                              "Sender",
		"{{$Per Domain}}":                          "Per Domain",
		"{{$Domain}}":                              "Domain",
		"{{$Recipient}}":                           "Recipient",
		"{{$Date}}":                                "Date",
		"{{$Size}}":                                "Size",
		"{{$ID}}":                                  "ID",
		"{{$Queue}}":                               "Queue",
	}

	result := content
	for placeholder, value := range translations {
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result
}

func (p *mailPlugin) showQueueSummary() (string, error) {
	queue, err := p.getQueue()
	if err != nil || len(queue) == 0 {
		return `<div class='widgetheader'><h1 class='widgetheader'>Mail Statistics</h1></div>
		<p>Queue is Empty, Back to <a href='?plugin=device'>Device</a></p>`, nil
	}

	queues := make(map[string]int)
	senders := make(map[string]int)
	domains := make(map[string]int)

	// Process queue entries
	for _, entry := range queue {
		entryMap, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}

		// Queue name
		if queueName, ok := entryMap["queue_name"].(string); ok {
			queues[queueName]++
		}

		// Sender
		if sender, ok := entryMap["sender"].(string); ok {
			senders[sender]++
		}

		// Recipients and domains
		if recipients, ok := entryMap["recipients"].([]interface{}); ok {
			for _, recipient := range recipients {
				if recipientMap, ok := recipient.(map[string]interface{}); ok {
					if addr, ok := recipientMap["address"].(string); ok {
						if idx := strings.Index(addr, "@"); idx != -1 {
							domain := addr[idx+1:]
							domains[domain]++
						}
					}
				}
			}
		}
	}

	// Build HTML output
	var output strings.Builder
	output.WriteString("<div class='widgetheader'><h1>Mail Statistics</h1></div>")

	// Queue table
	output.WriteString("<table class='summary-table'>")
	output.WriteString("<tr><th colspan=3><h3 class='widgetheader'>Mail Queue</h3></th></tr>")
	output.WriteString("<tr class='summary-header'><th>&nbsp;</th><th>Queue Name</th><th>#</th></tr>")
	idx := 0
	for name, qty := range queues {
		idx++
		class := "even"
		if idx%2 == 1 {
			class = "odd"
		}
		output.WriteString(fmt.Sprintf("<tr class='%s'><td>&nbsp;</td><td>%s</td><td>%d</td></tr>", class, name, qty))
	}
	output.WriteString("</table>")

	// Sender table
	output.WriteString("<table class='summary-table'>")
	output.WriteString("<tr><th colspan=3><h3 class='widgetheader'>Sender</h3></th></tr>")
	output.WriteString("<tr class='summary-header'><th>&nbsp;</th><th>Sender</th><th>#</th></tr>")
	for name, qty := range senders {
		idx++
		class := "even"
		if idx%2 == 1 {
			class = "odd"
		}
		output.WriteString(fmt.Sprintf("<tr class='%s'><td>&nbsp;</td><td>%s</td><td>%d</td></tr>", class, name, qty))
	}
	output.WriteString("</table>")

	// Domain table
	output.WriteString("<table class='summary-table'>")
	output.WriteString("<tr><th colspan=3><h3 class='widgetheader'>Per Domain</h3></th></tr>")
	output.WriteString("<tr class='summary-header'><th>&nbsp;</th><th>Domain</th><th>#</th></tr>")
	for name, qty := range domains {
		idx++
		class := "even"
		if idx%2 == 1 {
			class = "odd"
		}
		output.WriteString(fmt.Sprintf("<tr class='%s'><td>&nbsp;</td><td>%s</td><td>%d</td></tr>", class, name, qty))
	}
	output.WriteString("</table>")

	return output.String(), nil
}

// getFakeQueue loads fake queue data for testing
func (p *mailPlugin) getFakeQueue() ([]interface{}, error) {
	data, err := os.ReadFile("plugins/mail/templates/fake_queue.json")
	if err != nil {
		return nil, err
	}

	var queue []interface{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	for decoder.More() {
		var entry interface{}
		if err := decoder.Decode(&entry); err != nil {
			continue
		}
		queue = append(queue, entry)
	}

	return queue, nil
}
