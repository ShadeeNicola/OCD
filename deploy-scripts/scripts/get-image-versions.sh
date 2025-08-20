#!/bin/bash
# Script to get image versions from EKS cluster for RN Creation
# Usage: ./get-image-versions.sh <cluster_name>

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

# =============================================================================
# MAIN FUNCTION
# =============================================================================

get_image_versions_from_cluster() {
    local cluster_name="$1"
    
    if [[ -z "$cluster_name" ]]; then
        write_colored_output "Error: Cluster name is required" "red"
        exit 1
    fi
    
    write_colored_output "Getting image versions from cluster: $cluster_name" "blue"
    
    # Update kubeconfig to connect to the specified cluster
    write_colored_output "Updating kubeconfig for cluster: $cluster_name" "blue"
    if ! aws eks update-kubeconfig --name "$cluster_name" > /dev/null 2>&1; then
        write_colored_output "Error: Failed to update kubeconfig for cluster: $cluster_name" "red"
        exit 1
    fi
    
    # Execute the kubectl commands to get image versions
    write_colored_output "Fetching image versions from dop namespace..." "blue"
    
    # Get ATT image version
    local att_version
    att_version=$(bash -l -c 'proxy on 2>/dev/null || true && kubectl get pods -n dop -o yaml | grep -E "att/.*:10\.4.*develop.*SNAPSHOT" | grep -v customization | head -1 | sed "s/.*://" | tr -d " \""' 2>/dev/null)
    
    # Get Guided task image version
    local guided_version
    guided_version=$(bash -l -c 'proxy on 2>/dev/null || true && kubectl get pods -n dop -o yaml | grep -B10 -A10 "guided.*task" | grep -E ":10\.4.*develop.*SNAPSHOT" | head -1 | sed "s/.*://" | tr -d " \""' 2>/dev/null)
    
    # Get Customization image version
    local customization_version
    customization_version=$(bash -l -c 'proxy on 2>/dev/null || true && kubectl get pods -n dop -o yaml | grep -E "customization.*:10\.4.*develop.*SNAPSHOT" | head -1 | sed "s/.*://" | tr -d " \""' 2>/dev/null)
    
    if [[ $? -ne 0 ]]; then
        write_colored_output "Error: Failed to execute kubectl commands" "red"
        exit 1
    fi
    
    # Output the results in the required format
    echo "ATT image: $att_version"
    echo "Guided task image: $guided_version"
    echo "Customization image: $customization_version"
}

# =============================================================================
# SCRIPT EXECUTION
# =============================================================================

# Check if cluster name argument is provided
if [[ $# -eq 0 ]]; then
    write_colored_output "Usage: $0 <cluster_name>" "red"
    write_colored_output "Example: $0 my-eks-cluster" "blue"
    exit 1
fi

# Get the cluster name from command line argument
CLUSTER_NAME="$1"

# Execute the main function
get_image_versions_from_cluster "$CLUSTER_NAME"