#!/bin/bash

# run-demo.sh - Release validation for retrieval changes

set -e

# Robust path to library
DEMO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_PATH="$DEMO_ROOT/../demo-lib.sh"
if [ ! -f "$LIB_PATH" ]; then
    echo "Error: Could not find $LIB_PATH" >&2
    exit 1
fi
source "$LIB_PATH"
setup_demo "$DEMO_ROOT"

echo -e "${CYAN}🚀 Running retrieval release validation...${NC}"

doctor
reset "release_validation_docs"

step "02-provision"  execute "$global_DEMO_ROOT/01-provision.qql"
step "03-seed"       execute "$global_DEMO_ROOT/02-seed.qql"
step "04-inspect"    exec    "SHOW COLLECTION release_validation_docs"
step "05-explain"    explain "SEARCH release_validation_docs SIMILAR TO 'refund policy' LIMIT 3 USING HYBRID"
step "06-validate"   execute "$global_DEMO_ROOT/03-validate.qql"
step "07-search"     exec    "SEARCH release_validation_docs SIMILAR TO 'billing' LIMIT 3 USING SPARSE"
step "08-grouped"    exec    "SEARCH release_validation_docs SIMILAR TO 'security' LIMIT 3 GROUP BY team GROUP_SIZE 2"
step "09-backup"     dump    "release_validation_docs" "$global_ARTIFACTS/backup.qql"

finish
