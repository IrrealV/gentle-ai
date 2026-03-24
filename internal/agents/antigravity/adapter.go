package antigravity

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gentleman-programming/gentle-ai/internal/model"
	"github.com/gentleman-programming/gentle-ai/internal/system"
)

// statResult holds the outcome of a filesystem stat call.
// Kept package-local so the adapter can be tested without touching the real FS.
type statResult struct {
	isDir bool
	err   error
}

// Adapter implements agents.Adapter for the Antigravity IDE.
//
// Path contract: every method that takes homeDir uses it as the base — exactly
// like all other adapters in this project. The cross-platform config root is
// homeDir/.config/antigravity on Linux/macOS equivalent paths, resolved
// by platformConfigBase(). This keeps integration tests that pass t.TempDir()
// hermetic: no real system paths leak through.
type Adapter struct {
	statPath func(string) statResult
}

// NewAdapter returns an Adapter wired to the real filesystem.
func NewAdapter() *Adapter {
	return &Adapter{statPath: defaultStat}
}

// --- Identity ---

func (a *Adapter) Agent() model.AgentID {
	return model.AgentAntigravity
}

func (a *Adapter) Tier() model.SupportTier {
	return model.TierFull
}

// --- Detection ---

// Detect checks whether the Antigravity config directory is present.
// Antigravity is a desktop IDE — there is no binary on PATH to probe.
func (a *Adapter) Detect(_ context.Context, homeDir string) (bool, string, string, bool, error) {
	configPath := a.GlobalConfigDir(homeDir)

	stat := a.statPath(configPath)
	if stat.err != nil {
		if os.IsNotExist(stat.err) {
			return false, "", configPath, false, nil
		}
		return false, "", "", false, stat.err
	}

	// Desktop IDE — considered installed when the config dir is present.
	return stat.isDir, "", configPath, stat.isDir, nil
}

// --- Installation ---

func (a *Adapter) SupportsAutoInstall() bool {
	return false // Desktop IDE — cannot install via CLI.
}

func (a *Adapter) InstallCommand(_ system.PlatformProfile) ([][]string, error) {
	return nil, AgentNotInstallableError{Agent: model.AgentAntigravity}
}

// --- Config paths ---

// GlobalConfigDir returns the Antigravity config root for the given homeDir.
// The base directory is OS-native but always rooted in homeDir:
//
//	Windows: homeDir\AppData\Roaming\antigravity  (or %AppData%\antigravity)
//	macOS:   homeDir/Library/Application Support/antigravity
//	Linux:   $XDG_CONFIG_HOME/antigravity  or  homeDir/.config/antigravity
func (a *Adapter) GlobalConfigDir(homeDir string) string {
	return filepath.Join(platformConfigBase(homeDir), "antigravity")
}

func (a *Adapter) SystemPromptDir(homeDir string) string {
	return a.GlobalConfigDir(homeDir)
}

// SystemPromptFile returns the path to the Antigravity global rules file.
// Antigravity reads rules from rules.md inside its config directory.
func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(a.GlobalConfigDir(homeDir), "rules.md")
}

func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(a.GlobalConfigDir(homeDir), "skills")
}

// SettingsPath returns the path to Antigravity's settings.json.
// Gentle-AI merges content into this file (never overwrites it) so that
// user-defined settings — themes, fonts, preferred models — are preserved.
func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(a.GlobalConfigDir(homeDir), "settings.json")
}

// --- Config strategies ---

// SystemPromptStrategy returns StrategyAppendToFile because Antigravity's
// rules.md may contain user-defined rules that must be preserved.
func (a *Adapter) SystemPromptStrategy() model.SystemPromptStrategy {
	return model.StrategyAppendToFile
}

// MCPStrategy returns StrategyMCPConfigFile: Antigravity uses a dedicated
// mcp_config.json separate from settings.json.
func (a *Adapter) MCPStrategy() model.MCPStrategy {
	return model.StrategyMCPConfigFile
}

// --- MCP ---

func (a *Adapter) MCPConfigPath(homeDir string, _ string) string {
	return filepath.Join(a.GlobalConfigDir(homeDir), "mcp_config.json")
}

// --- Optional capabilities ---

func (a *Adapter) SupportsOutputStyles() bool     { return false }
func (a *Adapter) OutputStyleDir(_ string) string { return "" }
func (a *Adapter) SupportsSlashCommands() bool    { return false }
func (a *Adapter) CommandsDir(_ string) string    { return "" }
func (a *Adapter) SupportsSkills() bool           { return true }
func (a *Adapter) SupportsSystemPrompt() bool     { return true }
func (a *Adapter) SupportsMCP() bool              { return true }

// --- Helpers ---

// platformConfigBase returns the OS-native "config home" directory derived
// from homeDir. homeDir is always used as the root, so this function is
// hermetic: tests that pass an isolated temp dir receive a path inside that
// temp dir and never leak real system paths.
func platformConfigBase(homeDir string) string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(homeDir, "AppData", "Roaming")
	case "darwin":
		return filepath.Join(homeDir, "Library", "Application Support")
	default:
		return filepath.Join(homeDir, ".config")
	}
}

func defaultStat(path string) statResult {
	info, err := os.Stat(path)
	if err != nil {
		return statResult{err: err}
	}
	return statResult{isDir: info.IsDir()}
}

// --- Errors ---

// AgentNotInstallableError is returned when InstallCommand is called on a
// desktop-only IDE that cannot be managed via CLI.
type AgentNotInstallableError struct {
	Agent model.AgentID
}

func (e AgentNotInstallableError) Error() string {
	return fmt.Sprintf("agent %q is a desktop IDE and cannot be installed via CLI", string(e.Agent))
}
