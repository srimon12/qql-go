# run-demo.ps1 - Release validation for retrieval changes

. (Join-Path $PSScriptRoot "..\demo-lib.ps1")
Setup-Demo

Write-Host "🚀 Running retrieval release validation..." -ForegroundColor Cyan

Doctor
Reset "release_validation_docs"

Step "02-provision" execute (Join-Path $DEMO_ROOT "01-provision.qql")
Step "03-seed"      execute (Join-Path $DEMO_ROOT "02-seed.qql")
Step "04-inspect"   exec    "SHOW COLLECTION release_validation_docs"
Step "05-explain"   explain "SEARCH release_validation_docs SIMILAR TO 'refund policy' LIMIT 3 USING HYBRID"
Step "06-validate"  execute (Join-Path $DEMO_ROOT "03-validate.qql")
Step "07-search"    exec    "SEARCH release_validation_docs SIMILAR TO 'billing' LIMIT 3 USING SPARSE"
Step "08-grouped"   exec    "SEARCH release_validation_docs SIMILAR TO 'security' LIMIT 3 GROUP BY team GROUP_SIZE 2"
Step "09-backup"    dump    "release_validation_docs" (Join-Path $ARTIFACTS "backup.qql")

Finish
