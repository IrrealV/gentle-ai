package update

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// Package-level vars for testability (swap in tests via t.Cleanup).
var (
	execCommand   = exec.Command
	lookPath      = exec.LookPath
	osUserHomeDir = os.UserHomeDir
	osStat        = os.Stat
	runtimeGOOS   = runtime.GOOS
)

// versionRegexp extracts a semver-like version from command output.
// Same pattern as internal/system/deps.go for consistency.
var versionRegexp = regexp.MustCompile(`(\d+\.\d+(?:\.\d+)?)`)

// detectInstalledVersion determines the installed version of a tool.
// For tools with nil DetectCmd (gentle-ai), returns currentBuildVersion.
// For other tools, checks LookPath then runs the detect command.
func detectInstalledVersion(ctx context.Context, tool ToolInfo, currentBuildVersion string) string {
	if tool.DetectCmd == nil {
		return currentBuildVersion
	}

	if len(tool.DetectCmd) == 0 {
		return ""
	}

	binary := tool.DetectCmd[0]
	if _, err := lookPath(binary); err != nil {
		if binary == "gga" && ggaShimInstalledOnWindows() {
			return "unknown"
		}
		return "" // binary not found
	}

	cmd := execCommand(tool.DetectCmd[0], tool.DetectCmd[1:]...)
	out, err := cmd.Output()
	if err != nil {
		return "" // command failed — binary exists but version unknown
	}

	return parseVersionFromOutput(strings.TrimSpace(string(out)))
}

func ggaShimInstalledOnWindows() bool {
	if runtimeGOOS != "windows" {
		return false
	}

	homeDir, err := osUserHomeDir()
	if err != nil {
		return false
	}

	for _, name := range []string{"gga.ps1", "gga.exe", "gga"} {
		if _, err := osStat(filepath.Join(homeDir, "bin", name)); err == nil {
			return true
		}
	}

	return false
}

// parseVersionFromOutput extracts the first semver-like pattern from raw output.
func parseVersionFromOutput(output string) string {
	if output == "" {
		return ""
	}

	match := versionRegexp.FindStringSubmatch(output)
	if len(match) >= 2 {
		return match[1]
	}

	return ""
}
