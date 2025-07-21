package main

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

var appConfig *Config

func main() {
	startTime := time.Now()
	fmt.Printf("[%s] Application starting...\n", time.Now().Format("15:04:05.000"))

	// Get embedded web filesystem
	fmt.Printf("[%s] Loading embedded filesystem...\n", time.Now().Format("15:04:05.000"))
	webFS := GetWebFS()
	fmt.Printf("[%s] Filesystem loaded in %v\n", time.Now().Format("15:04:05.000"), time.Since(startTime))

	// Serve embedded static files
	http.Handle("/", http.FileServer(http.FS(webFS)))

	// Initialize logger first
	fmt.Printf("[%s] Initializing logger...\n", time.Now().Format("15:04:05.000"))
	initLogger()
	fmt.Printf("[%s] Logger initialized in %v\n", time.Now().Format("15:04:05.000"), time.Since(startTime))

	// Load configuration
	fmt.Printf("[%s] Loading configuration...\n", time.Now().Format("15:04:05.000"))
	appConfig = loadConfig()
	initCommandExecutor()
	fmt.Printf("[%s] Configuration loaded in %v\n", time.Now().Format("15:04:05.000"), time.Since(startTime))

	// Your existing handlers
	fmt.Printf("[%s] Setting up routes...\n", time.Now().Format("15:04:05.000"))
	http.HandleFunc("/api/browse", handleBrowse)
	http.HandleFunc("/api/deploy", handleDeploy)
	http.HandleFunc("/api/health", handleHealth)
	http.HandleFunc("/ws/deploy", handleWebSocketDeploy)
	fmt.Printf("[%s] Routes configured in %v\n", time.Now().Format("15:04:05.000"), time.Since(startTime))

	fmt.Printf("[%s] Server ready to start in %v\n", time.Now().Format("15:04:05.000"), time.Since(startTime))
	fmt.Printf("Server starting on http://localhost:%s\n", appConfig.Port)
	fmt.Printf("Operating System: %s\n", runtime.GOOS)
	fmt.Printf("WSL User: %s\n", appConfig.WSLUser)

	// Auto-open browser after a short delay
	go func() {
		time.Sleep(500 * time.Millisecond) // Reduced from 1 second
		fmt.Printf("[%s] Opening browser...\n", time.Now().Format("15:04:05.000"))
		openBrowser("http://localhost:" + appConfig.Port)
	}()

	fmt.Printf("[%s] Starting HTTP server...\n", time.Now().Format("15:04:05.000"))
	log.Fatal(http.ListenAndServe(":"+appConfig.Port, nil))
}

func setupRoutes() {
	http.Handle("/", http.FileServer(http.Dir("./web/")))
	http.HandleFunc("/api/browse", handleBrowse)
	http.HandleFunc("/api/deploy", handleDeploy)
	http.HandleFunc("/api/health", handleHealth)
	http.HandleFunc("/ws/deploy", handleWebSocketDeploy)
	appLogger.Info("Routes configured successfully")
}

// openBrowser opens the default browser to the specified URL
func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		fmt.Printf("Please open your browser and go to: %s\n", url)
		return
	}

	if err != nil {
		fmt.Printf("Could not open browser automatically. Please go to: %s\n", url)
	}
}
