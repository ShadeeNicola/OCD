package main

import (
	"fmt"
	"log"
	"net/http"
	"runtime"
)

func main() {
	setupRoutes()

	port := "8080"
	fmt.Printf("Server starting on http://localhost:%s\n", port)
	fmt.Printf("Operating System: %s\n", runtime.GOOS)
	fmt.Println("Open your browser and go to: http://localhost:8080")

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func setupRoutes() {
	http.Handle("/", http.FileServer(http.Dir("./web/")))
	http.HandleFunc("/api/browse", handleBrowse)
	http.HandleFunc("/api/deploy", handleDeploy)
	http.HandleFunc("/ws/deploy", handleWebSocketDeploy)
}
