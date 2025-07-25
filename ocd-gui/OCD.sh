#!/bin/bash
# File: C:\Intellij_Projects\OCD\untitled\OCD.sh
# OCD - One Click Deployer
# Detects changed microservices and builds/deploys only what's needed

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
# ENVIRONMENT DETECTION
# =============================================================================

detect_environment() {
    if [[ -n "$WSL_DISTRO_NAME" ]] || [[ "$(uname -r)" == *microsoft* ]] || [[ "$(uname -r)" == *WSL* ]]; then
        echo "WSL"
    else
        echo "WINDOWS"
    fi
}

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

# Global variables for Maven settings
MAVEN_SETTINGS_PATH=""
WINDOWS_USER=""

get_maven_settings() {
    if [[ "$RUNTIME_ENV" == "WSL" ]]; then
        local user=$(ls -1 /mnt/c/Users | grep -vE '^(Public|Default|desktop.ini|Default\ User|ADMINI~1|All\ Users)$' | head -n 1)

        if [[ -z "$user" ]]; then
            write_colored_output "Error: Could not determine Windows username" "red"
            exit 1
        fi

        if [[ ! -f "/mnt/c/Users/$user/.m2/settings.xml" ]]; then
            write_colored_output "Error: Maven settings.xml not found at /mnt/c/Users/$user/.m2/settings.xml" "red"
            exit 1
        fi
    else
        local user=$(ls -1 /c/Users | grep -vE '^(Public|Default|desktop.ini|Default\ User|ADMINI~1|All\ Users)$' | head -n 1)

        if [[ -z "$user" ]]; then
            write_colored_output "Error: Could not determine Windows username" "red"
            exit 1
        fi

        if [[ ! -f "/c/Users/$user/.m2/settings.xml" ]]; then
            write_colored_output "Error: Maven settings.xml not found at /c/Users/$user/.m2/settings.xml" "red"
            exit 1
        fi
    fi

    WINDOWS_USER="$user"
    MAVEN_SETTINGS_PATH="C:\\Users\\$user\\.m2\\settings.xml"
}

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

# Update the perform_connection_checks function to capture and display cluster info:

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

discover_microservices() {
    local microservices=()

    # Find all directories ending in -ms
    for ms_dir in *-ms/; do
        if [[ -d "$ms_dir" ]]; then
            local ms_name=$(basename "$ms_dir" | sed 's/-ms$//')
            microservices+=("$ms_name")
        fi
    done

    # Find microservices in dockers/ directory
    if [[ -d "dockers/" ]]; then
        for docker_dir in dockers/*/; do
            if [[ -d "$docker_dir" ]]; then
                local dir_name=$(basename "$docker_dir")
                local ms_name=$(echo "$dir_name" | sed -E 's/(-img|-img-job|-app)$//')
                microservices+=("$ms_name")
            fi
        done
    fi

    # Find microservices in helm/charts/ directory
    if [[ -d "helm/charts/" ]]; then
        for chart_dir in helm/charts/*/; do
            if [[ -d "$chart_dir" ]]; then
                local dir_name=$(basename "$chart_dir")
                local ms_name=$(echo "$dir_name" | sed -E 's/(-deploy)$//')
                microservices+=("$ms_name")
            fi
        done
    fi

    # Find standalone microservice directories
    for top_dir in */; do
        if [[ -d "$top_dir" ]]; then
            local dir_name=$(basename "$top_dir")
            if [[ ! "$dir_name" =~ ^(dockers|helm|integration-ms|jakarta-clientkits|terminated-users-removal)$ ]]; then
                if [[ -d "$top_dir/src/" ]] || [[ -f "$top_dir/pom.xml" ]] || [[ -d "$top_dir/target/" ]]; then
                    microservices+=("$dir_name")
                fi
            fi
        fi
    done

    # Remove duplicates and sort
    printf '%s\n' "${microservices[@]}" | sort -u
}

find_microservice_for_file() {
    local file_path="$1"
    local all_microservices=($(discover_microservices))

    for ms_name in "${all_microservices[@]}"; do
        if [[ "$file_path" =~ ^${ms_name}(-ms)?/ ]] || \
           [[ "$file_path" =~ ^dockers/${ms_name}(-img|-img-job|-app)?/ ]] || \
           [[ "$file_path" =~ ^helm/charts/${ms_name}(-deploy)?/ ]]; then
            echo "$ms_name"
            return
        fi
    done
}

get_changed_microservices() {
    local changed_files="$1"
    local detected_ms=()

    while IFS= read -r file; do
        if [[ -z "$file" ]]; then
            continue
        fi

        local ms_name=$(find_microservice_for_file "$file")
        if [[ -n "$ms_name" ]] && [[ ! " ${detected_ms[@]} " =~ " ${ms_name} " ]]; then
            detected_ms+=("$ms_name")
        fi
    done <<< "$changed_files"

    if [[ ${#detected_ms[@]} -gt 0 ]]; then
        for ms in "${detected_ms[@]}"; do
            write_colored_output "${ms}" "green"
        done
    else
        write_colored_output "No microservice changes detected" "yellow"
    fi

    printf '%s\n' "${detected_ms[@]}"
}

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

build_microservice() {
    export LANG=C.UTF-8
    export LC_ALL=C.UTF-8

    local microservice_name="$1"
    write_colored_output "Building microservice: $microservice_name" "blue"

    # Determine the correct directory
    local build_dir=""
    if [[ -d "${microservice_name}-ms" ]]; then
        build_dir="${microservice_name}-ms"
    elif [[ -d "$microservice_name" ]]; then
        build_dir="$microservice_name"
    else
        write_colored_output "Error: Could not find directory for $microservice_name" "red"
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
            write_colored_output "Build completed successfully for $microservice_name" "green"
            return 0
        else
            write_colored_output "Maven build failed for $microservice_name" "red"
            return 1
        fi
    else
        # Hide output in non-verbose mode
        if "$ps_executable" -Command "$ps_command" > /dev/null 2>&1; then
            write_colored_output "Build completed successfully for $microservice_name" "green"
            return 0
        else
            write_colored_output "Maven build failed for $microservice_name" "red"
            # Run again without -q to show error details
            write_colored_output "Error details:" "red"
            "$ps_executable" -Command "Set-Location '$target_windows_path'; mvn clean install -DskipTests -s '$MAVEN_SETTINGS_PATH'" 2>&1 | tail -20
            return 1
        fi
    fi
}

get_docker_artifact_name() {
    local microservice_name="$1"

    local docker_pom_paths=(
        "dockers/${microservice_name}-img/pom.xml"
        "dockers/${microservice_name}-app/pom.xml"
        "dockers/${microservice_name}/pom.xml"
    )

    for pom_path in "${docker_pom_paths[@]}"; do
        if [[ -f "$pom_path" ]]; then
            # Use awk to properly parse the XML and get the project's artifactId (not parent's)
            local artifact_id=$(awk '
                /<parent>/,/<\/parent>/ { next }
                /<artifactId>/ {
                    gsub(/.*<artifactId>/, "")
                    gsub(/<\/artifactId>.*/, "")
                    gsub(/^[ \t]+|[ \t]+$/, "")
                    print
                    exit
                }
            ' "$pom_path")

            if [[ -n "$artifact_id" && "$artifact_id" != "artifactId" ]]; then
                echo "$artifact_id"
                return 0
            fi
        fi
    done

    return 1
}

build_docker_image() {
    local microservice_name="$1"

    write_colored_output "Deploying microservice: $microservice_name" "yellow"

    # Find the Docker directory for this microservice
    local docker_build_dir=""
    local docker_pom_paths=(
        "dockers/${microservice_name}-img"
        "dockers/${microservice_name}-app"
        "dockers/${microservice_name}"
    )

    for docker_dir in "${docker_pom_paths[@]}"; do
        if [[ -d "$docker_dir" && -f "$docker_dir/pom.xml" ]]; then
            docker_build_dir="$docker_dir"
            break
        fi
    done

    if [[ -z "$docker_build_dir" ]]; then
        write_colored_output "Error: Could not find Docker directory for $microservice_name" "red"
        return 1
    fi

    # Get absolute path and convert to Windows path
    local docker_wsl_path=$(realpath "$docker_build_dir")
    local docker_windows_path=$(convert_to_windows_path "$docker_wsl_path")

    # Choose the right executable based on environment
    local ps_executable=""
    if [[ "$RUNTIME_ENV" == "WSL" ]]; then
        ps_executable="/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"
    else
        ps_executable="/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"
    fi

    # Build PowerShell command for Docker build - use verbose output if VERBOSE is true
    local docker_ps_command=""
    if [[ "$VERBOSE" == "true" ]]; then
        docker_ps_command="Set-Location '$docker_windows_path'; mvn clean install -DskipTests -s '$MAVEN_SETTINGS_PATH'"
    else
        docker_ps_command="Set-Location '$docker_windows_path'; mvn clean install -DskipTests -s '$MAVEN_SETTINGS_PATH' -q"
    fi

    log_command "$docker_ps_command"

    if [[ "$VERBOSE" == "true" ]]; then
        # Show full output in verbose mode
        if ! "$ps_executable" -Command "$docker_ps_command"; then
            write_colored_output "Docker image build failed for $microservice_name" "red"
            return 1
        fi
    else
        # Hide output in non-verbose mode
        if ! "$ps_executable" -Command "$docker_ps_command" > /dev/null 2>&1; then
            write_colored_output "Docker image build failed for $microservice_name" "red"
            # Run again without -q to show error details
            write_colored_output "Error details:" "red"
            "$ps_executable" -Command "Set-Location '$docker_windows_path'; mvn clean install -DskipTests -s '$MAVEN_SETTINGS_PATH'" 2>&1 | tail -20
            return 1
        fi
    fi

    write_colored_output "Docker image build completed successfully for $microservice_name" "green"
    return 0
}

get_registry_and_tag_from_settings() {
    local settings_file_path=""
    if [[ "$RUNTIME_ENV" == "WSL" ]]; then
        settings_file_path="/mnt/c/Users/$WINDOWS_USER/.m2/settings.xml"
    else
        settings_file_path="/c/Users/$WINDOWS_USER/.m2/settings.xml"
    fi

    local push_registry=$(grep -o '<docker\.push\.registry>[^<]*</docker\.push\.registry>' "$settings_file_path" | sed 's/<[^>]*>//g')
    local current_docker_tag=$(grep -o '<docker\.tag3>[^<]*</docker\.tag3>' "$settings_file_path" | sed 's/<[^>]*>//g')

    if [[ -z "$push_registry" ]]; then
        write_colored_output "Error: Could not find docker.push.registry in Maven settings" "red"
        return 1
    fi

    if [[ -z "$current_docker_tag" ]]; then
        write_colored_output "Error: Could not find docker.tag3 in Maven settings" "red"
        return 1
    fi

    echo "$push_registry|$current_docker_tag"
    return 0
}

construct_uploaded_image_tag() {
    local microservice_name="$1"
    local push_registry="$2"
    local docker_tag="$3"

    # Try to get the actual Docker artifact name from pom.xml
    local docker_artifact_name=$(get_docker_artifact_name "$microservice_name")

    if [[ -n "$docker_artifact_name" ]]; then
        local uploaded_image_tag="$push_registry/att/$docker_artifact_name:$docker_tag"
    else
        local uploaded_image_tag="$push_registry/att/$microservice_name:$docker_tag"
    fi

    echo "$uploaded_image_tag"
}

update_kubernetes_microservice() {
    local microservice_name="$1"
    local namespace="$2"
    local image_tag="$3"

    write_colored_output "Updating Kubernetes microservice $microservice_name with image: $image_tag" "blue"

    # Try different service naming patterns
    local service_names=(
        "${microservice_name}-service"
        "${microservice_name}"
        "${microservice_name}-ms"
        "${microservice_name}-app"
    )

    local found_service=""

    # Try to find the service with different naming patterns
    for service_name in "${service_names[@]}"; do
        log_command "kubectl get microservice '$service_name' -n '$namespace'"

        # Enable proxy and run kubectl command
        if bash -l -c "proxy on && kubectl get microservice '$service_name' -n '$namespace' --no-headers" > /dev/null 2>&1; then
            found_service="$service_name"
            write_colored_output "Found microservice: $found_service" "green"
            break
        fi
    done

    if [[ -z "$found_service" ]]; then
        write_colored_output "Error: Could not find microservice for $microservice_name in namespace $namespace" "red"
        write_colored_output "Tried the following service names:" "red"
        for service_name in "${service_names[@]}"; do
            write_colored_output "  - $service_name" "red"
        done
        return 1
    fi

    # Get the current microservice spec to find the initContainer index
    log_command "kubectl get microservice '$found_service' -n '$namespace' -o jsonpath='{range .spec.template.spec.initContainers[*]}{@.name}{\"\\n\"}{end}'"

    local init_containers_output=$(bash -l -c "proxy on && kubectl get microservice '$found_service' -n '$namespace' -o jsonpath='{range .spec.template.spec.initContainers[*]}{@.name}{\"\n\"}{end}'" 2>/dev/null)

    local init_container_index=$(echo "$init_containers_output" | grep -n "copy-application-files" | cut -d: -f1)

    if [[ -z "$init_container_index" ]]; then
        write_colored_output "Error: copy-application-files initContainer not found" "red"
        write_colored_output "Available initContainers:" "red"
        echo "$init_containers_output"
        return 1
    fi

    # Validate that we got a valid number
    if [[ ! "$init_container_index" =~ ^[0-9]+$ ]] || [[ "$init_container_index" -lt 1 ]]; then
        write_colored_output "Error: Invalid initContainer index: $init_container_index" "red"
        return 1
    fi

    # Convert to zero-based index
    init_container_index=$((init_container_index - 1))
    write_colored_output "Using initContainer index: $init_container_index" "blue"

    # Show current image before patching
    local current_image=$(bash -l -c "proxy on && kubectl get microservice '$found_service' -n '$namespace' -o jsonpath='{.spec.template.spec.initContainers[$init_container_index].image}'" 2>/dev/null)

    # Patch the microservice with new image in the correct initContainer
    local patch_command="kubectl patch microservice '$found_service' -n '$namespace' --type='json' -p='[{\"op\": \"replace\", \"path\": \"/spec/template/spec/initContainers/${init_container_index}/image\", \"value\": \"$image_tag\"}]'"

    log_command "$patch_command"

    if bash -l -c "proxy on && $patch_command" > /dev/null 2>&1; then
        write_colored_output "Microservice $found_service patched with new image" "green"

        # Verify the patch worked
        local updated_image=$(bash -l -c "proxy on && kubectl get microservice '$found_service' -n '$namespace' -o jsonpath='{.spec.template.spec.initContainers[$init_container_index].image}'" 2>/dev/null)

        return 0
    else
        write_colored_output "Failed to patch microservice" "red"
        return 1
    fi
}

deploy_microservice() {
    local microservice_name="$1"
    local namespace="$2"

    # Step 1: Build Docker image
    if ! build_docker_image "$microservice_name"; then
        return 1
    fi

    # Step 2: Get registry and tag from settings
    local registry_and_tag
    registry_and_tag=$(get_registry_and_tag_from_settings)
    if [[ $? -ne 0 ]]; then
        return 1
    fi

    # Parse the returned values
    local push_registry=$(echo "$registry_and_tag" | cut -d'|' -f1)
    local current_docker_tag=$(echo "$registry_and_tag" | cut -d'|' -f2)

    write_colored_output "Registry: $push_registry" "blue"
    write_colored_output "Docker Tag: $current_docker_tag" "blue"

    # Step 3: Construct the uploaded image tag
    local uploaded_image_tag
    uploaded_image_tag=$(construct_uploaded_image_tag "$microservice_name" "$push_registry" "$current_docker_tag")

    write_colored_output "Constructed image tag: $uploaded_image_tag" "blue"

    # Step 4: Update Kubernetes microservice
    if ! update_kubernetes_microservice "$microservice_name" "$namespace" "$uploaded_image_tag"; then
        return 1
    fi

    return 0
}

# =============================================================================
# MAIN EXECUTION
# =============================================================================

write_colored_output "OCD - One Click Deployer" "cyan"
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

# Detect changed microservices
changed_microservices_output=$(get_changed_microservices "$changed_files_output")

# Convert to proper array - filter out empty lines and debug output
changed_microservices=()
while IFS= read -r ms; do
    # Only add lines that look like microservice names (no spaces, no special chars)
    if [[ -n "$ms" && "$ms" =~ ^[a-zA-Z0-9_-]+$ ]]; then
        changed_microservices+=("$ms")
    fi
done <<< "$changed_microservices_output"

if [[ ${#changed_microservices[@]} -eq 0 && "$FORCE" != "true" ]]; then
    exit 0
fi

# Confirmation prompt if requested
if [[ "$CONFIRM" == "true" ]]; then
    confirm_deployment "${changed_microservices[@]}"
fi

declare -A build_results
declare -A deploy_results

# Build phase
if [[ "$SKIP_BUILD" != "true" ]]; then
    echo
    write_colored_output "BUILDING MICROSERVICES..." "magenta"

    for microservice in "${changed_microservices[@]}"; do
        if [[ -n "$microservice" ]]; then
            if build_microservice "$microservice"; then
                build_results["$microservice"]="success"
            else
                build_results["$microservice"]="failed"
                write_colored_output "Build failed for $microservice. Aborting execution." "red"
                exit 1
            fi
        fi
    done
fi

# Deploy phase
if [[ "$SKIP_DEPLOY" != "true" ]]; then
    echo
    write_colored_output "DEPLOYING MICROSERVICES..." "magenta"

    for microservice in "${changed_microservices[@]}"; do
        if [[ -n "$microservice" ]]; then
            if [[ "$SKIP_BUILD" == "true" || "${build_results[$microservice]}" == "success" ]]; then
                if deploy_microservice "$microservice" "$NAMESPACE"; then
                    deploy_results["$microservice"]="success"
                else
                    deploy_results["$microservice"]="failed"
                fi
            else
                write_colored_output "Skipping deployment of $microservice due to build failure" "red"
                deploy_results["$microservice"]="skipped"
            fi
        fi
    done
fi

# Summary
echo
write_colored_output "═══════════════════════════════════════════════════════════════" "cyan"
write_colored_output "                        EXECUTION SUMMARY                        " "cyan"
write_colored_output "═══════════════════════════════════════════════════════════════" "cyan"

success_count=0
total_count=${#changed_microservices[@]}

for microservice in "${changed_microservices[@]}"; do
    if [[ -n "$microservice" ]]; then
        build_status="SKIPPED"
        deploy_status="SKIPPED"

        if [[ "$SKIP_BUILD" != "true" ]]; then
            build_status="${build_results[$microservice]^^}"
        fi

        if [[ "$SKIP_DEPLOY" != "true" ]]; then
            deploy_status="${deploy_results[$microservice]^^}"
        fi

        # Format the microservice name with padding
        local formatted_ms=$(printf "%-20s" "$microservice")

        if [[ ("$build_status" == "SUCCESS" || "$build_status" == "SKIPPED") && ("$deploy_status" == "SUCCESS" || "$deploy_status" == "SKIPPED") ]]; then
            write_colored_output "    $formatted_ms │ Build: $build_status │ Deploy: $deploy_status" "green"
            ((success_count++))
        else
            write_colored_output "    $formatted_ms │ Build: $build_status │ Deploy: $deploy_status" "red"
        fi
    fi
done

echo
write_colored_output "═══════════════════════════════════════════════════════════════" "cyan"
if [[ $success_count -eq $total_count ]]; then
    write_colored_output "     SUCCESS: $success_count/$total_count microservices processed successfully!" "green"
    write_colored_output "═══════════════════════════════════════════════════════════════" "cyan"
    exit 0
else
    write_colored_output "      PARTIAL: $success_count/$total_count microservices processed successfully" "yellow"
    write_colored_output "═══════════════════════════════════════════════════════════════" "cyan"
    exit 1
fi
