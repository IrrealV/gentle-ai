package antigravity

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gentleman-programming/gentle-ai/internal/model"
	"github.com/gentleman-programming/gentle-ai/internal/system"
)

// makeAdapter builds a test Adapter with a stubbed stat function so tests
// never touch the real filesystem.
func makeAdapter(stat statResult) *Adapter {
	return &Adapter{
		statPath: func(string) statResult { return stat },
	}
}

// expectedConfigDir returns the path that GlobalConfigDir produces for the
// given homeDir under the current OS — mirrors platformConfigBase.
func expectedConfigDir(homeDir string) string {
	return filepath.Join(platformConfigBase(homeDir), "antigravity")
}

// --- Identity ---

func TestIdentity(t *testing.T) {
	a := NewAdapter()

	if got := a.Agent(); got != model.AgentAntigravity {
		t.Fatalf("Agent() = %q, want %q", got, model.AgentAntigravity)
	}
	if got := a.Tier(); got != model.TierFull {
		t.Fatalf("Tier() = %q, want %q", got, model.TierFull)
	}
}

// --- Strategies ---

func TestStrategies(t *testing.T) {
	a := NewAdapter()

	if got := a.SystemPromptStrategy(); got != model.StrategyAppendToFile {
		t.Fatalf("SystemPromptStrategy() = %v, want StrategyAppendToFile", got)
	}
	if got := a.MCPStrategy(); got != model.StrategyMCPConfigFile {
		t.Fatalf("MCPStrategy() = %v, want StrategyMCPConfigFile", got)
	}
}

// --- Detection ---

func TestDetect(t *testing.T) {
	const home = "/tmp/testhome"
	wantCfgPath := expectedConfigDir(home)

	tests := []struct {
		name            string
		stat            statResult
		wantInstalled   bool
		wantConfigPath  string
		wantConfigFound bool
		wantErr         bool
	}{
		{
			name:            "config directory present",
			stat:            statResult{isDir: true},
			wantInstalled:   true,
			wantConfigPath:  wantCfgPath,
			wantConfigFound: true,
		},
		{
			name:            "config directory missing",
			stat:            statResult{err: os.ErrNotExist},
			wantInstalled:   false,
			wantConfigPath:  wantCfgPath,
			wantConfigFound: false,
		},
		{
			name:    "stat error propagates",
			stat:    statResult{err: errors.New("permission denied")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := makeAdapter(tt.stat)
			installed, _, cfgPath, cfgFound, err := a.Detect(context.Background(), home)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Detect() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if installed != tt.wantInstalled {
				t.Fatalf("Detect() installed = %v, want %v", installed, tt.wantInstalled)
			}
			if cfgPath != tt.wantConfigPath {
				t.Fatalf("Detect() configPath = %q, want %q", cfgPath, tt.wantConfigPath)
			}
			if cfgFound != tt.wantConfigFound {
				t.Fatalf("Detect() configFound = %v, want %v", cfgFound, tt.wantConfigFound)
			}
		})
	}
}

// --- Config paths ---

func TestConfigPaths(t *testing.T) {
	const home = "/tmp/testhome"
	root := expectedConfigDir(home)
	a := NewAdapter()

	if got := a.GlobalConfigDir(home); got != root {
		t.Fatalf("GlobalConfigDir() = %q, want %q", got, root)
	}
	if got := a.SystemPromptDir(home); got != root {
		t.Fatalf("SystemPromptDir() = %q, want %q", got, root)
	}
	if got, want := a.SystemPromptFile(home), filepath.Join(root, "rules.md"); got != want {
		t.Fatalf("SystemPromptFile() = %q, want %q", got, want)
	}
	if got, want := a.SkillsDir(home), filepath.Join(root, "skills"); got != want {
		t.Fatalf("SkillsDir() = %q, want %q", got, want)
	}
	if got, want := a.SettingsPath(home), filepath.Join(root, "settings.json"); got != want {
		t.Fatalf("SettingsPath() = %q, want %q", got, want)
	}
	if got, want := a.MCPConfigPath(home, "ctx7"), filepath.Join(root, "mcp_config.json"); got != want {
		t.Fatalf("MCPConfigPath() = %q, want %q", got, want)
	}
}

// TestConfigPathsUseTempDir verifies that all path methods respect homeDir
// and never leak real system paths — the integration contract required by the
// project's test suite (which passes t.TempDir() as homeDir).
func TestConfigPathsUseTempDir(t *testing.T) {
	home := t.TempDir()
	a := NewAdapter()

	got := a.GlobalConfigDir(home)
	if !filepath.IsAbs(got) {
		t.Fatalf("GlobalConfigDir() = %q, want absolute path", got)
	}
	// Path must be rooted inside a directory that contains home as a prefix,
	// OR inside a known env-var config dir (e.g. $AppData on Windows).
	// On Linux/macOS without XDG_CONFIG_HOME set, it must be under home.
	if os.Getenv("XDG_CONFIG_HOME") == "" && os.Getenv("AppData") == "" {
		if !isUnder(got, home) {
			t.Fatalf("GlobalConfigDir() = %q, want path under homeDir %q", got, home)
		}
	}
}

func isUnder(path, base string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	// A path is under base when the relative path does NOT start with ".."
	// (which would indicate escaping the base). Hidden dirs like ".config"
	// start with "." but are still inside base.
	return rel != "" && !filepath.IsAbs(rel) && rel != ".." && len(rel) >= 2 && rel[:2] != ".."
}

// --- Installation ---

func TestDesktopIDENotAutoInstallable(t *testing.T) {
	a := NewAdapter()

	if a.SupportsAutoInstall() {
		t.Fatal("SupportsAutoInstall() = true, want false (desktop IDE)")
	}

	_, err := a.InstallCommand(system.PlatformProfile{})
	if err == nil {
		t.Fatal("InstallCommand() should return an error for a desktop IDE")
	}

	var notInstallable AgentNotInstallableError
	if !errors.As(err, &notInstallable) {
		t.Fatalf("InstallCommand() error type = %T, want AgentNotInstallableError", err)
	}
	if notInstallable.Agent != model.AgentAntigravity {
		t.Fatalf("AgentNotInstallableError.Agent = %q, want %q", notInstallable.Agent, model.AgentAntigravity)
	}
}

// --- Optional capabilities ---

func TestCapabilities(t *testing.T) {
	a := NewAdapter()

	if !a.SupportsSkills() {
		t.Error("SupportsSkills() = false, want true")
	}
	if !a.SupportsSystemPrompt() {
		t.Error("SupportsSystemPrompt() = false, want true")
	}
	if !a.SupportsMCP() {
		t.Error("SupportsMCP() = false, want true")
	}
	if a.SupportsOutputStyles() {
		t.Error("SupportsOutputStyles() = true, want false")
	}
	if a.SupportsSlashCommands() {
		t.Error("SupportsSlashCommands() = true, want false")
	}
	if got := a.OutputStyleDir("/tmp"); got != "" {
		t.Fatalf("OutputStyleDir() = %q, want empty", got)
	}
	if got := a.CommandsDir("/tmp"); got != "" {
		t.Fatalf("CommandsDir() = %q, want empty", got)
	}
}

// --- Error type ---

func TestAgentNotInstallableError_Message(t *testing.T) {
	err := AgentNotInstallableError{Agent: model.AgentAntigravity}
	want := fmt.Sprintf("agent %q is a desktop IDE and cannot be installed via CLI", string(model.AgentAntigravity))
	if err.Error() != want {
		t.Fatalf("Error() = %q, want %q", err.Error(), want)
	}
}
