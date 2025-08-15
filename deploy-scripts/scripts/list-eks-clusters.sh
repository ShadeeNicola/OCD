#!/bin/bash
# List EKS clusters with proper proxy setup

# Execute AWS CLI command with proxy setup like other scripts
bash -l -c "proxy on 2>/dev/null || true && aws eks list-clusters --query 'clusters[*]' --output text" 2>/dev/null