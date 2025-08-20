#!/bin/bash
# Script to get helm charts from EKS cluster for RN Creation
# Usage: ./get-helm-charts.sh <cluster_name>

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

get_helm_charts_from_cluster() {
    local cluster_name="$1"
    
    if [[ -z "$cluster_name" ]]; then
        write_colored_output "Error: Cluster name is required" "red"
        exit 1
    fi
    
    write_colored_output "Getting helm charts from cluster: $cluster_name" "blue"
    
    # Update kubeconfig to connect to the specified cluster
    write_colored_output "Updating kubeconfig for cluster: $cluster_name" "blue"
    if ! aws eks update-kubeconfig --name "$cluster_name" > /dev/null 2>&1; then
        write_colored_output "Error: Failed to update kubeconfig for cluster: $cluster_name" "red"
        exit 1
    fi
    
    # Execute the helm command to get charts from multiple namespaces
    write_colored_output "Fetching helm charts from namespaces..." "blue"
    
    # Use single helm command with consistent namespace ordering - runs once instead of 4 times  
    local helm_output
    helm_output=$(bash -l -c 'proxy on 2>/dev/null || true && helm ls --all-namespaces | awk '"'"'NR>1 && ($2=="default"||$2=="dop"||$2=="on-logging"||$2=="on-monitoring") {ns[$2]=ns[$2]$9"\n"} END {split("default dop on-logging on-monitoring", order); for(i=1;i<=4;i++) {if(ns[order[i]]) {print "\n"order[i]" namespace:"; printf "%s",ns[order[i]]}}}'"'"';' 2>/dev/null)
    
    if [[ $? -ne 0 ]]; then
        write_colored_output "Error: Failed to execute helm commands" "red"
        exit 1
    fi
    
    # Output the results
    echo "$helm_output"
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
get_helm_charts_from_cluster "$CLUSTER_NAME"