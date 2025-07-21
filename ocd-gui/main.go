package main

import (
	"fmt"
	"net/http"
	"runtime"
)

var appConfig *Config

func main() {
	// Initialize logger first
	initLogger()

	// Load configuration
	appConfig = loadConfig()
	initCommandExecutor()
	appLogger.Infof("Configuration loaded - Port: %s, WSL User: %s", appConfig.Port, appConfig.WSLUser)

	setupRoutes()

	appLogger.Infof("Server starting on http://localhost:%s", appConfig.Port)
	appLogger.Infof("Operating System: %s", runtime.GOOS)
	fmt.Printf("Server starting on http://localhost:%s\n", appConfig.Port)
	fmt.Printf("Operating System: %s\n", runtime.GOOS)
	fmt.Printf("WSL User: %s\n", appConfig.WSLUser)
	fmt.Println("Open your browser and go to: http://localhost:" + appConfig.Port)

	appLogger.Error("Server stopped:", http.ListenAndServe(":"+appConfig.Port, nil))
}

func setupRoutes() {
	http.Handle("/", http.FileServer(http.Dir("./web/")))
	http.HandleFunc("/api/browse", handleBrowse)
	http.HandleFunc("/api/deploy", handleDeploy)
	http.HandleFunc("/api/health", handleHealth)
	http.HandleFunc("/ws/deploy", handleWebSocketDeploy)
	appLogger.Info("Routes configured successfully")
}
