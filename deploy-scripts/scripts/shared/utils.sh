#!/bin/bash
# Shared utility functions for OCD scripts

# =============================================================================
# ENVIRONMENT DETECTION
# =============================================================================

detect_environment() {
    if [[ -n "$WSL_DISTRO_NAME" ]] || [[ "$(uname -r)" == *microsoft* ]] || [[ "$(uname -r)" == *WSL* ]]; then
        echo "WSL"
    else
        echo "WINDOWS"
    fi
}

# Set RUNTIME_ENV variable
RUNTIME_ENV=$(detect_environment)

# =============================================================================
# UTILITY FUNCTIONS
# =============================================================================

write_colored_output() {
    local message="$1"
    local color="$2"

    case $color in
        "red") color_code='\033[31m' ;;
        "green") color_code='\033[32m' ;;
        "yellow") color_code='\033[33m' ;;
        "blue") color_code='\033[34m' ;;
        "magenta") color_code='\033[35m' ;;
        "cyan") color_code='\033[36m' ;;
        "gray") color_code='\033[37m' ;;
        *) color_code='\033[0m' ;;
    esac

    printf "%b%s%b\n" "$color_code" "$message" "\033[0m"
}

log_command() {
    local command="$1"
    if [[ "$VERBOSE" == "true" ]]; then
        write_colored_output "EXECUTING: $command" "blue"
    fi
}

log_output() {
    local output="$1"
    if [[ "$VERBOSE" == "true" ]]; then
        write_colored_output "OUTPUT:" "gray"
        echo "$output" | while IFS= read -r line; do
            write_colored_output "  $line" "gray"
        done
    fi
}

convert_to_windows_path() {
    local input_path="$1"

    if [[ "$RUNTIME_ENV" == "WSL" ]]; then
        wslpath -w "$input_path"
    else
        echo "$input_path" | sed 's|^/c/|C:\\|' | sed 's|/|\\|g'
    fi
}

# =============================================================================
# PREREQUISITE CHECKS
# =============================================================================

test_prerequisites() {
    # Check if we're in a git repository
    if [[ ! -d ".git" ]]; then
        write_colored_output "Error: Not in a git repository" "red"
        exit 1
    fi

    # Check if kubectl is available
    if ! command -v kubectl &> /dev/null; then
        write_colored_output "Error: kubectl not found" "red"
        exit 1
    fi

    # Check if mvn is available
    if ! command -v mvn &> /dev/null; then
        write_colored_output "Error: Maven not found" "red"
        exit 1
    fi

    # Verify tools are working
    mvn --version > /dev/null 2>&1
    java -version > /dev/null 2>&1
}

perform_connection_checks() {
    write_colored_output "Performing connection checks and prerequisites..." "blue"

    local all_checks_passed=true
    local cluster_name=""
    local repo_name=""

    # Check if Maven is installed
    if command -v mvn &> /dev/null; then
        local mvn_version=$(mvn --version 2>/dev/null | head -n 1 | awk '{print $3}')
        write_colored_output "✓ Maven is installed (version: $mvn_version)" "green"
    else
        write_colored_output "✗ Maven is not installed or not in PATH" "red"
        all_checks_passed=false
    fi

    # Check if kubectl is installed
    if command -v kubectl &> /dev/null; then
        local kubectl_version=$(kubectl version --client --short 2>/dev/null | awk '{print $3}')
        write_colored_output "kubectl is installed (version: $kubectl_version)" "green"

        # Check cluster connectivity
        local current_context=$(kubectl config current-context 2>/dev/null)
        if [[ $? -eq 0 && -n "$current_context" ]]; then
            # Extract cluster name from ARN if it's an AWS EKS cluster
            if [[ "$current_context" =~ arn:aws:eks:.*:cluster/(.+)$ ]]; then
                cluster_name="${BASH_REMATCH[1]}"
            else
                cluster_name="$current_context"
            fi

            write_colored_output "Connected to cluster: $current_context" "green"

            # Test actual connectivity to the cluster
            if kubectl cluster-info &> /dev/null; then
                local cluster_info=$(kubectl cluster-info 2>/dev/null | head -n 1 | grep -o 'https://[^[:space:]]*')
                write_colored_output "Cluster is reachable at: $cluster_info" "green"
            else
                write_colored_output "Cannot reach the cluster (check proxy/network)" "red"
                all_checks_passed=false
            fi
        else
            write_colored_output "No kubectl context configured" "red"
            all_checks_passed=false
        fi
    else
        write_colored_output "kubectl is not installed or not in PATH" "red"
        all_checks_passed=false
    fi

    # Check if we're in a git repository and get repo name
    if [[ -d ".git" ]]; then
        local git_branch=$(git branch --show-current 2>/dev/null)

        # Try to get repository name from git remote origin URL
        local git_remote_url=$(git remote get-url origin 2>/dev/null)
        if [[ -n "$git_remote_url" ]]; then
            # Extract repo name from URL (handles both HTTPS and SSH URLs)
            repo_name=$(basename "$git_remote_url" .git)
        else
            # Fallback to current directory name
            repo_name=$(basename "$(pwd)")
        fi

        write_colored_output "Git repository detected (repo: $repo_name, branch: $git_branch)" "green"
    else
        write_colored_output "Not in a git repository" "red"
        all_checks_passed=false
    fi

    if [[ "$all_checks_passed" == "false" ]]; then
        write_colored_output "Prerequisites check failed. Please fix the issues above." "red"
        exit 1
    fi

    # Include both repo name and cluster name in the success message
    write_colored_output "All prerequisites checks passed! (Branch: $git_branch, Cluster: $cluster_name)" "green"
}

# =============================================================================
# GIT UTILITIES
# =============================================================================

get_changed_files() {
    local changed_files=$(git -c core.autocrlf=true status --porcelain | grep -E '^(A |M | M|MM|AM)' | awk '{print $2}')

    if [[ $? -ne 0 ]]; then
        write_colored_output "Error: Failed to get git status" "red"
        exit 1
    fi

    local file_array=()
    while IFS= read -r file; do
        if [[ -n "${file// }" ]]; then
            file_array+=("$file")
        fi
    done <<< "$changed_files"

    # Show first 10 files for reference
    if [[ ${#file_array[@]} -gt 0 ]]; then
        local count=0
        for file in "${file_array[@]}"; do
            if [[ $count -lt 10 ]]; then
                write_colored_output "  $file" "gray"
                ((count++))
            else
                break
            fi
        done

        if [[ ${#file_array[@]} -gt 10 ]]; then
            write_colored_output "  ... and $((${#file_array[@]} - 10)) more files" "gray"
        fi
    fi

    printf '%s\n' "${file_array[@]}"
}

# =============================================================================
# CONFIRMATION UTILITIES
# =============================================================================

confirm_deployment() {
    local microservices=("$@")

    echo
    write_colored_output "    DEPLOYMENT CONFIRMATION" "cyan"
    write_colored_output "The following microservices will be processed:" "yellow"

    for ms in "${microservices[@]}"; do
        write_colored_output "  - $ms" "yellow"
    done

    echo
    write_colored_output "Namespace: $NAMESPACE" "yellow"
    echo

    read -p "Do you want to continue? (y/N): " -n 1 -r
    echo

    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        write_colored_output "Deployment cancelled by user." "yellow"
        exit 0
    fi
}

# =============================================================================
# DOCKER SETTINGS UTILITIES
# =============================================================================

get_corporate_ip() {
    if ! command -v ifconfig &> /dev/null; then
        write_colored_output "Error: ifconfig command not found" "red" >&2
        return 1
    fi

    # Try eth0 first
    local corp_ip=$(ifconfig eth0 2>/dev/null | grep 'inet ' | awk '{print $2}' | head -1)

    if [[ -n "$corp_ip" && "$corp_ip" =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]]; then
        echo "$corp_ip"
        return 0
    fi

    # If eth0 not found or no valid IP, try eth1
    corp_ip=$(ifconfig eth1 2>/dev/null | grep 'inet ' | awk '{print $2}' | head -1)

    if [[ -n "$corp_ip" && "$corp_ip" =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]]; then
        echo "$corp_ip"
        return 0
    fi

    write_colored_output "Error: Could not find corporate IP address from eth0 or eth1" "red" >&2
    return 1
}

generate_docker_tag() {
    local timestamp=$(date +%Y%m%d-%H%M%S)
    local docker_tag="${WINDOWS_USER}-${timestamp}"
    echo "$docker_tag"
    return 0
}

update_docker_host_in_settings() {
    local new_ip="$1"

    local settings_file_path=""
    if [[ "$RUNTIME_ENV" == "WSL" ]]; then
        settings_file_path="/mnt/c/Users/$WINDOWS_USER/.m2/settings.xml"
    else
        settings_file_path="/c/Users/$WINDOWS_USER/.m2/settings.xml"
    fi

    if [[ ! -f "$settings_file_path" ]]; then
        write_colored_output "Error: Maven settings.xml not found at $settings_file_path" "red"
        return 1
    fi

    # Create backup
    cp "$settings_file_path" "$settings_file_path.backup.$(date +%Y%m%d_%H%M%S)"

    # Validate IP format
    if [[ ! "$new_ip" =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]]; then
        write_colored_output "Error: Invalid IP format: $new_ip" "red"
        return 1
    fi

    # Update docker.host using sed
    local sed_pattern="s/<docker\.host>tcp:\/\/[0-9]\{1,3\}\.[0-9]\{1,3\}\.[0-9]\{1,3\}\.[0-9]\{1,3\}:2375<\/docker\.host>/<docker.host>tcp:\/\/$new_ip:2375<\/docker.host>/g"

    if sed -i "$sed_pattern" "$settings_file_path"; then
        if grep -q "tcp://$new_ip:2375" "$settings_file_path"; then
            return 0
        else
            write_colored_output "Error: Failed to update docker.host value" "red"
            return 1
        fi
    else
        write_colored_output "Error: Sed command failed" "red"
        return 1
    fi
}

update_docker_tag_in_settings() {
    local new_tag="$1"

    local settings_file_path=""
    if [[ "$RUNTIME_ENV" == "WSL" ]]; then
        settings_file_path="/mnt/c/Users/$WINDOWS_USER/.m2/settings.xml"
    else
        settings_file_path="/c/Users/$WINDOWS_USER/.m2/settings.xml"
    fi

    if [[ ! -f "$settings_file_path" ]]; then
        write_colored_output "Error: Maven settings.xml not found at $settings_file_path" "red"
        return 1
    fi

    # Validate tag format
    if [[ ! "$new_tag" =~ ^[a-zA-Z0-9._-]+$ ]]; then
        write_colored_output "Error: Invalid tag format: $new_tag" "red"
        return 1
    fi

    # Update docker.tag3 using sed
    local sed_pattern="s/<docker\.tag3>[^<]*<\/docker\.tag3>/<docker.tag3>$new_tag<\/docker.tag3>/g"

    if sed -i "$sed_pattern" "$settings_file_path"; then
        if grep -q "<docker.tag3>$new_tag</docker.tag3>" "$settings_file_path"; then
            return 0
        else
            write_colored_output "Error: Failed to update docker.tag3 value" "red"
            return 1
        fi
    else
        write_colored_output "Error: Sed command failed" "red"
        return 1
    fi
}

auto_update_docker_settings() {
    # Update Docker host IP
    local corp_ip=$(get_corporate_ip)
    if [[ $? -eq 0 && -n "$corp_ip" ]]; then
        if ! update_docker_host_in_settings "$corp_ip"; then
            write_colored_output "Error: Failed to update Docker host IP in settings.xml" "red"
            exit 1
        fi
    else
        write_colored_output "Error: Could not detect corporate IP address" "red"
        exit 1
    fi

    # Update Docker tag
    local docker_tag=$(generate_docker_tag)
    if [[ $? -eq 0 && -n "$docker_tag" ]]; then
        if ! update_docker_tag_in_settings "$docker_tag"; then
            write_colored_output "Error: Failed to update Docker tag in settings.xml" "red"
            exit 1
        fi
    else
        write_colored_output "Error: Could not generate Docker tag" "red"
        exit 1
    fi

    write_colored_output "Maven Settings XML Updated (IP: $corp_ip, Tag: $docker_tag)" "green"
}

# =============================================================================
# POWERSHELL UTILITIES
# =============================================================================

# Get the appropriate PowerShell executable based on environment
get_powershell_executable() {
    if [[ "$RUNTIME_ENV" == "WSL" ]]; then
        echo "/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"
    else
        echo "/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"
    fi
}

# Build Maven command for PowerShell execution with verbose/quiet support
build_maven_command() {
    local target_windows_path="$1"
    local additional_flags="${2:-}"
    
    local base_command="Set-Location '$target_windows_path'; mvn clean install -DskipTests -s '$MAVEN_SETTINGS_PATH'"
    
    if [[ -n "$additional_flags" ]]; then
        base_command="$base_command $additional_flags"
    fi
    
    if [[ "$VERBOSE" == "true" ]]; then
        echo "$base_command"
    else
        echo "$base_command -q"
    fi
}

# Execute Maven command with PowerShell, with verbose/quiet fallback
execute_maven_with_fallback() {
    local ps_executable="$1"
    local ps_command="$2"
    local service_name="$3"
    
    # Set proper encoding for Maven builds
    export LANG=C.UTF-8
    export LC_ALL=C.UTF-8
    
    write_colored_output "Building $service_name..." "blue"
    
    if [[ "$VERBOSE" == "true" ]]; then
        "$ps_executable" -Command "$ps_command"
        local exit_code=$?
    else
        local output=$("$ps_executable" -Command "$ps_command" 2>&1)
        local exit_code=$?
        if [[ $exit_code -ne 0 ]]; then
            write_colored_output "Build failed with quiet mode. Retrying with verbose output..." "yellow"
            "$ps_executable" -Command "${ps_command% -q}"
            exit_code=$?
        fi
    fi
    
    if [[ $exit_code -eq 0 ]]; then
        write_colored_output "$service_name build completed successfully" "green"
        return 0
    else
        write_colored_output "Error: $service_name build failed" "red"
        return 1
    fi
}


