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