# run-demo.ps1 - Support incident response demo

. (Join-Path $PSScriptRoot "..\demo-lib.ps1")
Setup-Demo

Write-Host "🚀 Running support incident response..." -ForegroundColor Cyan

Doctor
Reset "support_incident_response"

Step "02-seed"      execute (Join-Path $DEMO_ROOT "01-seed.qql")
Step "03-inspect"   exec    "SHOW COLLECTION support_incident_response"
Step "04-select"    exec    "SELECT * FROM support_incident_response WHERE id = 2101"
Step "05-scroll"    exec    "SCROLL FROM support_incident_response WHERE priority = 'high' LIMIT 10"
Step "06-explain"   explain "SEARCH support_incident_response SIMILAR TO 'billing search' LIMIT 3 USING HYBRID"
Step "07-recommend" exec    "RECOMMEND FROM support_incident_response POSITIVE IDS (2104) LIMIT 3"
Step "08-update"    exec    "UPDATE support_incident_response SET PAYLOAD WHERE id = 2104 {'status': 'reviewed'}"

Finish
