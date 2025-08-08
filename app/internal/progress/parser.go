package progress

import (
    "fmt"
    "regexp"
    "strings"
)

func ParseProgressFromOutput(line string) *ProgressUpdate {
    line = strings.TrimSpace(line)
    cleanLine := removeAnsiEscapes(line)

    if strings.Contains(cleanLine, "Performing connection checks and prerequisites") {
        return &ProgressUpdate{Type: "progress", Stage: "prerequisites", Status: "running", Message: "Connection Checks & Prerequisites"}
    }
    if strings.Contains(cleanLine, "All prerequisites checks passed!") {
        details := extractDetailsFromLine(cleanLine)
        return &ProgressUpdate{Type: "progress", Stage: "prerequisites", Status: "success", Message: "Connection Checks & Prerequisites", Details: details}
    }
    if strings.Contains(cleanLine, "Prerequisites check failed") {
        return &ProgressUpdate{Type: "progress", Stage: "prerequisites", Status: "error", Message: "Connection Checks & Prerequisites"}
    }

    if strings.Contains(cleanLine, "Maven Settings XML Updated") {
        details := extractDetailsFromLine(cleanLine)
        return &ProgressUpdate{Type: "progress", Stage: "settings", Status: "success", Message: "Maven Settings XML Update", Details: details}
    }
    if strings.Contains(cleanLine, "Building microservice:") {
        service := extractServiceName(cleanLine, "Building microservice:")
        return &ProgressUpdate{Type: "progress", Stage: "build", Service: service, Status: "running", Message: fmt.Sprintf("Building %s", service)}
    }
    if strings.Contains(cleanLine, "Build completed successfully for") {
        service := extractServiceName(cleanLine, "Build completed successfully for")
        return &ProgressUpdate{Type: "progress", Stage: "build", Service: service, Status: "success", Message: fmt.Sprintf("Build completed for %s", service)}
    }
    if strings.Contains(cleanLine, "BUILD FAILURE") || strings.Contains(cleanLine, "Build failed for") || strings.Contains(cleanLine, "Failed to execute goal") || strings.Contains(cleanLine, "Compilation failure") {
        service := ""
        if strings.Contains(cleanLine, "Build failed for") { service = extractServiceName(cleanLine, "Build failed for") }
        if service != "" {
            return &ProgressUpdate{Type: "progress", Stage: "build", Service: service, Status: "error", Message: fmt.Sprintf("Build failed for %s", service), Details: cleanLine}
        }
        return &ProgressUpdate{Type: "progress", Stage: "build", Status: "error", Message: "Building Microservices"}
    }

    if strings.Contains(cleanLine, "Building") && strings.Contains(cleanLine, "---") {
        if idx := strings.Index(cleanLine, "Building "); idx != -1 {
            remaining := cleanLine[idx+9:]
            if endIdx := strings.Index(remaining, " "); endIdx != -1 {
                service := remaining[:endIdx]
                return &ProgressUpdate{Type: "progress", Stage: "build", Service: service, Status: "running", Message: fmt.Sprintf("Maven building %s", service)}
            }
        }
    }

    if strings.Contains(cleanLine, "DOCKER>") {
        if strings.Contains(cleanLine, "Step") {
            return &ProgressUpdate{Type: "progress", Stage: "deploy", Status: "running", Message: "Building Docker image", Details: extractDockerStep(cleanLine)}
        }
        if strings.Contains(cleanLine, "Successfully built") {
            return &ProgressUpdate{Type: "progress", Stage: "deploy", Status: "running", Message: "Docker image built successfully", Details: extractDockerImageId(cleanLine)}
        }
        if strings.Contains(cleanLine, "Successfully tagged") {
            return &ProgressUpdate{Type: "progress", Stage: "deploy", Status: "running", Message: "Docker image tagged", Details: extractDockerTag(cleanLine)}
        }
    }

    if strings.Contains(cleanLine, "The push refers to repository") {
        return &ProgressUpdate{Type: "progress", Stage: "deploy", Status: "running", Message: "Pushing to registry", Details: extractRepository(cleanLine)}
    }
    if strings.Contains(cleanLine, "Pushed") && strings.Contains(cleanLine, ":") {
        return &ProgressUpdate{Type: "progress", Stage: "deploy", Status: "running", Message: "Uploading layers", Details: "Layer pushed successfully"}
    }
    if strings.Contains(cleanLine, "Pushing") && strings.Contains(cleanLine, "[") && strings.Contains(cleanLine, "]") {
        progress := extractPushProgress(cleanLine)
        return &ProgressUpdate{Type: "progress", Stage: "deploy", Status: "running", Message: "Uploading to Nexus", Details: progress}
    }

    if strings.Contains(cleanLine, "Deploying microservice:") {
        service := extractServiceName(cleanLine, "Deploying microservice:")
        return &ProgressUpdate{Type: "progress", Stage: "deploy", Service: service, Status: "running", Message: fmt.Sprintf("Deploying %s", service)}
    }
    if strings.Contains(cleanLine, "Docker image build completed successfully for") {
        service := extractServiceName(cleanLine, "Docker image build completed successfully for")
        return &ProgressUpdate{Type: "progress", Stage: "deploy", Service: service, Status: "success", Message: fmt.Sprintf("Docker image built for %s", service)}
    }
    if strings.Contains(cleanLine, "Microservice") && strings.Contains(cleanLine, "patched with new image") {
        service := extractServiceFromPatchLine(cleanLine)
        return &ProgressUpdate{Type: "progress", Stage: "patch", Service: service, Status: "success", Message: fmt.Sprintf("Microservice %s updated", service)}
    }
    if strings.Contains(cleanLine, "Error: Could not find microservice for") {
        parts := strings.Fields(cleanLine)
        for i, part := range parts {
            if part == "for" && i+1 < len(parts) {
                serviceName := parts[i+1]
                if strings.Contains(serviceName, " ") { serviceName = strings.Split(serviceName, " ")[0] }
                return &ProgressUpdate{Type: "progress", Stage: "patch", Service: serviceName, Status: "error", Message: fmt.Sprintf("Deployment failed for %s", serviceName), Details: "Microservice not found in cluster"}
            }
        }
    }
    if strings.Contains(cleanLine, "Deploy: FAILED") {
        return &ProgressUpdate{Type: "progress", Stage: "patch", Status: "error", Message: "Kubernetes Deployment", Details: "One or more deployments failed"}
    }
    if strings.Contains(cleanLine, "PARTIAL:") && strings.Contains(cleanLine, "microservices processed successfully") {
        return &ProgressUpdate{Type: "progress", Stage: "patch", Status: "error", Message: "Kubernetes Deployment", Details: "Partial deployment - some services failed"}
    }
    return nil
}

func removeAnsiEscapes(text string) string {
    ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*m`)
    return ansiRegex.ReplaceAllString(text, "")
}

func extractDockerStep(line string) string {
    if idx := strings.Index(line, "Step "); idx != -1 {
        remaining := line[idx:]
        if endIdx := strings.Index(remaining, ":"); endIdx != -1 {
            return remaining[:endIdx]
        }
    }
    return ""
}

func extractDockerImageId(line string) string {
    if idx := strings.Index(line, "Successfully built "); idx != -1 {
        remaining := line[idx+19:]
        parts := strings.Fields(remaining)
        if len(parts) > 0 { return "Image ID: " + parts[0] }
    }
    return ""
}

func extractDockerTag(line string) string {
    if idx := strings.Index(line, "Successfully tagged "); idx != -1 {
        remaining := line[idx+20:]
        parts := strings.Fields(remaining)
        if len(parts) > 0 { return parts[0] }
    }
    return ""
}

func extractRepository(line string) string {
    if idx := strings.Index(line, "["); idx != -1 {
        if endIdx := strings.Index(line[idx:], "]"); endIdx != -1 {
            return line[idx+1 : idx+endIdx]
        }
    }
    return ""
}

func extractPushProgress(line string) string {
    if idx := strings.Index(line, "["); idx != -1 {
        if endIdx := strings.Index(line[idx:], "]"); endIdx != -1 {
            progressPart := line[idx : idx+endIdx+1]
            remaining := line[idx+endIdx+1:]
            parts := strings.Fields(remaining)
            if len(parts) > 0 { return progressPart + " " + parts[0] }
            return progressPart
        }
    }
    return ""
}

func extractServiceName(line, prefix string) string {
    if idx := strings.Index(line, prefix); idx != -1 {
        remaining := strings.TrimSpace(line[idx+len(prefix):])
        parts := strings.Fields(remaining)
        if len(parts) > 0 { return removeAnsiEscapes(parts[0]) }
    }
    return ""
}

func extractServiceFromPatchLine(line string) string {
    if strings.Contains(line, "Microservice") && strings.Contains(line, "patched") {
        parts := strings.Fields(line)
        for i, part := range parts {
            if part == "Microservice" && i+1 < len(parts) { return removeAnsiEscapes(parts[i+1]) }
        }
    }
    return ""
}

func extractDetailsFromLine(line string) string {
    if strings.Contains(line, "(") && strings.Contains(line, ")") {
        start := strings.Index(line, "(")
        end := strings.LastIndex(line, ")")
        if start != -1 && end != -1 && end > start { return line[start+1 : end] }
    }
    return ""
}


