package system

import (
	"os"
	"path/filepath"
	"runtime"
)

// ConfigState records the filesystem presence of an agent's global config directory.
// All known registry agents are always represented — Exists=false for absent dirs.
// This contract is consumed by the TUI detection screen and install/validate flows.
type ConfigState struct {
	Agent       string
	Path        string
	Exists      bool
	IsDirectory bool
}

// knownAgentConfigDirs enumerates every agent's GlobalConfigDir as a
// (agentID, path) pair for the given homeDir. This is a compatibility shim
// that mirrors the adapter registry's full set without importing the agents
// package (which would create an import cycle: system ← agents ← system).
//
// When a new agent is added to the registry, its entry must also be added here
// until the import cycle is resolved and ScanConfigs can delegate directly to
// agents.DiscoverInstalled.
func knownAgentConfigDirs(homeDir string) []ConfigState {
	return []ConfigState{
		{Agent: "claude-code", Path: filepath.Join(homeDir, ".claude")},
		{Agent: "opencode", Path: filepath.Join(homeDir, ".config", "opencode")},
		{Agent: "gemini-cli", Path: filepath.Join(homeDir, ".gemini")},
		{Agent: "cursor", Path: filepath.Join(homeDir, ".cursor")},
		{Agent: "vscode-copilot", Path: vscodeCopilotGlobalConfigDir(homeDir)},
		{Agent: "codex", Path: filepath.Join(homeDir, ".codex")},
		{Agent: "antigravity", Path: antigravityGlobalConfigDir(homeDir)},
	}
}

// vscodeCopilotGlobalConfigDir returns ~/.copilot, the GlobalConfigDir used by
// the vscode-copilot adapter across all platforms. The vscode adapter's
// SystemPromptDir and SettingsPath are OS-dependent, but GlobalConfigDir is not.
func vscodeCopilotGlobalConfigDir(homeDir string) string {
	return filepath.Join(homeDir, ".copilot")
}

// antigravityGlobalConfigDir returns the OS-native path for Antigravity's
// config root. It mirrors the configDir() logic from the antigravity adapter
// without importing it (import-cycle shim).
//
// IMPORTANT: like all other helpers in this shim, homeDir is used as the base
// so tests that pass an isolated temp directory remain hermetic.
// os.UserConfigDir() is intentionally NOT used here because it always resolves
// to the real system path, which breaks tests when ~/.config/antigravity exists.
//
//   - Windows: %AppData%\antigravity  (fallback: homeDir\AppData\Roaming)
//   - macOS:   homeDir/Library/Application Support/antigravity
//   - Linux:   $XDG_CONFIG_HOME/antigravity  or  homeDir/.config/antigravity
func antigravityGlobalConfigDir(homeDir string) string {
	var base string
	switch runtime.GOOS {
	case "windows":
		if appData := os.Getenv("AppData"); appData != "" {
			base = appData
		} else {
			base = filepath.Join(homeDir, "AppData", "Roaming")
		}
	case "darwin":
		base = filepath.Join(homeDir, "Library", "Application Support")
	default:
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			base = xdg
		} else {
			base = filepath.Join(homeDir, ".config")
		}
	}
	return filepath.Join(base, "antigravity")
}

// ScanConfigs returns the presence state of every known managed agent's global
// config directory. All agents are always represented in the result; Exists and
// IsDirectory reflect the actual filesystem state at call time.
//
// This is a compatibility shim: it preserves the ConfigState contract for TUI
// and validation callers while the canonical discovery (agents.DiscoverInstalled)
// is used by sync and upgrade flows. Full delegation is deferred until the
// system ← agents import cycle is resolved (follow-up change).
func ScanConfigs(homeDir string) []ConfigState {
	states := knownAgentConfigDirs(homeDir)

	for idx := range states {
		info, err := os.Stat(states[idx].Path)
		if err != nil {
			continue
		}

		states[idx].Exists = true
		states[idx].IsDirectory = info.IsDir()
	}

	return states
}
