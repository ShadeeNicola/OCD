#!/bin/bash
# Shared argument parsing for OCD scripts

# Initialize common variables with defaults
initialize_common_arguments() {
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
}

# Parse common command line arguments
parse_common_arguments() {
    local script_name="$1"
    shift  # Remove script name from arguments
    
    initialize_common_arguments
    
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
                show_help "$script_name"
                exit 0
                ;;
            *)
                echo "Unknown option: $1"
                exit 1
                ;;
        esac
    done
}

# Show help message
show_help() {
    local script_name="$1"
    echo "Usage: $script_name [OPTIONS]"
    echo "Options:"
    echo "  -n, --namespace NS       Kubernetes namespace (default: dop)"
    echo "  --skip-build            Skip build phase"
    echo "  --skip-deploy           Skip deploy phase"
    echo "  --force                 Run even if no changes detected"
    echo "  --confirm               Prompt for confirmation before deployment"
    echo "  -v, --verbose           Show detailed command output"
    echo "  -h, --help              Show this help"
}