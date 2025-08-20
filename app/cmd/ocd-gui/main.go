package main

import (
    "fmt"
    "log"
    "net/http"
    "runtime"
    "time"
    "os/exec"

    cfgpkg "app/internal/config"
    httpapi "app/internal/http"
    "app/internal/jenkins"
    "app/internal/ui"
    "app/internal/executor"
    "app/internal/logging"
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

    http.Handle("/", http.FileServer(http.FS(webFS)))

    logger := logging.New()

    fmt.Printf("[%s] Loading configuration...\n", time.Now().Format("15:04:05.000"))
    cfg := cfgpkg.Load()
    runner := executor.NewRunner(executor.NewCommandExecutor(cfg))
    jenkinsClient, err := jenkins.NewClient(cfg.Jenkins)
    if err != nil {
        log.Fatalf("Failed to create Jenkins client: %v", err)
    }
    fmt.Printf("[%s] Configuration loaded in %v\n", time.Now().Format("15:04:05.000"), time.Since(startTime))

    fmt.Printf("[%s] Setting up routes...\n", time.Now().Format("15:04:05.000"))
    http.HandleFunc("/api/browse", httpapi.HandleBrowse)
    http.HandleFunc("/api/deploy", httpapi.HandleDeploy(runner))
    http.HandleFunc("/api/health", httpapi.HandleHealth)
    
    // Jenkins routes using new service architecture
    jenkinsHandlers := httpapi.NewJenkinsHandlers(jenkinsClient)
    jenkinsHandlers.RegisterJenkinsRoutes(http.DefaultServeMux)
    
    // AWS EKS routes
    http.HandleFunc("/api/eks/clusters", httpapi.HandleEKSClusters)
    
    // Git routes
    http.HandleFunc("/api/git/branches/customization", httpapi.HandleGitBranchesCustomization)
    
    // RN Creation routes
    http.HandleFunc("/api/rn/create", httpapi.HandleRNCreate)
    
    // SSE-based deployment routes
    http.HandleFunc("/api/deploy/start", httpapi.HandleDeployStart(cfg, runner))
    http.HandleFunc("/api/deploy/stream/", httpapi.HandleDeployStream)
    http.HandleFunc("/api/deploy/cancel/", httpapi.HandleDeployCancel)
    
    logger.Info("Routes configured successfully")

    fmt.Printf("[%s] Server ready to start in %v\n", time.Now().Format("15:04:05.000"), time.Since(startTime))
    fmt.Printf("Server starting on http://localhost:%s\n", cfg.Port)
    fmt.Printf("Operating System: %s\n", runtime.GOOS)
    fmt.Printf("WSL User: %s\n", cfg.WSLUser)

    go func() {
        time.Sleep(500 * time.Millisecond)
        openBrowser("http://localhost:" + cfg.Port)
    }()

    log.Fatal(http.ListenAndServe("127.0.0.1:"+cfg.Port, nil))
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
    if err != nil { fmt.Printf("Could not open browser automatically. Please go to: %s\n", url) }
}

func execCommand(name string, arg ...string) error {
    cmd := exec.Command(name, arg...)
    return cmd.Start()
}


