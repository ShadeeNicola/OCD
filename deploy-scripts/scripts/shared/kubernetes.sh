#!/bin/bash
# Shared Kubernetes functions for OCD scripts

# =============================================================================
# KUBERNETES UPDATE FUNCTIONS
# =============================================================================

find_backend_microservice() {
    local namespace="$1"
    
    # Get all microservices containing "backend" in a single fast call
    local backend_microservices=$(bash -l -c "proxy on 2>/dev/null || true && kubectl get microservice -n '$namespace' --no-headers -o custom-columns=NAME:.metadata.name 2>/dev/null | grep -i backend")
    
    if [[ -z "$backend_microservices" ]]; then
        return 1
    fi
    
    # Priority order for backend microservice matching
    local priority_patterns=(
        "dop-backend-oso"
        "dop-backend"
        "backend-oso" 
        "oso-backend"
        "backend"
    )
    
    # Check priority patterns first
    for pattern in "${priority_patterns[@]}"; do
        echo "$backend_microservices" | grep -q "^$pattern$" && echo "$pattern" && return 0
    done
    
    # If no exact match, return the first backend microservice found
    echo "$backend_microservices" | head -n 1
    return 0
}

find_init_container_by_pattern() {
    local microservice_name="$1"
    local namespace="$2"
    local container_pattern="$3"
    
    # Get both names and images in a single call to avoid race conditions
    local combined_output=$(bash -l -c "proxy on 2>/dev/null || true && kubectl get microservice '$microservice_name' -n '$namespace' -o jsonpath='{range .spec.template.spec.initContainers[*]}{@.name}{\"|\"}{@.image}{\"\\n\"}{end}'" 2>/dev/null)
    
    # Extract just the names for display
    local init_containers_output=$(echo "$combined_output" | cut -d'|' -f1 | grep -v '^$')
    
    # Extract just the images for pattern matching
    local init_container_images=$(echo "$combined_output" | cut -d'|' -f2 | grep -v '^$')
    
    # First try: Find initContainer by image pattern
    local init_container_index=$(echo "$init_container_images" | grep -n "$container_pattern" | head -n 1 | cut -d: -f1)
    local detection_method="image-based"
    
    # Fallback: Find initContainer by name pattern
    if [[ -z "$init_container_index" ]]; then
        init_container_index=$(echo "$init_containers_output" | grep -n -E "$container_pattern" | head -n 1 | cut -d: -f1)
        detection_method="name-based"
    fi
    
    # Return the results (use simple approach - write to temp variables)
    echo "INDEX:${init_container_index}"
    echo "METHOD:${detection_method}"  
    echo "CONTAINERS_START"
    echo "${init_containers_output}"
    echo "CONTAINERS_END"
}

update_kubernetes_microservice_generic() {
    local image_tag="$1"
    local namespace="$2"
    local microservice_name="$3"
    local container_pattern="$4"
    local description="${5:-initContainer}"

    write_colored_output "Updating Kubernetes microservice $microservice_name with $description image: $image_tag" "blue"

    # Get the initContainer index using the generic function
    local container_info=$(find_init_container_by_pattern "$microservice_name" "$namespace" "$container_pattern")
    
    # Parse the simple format
    local init_container_index=$(echo "$container_info" | grep "^INDEX:" | cut -d: -f2)
    local detection_method=$(echo "$container_info" | grep "^METHOD:" | cut -d: -f2)
    local init_containers_output=$(echo "$container_info" | sed -n '/^CONTAINERS_START$/,/^CONTAINERS_END$/p' | grep -v "^CONTAINERS_START$" | grep -v "^CONTAINERS_END$")


    if [[ -z "$init_container_index" ]]; then
        write_colored_output "Error: No initContainer matching pattern '$container_pattern' found" "red"
        write_colored_output "Available initContainers:" "red"
        echo "$init_containers_output"
        return 1
    fi

    # Validate that we got a valid number (index should be >= 1 since grep -n starts from 1)
    if [[ ! "$init_container_index" =~ ^[0-9]+$ ]] || [[ "$init_container_index" -lt 1 ]]; then
        write_colored_output "Error: Invalid initContainer index: $init_container_index" "red"
        write_colored_output "Available initContainers:" "red"
        echo "$init_containers_output"
        return 1
    fi

    # Convert to zero-based index
    init_container_index=$((init_container_index - 1))
    write_colored_output "Using $description initContainer index: $init_container_index (detected via $detection_method)" "blue"

    # Show current image before patching
    local current_image=$(bash -l -c "proxy on 2>/dev/null || true && kubectl get microservice '$microservice_name' -n '$namespace' -o jsonpath='{.spec.template.spec.initContainers[$init_container_index].image}'" 2>/dev/null)
    write_colored_output "Current image: $current_image" "blue"

    # Patch the microservice with new image in the detected initContainer
    local patch_command="kubectl patch microservice '$microservice_name' -n '$namespace' --type='json' -p='[{\"op\": \"replace\", \"path\": \"/spec/template/spec/initContainers/${init_container_index}/image\", \"value\": \"$image_tag\"}]'"

    log_command "$patch_command"

    if bash -l -c "proxy on 2>/dev/null || true && $patch_command" > /dev/null 2>&1; then
        write_colored_output "Microservice $microservice_name patched with new $description image" "green"

        # Verify the patch worked
        local updated_image=$(bash -l -c "proxy on 2>/dev/null || true && kubectl get microservice '$microservice_name' -n '$namespace' -o jsonpath='{.spec.template.spec.initContainers[$init_container_index].image}'" 2>/dev/null)
        write_colored_output "Updated image: $updated_image" "green"

        return 0
    else
        write_colored_output "Failed to patch microservice" "red"
        return 1
    fi
}

update_kubernetes_microservice_customization() {
    local image_tag="$1"
    local namespace="$2"

    write_colored_output "Finding backend microservice in namespace $namespace..." "blue"
    
    # Find the best backend microservice match
    local microservice_name
    microservice_name=$(find_backend_microservice "$namespace")
    
    if [[ -z "$microservice_name" ]]; then
        write_colored_output "Error: Could not find any microservice containing 'backend' in namespace $namespace" "red"
        return 1
    fi

    write_colored_output "Found backend microservice: $microservice_name" "green"
    
    # Use the generic function to update the customization container
    update_kubernetes_microservice_generic "$image_tag" "$namespace" "$microservice_name" "customization" "customization"
}