package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	plugin "observer/base"
	"observer/plugins"
	// Import all plugins - they self-register via init()
	_ "observer/plugins/api"
	_ "observer/plugins/collection"
	_ "observer/plugins/device"
	_ "observer/plugins/flow"
	_ "observer/plugins/local"
	_ "observer/plugins/mail"
	_ "observer/plugins/network"
	_ "observer/plugins/periscope"
	_ "observer/plugins/snmp"
	_ "observer/plugins/sshcollect"
	_ "observer/plugins/textui"
	_ "observer/plugins/wasm"
	"observer/store"
)

type Server struct {
	controller *plugin.Controller
	config     map[string]interface{}
}

func main() {
	port := flag.String("port", "8080", "HTTP server port")
	flag.Parse()

	// Create controller
	controller := plugin.NewController()

	// Open database store if configured
	if cfgData, err := os.ReadFile("data/config.json"); err == nil {
		var dbCfg struct {
			Database plugin.DatabaseConfig `json:"database"`
		}
		if json.Unmarshal(cfgData, &dbCfg) == nil && dbCfg.Database.URL != "" {
			st, err := store.Open(dbCfg.Database.URL)
			if err != nil {
				log.Printf("Warning: could not open database: %v", err)
			} else if st != nil {
				controller.Store = st
				defer st.Close()
				log.Printf("Database connected: %s", dbCfg.Database.URL)
			}
		}
	}

	// Register all plugins
	for _, p := range plugins.All {
		controller.AddPlugin(p)
		log.Printf("Registered plugin: %s", p.Name())
	}

	// Load config as raw JSON
	var config map[string]interface{}
	if cfgData, err := os.ReadFile("data/config.json"); err == nil {
		json.Unmarshal(cfgData, &config)
	}

	server := &Server{
		controller: controller,
		config:     config,
	}

	// Serve static files from cmd/nordui/static (takes priority)
	http.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("cmd/nordui/static/css"))))
	http.Handle("/fontawesome/", http.StripPrefix("/fontawesome/", http.FileServer(http.Dir("cmd/nordui/static/fontawesome"))))
	http.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir("cmd/nordui/static/images"))))
	http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("cmd/nordui/static/js"))))
	
	// Serve themes from tdc/frontend
	http.Handle("/themes/", http.StripPrefix("/themes/", http.FileServer(http.Dir("tdc/frontend/themes"))))

	// Main page
	http.HandleFunc("/", server.handleIndex)

	// API endpoints
	http.HandleFunc("/api.php", server.handleAPI)
	http.HandleFunc("/backend/data/", server.handleDataFiles)

	addr := fmt.Sprintf(":%s", *port)
	log.Printf("Nord UI starting on http://localhost:%s", *port)
	log.Printf("Serving frontend from tdc/frontend")
	log.Printf("Data from data/ directory")
	log.Fatal(http.ListenAndServe(addr, nil))
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	// Get plugin and page from query params (like PHP does)
	pluginName := r.URL.Query().Get("plugin")
	page := r.URL.Query().Get("page")
	deviceID := r.URL.Query().Get("device_id")

	if pluginName == "" {
		pluginName = "device"
	}
	if page == "" {
		page = "list"
	}

	// Get the plugin
	p := s.controller.Plugins[strings.ToLower(pluginName)]
	if p == nil {
		http.Error(w, "Plugin not found", http.StatusNotFound)
		return
	}

	// Call ShowPage
	params := map[string]string{
		"plugin":    pluginName,
		"page":      page,
		"device_id": deviceID,
	}

	content, err := p.ShowPage(params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Wrap in main template
	s.renderPage(w, content)
}

func (s *Server) renderPage(w http.ResponseWriter, content string) {
	// Read the main template from tdc
	templateData, err := os.ReadFile("tdc/frontend/themes/bootstrap/main.tpl")
	if err != nil {
		// Fallback to simple wrapper
		fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
<title>Nord</title>
<link rel="stylesheet" href="/fontawesome/css/all.min.css">
</head>
<body>
<nav><a href="/?plugin=device&page=list">Devices</a></nav>
%s
</body>
</html>`, content)
		return
	}

	// Load palette CSS like PHP does
	palette := "dark-blue"
	if s.config != nil {
		if p, ok := s.config["palette"].(string); ok {
			palette = p
		}
	}
	
	// Build CSS content
	var cssContent strings.Builder
	cssContent.WriteString("<style>\n")
	
	// Load base.css
	if baseCSS, err := os.ReadFile("tdc/frontend/css/base.css"); err == nil {
		cssContent.Write(baseCSS)
		cssContent.WriteString("\n")
	}
	
	// Load palette CSS
	paletteFile := fmt.Sprintf("tdc/frontend/css/color-%s.css", palette)
	if paletteCSS, err := os.ReadFile(paletteFile); err == nil {
		cssContent.Write(paletteCSS)
		cssContent.WriteString("\n")
	}
	
	cssContent.WriteString("</style>\n")
	
	// Wrap content like PHP does: <style>CSS</style><div id='content'>PLUGIN_OUTPUT</div>
	wrappedContent := cssContent.String() + "<div id='content'>" + content + "</div>"

	// Generate navigation menu from plugins
	navigationMenu := s.generateMenu()

	// Simple template replacement
	html := string(templateData)
	
	// Fix paths FIRST - handle combined patterns before individual replacements
	html = strings.Replace(html, "/{{$sitepath}}/{{$logoimg}}", "/images/TowerLogo.png", -1)
	html = strings.Replace(html, "href=\"fontawesome/", "href=\"/fontawesome/", -1)
	
	// Now do content and other replacements
	html = strings.Replace(html, "{{$content}}", wrappedContent, 1)
	html = strings.Replace(html, "{{$title}}", "Nord Monitoring", -1)
	html = strings.Replace(html, "{{$navigationmenu}}", navigationMenu, 1)
	html = strings.Replace(html, "{{$footer}}", "Nord Monitoring", 1)
	html = strings.Replace(html, "{{$pagestyle}}", "", 1)
	
	// Set remaining template variables
	html = strings.Replace(html, "{{$themepath}}", "/themes/bootstrap", -1)
	html = strings.Replace(html, "{{$sitepath}}", "", -1)
	html = strings.Replace(html, "{{$logoimg}}", "images/TowerLogo.png", -1)
	html = strings.Replace(html, "{{$favicon}}", "/images/favicon.ico", -1)
	html = strings.Replace(html, "{{$pagebefore}}", "", -1)
	html = strings.Replace(html, "{{$contentbefore}}", "", -1)
	html = strings.Replace(html, "{{$contentafter}}", "", -1)
	html = strings.Replace(html, "{{$pageafter}}", "", -1)
	html = strings.Replace(html, "{{nocache}}", "", -1)
	html = strings.Replace(html, "{{/nocache}}", "", -1)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func (s *Server) generateMenu() string {
	// Collect menus from all plugins
	allMenus := make(map[string]plugin.MenuItem)
	for _, p := range s.controller.Plugins {
		menus := p.GetMenus()
		for key, menu := range menus {
			allMenus[key] = menu
		}
	}

	// Sort by weight and generate HTML
	var output strings.Builder
	output.WriteString("<ul class=\"navbar-nav mr-auto\">\n")
	
	for _, menu := range allMenus {
		link := "#"
		if menu.URL != "" {
			link = menu.URL
		} else if menu.Plugin != "" {
			link = fmt.Sprintf("?plugin=%s&page=%s", menu.Plugin, menu.Page)
			if menu.Action != "" {
				link += fmt.Sprintf("&action=%s", menu.Action)
			}
		}

		if len(menu.Children) > 0 {
			output.WriteString("<li class=\"nav-item dropdown\">")
			output.WriteString(fmt.Sprintf("<a class=\"nav-link nav-link-primary dropdown-toggle\" role=\"button\" data-toggle=\"dropdown\" aria-haspopup=\"true\" aria-expanded=\"true\" href=\"%s\">%s</a>\n", link, menu.Text))
			output.WriteString("<div class=\"dropdown-menu\" aria-labelledby=\"navbarDropdown\">")
			
			for _, child := range menu.Children {
				childLink := "#"
				if child.URL != "" {
					childLink = child.URL
				} else if child.Plugin != "" {
					childLink = fmt.Sprintf("?plugin=%s&page=%s", child.Plugin, child.Page)
					if child.Action != "" {
						childLink += fmt.Sprintf("&action=%s", child.Action)
					}
				}
				output.WriteString(fmt.Sprintf("<a class=\"dropdown-item\" href=\"%s\">%s</a>\n", childLink, child.Text))
			}
			
			output.WriteString("</div>")
		} else {
			output.WriteString("<li class=\"nav-item\">\n")
			output.WriteString(fmt.Sprintf("<a class=\"nav-link\" href=\"%s\">%s</a>\n", link, menu.Text))
		}
		output.WriteString("</li>\n")
	}
	
	output.WriteString("</ul>\n")
	return output.String()
}

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get plugin and action from query params
	pluginName := r.URL.Query().Get("plugin")
	action := r.URL.Query().Get("action")

	if pluginName == "" {
		pluginName = "api"
	}

	// Get the plugin
	p := s.controller.Plugins[strings.ToLower(pluginName)]
	if p == nil {
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Plugin not found",
		})
		return
	}

	// Handle API plugin specially (receives remote data)
	if pluginName == "api" {
		s.handleAPIServer(w, r, action)
		return
	}

	// For other plugins, call OnCollect with action
	result, err := p.OnCollect(map[string]interface{}{
		"action": action,
	})

	if err != nil {
		json.NewEncoder(w).Encode(map[string]string{
			"error": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleAPIServer(w http.ResponseWriter, r *http.Request, action string) {
	// Handle POST requests (receiving data from remote nodes)
	if r.Method == "POST" {
		// Check authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code":  401,
				"error": "Unauthorized: Missing authorization header",
			})
			return
		}

		// Extract token
		authToken := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer"))

		// Authenticate token
		if !s.authenticate(authToken) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code":  401,
				"error": "Unauthorized: Invalid token",
			})
			return
		}

		// Parse form data
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code":  400,
				"error": "Bad request",
			})
			return
		}

		// Get JSON payload
		jsonPayload := r.FormValue("json_payload")
		if jsonPayload == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code":  400,
				"error": "Missing json_payload",
			})
			return
		}

		// Parse and store the data
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(jsonPayload), &data); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code":  400,
				"error": "Invalid JSON",
			})
			return
		}

		// Store to file
		filename := fmt.Sprintf("data/remote_%s.json", authToken)
		jsonData, _ := json.MarshalIndent(data, "", "  ")
		if err := os.WriteFile(filename, jsonData, 0644); err != nil {
			log.Printf("Error writing remote data: %v", err)
		}

		log.Printf("Received remote data from token: %s", authToken)

		// Return success
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":   200,
			"status": "success",
		})
		return
	}

	// For GET requests, return method not allowed
	w.WriteHeader(http.StatusMethodNotAllowed)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":  405,
		"error": "Method Not Allowed: Only POST requests are allowed",
	})
}

func (s *Server) authenticate(token string) bool {
	// Check if token exists in config
	if remote, ok := s.config["remote"].(map[string]interface{}); ok {
		if tokens, ok := remote["tokens"].(map[string]interface{}); ok {
			_, exists := tokens[token]
			return exists
		}
	}
	return false
}

func (s *Server) handleDataFiles(w http.ResponseWriter, r *http.Request) {
	// Map /backend/data/file.json to data/file.json
	filename := filepath.Base(r.URL.Path)
	dataPath := filepath.Join("data", filename)

	// Security check - only allow .json files
	if !strings.HasSuffix(filename, ".json") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	data, err := os.ReadFile(dataPath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}
