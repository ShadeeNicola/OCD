package main

import (
	"fmt"
	"log"
	"net/http"
	"runtime"
)

var appConfig *Config

func main() {
	// Load configuration
	appConfig = loadConfig()

	setupRoutes()

	fmt.Printf("Server starting on http://localhost:%s\n", appConfig.Port)
	fmt.Printf("Operating System: %s\n", runtime.GOOS)
	fmt.Printf("WSL User: %s\n", appConfig.WSLUser)
	fmt.Println("Open your browser and go to: http://localhost:" + appConfig.Port)

	log.Fatal(http.ListenAndServe(":"+appConfig.Port, nil))
}

func setupRoutes() {
	http.Handle("/", http.FileServer(http.Dir("./web/")))
	http.HandleFunc("/api/browse", handleBrowse)
	http.HandleFunc("/api/deploy", handleDeploy)
	http.HandleFunc("/ws/deploy", handleWebSocketDeploy)
}
