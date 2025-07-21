package main

import (
	"os"
	"strconv"
	"strings"
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
		AllowedOrigins: getAllowedOrigins(),
		CommandTimeout: getEnvIntOrDefault("OCD_COMMAND_TIMEOUT", 1800), // 30 minutes
	}
}

func getAllowedOrigins() []string {
	originsEnv := getEnvOrDefault("OCD_ALLOWED_ORIGINS", "localhost,127.0.0.1")
	if originsEnv == "*" {
		return []string{"*"} // Allow all if explicitly set
	}

	origins := strings.Split(originsEnv, ",")
	var cleanOrigins []string
	for _, origin := range origins {
		cleanOrigins = append(cleanOrigins, strings.TrimSpace(origin))
	}
	return cleanOrigins
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
