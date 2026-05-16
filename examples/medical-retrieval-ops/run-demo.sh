#!/bin/bash

# run-demo.sh - Medical retrieval operations demo

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

echo -e "${CYAN}🚀 Running medical retrieval operations...${NC}"

doctor
reset "medical_retrieval_ops"

step "02-provision" execute "$global_DEMO_ROOT/01-provision.qql"
step "03-seed"      execute "$global_DEMO_ROOT/02-seed.qql"
step "04-inspect"   exec    "SHOW COLLECTION medical_retrieval_ops"
step "05-stroke"    exec    "SEARCH medical_retrieval_ops SIMILAR TO 'acute stroke' LIMIT 3 USING HYBRID"
step "06-cardiac"   exec    "SEARCH medical_retrieval_ops SIMILAR TO 'chest pain' LIMIT 3 USING HYBRID WHERE priority = 'high'"
step "07-grouped"   exec    "SEARCH medical_retrieval_ops SIMILAR TO 'emergency' LIMIT 3 GROUP BY specialty GROUP_SIZE 2"
step "08-recommend" exec    "RECOMMEND FROM medical_retrieval_ops POSITIVE IDS (3101) LIMIT 3"
step "09-backup"    dump    "medical_retrieval_ops" "$global_ARTIFACTS/backup.qql"

finish
