
package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"observer/base"
	"observer/plugins"
	"strings"
	"time"
)

// --- Plugin Implementation ---

type apiPlugin struct {
	plugin.BasePlugin
}

func init() {
	plugins.Register(&apiPlugin{})
}

func (p *apiPlugin) Name() string {
	return "Api"
}

func (p *apiPlugin) OnCommand(args map[string]string) error {
	action := args["action"]
	if action == "send" {
		return p.sendRemoteData()
	}
	return fmt.Errorf("unknown command for Api plugin: %s", action)
}

func (p *apiPlugin) sendRemoteData() error {
	fmt.Println("--- Sending data to remote servers ---")

	// 1. Load Config
	configFile, err := ioutil.ReadFile("data/config.json")
	if err != nil {
		return fmt.Errorf("could not read config file: %w", err)
	}
	var config plugin.Config
	if err := json.Unmarshal(configFile, &config); err != nil {
		return fmt.Errorf("could not parse config file: %w", err)
	}

	// 2. Load collection data
	collectionFile, err := ioutil.ReadFile("data/collection.json")
	if err != nil {
		return fmt.Errorf("could not read collection.json: %w", err)
	}
	var collectionData interface{}
	if err := json.Unmarshal(collectionFile, &collectionData); err != nil {
		return fmt.Errorf("could not parse collection.json: %w", err)
	}

	// 3. Iterate destinations and send data
	for name, dest := range config.Remote.Destinations {
		if !dest.Active {
			fmt.Printf("  |_ Skipping destination '%s' (inactive)\n", name)
			continue
		}
		fmt.Printf("  |_ Contacting destination: %s (%s)\n", name, dest.Endpoint)

		if err := p.sendDataToDestination(dest, collectionData, config.Hosts); err != nil {
			fmt.Printf("      !_ Error: %v\n", err)
		} else {
			fmt.Println("      |_ Success.")
		}
	}

	return nil
}

func (p *apiPlugin) sendDataToDestination(dest plugin.Destination, collectionData interface{}, hostsData map[string]plugin.Host) error {
	// Create the payload as expected by the PHP server
	payload := make(map[string]interface{})
	payload["collection"] = collectionData

	// JSON-encode the payload into a string
	jsonPayloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal collection payload: %w", err)
	}

	// JSON-encode the hosts data into a string
	hostsBytes, err := json.Marshal(hostsData)
	if err != nil {
		return fmt.Errorf("failed to marshal hosts payload: %w", err)
	}

	// Build the x-www-form-urlencoded data
	formData := url.Values{}
	formData.Set("json_payload", string(jsonPayloadBytes))
	formData.Set("hosts", string(hostsBytes))

	// Create the request
	req, err := http.NewRequest("POST", dest.Endpoint, strings.NewReader(formData.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+dest.Token)

	// Send the request
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read and print response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	fmt.Printf("      |_ Server response: %s\n", string(body))

	if resp.StatusCode >= 400 {
		return fmt.Errorf("server returned error status: %s", resp.Status)
	}

	return nil
}
