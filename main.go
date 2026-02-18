
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	plugin "observer/base"
	"observer/plugins"
	"observer/store"
	_ "observer/plugins/textui" // Import for side effect (plugin registration)
)

func main() {
	// Define command-line flags
	pluginName := flag.String("p", "", "Plugin to command")
	action := flag.String("a", "", "Action to perform on the plugin")
	collect := flag.Bool("collect", false, "Run data collection using the 'collection' plugin")
	perception := flag.Bool("perception", false, "Run network discovery (perception) using the 'network' plugin")
	remote := flag.Bool("remote", false, "Send collected data to remote server(s) using the 'api' plugin")
	ui := flag.Bool("ui", false, "Start the Text User Interface (TUI)")

	flag.Parse()

	// Create a new controller
	controller := plugin.NewController()

	// Open database store if configured.
	// Parse only the database section to avoid errors from complex collect fields.
	if cfgData, err := os.ReadFile("data/config.json"); err == nil {
		var dbCfg struct {
			Database plugin.DatabaseConfig `json:"database"`
		}
		if json.Unmarshal(cfgData, &dbCfg) == nil && dbCfg.Database.URL != "" {
			st, err := store.Open(dbCfg.Database.URL)
			if err != nil {
				fmt.Printf("Warning: could not open database: %v\n", err)
			} else if st != nil {
				controller.Store = st
				defer st.Close()
				fmt.Printf("Database connected: %s\n", dbCfg.Database.URL)
			}
		}
	}

	// Register all plugins that have been imported.
	for _, p := range plugins.All {
		controller.AddPlugin(p)
	}

	fmt.Println("Nord Observability, Reliability & Discovery")

	// Handle the --ui flag
	if *ui {
		err := controller.OnCommand("textui", map[string]string{"action": "start"})
		if err != nil {
			fmt.Printf("Error starting TUI: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle the --collect flag as a shortcut
	if *collect {
		err := controller.OnCommand("collection", map[string]string{"action": "collect"})
		if err != nil {
			fmt.Printf("Error during collection: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle the --perception flag
	if *perception {
		err := controller.OnCommand("network", map[string]string{"action": "perception"})
		if err != nil {
			fmt.Printf("Error during perception: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle the --remote flag
	if *remote {
		err := controller.OnCommand("api", map[string]string{"action": "send"})
		if err != nil {
			fmt.Printf("Error during remote send: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Handle plugin-specific commands
	if *pluginName != "" {
		if *action == "" {
			fmt.Println("Error: No action specified for the plugin.")
			flag.Usage()
			os.Exit(1)
		}
		args := make(map[string]string)
		args["action"] = *action
		args["args"] = flag.Arg(0)

		err := controller.OnCommand(*pluginName, args)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// If no commands were handled, print usage
	flag.Usage()
}
