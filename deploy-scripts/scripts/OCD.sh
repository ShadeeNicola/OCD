#!/bin/bash
# OCD - One Click Deployer for ATT Projects
# Detects changed microservices and builds/deploys only what's needed

# =============================================================================
# LOAD SHARED FUNCTIONS
# =============================================================================

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SHARED_DIR="$SCRIPT_DIR/shared"

# Source all shared functions dynamically
for shared_script in "$SHARED_DIR"/*.sh; do
    if [[ -f "$shared_script" ]]; then
        source "$shared_script"
    fi
done

# Parse command line arguments using shared function
parse_common_arguments "$0" "$@"

# =============================================================================
# ATT-SPECIFIC FUNCTIONS
# =============================================================================

# Global variables for Maven settings
MAVEN_SETTINGS_PATH=""
WINDOWS_USER=""











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

    # Skip helm/charts/ directory - these are deployment configs, not buildable microservices

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

    # Skip helm chart changes - these don't trigger microservice builds
    if [[ "$file_path" =~ ^helm/ ]]; then
        return
    fi

    local all_microservices=($(discover_microservices))

    for ms_name in "${all_microservices[@]}"; do
        if [[ "$file_path" =~ ^${ms_name}(-ms)?/ ]] || \
           [[ "$file_path" =~ ^dockers/${ms_name}(-img|-img-job|-app)?/ ]]; then
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

build_microservice() {
    export LANG=C.UTF-8
    export LC_ALL=C.UTF-8

    local microservice_name="$1"
    write_colored_output "Building microservice: $microservice_name" "blue"

    # Determine the correct directory with flexible matching
    local build_dir=""
    if [[ -d "${microservice_name}-ms" ]]; then
        build_dir="${microservice_name}-ms"
    elif [[ -d "$microservice_name" ]]; then
        build_dir="$microservice_name"
    else
        # Try multiple patterns to find matching directories
        local matching_dirs=()

        # Try pattern 1: *microservice_name*-ms (contains the name)
        local pattern1=($(find . -maxdepth 1 -type d -name "*${microservice_name}*-ms" 2>/dev/null))
        matching_dirs+=("${pattern1[@]}")

        # Try pattern 2: Extract key parts and search (e.g., for purge-tool-oni, search for *oni*-ms)
        local key_part=""
        if [[ "$microservice_name" == *-* ]]; then
            key_part=$(echo "$microservice_name" | rev | cut -d'-' -f1 | rev)  # Get last part after -
            local pattern2=($(find . -maxdepth 1 -type d -name "*${key_part}*-ms" 2>/dev/null))
            matching_dirs+=("${pattern2[@]}")
        fi

        # Remove duplicates and limit results
        matching_dirs=($(printf '%s\n' "${matching_dirs[@]}" | sort -u | head -5))

        if [[ ${#matching_dirs[@]} -eq 1 ]]; then
            build_dir=$(basename "${matching_dirs[0]}")
            write_colored_output "Found matching directory: $build_dir" "yellow"
        elif [[ ${#matching_dirs[@]} -gt 1 ]]; then
            write_colored_output "Multiple directories found matching $microservice_name:" "yellow"
            for dir in "${matching_dirs[@]}"; do
                write_colored_output "  - $(basename "$dir")" "yellow"
            done
            write_colored_output "Using first match: $(basename "${matching_dirs[0]}")" "yellow"
            build_dir=$(basename "${matching_dirs[0]}")
        else
            write_colored_output "Error: Could not find directory for $microservice_name" "red"
            write_colored_output "Available directories ending with -ms:" "red"
            find . -maxdepth 1 -type d -name "*-ms" | sed 's|./||' | sort | head -10
            return 1
        fi
    fi

    # Get absolute path and convert to Windows path
    local target_wsl_path=$(realpath "$build_dir")
    local target_windows_path=$(convert_to_windows_path "$target_wsl_path")

    # Use shared PowerShell utilities
    local ps_executable=$(get_powershell_executable)
    local ps_command=$(build_maven_command "$target_windows_path")
    
    log_command "$ps_command"
    
    # Execute with fallback handling
    execute_maven_with_fallback "$ps_executable" "$ps_command" "$microservice_name"
}

get_docker_artifact_name() {
    local microservice_name="$1"

    # First try exact matches
    local docker_pom_paths=(
        "dockers/${microservice_name}-img/pom.xml"
        "dockers/${microservice_name}-app/pom.xml"
        "dockers/${microservice_name}/pom.xml"
    )

    for pom_path in "${docker_pom_paths[@]}"; do
        if [[ -f "$pom_path" ]]; then
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

    # If no exact match, try pattern-based search similar to build_docker_image function
    if [[ -d "dockers/" ]]; then
        write_colored_output "No exact Docker pom.xml match found for $microservice_name, trying pattern matching..." "yellow" >&2

        # Extract key part from microservice name for pattern matching
        local key_part=""
        if [[ "$microservice_name" == *-* ]]; then
            key_part=$(echo "$microservice_name" | rev | cut -d'-' -f1 | rev)
        else
            key_part="$microservice_name"
        fi

        # Try pattern matching in dockers directory
        local matching_docker_dirs=($(find dockers/ -maxdepth 1 -type d -name "*${key_part}*-app" -o -name "*${key_part}*-img" 2>/dev/null | head -3))

        for docker_dir in "${matching_docker_dirs[@]}"; do
            local pom_path="$docker_dir/pom.xml"
            if [[ -f "$pom_path" ]]; then
                write_colored_output "Found matching Docker directory: $docker_dir" "yellow" >&2
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
                    write_colored_output "Found Docker artifact name: $artifact_id" "green" >&2
                    echo "$artifact_id"
                    return 0
                fi
            fi
        done
    fi

    write_colored_output "Error: Could not find Docker artifact name for microservice: $microservice_name" "red" >&2
    write_colored_output "Searched in:" "red" >&2
    for pom_path in "${docker_pom_paths[@]}"; do
        write_colored_output "  - $pom_path" "red" >&2
    done
    if [[ -d "dockers/" ]]; then
        write_colored_output "Available Docker directories:" "red" >&2
        find dockers/ -maxdepth 1 -type d -name "*-app" -o -name "*-img" 2>/dev/null | head -10 | while read dir; do
            write_colored_output "  - $dir" "red" >&2
        done
    fi
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

    # Try exact matches first
    for docker_dir in "${docker_pom_paths[@]}"; do
        if [[ -d "$docker_dir" && -f "$docker_dir/pom.xml" ]]; then
            docker_build_dir="$docker_dir"
            break
        fi
    done

    # If no exact match, try flexible pattern matching
    if [[ -z "$docker_build_dir" ]]; then
        # Extract key part from microservice name for pattern matching
        local key_part=""
        if [[ "$microservice_name" == *-* ]]; then
            key_part=$(echo "$microservice_name" | rev | cut -d'-' -f1 | rev)
        fi

        # Try pattern matching in dockers directory
        local matching_docker_dirs=($(find dockers/ -maxdepth 1 -type d -name "*${key_part}*-app" -o -name "*${key_part}*-img" 2>/dev/null | head -3))

        for docker_dir in "${matching_docker_dirs[@]}"; do
            if [[ -f "$docker_dir/pom.xml" ]]; then
                docker_build_dir="$docker_dir"
                write_colored_output "Found matching Docker directory: $docker_build_dir" "yellow"
                break
            fi
        done
    fi

    if [[ -z "$docker_build_dir" ]]; then
        write_colored_output "Error: Could not find Docker directory for $microservice_name" "red"
        write_colored_output "Available Docker directories:" "red"
        find dockers/ -maxdepth 1 -type d -name "*-app" -o -name "*-img" 2>/dev/null | head -10
        return 1
    fi

    # Get absolute path and convert to Windows path
    local docker_wsl_path=$(realpath "$docker_build_dir")
    local docker_windows_path=$(convert_to_windows_path "$docker_wsl_path")

    # Use shared PowerShell utilities for Docker build
    local ps_executable=$(get_powershell_executable)
    local docker_ps_command=$(build_maven_command "$docker_windows_path")
    
    log_command "$docker_ps_command"
    
    # Execute with fallback handling (negated for docker build failure check)
    if ! execute_maven_with_fallback "$ps_executable" "$docker_ps_command" "$microservice_name (Docker)"; then
        return 1
    fi

    write_colored_output "Docker image build completed successfully for $microservice_name" "green"
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

    local found_service=""

    # Search for microservice by pattern matching instead of hardcoded guesses
    write_colored_output "Searching for microservice containing '$microservice_name' in namespace '$namespace'..." "blue"

    # Get all microservices and filter by pattern
    local all_services=$(bash -l -c "proxy on 2>/dev/null || true && kubectl get microservice -n '$namespace' --no-headers -o custom-columns=NAME:.metadata.name" 2>/dev/null || echo "")

    if [[ -n "$all_services" ]]; then
        # Try to find a service that contains key parts of the microservice name
        local key_parts=($(echo "$microservice_name" | tr '-' ' '))

        for service in $all_services; do
            local match_count=0
            for part in "${key_parts[@]}"; do
                if [[ "$service" == *"$part"* ]]; then
                    ((match_count++))
                fi
            done

            # If most parts match, use this service
            if [[ $match_count -ge $((${#key_parts[@]} - 1)) ]]; then
                found_service="$service"
                write_colored_output "Found microservice: $found_service (matched $match_count/${#key_parts[@]} parts)" "green"
                break
            fi
        done
    fi

    if [[ -z "$found_service" ]]; then
        write_colored_output "Error: Could not find microservice matching '$microservice_name' in namespace '$namespace'" "red"
        write_colored_output "Available microservices in namespace:" "red"
        if [[ -n "$all_services" ]]; then
            echo "$all_services" | head -20 | while read service; do
                write_colored_output "  - $service" "red"
            done
        else
            write_colored_output "  No microservices found or kubectl access failed" "red"
        fi
        return 1
    fi

    # Use the generic function to update the application container
    update_kubernetes_microservice_generic "$image_tag" "$namespace" "$found_service" "(copy-application-files|source-code)" "application"
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

write_colored_output "OCD - One Click Deployer for ATT Projects" "cyan"
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
        formatted_ms=$(printf "%-20s" "$microservice")

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


