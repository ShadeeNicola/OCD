package main

import (
	"os"
	"strconv"
)

type Config struct {
	Port           string
	WSLUser        string
	ScriptName     string
	AllowedOrigins []string
	MaxOutputLines int
	CommandTimeout int // seconds
}

func loadConfig() *Config {
	return &Config{
		Port:           getEnvOrDefault("OCD_PORT", "8080"),
		WSLUser:        getEnvOrDefault("OCD_WSL_USER", "k8s"),
		ScriptName:     getEnvOrDefault("OCD_SCRIPT_NAME", "OCD.sh"),
		AllowedOrigins: []string{"*"}, // TODO: Make this configurable
		MaxOutputLines: getEnvIntOrDefault("OCD_MAX_OUTPUT_LINES", 1000),
		CommandTimeout: getEnvIntOrDefault("OCD_COMMAND_TIMEOUT", 1800), // 30 minutes
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
