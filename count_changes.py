#!/usr/bin/env python3
import subprocess
import re

# Get git log with shortstat
result = subprocess.run(['git', 'log', '--shortstat', '--oneline'], capture_output=True, text=True)
lines = result.stdout.split('\n')

total_insertions = 0
total_deletions = 0

for line in lines:
    if 'insertions' in line or 'insertion' in line:
        # Extract number before "insertions(+)"
        match = re.search(r'(\d+)\s+insertions?\(\+\)', line)
        if match:
            total_insertions += int(match.group(1))
    
    if 'deletions' in line or 'deletion' in line:
        # Extract number before "deletions(-)"
        match = re.search(r'(\d+)\s+deletions?\(-\)', line)
        if match:
            total_deletions += int(match.group(1))

print(f"Total insertions: {total_insertions}")
print(f"Total deletions: {total_deletions}")
print(f"Net change: {total_insertions - total_deletions}")
