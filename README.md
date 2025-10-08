# NORD - Network Observer and Remote Discovery

Nord is a simple monitoring and collection tool. It's designed to be a plugin-based system for gathering data from various sources (local system, network devices via SSH/SNMP, etc.) and can send this data to remote endpoints.

## Features

*   **Plugin-based Architecture**: Easily extendable with new data collection modules.
*   **Data Collection (`--collect`)**: Gathers metrics from configured hosts and plugins, storing results in `data/collection.json`.
*   **Network Perception (`--perception`)**: Discovers hosts on the network using `nmap` and identifies available services, storing results in `data/perception.json`.
*   **Remote Data Sending (`--remote`)**: Sends collected data to configured remote API endpoints.
*   **Local System Monitoring**: Collects CPU, memory, and uptime metrics.
*   **Network Checks**: Performs ping, SSH port, and URL availability checks.
*   **SSH Collection**: Connects to devices via SSH, runs commands, and parses output based on device-specific definitions.
*   **Mail Server Monitoring**: Gathers Postfix mail queue and service status.
*   **SNMP Collection**: Queries network devices via SNMP for specified OIDs.

## Installation

### Prerequisites

*   **Go**: Go 1.16 or higher.
*   **nmap**: For the network perception feature (`--perception`). Install via your system's package manager (e.g., `sudo apt install nmap` on Debian/Ubuntu, `brew install nmap` on macOS).
*   **sudo**: Some features (like `nmap` and Postfix control) require `sudo` privileges.

### Steps

1.  **Navigate to the project directory**:
    ```bash
    cd observer
    ```
2.  **Install Go Modules**: This will download all necessary dependencies.
    ```bash
    go mod tidy
    ```

## Configuration

The tool relies on `data/config.json` for its operational parameters.

### `data/config.json` Structure

```json
{
    "lang": "en",
    "debug": 0,
    "remote": {
        "destinations": {
            "primary_server": {
                "endpoint": "http://your-remote-server.com/api/endpoint",
                "token": "YOUR_SECRET_TOKEN",
                "active": true
            }
        }
    },
    "perception": {
        "local_network": {
            "ranges": ["192.168.1.0/24"],
            "method": "nmap",
            "enabled": true,
            "detection": ["network.ping", "network.ssh"]
        }
    },
    "hosts": {
        "internet": {
            "address": "8.8.8.8",
            "name": "Internet",
            "collect": [
                {"metric": "network.ping"}
            ]
        },
        "router": {
            "address": "192.168.1.254",
            "name": "Router",
            "collect": [
                {"metric": "network.ping"},
                {"metric": "network.ssh"},
                {"metric": "sshcollect", "credentials": "router"},
                {"metric": "snmpcollect", "credentials": "router_snmp"}
            ]
        }
    },
    "credentials": {
        "router": {
            "user": "admin",
            "pass": "admin",
            "host": "192.168.1.254",
            "port": 22,
            "type": "nokia2425"
        },
        "router_snmp": {
            "host": "192.168.1.254",
            "port": 161,
            "type": "generic",
            "community": "public",
            "version": "2c"
        }
    }
}
```

*   **`remote`**: Defines endpoints for sending collected data.
*   **`perception`**: Configures network discovery scans.
*   **`hosts`**: Lists devices to monitor and the collection tasks for each.
*   **`credentials`**: Stores sensitive access information for devices.

### Device Definitions

*   **SSH Devices**: SSH command sequences and parsing rules are defined in JSON files located in `observer/plugins/sshcollect/devices/` (e.g., `nokia2425.json`).
*   **SNMP Devices**: SNMP OID definitions are in JSON files located in `observer/plugins/snmp/devices/` (e.g., `generic.json`).

## Usage

Run the tool from the `observer/` directory.

```bash
cd observer
```

### Main Actions

*   **Collect All Data**: Runs all configured collection tasks for all hosts.
    ```bash
    go run . --collect
    ```
*   **Run Network Perception**: Discovers hosts on the network.
    ```bash
    go run . --perception
    ```
*   **Send Data Remotely**: Sends the contents of `data/collection.json` to configured remote endpoints.
    ```bash
    go run . --remote
    ```

### Plugin-Specific Commands

You can also run specific actions on individual plugins:

```bash
# Example: Run a specific action on the mail plugin
go run . -p mail -a pause

# Example: Run a specific action on the network plugin (e.g., perception)
go run . -p network -a perception
```

## Extending the Tool (Adding New Plugins)

1.  Create a new directory for your plugin under `observer/plugins/` (e.g., `observer/plugins/myplugin`).
2.  Create a `.go` file inside your plugin directory (e.g., `myplugin.go`).
3.  Define a struct that embeds `plugin.BasePlugin` and implements the `plugin.Plugin` interface.
4.  Add an `init()` function to your plugin file that calls `plugins.Register(&MyPlugin{})`.
5.  Add an import for your new plugin package in `observer/plugins.go` (e.g., `_ "observer/plugins/myplugin"`).
6.  Run `go mod tidy` to ensure dependencies are updated.

## Troubleshooting

*   **`go: go.mod file not found`**: Run `go mod init observer` in the `observer/` directory.
*   **`no required module provides package ...`**: Run `go mod tidy`.
*   **`nmap` not found**: Ensure `nmap` is installed and in your system's PATH.
*   **`sudo` password prompt**: Some commands require `sudo` privileges.
*   **`panic: interface conversion`**: Check your `data/config.json` for missing or incorrectly formatted fields.