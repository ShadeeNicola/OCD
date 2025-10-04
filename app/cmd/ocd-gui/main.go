package main

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	configurationpkg "app/internal/config"
	"app/internal/executor"
	httpapi "app/internal/http"
	"app/internal/jenkins"
	"app/internal/logging"
	"app/internal/ui"
)

func main() {
	startTime := time.Now()
	fmt.Printf("[%s] Application starting...\n", time.Now().Format("15:04:05.000"))

	fmt.Printf("[%s] Loading embedded filesystem...\n", time.Now().Format("15:04:05.000"))
	webFS, err := ui.GetWebFS()
	if err != nil {
		log.Fatalf("Failed to load embedded filesystem: %v", err)
	}
	fmt.Printf("[%s] Filesystem loaded in %v\n", time.Now().Format("15:04:05.000"), time.Since(startTime))

	logger := logging.New()

	fmt.Printf("[%s] Loading configuration...\n", time.Now().Format("15:04:05.000"))
	configuration := configurationpkg.Load()
	runner := executor.NewRunner(executor.NewCommandExecutor(configuration))
	jenkinsClient, err := jenkins.NewClient(configuration.Jenkins)
	if err != nil {
		log.Fatalf("Failed to create Jenkins client: %v", err)
	}
	fmt.Printf("[%s] Configuration loaded in %v\n", time.Now().Format("15:04:05.000"), time.Since(startTime))

	mux := http.NewServeMux()

	fmt.Printf("[%s] Setting up routes...\n", time.Now().Format("15:04:05.000"))
	mux.Handle("/", http.FileServer(http.FS(webFS)))
	mux.HandleFunc("/api/browse", httpapi.HandleBrowse)
	mux.HandleFunc("/api/deploy", httpapi.HandleDeploy(runner))
	mux.HandleFunc("/api/health", httpapi.HandleHealth)

	// Jenkins routes using new service architecture
	jenkinsHandlers := httpapi.NewJenkinsHandlers(configuration, jenkinsClient)
	jenkinsHandlers.RegisterJenkinsRoutes(mux)

	// AWS EKS routes
	mux.HandleFunc("/api/eks/clusters", httpapi.HandleEKSClusters)

	// Git routes
	mux.HandleFunc("/api/git/branches/customization", httpapi.HandleGitBranchesCustomization(configuration))

	// RN Creation routes
	mux.HandleFunc("/api/rn/create", httpapi.HandleRNCreate(configuration))

	// HF Adoption routes
	mux.HandleFunc("/api/hf/parse-email", httpapi.HandleHFParseEmail)
	mux.HandleFunc("/api/hf/update-pom", httpapi.HandleHFUpdatePOM)

	// SSE-based deployment routes
	mux.HandleFunc("/api/deploy/start", httpapi.HandleDeployStart(configuration, runner))
	mux.HandleFunc("/api/deploy/stream/", httpapi.HandleDeployStream)
	mux.HandleFunc("/api/deploy/cancel/", httpapi.HandleDeployCancel)
	mux.HandleFunc("/api/config/public", httpapi.HandlePublicConfig(configuration))

	logger.Info("Routes configured successfully")

	fmt.Printf("[%s] Server ready to start in %v\n", time.Now().Format("15:04:05.000"), time.Since(startTime))
	fmt.Printf("Server starting on http://localhost:%s\n", configuration.Port)
	fmt.Printf("Operating System: %s\n", runtime.GOOS)
	fmt.Printf("WSL User: %s\n", configuration.WSLUser)

	go func() {
		time.Sleep(500 * time.Millisecond)
		openBrowser("http://localhost:" + configuration.Port)
	}()

	server := &http.Server{
		Addr:              "127.0.0.1:" + configuration.Port,
		Handler:           httpapi.NewCORSMiddleware(configuration.AllowedOrigins)(mux),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       90 * time.Second,
	}

	log.Fatal(server.ListenAndServe())
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = execCommand("xdg-open", url)
	case "windows":
		err = execCommand("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		err = execCommand("open", url)
	default:
		fmt.Printf("Please open your browser and go to: %s\n", url)
		return
	}
	if err != nil {
		fmt.Printf("Could not open browser automatically. Please go to: %s\n", url)
	}
}

func execCommand(name string, arg ...string) error {
	cmd := exec.Command(name, arg...)
	return cmd.Start()
}
