//go:build ignore

// dev_tasks.go — Developer automation for qql-go
//
// Usage:
//
//	go run docs/dev_tasks.go fmt
//	go run docs/dev_tasks.go check
//	go run docs/dev_tasks.go prepare-release --version 0.2.0
//	go run docs/dev_tasks.go release-validate [--version 0.2.0]
//
// This script is intentionally self-contained and uses only the Go standard library.
// It can be run from the repository root without any external dependencies.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

var (
	rootDir string

	// Files that must agree on the release version
	versionFile string
	commandsGo  string
)

func init() {
	// Find repository root (parent of developer_guide/)
	_, thisFile, _, _ := runtime.Caller(0)
	rootDir = filepath.Dir(filepath.Dir(thisFile))

	versionFile = filepath.Join(rootDir, "VERSION")
	commandsGo = filepath.Join(rootDir, "internal", "cli", "commands", "executor.go")
}

// semver pattern: MAJOR.MINOR.PATCH
var semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

func normalizeVersion(v string) string {
	v = strings.TrimPrefix(v, "v")
	if !semverRe.MatchString(v) {
		fatalf("invalid version %q; expected MAJOR.MINOR.PATCH", v)
	}
	return v
}

func readVersionFile() string {
	data, err := os.ReadFile(versionFile)
	if err != nil {
		fatalf("cannot read VERSION: %v", err)
	}
	return strings.TrimSpace(string(data))
}

func readCommandsGoVersion() string {
	data, err := os.ReadFile(commandsGo)
	if err != nil {
		fatalf("cannot read %s: %v", commandsGo, err)
	}
	re := regexp.MustCompile(`var\s+Version\s*=\s*"([^"]+)"`)
	m := re.FindStringSubmatch(string(data))
	if m == nil {
		fatalf("cannot find var Version in %s", commandsGo)
	}
	return m[1]
}

func assertVersionSync() string {
	version := readVersionFile()
	cmdVersion := readCommandsGoVersion()
	if version != cmdVersion {
		fatalf("version mismatch: VERSION=%s, commands.go=%s", version, cmdVersion)
	}
	return version
}

// --- Commands ---

func cmdFmt() {
	fmt.Println("Running gofmt on all Go files...")
	files := findGoFiles()
	changed := false
	for _, f := range files {
		out, err := exec.Command("gofmt", "-l", f).Output()
		if err != nil {
			fatalf("gofmt failed on %s: %v", f, err)
		}
		if len(strings.TrimSpace(string(out))) > 0 {
			changed = true
			fmt.Printf("  formatting %s\n", filepath.Base(f))
			runCmd("gofmt", "-w", f)
		}
	}
	if !changed {
		fmt.Println("All files already formatted.")
	}
}

func cmdCheck() {
	fmt.Println("=== Running quality checks ===\n")

	// 1. Verify version sync
	fmt.Println("[1/5] Version sync...")
	version := assertVersionSync()
	fmt.Printf("  VERSION=%s, commands.go=%s — OK\n\n", version, version)

	// 2. gofmt check
	fmt.Println("[2/5] gofmt --check...")
	files := findGoFiles()
	unformatted := []string{}
	for _, f := range files {
		out, err := exec.Command("gofmt", "-l", f).Output()
		if err != nil {
			fatalf("gofmt failed on %s: %v", f, err)
		}
		if len(strings.TrimSpace(string(out))) > 0 {
			unformatted = append(unformatted, f)
		}
	}
	if len(unformatted) > 0 {
		fmt.Println("  FAIL: unformatted files:")
		for _, f := range unformatted {
			fmt.Printf("    %s\n", f)
		}
		fmt.Println("\n  Run: go run docs/dev_tasks.go fmt")
		os.Exit(1)
	}
	fmt.Println("  OK\n")

	// 3. go vet
	fmt.Println("[3/5] go vet ./...")
	runCmd("go", "vet", "./...")
	fmt.Println("  OK\n")

	// 4. go test
	fmt.Println("[4/5] go test ./...")
	runCmd("go", "test", "./...")
	fmt.Println("  OK\n")

	// 5. go build
	fmt.Println("[5/5] go build ./cmd/qql-go...")
	runCmd("go", "build", "./cmd/qql-go")
	fmt.Println("  OK\n")

	fmt.Println("=== All checks passed ===")
}

func cmdPrepareRelease(version string) {
	version = normalizeVersion(version)
	today := time.Now().Format("2006-01-02")

	fmt.Printf("Preparing release %s...\n\n", version)

	// 1. Update VERSION file
	writeFile(versionFile, version+"\n")
	fmt.Printf("  Updated %s\n", filepath.Base(versionFile))

	// 2. Update commands.go
	updateCommandsGoVersion(version)
	fmt.Printf("  Updated %s\n", filepath.Base(commandsGo))

	// 3. Ensure release notes exist
	releaseNotes := filepath.Join(rootDir, "docs", "releases", version+".md")
	ensureReleaseNotes(releaseNotes, version, today)
	fmt.Printf("  Updated %s\n", filepath.Base(releaseNotes))

	// 4. Ensure CHANGELOG entry exists
	changelog := filepath.Join(rootDir, "CHANGELOG.md")
	ensureChangelogEntry(changelog, version, today)
	fmt.Printf("  Updated %s\n", filepath.Base(changelog))

	fmt.Printf("\nRelease %s prepared. Review the files and commit.\n", version)
}

func cmdReleaseValidate(version string) {
	version = normalizeVersion(version)

	fmt.Printf("=== Validating release %s ===\n\n", version)

	// 1. Verify version sync
	fmt.Println("[1/4] Version sync...")
	assertVersionSync()
	fmt.Println("  OK\n")

	// 2. Run checks
	fmt.Println("[2/4] Quality checks...")
	cmdCheck()
	fmt.Println()

	// 3. Build release binary
	fmt.Printf("[3/4] Building release binary...\n")
	binaryName := "qql-go"
	if runtime.GOOS == "windows" {
		binaryName = "qql-go.exe"
	}

	distDir := filepath.Join(rootDir, "dist", "local")
	os.MkdirAll(distDir, 0o755)
	binaryPath := filepath.Join(distDir, binaryName)

	// Build with ldflags to embed version
	runCmd("go", "build",
		"-ldflags", fmt.Sprintf("-X main.Version=%s", version),
		"-o", binaryPath,
		"./cmd/qql-go",
	)
	fmt.Printf("  Built: %s\n\n", binaryPath)

	// 4. Verify binary reports version
	fmt.Println("[4/4] Verifying binary version...")
	out := captureCmd(binaryPath, "--version")
	gotVersion := strings.TrimSpace(string(out))
	if !strings.Contains(gotVersion, version) {
		fatalf("binary version mismatch: expected %s, got %q", version, gotVersion)
	}
	fmt.Printf("  Binary reports: %s — OK\n\n", gotVersion)

	fmt.Println("=== Release validation passed ===")
	fmt.Printf("Binary: %s\n", binaryPath)
}

// --- Helpers ---

func findGoFiles() []string {
	var files []string
	filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		// Skip hidden dirs, vendor, dist
		name := d.Name()
		if strings.HasPrefix(name, ".") || name == "vendor" || name == "dist" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(name, ".go") {
			files = append(files, path)
		}
		return nil
	})
	return files
}

func runCmd(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = rootDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatalf("command failed: %s %s", name, strings.Join(args, " "))
	}
}

func captureCmd(name string, args ...string) []byte {
	cmd := exec.Command(name, args...)
	cmd.Dir = rootDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		fatalf("command failed: %s %s\n%s", name, strings.Join(args, " "), out)
	}
	return out
}

func writeFile(path, content string) {
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		fatalf("cannot write %s: %v", path, err)
	}
}

func updateCommandsGoVersion(version string) {
	data, err := os.ReadFile(commandsGo)
	if err != nil {
		fatalf("cannot read %s: %v", commandsGo, err)
	}
	re := regexp.MustCompile(`var\s+Version\s*=\s*"[^"]+"`)
	updated := re.ReplaceAllString(string(data), fmt.Sprintf(`var Version = "%s"`, version))
	if updated == string(data) {
		fatalf("failed to update Version in %s", commandsGo)
	}
	writeFile(commandsGo, updated)
}

func ensureReleaseNotes(path, version, today string) {
	if _, err := os.Stat(path); err == nil {
		return // already exists
	}

	content := fmt.Sprintf(`# qql-go %s

Release date: %s

## Summary

%s adds new features and fixes to the QQL CLI for Qdrant.

## Highlights

- TODO: summarize the most important user-facing changes for this release.

## Packaging

Release bundles include:

- `+"`qql-go`"+` (Linux amd64, Linux arm64, Windows amd64, macOS arm64)

Tagged releases also publish:

- `+"`qql-go_%s_checksums.txt`"+`

## Known limits

- `+"`RERANK`"+` is cloud-only. Local and external modes reject rerank queries.
`, version, today, version, version)

	writeFile(path, content)
}

func ensureChangelogEntry(path, version, today string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fatalf("cannot read CHANGELOG.md: %v", err)
	}
	content := string(data)

	// Check if entry already exists
	header := fmt.Sprintf("## [%s] - %s", version, today)
	if strings.Contains(content, header) {
		return
	}

	// Replace Unreleased marker
	marker := "## [Unreleased]\n\n- No unreleased changes yet.\n"
	if !strings.Contains(content, marker) {
		fatalf("CHANGELOG.md does not contain the expected Unreleased marker")
	}

	entry := fmt.Sprintf(`%s
## [%s] - %s

### Added

- TODO: summarize the release additions.

### Changed

- TODO: summarize notable behavior or workflow changes.

### Fixed

- TODO: summarize important fixes.
`, marker, version, today)

	writeFile(path, strings.Replace(content, marker, entry, 1))
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "fmt":
		cmdFmt()
	case "check":
		cmdCheck()
	case "prepare-release":
		version := parseFlag(os.Args[2:], "--version")
		if version == "" {
			fatalf("usage: dev_tasks.go prepare-release --version X.Y.Z")
		}
		cmdPrepareRelease(version)
	case "release-validate":
		version := parseFlag(os.Args[2:], "--version")
		if version == "" {
			version = readVersionFile()
		}
		cmdReleaseValidate(version)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func parseFlag(args []string, flag string) string {
	for i, arg := range args {
		if arg == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func printUsage() {
	fmt.Println(`Developer automation for qql-go

Usage:
  go run docs/dev_tasks.go <command> [flags]

Commands:
  fmt                     Apply gofmt to all Go files
  check                   Run version sync, gofmt, go vet, go test, go build
  prepare-release         Update VERSION, commands.go, CHANGELOG.md, release notes
    --version X.Y.Z       Required. Semantic version to prepare.
  release-validate        Run checks, build binary, verify --version output
    --version X.Y.Z       Optional. Defaults to VERSION file.

Examples:
  go run docs/dev_tasks.go check
  go run docs/dev_tasks.go prepare-release --version 0.2.0
  go run docs/dev_tasks.go release-validate
  go run docs/dev_tasks.go release-validate --version 0.2.0`)
}
