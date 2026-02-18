
package plugin

// DatabaseConfig holds the connection URL for the metrics store.
// Supported URL schemes: sqlite://, postgres://, mysql://
// Leave URL empty to disable database persistence.
type DatabaseConfig struct {
	URL string `json:"url"`
}

// Config is the root configuration structure.
type Config struct {
	Hosts       map[string]Host          `json:"hosts"`
	Credentials map[string]Credential    `json:"credentials"`
	Remote      RemoteConfig             `json:"remote"`
	Perception  map[string]PerceptionEnv `json:"perception"`
	Database    DatabaseConfig           `json:"database"`
}

// Host defines a single machine to be monitored.
type Host struct {
	Address     string        `json:"address"`
	Name        string        `json:"name"`
	Collect     []CollectTask `json:"collect"`
	Credentials []string      `json:"credentials"`
}

// CollectTask defines a single collection task for a host.
type CollectTask struct {
	Metric      string `json:"metric"`
	Credentials string `json:"credentials"`
}

// Credential defines a set of credentials for accessing a device.
type Credential struct {
	User      string `json:"user"`
	Pass      string `json:"pass"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Type      string `json:"type"` // The device type, e.g., "nokia2425", "generic_snmp"
	Community string `json:"community"`
	Version   string `json:"version"` // e.g., "2c", "3"
}

// RemoteConfig holds the configuration for sending data to remote servers.
type RemoteConfig struct {
	Destinations map[string]Destination `json:"destinations"`
}

// Destination defines a single remote server endpoint.
type Destination struct {
	Endpoint string `json:"endpoint"`
	Token    string `json:"token"`
	Active   bool   `json:"active"`
}

// PerceptionEnv defines a network discovery environment.
type PerceptionEnv struct {
	Ranges    []string `json:"ranges"`
	Method    string   `json:"method"`
	Enabled   bool     `json:"enabled"`
	Detection []string `json:"detection"`
}
