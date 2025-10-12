package textui

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss" // Re-add lipgloss

	plugin "observer/base" // Correct import for the base package
	"observer/plugins"
)

// Ensure textuiPlugin implements the plugin.Plugin interface.
var _ plugin.Plugin = (*textuiPlugin)(nil)

// textuiPlugin is the main struct for our TUI plugin.
type textuiPlugin struct {
	plugin.BasePlugin
	controller *plugin.Controller // Reference to the main controller
}

// Name returns the name of the plugin.
func (p *textuiPlugin) Name() string {
	return "textui"
}

// Init initializes the textui plugin.
func (p *textuiPlugin) Init(c *plugin.Controller) {
	p.controller = c
}

// OnCommand handles commands for the textui plugin.
func (p *textuiPlugin) OnCommand(args map[string]string) error {
	// This is the entry point for our TUI
	if args["action"] == "start" {
		// Load devices here
		devices, err := p.loadDevices()
		if err != nil {
			return fmt.Errorf("failed to load devices: %w", err)
		}

		initialModel := newModel(devices)
		if _, err := tea.NewProgram(initialModel).Run(); err != nil {
			return fmt.Errorf("failed to start TUI: %w", err)
		}
		return nil
	}
	return fmt.Errorf("unknown command for textui plugin: %s", args["action"])
}

// OnUpdate is not used for the textui plugin.
func (p *textuiPlugin) OnUpdate() error {
	return nil
}

// OnCollect is not used for the textui plugin.
func (p *textuiPlugin) OnCollect(options map[string]interface{}) (map[string]interface{}, error) {
	return nil, fmt.Errorf("OnCollect not implemented for textui plugin")
}

// --- TUI Model, Update, View ---

// Styles for the TUI
var (
	appStyle = lipgloss.NewStyle().Padding(1, 2)
	titleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFDF5")).
				Background(lipgloss.Color("#25A065")).
				Padding(0, 1)

	// Status styles
	upStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))  // Green
	downStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))   // Red
	warningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("226")) // Yellow

	itemStyle = lipgloss.NewStyle().PaddingLeft(2).Width(40)
	selectedItemStyle = lipgloss.NewStyle().
						PaddingLeft(2). // Consistent padding
						Foreground(lipgloss.Color("170")). // Green
						Background(lipgloss.Color("236")). // Dark Gray
						Bold(true).
						Reverse(true). // Indicate selection by reversing colors
						Width(40)
	detailStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), true).
				BorderForeground(lipgloss.Color("63")). // Blue
				Padding(1, 2).
				Width(60)
	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// device represents a simplified device for TUI display.
type device struct {
	plugin.Host // Embed the full Host struct
	Credential  plugin.Credential // Store the associated credential for details
	Type        string            // Redundant but useful for quick display
	Status      string            // Operational status: "up", "down", "warning"
}

// model is the Bubble Tea application model.
type model struct {
	devices        []device
	cursor         int
	selectedDevice *device
	mode           mode
	err            error
}

type mode int

const (
	modeList mode = iota
	modeDetail
)

func newModel(devs []device) model {
	return model{
		devices: devs,
		cursor:  0,
		mode:    modeList,
	}
}

// Init is the first function that will be called. It returns an optional
// initial command. To not perform an initial command, return nil.
func (m model) Init() tea.Cmd {
	return nil
}

// Update is called when messages are received. The function returns a new model
// and an optional command.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if m.mode == modeList {
				if m.cursor > 0 {
					m.cursor--
				}
			}

		case "down", "j":
			if m.mode == modeList {
				if m.cursor < len(m.devices)-1 {
					m.cursor++
				}
			}

		case "enter":
			if m.mode == modeList && len(m.devices) > 0 {
				m.selectedDevice = &m.devices[m.cursor]
				m.mode = modeDetail
			}

		case "esc":
			if m.mode == modeDetail {
				m.mode = modeList
				m.selectedDevice = nil
			}
		}
	}

	return m, nil
}

// View renders the program's UI, which is just a string.
func (m model) View() string {
	s := strings.Builder{}

	if m.err != nil {
		return appStyle.Render(fmt.Sprintf("Error: %v\n", m.err))
	}

	if m.mode == modeList {
		s.WriteString(titleStyle.Render("Device List") + "\n\n")
		for i, d := range m.devices {
			row := fmt.Sprintf("%s (%s) - %s", d.Name, d.Type, d.Address)
			
			var statusColorStyle lipgloss.Style
			switch d.Status {
			case "down":
				statusColorStyle = downStyle
			case "warning":
				statusColorStyle = warningStyle
			case "up":
				statusColorStyle = upStyle
			default:
				statusColorStyle = lipgloss.NewStyle() // No specific color if status is unknown
			}

			var finalStyle lipgloss.Style
			if m.cursor == i {
				// Start with selectedItemStyle, then apply the status color
				finalStyle = selectedItemStyle.Copy().Foreground(statusColorStyle.GetForeground())
			} else {
				// Start with itemStyle, then apply the status color
				finalStyle = itemStyle.Copy().Foreground(statusColorStyle.GetForeground())
			}
			s.WriteString(finalStyle.Render(row) + "\n")
		}
		s.WriteString(helpStyle.Render("\nPress 'q' to quit, 'enter' to view details.") + "\n")
	} else if m.mode == modeDetail && m.selectedDevice != nil {
		s.WriteString(titleStyle.Render("Device Details") + "\n\n")
		detailContent := strings.Builder{}
		detailContent.WriteString(fmt.Sprintf("Name:        %s\n", m.selectedDevice.Name))
		detailContent.WriteString(fmt.Sprintf("Address:     %s\n", m.selectedDevice.Address))
		detailContent.WriteString(fmt.Sprintf("Type:        %s\n", m.selectedDevice.Type))
		detailContent.WriteString(fmt.Sprintf("User:        %s\n", m.selectedDevice.Credential.User))
		detailContent.WriteString(fmt.Sprintf("Port:        %d\n", m.selectedDevice.Credential.Port))
		detailContent.WriteString(fmt.Sprintf("Community:   %s\n", m.selectedDevice.Credential.Community))
		detailContent.WriteString(fmt.Sprintf("SNMP Version: %s\n", m.selectedDevice.Credential.Version))
		detailContent.WriteString(fmt.Sprintf("Collect Tasks:\n"))
		for _, task := range m.selectedDevice.Collect {
			detailContent.WriteString(fmt.Sprintf("  - Metric: %s, Credentials: %s\n", task.Metric, task.Credentials))
		}
		// Add more details from plugin.Host and plugin.Credential as needed
		s.WriteString(detailStyle.Render(detailContent.String()) + "\n")
		s.WriteString(helpStyle.Render("\nPress 'esc' to go back to list, 'q' to quit.") + "\n")
	}

	return appStyle.Render(s.String())
}

// loadDevices loads device configuration from data/config.json and data/perception.json.
func (p *textuiPlugin) loadDevices() ([]device, error) {
	// This logic is adapted from plugins/collection/collection.go and plugins/api/api.go
	// to load the config and then extract hosts.

	// 1. Load Config
	configFile, err := os.ReadFile("data/config.json")
	if err != nil {
		return nil, fmt.Errorf("could not read config file: %w", err)
	}

	var cfg plugin.Config // Use plugin.Config
	if err := json.Unmarshal(configFile, &cfg); err != nil {
		return nil, fmt.Errorf("could not parse config file: %w", err)
	}

	// 2. Load and merge hosts from perception.json
	perceptionFile, err := os.ReadFile("data/perception.json")
	if err == nil { // Only try to unmarshal if file exists
		var perceptionData struct {
			Hosts map[string]plugin.Host `json:"hosts"`
		}
		if err := json.Unmarshal(perceptionFile, &perceptionData); err == nil {
			for ip, host := range perceptionData.Hosts {
				if _, exists := cfg.Hosts[ip]; !exists {
					cfg.Hosts[ip] = host // Add the host if it doesn't already exist
				}
			}
		} else {
			fmt.Fprintf(os.Stderr, "Warning: could not parse perception.json: %v\n", err)
		}
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Warning: could not read perception.json: %v\n", err)
	}

	var loadedDevices []device
	statusCycle := []string{"up", "down", "warning"}
	statusIndex := 0
	for _, host := range cfg.Hosts {
		deviceType := "unknown"
		var cred plugin.Credential
		if len(host.Credentials) > 0 {
			if c, ok := cfg.Credentials[host.Credentials[0]]; ok {
				cred = c
				deviceType = c.Type
			}
		}

		loadedDevices = append(loadedDevices, device{
			Host:       host, // Embed the full host
			Credential: cred,
			Type:       deviceType,
			Status:     statusCycle[statusIndex%len(statusCycle)], // Assign alternating status
		})
		statusIndex++
	}

	// Sort devices by name for consistent display
	sort.Slice(loadedDevices, func(i, j int) bool {
		return loadedDevices[i].Name < loadedDevices[j].Name
	})

	return loadedDevices, nil
}

// init function to register the plugin
func init() {
	plugins.Register(&textuiPlugin{})
}
