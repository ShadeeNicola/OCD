#!/usr/bin/env bash
set -euo pipefail

# Ensure corporate proxy is configured if available
proxy on 2>/dev/null || true

REPO=""
BRANCH=""
TARGET_DIR=""
USERNAME=""
TOKEN=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo) REPO="$2"; shift 2 ;;
    --branch) BRANCH="$2"; shift 2 ;;
    --dir) TARGET_DIR="$2"; shift 2 ;;
    --username) USERNAME="$2"; shift 2 ;;
    --token) TOKEN="$2"; shift 2 ;;
    *) echo "Unknown arg: $1" 1>&2; exit 2 ;;
  esac
done

if [[ -z "$REPO" || -z "$BRANCH" || -z "$TARGET_DIR" ]]; then
  echo "ERROR: Missing required args (repo/branch/dir)" 1>&2
  exit 1
fi

mkdir -p "$(dirname "$TARGET_DIR")"

# Debug: print inputs (mask token)
echo "[HF][git-clone] REPO=$REPO" 1>&2
echo "[HF][git-clone] BRANCH=$BRANCH" 1>&2
echo "[HF][git-clone] TARGET_DIR=$TARGET_DIR" 1>&2
if [[ -n "${USERNAME:-}" ]]; then echo "[HF][git-clone] USERNAME=$USERNAME" 1>&2; fi
if [[ -n "${TOKEN:-}" ]]; then echo "[HF][git-clone] TOKEN=***masked***" 1>&2; fi
echo "[HF][git-clone] HTTP_PROXY=${HTTP_PROXY:-} HTTPS_PROXY=${HTTPS_PROXY:-} NO_PROXY=${NO_PROXY:-}" 1>&2

AUTH_ARGS=()
if [[ -n "$USERNAME" && -n "$TOKEN" ]]; then
  BASIC="$(printf "%s:%s" "$USERNAME" "$TOKEN" | base64 -w0 2>/dev/null || printf "%s:%s" "$USERNAME" "$TOKEN" | base64 | tr -d '\n')"
  AUTH_ARGS=(-c "http.extraHeader=Authorization: Basic $BASIC")
fi

# Extract host and optional port from the repo URL
HOSTPORT="$(echo "$REPO" | sed -E 's#^[a-zA-Z]+://([^/]+)/.*#\1#')"
HOST="${HOSTPORT%%:*}"
PORT="${HOSTPORT##*:}"
if [[ "$PORT" == "$HOST" ]]; then PORT=""; fi

# Bypass proxy for internal host
if [[ -n "$HOST" ]]; then
  if [[ -n "${NO_PROXY:-}" ]]; then export NO_PROXY="$NO_PROXY,$HOST"; else export NO_PROXY="$HOST"; fi
  if [[ -n "$PORT" ]]; then export NO_PROXY="$NO_PROXY,$HOST:$PORT"; fi
fi

MASKED_HDR="Authorization: Basic ***masked***"

# Hard-bypass any environment proxies for this git process
export HTTP_PROXY="" HTTPS_PROXY="" ALL_PROXY="" http_proxy="" https_proxy="" all_proxy=""
echo "[HF][git-clone] OVERRIDE: cleared HTTP_PROXY/HTTPS_PROXY/ALL_PROXY for git process" 1>&2

echo "[HF][git-clone] NO_PROXY=$NO_PROXY" 1>&2
echo "[HF][git-clone] CMD: git -c \"http.proxy=\" -c \"https.proxy=\" -c \"http.noProxy=$NO_PROXY\" -c \"http.version=HTTP/1.1\" -c \"http.extraHeader=$MASKED_HDR\" clone --depth 1 --branch $BRANCH $REPO $TARGET_DIR" 1>&2

git -c "http.proxy=" -c "https.proxy=" -c "http.noProxy=$NO_PROXY" -c "http.version=HTTP/1.1" "${AUTH_ARGS[@]}" clone --depth 1 --branch "$BRANCH" "$REPO" "$TARGET_DIR"
echo "OK:CLONED:$TARGET_DIR"


