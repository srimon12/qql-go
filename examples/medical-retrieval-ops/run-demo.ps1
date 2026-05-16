# run-demo.ps1 - Medical retrieval operations demo

. (Join-Path $PSScriptRoot "..\demo-lib.ps1")
Setup-Demo

Write-Host "🚀 Running medical retrieval operations..." -ForegroundColor Cyan

Doctor
Reset "medical_retrieval_ops"

Step "02-provision" execute (Join-Path $DEMO_ROOT "01-provision.qql")
Step "03-seed"      execute (Join-Path $DEMO_ROOT "02-seed.qql")
Step "04-inspect"   exec    "SHOW COLLECTION medical_retrieval_ops"
Step "05-stroke"    exec    "SEARCH medical_retrieval_ops SIMILAR TO 'acute stroke' LIMIT 3 USING HYBRID"
Step "06-cardiac"   exec    "SEARCH medical_retrieval_ops SIMILAR TO 'chest pain' LIMIT 3 USING HYBRID WHERE priority = 'high'"
Step "07-grouped"   exec    "SEARCH medical_retrieval_ops SIMILAR TO 'emergency' LIMIT 3 GROUP BY specialty GROUP_SIZE 2"
Step "08-recommend" exec    "RECOMMEND FROM medical_retrieval_ops POSITIVE IDS (3101) LIMIT 3"
Step "09-backup"    dump    "medical_retrieval_ops" (Join-Path $ARTIFACTS "backup.qql")

Finish
