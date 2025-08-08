#!/bin/bash
# OCD - One Click Deployer for Customization Projects
# Detects changed services in app/backend and builds/deploys them

# Default values
NAMESPACE="dop"
SKIP_BUILD=false
SKIP_DEPLOY=false
FORCE=false
CONFIRM=false
VERBOSE=true

# Check for environment variable override
if [[ "$OCD_VERBOSE" == "true" ]]; then
    VERBOSE=true
fi

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        --skip-build)
            SKIP_BUILD=true
            shift
            ;;
        --skip-deploy)
            SKIP_DEPLOY=true
            shift
            ;;
        --force)
            FORCE=true
            shift
            ;;
        --confirm)
            CONFIRM=true
            shift
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo "Options:"
            echo "  -n, --namespace NS       Kubernetes namespace (default: dop)"
            echo "  --skip-build            Skip build phase"
            echo "  --skip-deploy           Skip deploy phase"
            echo "  --force                 Run even if no changes detected"
            echo "  --confirm               Prompt for confirmation before deployment"
            echo "  -v, --verbose           Show detailed command output"
            echo "  -h, --help              Show this help"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# =============================================================================
# LOAD SHARED FUNCTIONS
# =============================================================================

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SHARED_DIR="$SCRIPT_DIR/shared"

# Source shared functions
source "$SHARED_DIR/utils.sh"
source "$SHARED_DIR/maven.sh"

# =============================================================================
# CUSTOMIZATION-SPECIFIC FUNCTIONS
# =============================================================================

discover_customization_services() {
    local services=()
    
    # Find all directories under app/backend that have pom.xml
    if [[ -d "app/backend" ]]; then
        for service_dir in app/backend/*/; do
            if [[ -d "$service_dir" && -f "$service_dir/pom.xml" ]]; then
                local service_name=$(basename "$service_dir")
                services+=("$service_name")
            fi
        done
    fi
    
    # Remove duplicates and sort
    printf '%s\n' "${services[@]}" | sort -u
}

get_changed_customization_services() {
    local changed_files="$1"
    local detected_services=()
    
    while IFS= read -r file; do
        if [[ -z "$file" ]]; then
            continue
        fi
        
        # Check if file is in app/backend/[service]/
        if [[ "$file" =~ ^app/backend/([^/]+)/ ]]; then
            local service_name="${BASH_REMATCH[1]}"
            if [[ ! " ${detected_services[@]} " =~ " ${service_name} " ]]; then
                detected_services+=("$service_name")
            fi
        fi
    done <<< "$changed_files"
    
    # Only output service names, not status messages
    printf '%s\n' "${detected_services[@]}"
}

build_customization_service() {
    export LANG=C.UTF-8
    export LC_ALL=C.UTF-8

    local service_name="$1"
    local build_dir="app/backend/$service_name"
    
    write_colored_output "Building customization service: $service_name" "blue"
    
    if [[ ! -d "$build_dir" ]]; then
        write_colored_output "Error: Could not find directory for $service_name" "red"
        return 1
    fi
    
    # Get absolute path and convert to Windows path
    local target_wsl_path=$(realpath "$build_dir")
    local target_windows_path=$(convert_to_windows_path "$target_wsl_path")
    
    # Choose the right executable based on environment
    local ps_executable=""
    if [[ "$RUNTIME_ENV" == "WSL" ]]; then
        ps_executable="/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"
    else
        ps_executable="/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"
    fi
    
    # Build PowerShell command - use verbose output if VERBOSE is true
    local ps_command=""
    if [[ "$VERBOSE" == "true" ]]; then
        ps_command="Set-Location '$target_windows_path'; mvn clean install -DskipTests -s '$MAVEN_SETTINGS_PATH'"
    else
        ps_command="Set-Location '$target_windows_path'; mvn clean install -DskipTests -s '$MAVEN_SETTINGS_PATH' -q"
    fi
    
    log_command "$ps_command"
    
    if [[ "$VERBOSE" == "true" ]]; then
        # Show full output in verbose mode
        if "$ps_executable" -Command "$ps_command"; then
            write_colored_output "Build completed successfully for $service_name" "green"
            return 0
        else
            write_colored_output "Maven build failed for $service_name" "red"
            return 1
        fi
    else
        # Hide output in non-verbose mode
        if "$ps_executable" -Command "$ps_command" > /dev/null 2>&1; then
            write_colored_output "Build completed successfully for $service_name" "green"
            return 0
        else
            write_colored_output "Maven build failed for $service_name" "red"
            # Run again without -q to show error details
            write_colored_output "Error details:" "red"
            "$ps_executable" -Command "Set-Location '$target_windows_path'; mvn clean install -DskipTests -s '$MAVEN_SETTINGS_PATH'" 2>&1 | tail -20
            return 1
        fi
    fi
}

build_customization_metadata() {
    export LANG=C.UTF-8
    export LC_ALL=C.UTF-8

    write_colored_output "Building customization metadata..." "blue"
    
    local build_dir="app/metadata"
    
    if [[ ! -d "$build_dir" ]]; then
        write_colored_output "Error: Metadata directory $build_dir not found" "red"
        return 1
    fi
    
    # Get absolute path and convert to Windows path
    local target_wsl_path=$(realpath "$build_dir")
    local target_windows_path=$(convert_to_windows_path "$target_wsl_path")
    
    # Choose the right executable based on environment
    local ps_executable=""
    if [[ "$RUNTIME_ENV" == "WSL" ]]; then
        ps_executable="/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"
    else
        ps_executable="/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"
    fi
    
    # Build PowerShell command - use verbose output if VERBOSE is true
    local ps_command=""
    if [[ "$VERBOSE" == "true" ]]; then
        ps_command="Set-Location '$target_windows_path'; mvn clean install -DskipTests -s '$MAVEN_SETTINGS_PATH'"
    else
        ps_command="Set-Location '$target_windows_path'; mvn clean install -DskipTests -s '$MAVEN_SETTINGS_PATH' -q"
    fi
    
    log_command "$ps_command"
    
    if [[ "$VERBOSE" == "true" ]]; then
        # Show full output in verbose mode
        if "$ps_executable" -Command "$ps_command"; then
            write_colored_output "Metadata build completed successfully" "green"
            return 0
        else
            write_colored_output "Metadata build failed" "red"
            return 1
        fi
    else
        # Hide output in non-verbose mode
        if "$ps_executable" -Command "$ps_command" > /dev/null 2>&1; then
            write_colored_output "Metadata build completed successfully" "green"
            return 0
        else
            write_colored_output "Metadata build failed" "red"
            # Run again without -q to show error details
            write_colored_output "Error details:" "red"
            "$ps_executable" -Command "Set-Location '$target_windows_path'; mvn clean install -DskipTests -s '$MAVEN_SETTINGS_PATH'" 2>&1 | tail -20
            return 1
        fi
    fi
}

build_customization_docker() {
    export LANG=C.UTF-8
    export LC_ALL=C.UTF-8

    write_colored_output "Building customization Docker images..." "blue"
    
    local build_dir="dockers/customization-jars"
    
    if [[ ! -d "$build_dir" ]]; then
        write_colored_output "Error: Docker directory $build_dir not found" "red"
        return 1
    fi
    
    # Get absolute path and convert to Windows path
    local target_wsl_path=$(realpath "$build_dir")
    local target_windows_path=$(convert_to_windows_path "$target_wsl_path")
    
    # Choose the right executable based on environment
    local ps_executable=""
    if [[ "$RUNTIME_ENV" == "WSL" ]]; then
        ps_executable="/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"
    else
        ps_executable="/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"
    fi
    
    # Build PowerShell command - use verbose output if VERBOSE is true
    local ps_command=""
    if [[ "$VERBOSE" == "true" ]]; then
        ps_command="Set-Location '$target_windows_path'; mvn clean install -DskipTests -s '$MAVEN_SETTINGS_PATH'"
    else
        ps_command="Set-Location '$target_windows_path'; mvn clean install -DskipTests -s '$MAVEN_SETTINGS_PATH' -q"
    fi
    
    log_command "$ps_command"
    
    if [[ "$VERBOSE" == "true" ]]; then
        # Show full output in verbose mode
        if "$ps_executable" -Command "$ps_command"; then
            write_colored_output "Docker build completed successfully" "green"
            return 0
        else
            write_colored_output "Docker build failed" "red"
            return 1
        fi
    else
        # Hide output in non-verbose mode
        if "$ps_executable" -Command "$ps_command" > /dev/null 2>&1; then
            write_colored_output "Docker build completed successfully" "green"
            return 0
        else
            write_colored_output "Docker build failed" "red"
            # Run again without -q to show error details
            write_colored_output "Error details:" "red"
            "$ps_executable" -Command "Set-Location '$target_windows_path'; mvn clean install -DskipTests -s '$MAVEN_SETTINGS_PATH'" 2>&1 | tail -20
            return 1
        fi
    fi
}

deploy_customization_service() {
    local service_name="$1"
    local namespace="$2"
    
    write_colored_output "Deploying customization service: $service_name" "yellow"
    
    # TODO: Implement the specific patch command for customization
    write_colored_output "Customization deployment for $service_name - TBD (patch command not implemented yet)" "yellow"
    
    return 0
}

# =============================================================================
# MAIN EXECUTION
# =============================================================================

write_colored_output "OCD - One Click Deployer for Customization Projects" "cyan"
if [[ "$VERBOSE" == "true" ]]; then
    write_colored_output "Verbose mode enabled - showing all command outputs" "yellow"
fi
echo

# Check prerequisites
test_prerequisites

# Perform connection checks and prerequisites
perform_connection_checks

echo

# Get Maven settings
get_maven_settings

# Auto-update Docker settings (host IP and tag)
auto_update_docker_settings

echo
write_colored_output "    EXECUTION PLAN" "cyan"

# Get changed files
changed_files_output=$(get_changed_files)

# Convert to proper array - filter out empty lines
changed_files=()
while IFS= read -r file; do
    if [[ -n "${file// }" ]]; then
        changed_files+=("$file")
    fi
done <<< "$changed_files_output"

if [[ ${#changed_files[@]} -eq 0 && "$FORCE" != "true" ]]; then
    write_colored_output "No changes detected. Use --force to run anyway." "yellow"
    exit 0
fi

# Detect changed customization services
changed_services_output=$(get_changed_customization_services "$changed_files_output")

# Convert to proper array - filter out empty lines
changed_services=()
while IFS= read -r service; do
    if [[ -n "${service// }" ]]; then
        changed_services+=("$service")
    fi
done <<< "$changed_services_output"

# Show detected services
if [[ ${#changed_services[@]} -gt 0 ]]; then
    for service in "${changed_services[@]}"; do
        write_colored_output "${service}" "green"
    done
else
    write_colored_output "No customization service changes detected" "yellow"
fi

if [[ ${#changed_services[@]} -eq 0 && "$FORCE" != "true" ]]; then
    write_colored_output "No customization service changes detected. Use --force to run anyway." "yellow"
    exit 0
fi

# Confirmation prompt if requested
if [[ "$CONFIRM" == "true" ]]; then
    confirm_deployment "${changed_services[@]}"
fi

declare -A build_results
declare -A deploy_results

# Build phase
if [[ "$SKIP_BUILD" != "true" ]]; then
    echo
    write_colored_output "BUILDING CUSTOMIZATION SERVICES..." "magenta"
    
    # Build changed services
    for service in "${changed_services[@]}"; do
        if [[ -n "$service" ]]; then
            if build_customization_service "$service"; then
                build_results["$service"]="success"
            else
                build_results["$service"]="failed"
                write_colored_output "Build failed for $service. Aborting execution." "red"
                exit 1
            fi
        fi
    done
    
    # Always build metadata
    write_colored_output "Building metadata..." "blue"
    if build_customization_metadata; then
        build_results["metadata"]="success"
    else
        build_results["metadata"]="failed"
        write_colored_output "Metadata build failed. Aborting execution." "red"
        exit 1
    fi
    
    # Always build Docker
    write_colored_output "Building Docker images..." "blue"
    if build_customization_docker; then
        build_results["docker"]="success"
    else
        build_results["docker"]="failed"
        write_colored_output "Docker build failed. Aborting execution." "red"
        exit 1
    fi
fi

# Deploy phase
if [[ "$SKIP_DEPLOY" != "true" ]]; then
    echo
    write_colored_output "DEPLOYING CUSTOMIZATION SERVICES..." "magenta"
    
    for service in "${changed_services[@]}"; do
        if [[ -n "$service" ]]; then
            if [[ "$SKIP_BUILD" == "true" || "${build_results[$service]}" == "success" ]]; then
                if deploy_customization_service "$service" "$NAMESPACE"; then
                    deploy_results["$service"]="success"
                else
                    deploy_results["$service"]="failed"
                fi
            else
                write_colored_output "Skipping deployment of $service due to build failure" "red"
                deploy_results["$service"]="skipped"
            fi
        fi
    done
fi

# Summary
echo
write_colored_output "═══════════════════════════════════════════════════════════════" "cyan"
write_colored_output "                CUSTOMIZATION EXECUTION SUMMARY                " "cyan"
write_colored_output "═══════════════════════════════════════════════════════════════" "cyan"

success_count=0
total_count=${#changed_services[@]}

for service in "${changed_services[@]}"; do
    if [[ -n "$service" ]]; then
        build_status="SKIPPED"
        deploy_status="SKIPPED"
        
        if [[ "$SKIP_BUILD" != "true" ]]; then
            build_status="${build_results[$service]^^}"
        fi
        
        if [[ "$SKIP_DEPLOY" != "true" ]]; then
            deploy_status="${deploy_results[$service]^^}"
        fi
        
        # Format the service name with padding
        local formatted_service=$(printf "%-20s" "$service")
        
        if [[ ("$build_status" == "SUCCESS" || "$build_status" == "SKIPPED") && ("$deploy_status" == "SUCCESS" || "$deploy_status" == "SKIPPED") ]]; then
            write_colored_output "    $formatted_service │ Build: $build_status │ Deploy: $deploy_status" "green"
            ((success_count++))
        else
            write_colored_output "    $formatted_service │ Build: $build_status │ Deploy: $deploy_status" "red"
        fi
    fi
done

# Show metadata and docker status
if [[ "$SKIP_BUILD" != "true" ]]; then
    write_colored_output "    metadata            │ Build: ${build_results[metadata]^^} │ Deploy: N/A" "green"
    write_colored_output "    docker              │ Build: ${build_results[docker]^^} │ Deploy: N/A" "green"
fi

echo
write_colored_output "═══════════════════════════════════════════════════════════════" "cyan"
if [[ $success_count -eq $total_count ]]; then
    write_colored_output "     SUCCESS: $success_count/$total_count customization services processed successfully!" "green"
    write_colored_output "═══════════════════════════════════════════════════════════════" "cyan"
    exit 0
else
    write_colored_output "      PARTIAL: $success_count/$total_count customization services processed successfully" "yellow"
    write_colored_output "═══════════════════════════════════════════════════════════════" "cyan"
    exit 1
fi


