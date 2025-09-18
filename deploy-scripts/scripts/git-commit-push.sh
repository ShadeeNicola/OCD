#!/usr/bin/env bash
set -euo pipefail

proxy on 2>/dev/null || true

REPO_DIR=""
BRANCH=""
MESSAGE=""
USERNAME=""
TOKEN=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo-dir) REPO_DIR="$2"; shift 2 ;;
    --branch) BRANCH="$2"; shift 2 ;;
    --message) MESSAGE="$2"; shift 2 ;;
    --username) USERNAME="$2"; shift 2 ;;
    --token) TOKEN="$2"; shift 2 ;;
    *) echo "Unknown arg: $1" 1>&2; exit 2 ;;
  esac
done

if [[ -z "$REPO_DIR" || -z "$BRANCH" || -z "$MESSAGE" ]]; then
  echo "ERROR: Missing required args (repo-dir/branch/message)" 1>&2
  exit 1
fi

cd "$REPO_DIR"

AUTH_ARGS=()
if [[ -n "$USERNAME" && -n "$TOKEN" ]]; then
  BASIC="$(printf "%s:%s" "$USERNAME" "$TOKEN" | base64 -w0 2>/dev/null || printf "%s:%s" "$USERNAME" "$TOKEN" | base64 | tr -d '\n')"
  AUTH_ARGS=(-c "http.extraHeader=Authorization: Basic $BASIC")
fi

echo "[HF][git-push] REPO_DIR=$REPO_DIR BRANCH=$BRANCH" 1>&2
echo "[HF][git-push] NO_PROXY=${NO_PROXY:-}" 1>&2

git add pom.xml
git commit -m "$MESSAGE" || echo "No changes to commit" && true

# Ensure we respect NO_PROXY and bypass any env proxy for internal host
export HTTP_PROXY="" HTTPS_PROXY="" ALL_PROXY="" http_proxy="" https_proxy="" all_proxy=""
export GIT_TRACE_CURL=1 GIT_CURL_VERBOSE=1
MASKED_HDR="Authorization: Basic ***masked***"
echo "[HF][git-push] OVERRIDE: cleared HTTP_PROXY/HTTPS_PROXY/ALL_PROXY for git process" 1>&2
echo "[HF][git-push] CMD: git -c \"http.proxy=\" -c \"https.proxy=\" -c \"http.noProxy=$NO_PROXY\" -c \"http.version=HTTP/1.1\" -c \"http.expect=never\" -c \"http.extraHeader=$MASKED_HDR\" push origin $BRANCH" 1>&2

# Try primary push (HTTP/1.1)
set +e
git -c "http.proxy=" -c "https.proxy=" -c "http.noProxy=$NO_PROXY" -c "http.version=HTTP/1.1" -c "http.expect=never" "${AUTH_ARGS[@]}" push origin "$BRANCH"
rc=$?
set -e

if [[ $rc -ne 0 ]]; then
  echo "[HF][git-push] primary push failed (rc=$rc). Retrying with HTTP/1.0 + postBuffer..." 1>&2
  echo "[HF][git-push] RETRY CMD: git -c \"http.proxy=\" -c \"https.proxy=\" -c \"http.noProxy=$NO_PROXY\" -c \"http.version=HTTP/1.0\" -c \"http.postBuffer=524288000\" -c \"http.maxRequests=1\" -c \"http.expect=never\" -c \"http.extraHeader=$MASKED_HDR\" push origin $BRANCH" 1>&2
  git -c "http.proxy=" -c "https.proxy=" -c "http.noProxy=$NO_PROXY" -c "http.version=HTTP/1.0" -c "http.postBuffer=524288000" -c "http.maxRequests=1" -c "http.expect=never" "${AUTH_ARGS[@]}" push origin "$BRANCH"
fi
echo "OK:PUSHED:$BRANCH"


