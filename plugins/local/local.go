
package local

import (
	"fmt"
	"observer/base"
	"observer/plugins"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
)

// localPlugin collects metrics from the local machine.
type localPlugin struct {
	plugin.BasePlugin
}

func init() {
	plugins.Register(&localPlugin{})
}

// Name returns the plugin's name.
func (p *localPlugin) Name() string {
	return "Local"
}

// OnCollect gathers and returns local system metrics.
func (p *localPlugin) OnCollect(options map[string]interface{}) (map[string]interface{}, error) {
	metrics := make(map[string]interface{})

	// Uptime
	uptime, err := p.getUptime()
	if (err != nil) {
		metrics["uptime"] = p.errorMetric("Uptime", "system", err)
	} else {
		metrics["uptime"] = uptime
	}

	// Memory
	memTotal, memFree, swapPct, err := p.getMemory()
	if (err != nil) {
		metrics["mem_total"] = p.errorMetric("Total Memory", "memory", err)
	} else {
		metrics["mem_total"] = map[string]interface{}{"name": "Total Memory", "label": "Total", "value": memTotal, "type": "text", "category": "memory"}
		metrics["mem_free"] = map[string]interface{}{"name": "Free Memory", "label": "Free", "value": memFree, "type": "text", "category": "memory"}
		metrics["swap"] = map[string]interface{}{"name": "Swap", "label": "Swap", "value": swapPct, "type": "percent", "category": "memory"}
	}

	// Load
	load, err := p.getLoad()
	if (err != nil) {
		metrics["load"] = p.errorMetric("Load", "system", err)
	} else {
		metrics["load"] = load
	}

	return map[string]interface{}{"metrics": metrics}, nil
}

func (p *localPlugin) getUptime() (map[string]interface{}, error) {
	uptimeSeconds, err := host.Uptime()
	if err != nil {
		return nil, err
	}

	days := uptimeSeconds / (3600 * 24)
	hours := (uptimeSeconds / 3600) % 24
	minutes := (uptimeSeconds / 60) % 60
	seconds := uptimeSeconds % 60

	var uptimeStr string
	if days > 0 {
		uptimeStr = fmt.Sprintf("%d days %02d:%02d:%02d", days, hours, minutes, seconds)
	} else {
		uptimeStr = fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}

	return map[string]interface{}{"type": "text", "category": "system", "label": "Uptime", "value": uptimeStr}, nil
}

func (p *localPlugin) getMemory() (total uint64, free uint64, swapPercent float64, err error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return 0, 0, 0, err
	}
	s, err := mem.SwapMemory()
	if err != nil {
		// Don't fail if swap isn't available
		swapPercent = 0
	} else {
		swapPercent = s.UsedPercent
	}

	return v.Total / 1024 / 1024, v.Free / 1024 / 1024, swapPercent, nil
}

func (p *localPlugin) getLoad() (map[string]interface{}, error) {
	_, err := cpu.Counts(true)
	if err != nil {
		return nil, err
	}
	avg, err := load.Avg()
	if err != nil {
		return nil, err
	}

	// gopsutil load avg is already normalized for CPU count, so we just format it.
	utilization := []int{
		int(avg.Load1),
		int(avg.Load5),
		int(avg.Load15),
	}

	return map[string]interface{}{"category": "system", "type": "histogram", "label": "Load", "value": utilization}, nil
}

func (p *localPlugin) errorMetric(label, category string, err error) map[string]interface{} {
	return map[string]interface{}{
		"type":     "text",
		"category": category,
		"label":    label,
		"value":    fmt.Sprintf("Error: %v", err),
	}
}
