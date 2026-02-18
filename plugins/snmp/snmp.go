package snmp

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	plugin "observer/base"
	"observer/plugins"
	"observer/store"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
)

// --- Device Definition Structs ---

// DeviceDefinition defines the structure for SNMP device JSON files.
type DeviceDefinition struct {
	OIDs   []OIDDefinition   `json:"oids"`
	Tables []TableDefinition `json:"tables"`
}

// OIDDefinition defines a single scalar OID to query.
type OIDDefinition struct {
	OID    string `json:"oid"`
	Name   string `json:"name"`
	Format string `json:"format"` // string, timeticks, integer, counter, gauge
}

// TableDefinition describes an SNMP table to walk (e.g. ifTable).
type TableDefinition struct {
	BaseOID string            `json:"base_oid"` // e.g. "1.3.6.1.2.1.2.2.1"
	Type    string            `json:"type"`     // "interface" → populates interfaces table
	Columns []TableColumnDef  `json:"columns"`
}

// TableColumnDef maps a column sub-OID to its name, format, and role.
type TableColumnDef struct {
	SubOID string `json:"sub_oid"` // numeric suffix after base_oid, e.g. "2" for ifDescr
	Name   string `json:"name"`
	Format string `json:"format"`
	Role   string `json:"role"` // "name", "alias", "type", "speed", "mac", "admin_status", "oper_status", "metric"
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

// querySNMP connects to the device, queries scalar OIDs, and walks any tables.
func (p *snmpPlugin) querySNMP(host string, port uint16, community, version string, deviceDef *DeviceDefinition) (map[string]interface{}, error) {
	snmpClient := &gosnmp.GoSNMP{
		Target:    host,
		Port:      port,
		Community: community,
		Version:   p.getSNMPVersion(version),
		Timeout:   time.Duration(5) * time.Second,
		Retries:   3,
	}

	err := snmpClient.Connect()
	if err != nil {
		return nil, fmt.Errorf("SNMP connect failed: %w", err)
	}
	defer snmpClient.Conn.Close()

	metrics := make(map[string]interface{})

	// --- Scalar OID queries ---
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

	// --- Table walks ---
	var interfaceList []map[string]interface{}

	for _, tableDef := range deviceDef.Tables {
		rows, err := p.walkTable(snmpClient, tableDef)
		if err != nil {
			fmt.Printf("          !_ SNMP: table walk %s failed: %v\n", tableDef.BaseOID, err)
			continue
		}

		switch tableDef.Type {
		case "interface":
			ifaces, ifMetrics := p.processInterfaceTable(rows, tableDef, host)
			interfaceList = append(interfaceList, ifaces...)
			for k, v := range ifMetrics {
				metrics[k] = v
			}
		}
	}

	result := map[string]interface{}{"metrics": metrics}
	if len(interfaceList) > 0 {
		result["interfaces"] = interfaceList
	}
	return result, nil
}

// walkTable performs a BulkWalk on the table's base OID and groups PDUs by row index.
// Returns map[rowIndex]map[subOID]SnmpPDU.
func (p *snmpPlugin) walkTable(client *gosnmp.GoSNMP, table TableDefinition) (map[string]map[string]gosnmp.SnmpPDU, error) {
	base := strings.TrimPrefix(table.BaseOID, ".")

	pdus, err := client.BulkWalkAll(base)
	if err != nil {
		// Fall back to WalkAll for SNMPv1
		pdus, err = client.WalkAll(base)
		if err != nil {
			return nil, err
		}
	}

	// Build sub-OID lookup set from column definitions.
	wantedCols := make(map[string]bool, len(table.Columns))
	for _, col := range table.Columns {
		wantedCols[col.SubOID] = true
	}

	rows := make(map[string]map[string]gosnmp.SnmpPDU)
	for _, pdu := range pdus {
		oid := strings.TrimPrefix(pdu.Name, ".")
		if !strings.HasPrefix(oid, base) {
			continue
		}
		suffix := strings.TrimPrefix(oid, base)
		suffix = strings.TrimPrefix(suffix, ".")
		// suffix is "colSubOID.rowIndex"
		dot := strings.Index(suffix, ".")
		if dot < 0 {
			continue
		}
		colSubOID := suffix[:dot]
		rowIndex := suffix[dot+1:]

		if !wantedCols[colSubOID] {
			continue
		}
		if rows[rowIndex] == nil {
			rows[rowIndex] = make(map[string]gosnmp.SnmpPDU)
		}
		rows[rowIndex][colSubOID] = pdu
	}
	return rows, nil
}

// processInterfaceTable converts walked ifTable rows into interface entity records
// and per-interface counter metrics (ifInOctets, ifOutOctets, etc.).
func (p *snmpPlugin) processInterfaceTable(
	rows map[string]map[string]gosnmp.SnmpPDU,
	table TableDefinition,
	hostAddr string,
) ([]map[string]interface{}, map[string]interface{}) {

	// Build sub-OID → column def index.
	colBySubOID := make(map[string]TableColumnDef, len(table.Columns))
	for _, col := range table.Columns {
		colBySubOID[col.SubOID] = col
	}

	var interfaces []map[string]interface{}
	metrics := make(map[string]interface{})

	for rowIndex, colPDUs := range rows {
		iface := map[string]interface{}{"if_index": rowIndex}

		// Resolve ifDescr first so we can use it as the instance label for metrics.
		ifName := rowIndex // fallback to numeric index

		for subOID, pdu := range colPDUs {
			col, ok := colBySubOID[subOID]
			if !ok {
				continue
			}

			value := p.formatValue(pdu, col.Format)

			switch col.Role {
			case "name":
				if s, ok := value.(string); ok && s != "" {
					ifName = s
				}
				iface["name"] = value
			case "alias":
				iface["alias"] = value
			case "type":
				iface["type"] = value
			case "speed":
				iface["speed"] = value
			case "mac":
				iface["mac_address"] = value
			case "admin_status":
				iface["admin_status"] = value
			case "oper_status":
				iface["oper_status"] = value
			case "metric":
				// Time-series counter — stored in metrics with instance=ifName.
				// Key includes instance so multiple interfaces don't collide.
				metricKey := fmt.Sprintf("%s_%s", col.Name, rowIndex)
				metrics[metricKey] = map[string]interface{}{
					"category": "snmp",
					"name":     col.Name,
					"value":    fmt.Sprintf("%v", value),
					"type":     "counter",
					"oid":      pdu.Name,
					"instance": ifName,
				}
				fmt.Printf("          |_ SNMP: %s[%s] = %v\n", col.Name, ifName, value)
			}
		}

		fmt.Printf("          |_ SNMP interface: idx=%s name=%v admin=%v oper=%v\n",
			rowIndex, iface["name"], iface["admin_status"], iface["oper_status"])
		interfaces = append(interfaces, iface)
	}

	return interfaces, metrics
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

	case "physaddr":
		// Format raw bytes as MAC address xx:xx:xx:xx:xx:xx
		if b, ok := variable.Value.([]byte); ok && len(b) == 6 {
			return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", b[0], b[1], b[2], b[3], b[4], b[5])
		}
		return ""

	case "ifstatus":
		// RFC 2863 ifAdminStatus / ifOperStatus integer mapping
		var n int
		switch v := variable.Value.(type) {
		case int:
			n = v
		case uint:
			n = int(v)
		case uint32:
			n = int(v)
		}
		switch n {
		case 1:
			return "up"
		case 2:
			return "down"
		case 3:
			return "testing"
		case 4:
			return "unknown"
		case 5:
			return "dormant"
		case 6:
			return "notPresent"
		case 7:
			return "lowerLayerDown"
		default:
			return fmt.Sprintf("%d", n)
		}

	default:
		switch variable.Type {
		case gosnmp.OctetString:
			return string(variable.Value.([]byte))
		default:
			return fmt.Sprintf("%v", variable.Value)
		}
	}
}

// interfaceListToStore converts raw interface maps from OnCollect to store.InterfaceRecord.
// Called by the collection plugin.
func InterfaceListToRecords(hostKey, hostName, hostAddress string, ifaces []map[string]interface{}) []store.InterfaceRecord {
	records := make([]store.InterfaceRecord, 0, len(ifaces))
	for _, iface := range ifaces {
		rec := store.InterfaceRecord{
			HostKey:     hostKey,
			HostName:    hostName,
			HostAddress: hostAddress,
		}

		if v, ok := iface["if_index"].(string); ok {
			if n, err := strconv.Atoi(v); err == nil {
				rec.IfIndex = n
			}
		}
		if v, ok := iface["name"].(string); ok {
			rec.Name = v
		}
		if v, ok := iface["alias"].(string); ok {
			rec.Alias = v
		}
		if v, ok := iface["type"].(int); ok {
			rec.Type = v
		}
		if v, ok := iface["speed"]; ok {
			switch sv := v.(type) {
			case uint:
				s := int64(sv)
				rec.Speed = &s
			case uint32:
				s := int64(sv)
				rec.Speed = &s
			case uint64:
				s := int64(sv)
				rec.Speed = &s
			case int:
				s := int64(sv)
				rec.Speed = &s
			}
		}
		if v, ok := iface["mac_address"].(string); ok {
			rec.MACAddress = v
		}
		if v, ok := iface["admin_status"].(string); ok {
			rec.AdminStatus = v
		}
		if v, ok := iface["oper_status"].(string); ok {
			rec.OperStatus = v
		}
		records = append(records, rec)
	}
	return records
}
