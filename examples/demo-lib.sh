#!/bin/bash

# demo-lib.sh - Fluent engine for qql-go demos

# --- INTERNAL HELPERS ---

resolve_qql_binary() {
    local repo_root="$1"
    
    # 1. Try PATH first (highest priority, handles WSL correctly)
    if command -v qql-go >/dev/null 2>&1; then
        command -v qql-go
        return 0
    fi

    # 2. Try repo root (Linux binary)
    if [ -f "$repo_root/qql-go" ]; then
        echo "$repo_root/qql-go"
        return 0
    fi
    
    # 3. Try repo root (Windows binary)
    if [ -f "$repo_root/qql-go.exe" ]; then
        echo "$repo_root/qql-go.exe"
        return 0
    fi
    
    echo "Error: Could not find qql-go binary." >&2
    exit 1
}

initialize_demo_artifacts() {
    local artifact_dir="$1/artifacts"
    [ -d "$artifact_dir" ] && rm -rf "$artifact_dir"
    mkdir -p "$artifact_dir"
    echo "$artifact_dir"
}

# --- FLUENT API (User Facing) ---

CYAN='\033[0;36m'
GREEN='\033[0;32m'
NC='\033[0m'

# Auto-setup paths and binary
setup_demo() {
    global_DEMO_ROOT="$1"
    [ -z "$global_DEMO_ROOT" ] && echo "Error: setup_demo requires DEMO_ROOT path" >&2 && exit 1
    
    global_REPO_ROOT="$(cd "$global_DEMO_ROOT/../.." && pwd)"
    global_QQL=$(resolve_qql_binary "$global_REPO_ROOT")
    global_ARTIFACTS=$(initialize_demo_artifacts "$global_DEMO_ROOT")
}

# Execute a named step
step() {
    local id="$1"
    shift
    local cmd="$1"
    shift
    
    local artifact="$global_ARTIFACTS/$id.json"
    local raw
    
    # Run qql-go
    raw=$("$global_QQL" "$cmd" --quiet --json "$@" 2>&1) || {
        echo -e "${NC}Error in step '$id':\n$raw" >&2
        exit 1
    }
    
    echo "$raw" > "$artifact"
    
    # Validation if jq is present
    if command -v jq >/dev/null 2>&1; then
        local ok=$(echo "$raw" | jq -r '.ok 2>/dev/null || "false"')
        if [ "$ok" != "true" ]; then
            local err=$(echo "$raw" | jq -r '.error 2>/dev/null || "Unknown error"')
            echo -e "Step '$id' failed: $err" >&2
            exit 1
        fi
    fi
}

reset() {
    "$global_QQL" exec --quiet --json "DROP COLLECTION $1" > /dev/null 2>&1 || true
}

doctor() {
    step "01-doctor" doctor
}

finish() {
    echo -e "${GREEN}✓ Workflow complete. Artifacts saved to: $global_ARTIFACTS${NC}"
}
