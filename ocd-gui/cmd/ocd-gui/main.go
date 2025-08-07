package main

import (
    "fmt"
    "log"
    "net/http"
    "runtime"
    "time"
    "os/exec"

    cfgpkg "ocd-gui/internal/config"
    httpapi "ocd-gui/internal/http"
    "ocd-gui/internal/ui"
    "ocd-gui/internal/executor"
    "ocd-gui/internal/logging"
)

func main() {
    startTime := time.Now()
    fmt.Printf("[%s] Application starting...\n", time.Now().Format("15:04:05.000"))

    fmt.Printf("[%s] Loading embedded filesystem...\n", time.Now().Format("15:04:05.000"))
    webFS := ui.GetWebFS()
    fmt.Printf("[%s] Filesystem loaded in %v\n", time.Now().Format("15:04:05.000"), time.Since(startTime))

    http.Handle("/", http.FileServer(http.FS(webFS)))

    logger := logging.New()

    fmt.Printf("[%s] Loading configuration...\n", time.Now().Format("15:04:05.000"))
    cfg := cfgpkg.Load()
    executor.InitExecutor(executor.NewCommandExecutor(cfg))
    fmt.Printf("[%s] Configuration loaded in %v\n", time.Now().Format("15:04:05.000"), time.Since(startTime))

    fmt.Printf("[%s] Setting up routes...\n", time.Now().Format("15:04:05.000"))
    http.HandleFunc("/api/browse", httpapi.HandleBrowse)
    http.HandleFunc("/api/deploy", httpapi.HandleDeploy)
    http.HandleFunc("/api/health", httpapi.HandleHealth)
    http.HandleFunc("/ws/deploy", httpapi.HandleWebSocketDeploy(cfg))
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
    cmd := execCommandCreator(name, arg...)
    return cmd()
}

var execCommandCreator = func(name string, arg ...string) func() error {
    return func() error {
        cmd := execCommandFactory(name, arg...)
        return cmd()
    }
}

var execCommandFactory = func(name string, arg ...string) func() error {
    return func() error {
        cmd := newCmd(name, arg...)
        return cmd.Run()
    }
}

// thin wrapper for testability
type command interface { Run() error }

var newCmd = func(name string, arg ...string) command {
    return &realCmd{name: name, args: arg}
}

type realCmd struct { name string; args []string }

func (c *realCmd) Run() error {
    cmd := exec.Command(c.name, c.args...)
    return cmd.Start()
}


