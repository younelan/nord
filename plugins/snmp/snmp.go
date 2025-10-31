package snmp

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	plugin "observer/base"
	"observer/plugins"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
)

// --- Device Definition Structs ---

// DeviceDefinition defines the structure for SNMP device JSON files.
type DeviceDefinition struct {
	OIDs []OIDDefinition `json:"oids"`
}

// OIDDefinition defines a single OID to query.
type OIDDefinition struct {
	OID    string `json:"oid"`
	Name   string `json:"name"`
	Format string `json:"format"` // string, timeticks, integer, counter, gauge
}

// --- Plugin Implementation ---

// snmpPlugin performs SNMP queries on network devices.
type snmpPlugin struct {
	plugin.BasePlugin
}

func init() {
	plugins.Register(&snmpPlugin{})
}

// Name returns the plugin's name.
func (p *snmpPlugin) Name() string {
	return "Snmp"
}

// OnCommand handles actions for the SNMP plugin.
func (p *snmpPlugin) OnCommand(args map[string]string) error {
	action := args["action"]
	return fmt.Errorf("unknown command for SNMP plugin: %s", action)
}

// OnCollect handles data collection for the SNMP plugin.
func (p *snmpPlugin) OnCollect(options map[string]interface{}) (map[string]interface{}, error) {
	// Extract credentials from options
	credentials, ok := options["credentials"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("SNMP: credentials not provided")
	}

	// Get SNMP connection parameters
	host, _ := credentials["host"].(string)
	if host == "" {
		// Fallback to host address if credentials don't specify host
		if hostMap, ok := options["host"].(map[string]interface{}); ok {
			host, _ = hostMap["address"].(string)
		}
	}

	portFloat, _ := credentials["port"].(float64)
	port := uint16(portFloat)
	if port == 0 {
		port = 161 // Default SNMP port
	}

	community, _ := credentials["community"].(string)
	if community == "" {
		community = "public" // Default community
	}

	version, _ := credentials["version"].(string)
	deviceType, _ := credentials["type"].(string)
	if deviceType == "" {
		deviceType = "generic"
	}

	fmt.Printf("          |_ SNMP: Querying %s:%d (community: %s, version: %s, type: %s)\n",
		host, port, community, version, deviceType)

	// Load device definition
	deviceDef, err := p.loadDeviceDefinition(deviceType)
	if err != nil {
		return nil, fmt.Errorf("SNMP: failed to load device definition: %w", err)
	}

	// Perform SNMP queries
	results, err := p.querySNMP(host, port, community, version, deviceDef)
	if err != nil {
		return nil, fmt.Errorf("SNMP: query failed: %w", err)
	}

	return results, nil
}

// loadDeviceDefinition loads the SNMP device definition from JSON.
func (p *snmpPlugin) loadDeviceDefinition(deviceType string) (*DeviceDefinition, error) {
	filename := filepath.Join("plugins", "snmp", "devices", deviceType+".json")
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("could not read device file %s: %w", filename, err)
	}

	var deviceDef DeviceDefinition
	if err := json.Unmarshal(data, &deviceDef); err != nil {
		return nil, fmt.Errorf("could not parse device file %s: %w", filename, err)
	}

	return &deviceDef, nil
}

// querySNMP connects to the device and queries all defined OIDs.
func (p *snmpPlugin) querySNMP(host string, port uint16, community, version string, deviceDef *DeviceDefinition) (map[string]interface{}, error) {
	// Configure SNMP connection
	snmpClient := &gosnmp.GoSNMP{
		Target:    host,
		Port:      port,
		Community: community,
		Version:   p.getSNMPVersion(version),
		Timeout:   time.Duration(5) * time.Second,
		Retries:   3,
	}

	// Connect to the device
	err := snmpClient.Connect()
	if err != nil {
		return nil, fmt.Errorf("SNMP connect failed: %w", err)
	}
	defer snmpClient.Conn.Close()

	// Query each OID
	metrics := make(map[string]interface{})

	for _, oidDef := range deviceDef.OIDs {
		result, err := snmpClient.Get([]string{oidDef.OID})
		if err != nil {
			fmt.Printf("          !_ SNMP: Failed to query OID %s (%s): %v\n", oidDef.OID, oidDef.Name, err)
			continue
		}

		if len(result.Variables) == 0 {
			continue
		}

		variable := result.Variables[0]
		value := p.formatValue(variable, oidDef.Format)

		// Create metric in the expected format
		metricKey := strings.ReplaceAll(oidDef.Name, " ", "_")
		metrics[metricKey] = map[string]interface{}{
			"category": "snmp",
			"name":     oidDef.Name,
			"value":    value,
			"type":     "gauge",
			"oid":      oidDef.OID,
		}

		fmt.Printf("          |_ SNMP: %s = %v\n", oidDef.Name, value)
	}

	return map[string]interface{}{"metrics": metrics}, nil
}

// getSNMPVersion converts version string to gosnmp version constant.
func (p *snmpPlugin) getSNMPVersion(version string) gosnmp.SnmpVersion {
	switch strings.ToLower(version) {
	case "1":
		return gosnmp.Version1
	case "2c", "2":
		return gosnmp.Version2c
	case "3":
		return gosnmp.Version3
	default:
		return gosnmp.Version2c // Default to v2c
	}
}

// formatValue converts SNMP variable to appropriate Go type based on format.
func (p *snmpPlugin) formatValue(variable gosnmp.SnmpPDU, format string) interface{} {
	switch format {
	case "string":
		switch variable.Type {
		case gosnmp.OctetString:
			return string(variable.Value.([]byte))
		default:
			return fmt.Sprintf("%v", variable.Value)
		}

	case "timeticks":
		if ticks, ok := variable.Value.(uint32); ok {
			// Convert timeticks (hundredths of a second) to duration
			duration := time.Duration(ticks) * 10 * time.Millisecond
			days := int(duration.Hours() / 24)
			hours := int(duration.Hours()) % 24
			minutes := int(duration.Minutes()) % 60
			seconds := int(duration.Seconds()) % 60
			return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
		}
		return fmt.Sprintf("%v", variable.Value)

	case "integer":
		switch v := variable.Value.(type) {
		case int:
			return v
		case uint:
			return int(v)
		case int64:
			return int(v)
		case uint64:
			return int(v)
		case []byte:
			if len(v) > 0 {
				val, _ := strconv.Atoi(string(v))
				return val
			}
		}
		return fmt.Sprintf("%v", variable.Value)

	case "counter", "gauge":
		switch v := variable.Value.(type) {
		case uint:
			return v
		case uint32:
			return v
		case uint64:
			return v
		case int:
			return v
		}
		return fmt.Sprintf("%v", variable.Value)

	default:
		// Default: return as string
		switch variable.Type {
		case gosnmp.OctetString:
			return string(variable.Value.([]byte))
		default:
			return fmt.Sprintf("%v", variable.Value)
		}
	}
}
