#!/bin/bash

# run-demo.sh - Support incident response demo

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

echo -e "${CYAN}🚀 Running support incident response...${NC}"

doctor
reset "support_incident_response"

step "02-seed"      execute "$global_DEMO_ROOT/01-seed.qql"
step "03-inspect"   exec    "SHOW COLLECTION support_incident_response"
step "04-select"    exec    "SELECT * FROM support_incident_response WHERE id = 2101"
step "05-scroll"    exec    "SCROLL FROM support_incident_response WHERE priority = 'high' LIMIT 10"
step "06-explain"   explain "SEARCH support_incident_response SIMILAR TO 'billing search' LIMIT 3 USING HYBRID"
step "07-recommend" exec    "RECOMMEND FROM support_incident_response POSITIVE IDS (2104) LIMIT 3"
step "08-update"    exec    "UPDATE support_incident_response SET PAYLOAD WHERE id = 2104 {'status': 'reviewed'}"

finish
