package main

import (
	// By importing the plugin packages, we cause their init() functions to run,
	// which in turn register the plugins with the central registry.
	_ "observer/plugins/api"
	_ "observer/plugins/collection"
	_ "observer/plugins/local"
	_ "observer/plugins/mail"
	_ "observer/plugins/network"
	_ "observer/plugins/snmp"
	_ "observer/plugins/sshcollect"
)
