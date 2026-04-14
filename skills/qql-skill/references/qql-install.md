# qql-go Install Guide

Use this guide when the skill is installed on its own and the `qql-go` CLI is not available yet.

## Preferred install paths

Install the latest release on macOS or Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/srimon12/qql-go/main/install.sh | sh
```

Install on Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/srimon12/qql-go/main/install.ps1 | iex
```

Install with Go:

```bash
go install github.com/srimon12/qql-go/cmd/qql-go@latest
```

Build from source:

```bash
git clone https://github.com/srimon12/qql-go.git
cd qql-go
go build -o qql-go ./cmd/qql-go
```

## Expected binary name

The CLI binary is named:

- `qql-go` on macOS/Linux
- `qql-go.exe` on Windows

## PATH expectations

The helper script first checks:

1. `QQL_BIN`
2. `qql-go` on `PATH`
3. a repo-local fallback

If `qql-go` is installed somewhere custom, set:

```bash
export QQL_BIN=/absolute/path/to/qql-go
```

On Windows:

```powershell
$env:QQL_BIN = "C:\path\to\qql-go.exe"
```
