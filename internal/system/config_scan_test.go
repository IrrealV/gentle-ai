package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKnownAgentConfigDirs_MatchesSupportedAgentSetOrder(t *testing.T) {
	home := t.TempDir()
	configs := knownAgentConfigDirs(home)

	expected := []struct {
		agent  string
		suffix string
	}{
		{agent: "claude-code", suffix: filepath.Join(".claude")},
		{agent: "opencode", suffix: filepath.Join(".config", "opencode")},
		{agent: "kilocode", suffix: filepath.Join(".config", "kilo")},
		{agent: "gemini-cli", suffix: filepath.Join(".gemini")},
		{agent: "codex", suffix: filepath.Join(".codex")},
		{agent: "cursor", suffix: filepath.Join(".cursor")},
		{agent: "vscode-copilot", suffix: filepath.Join(".copilot")},
		{agent: "antigravity", suffix: filepath.Join(".gemini", "antigravity")},
		{agent: "windsurf", suffix: filepath.Join(".codeium", "windsurf")},
		{agent: "qwen-code", suffix: filepath.Join(".qwen")},
		{agent: "kiro-ide", suffix: filepath.Join(".kiro")},
	}

	if len(configs) != len(expected) {
		t.Fatalf("knownAgentConfigDirs() len=%d, want %d", len(configs), len(expected))
	}

	for i, want := range expected {
		got := configs[i]
		if got.Agent != want.agent {
			t.Fatalf("knownAgentConfigDirs()[%d].Agent=%q, want %q", i, got.Agent, want.agent)
		}

		if got.Path != filepath.Join(home, want.suffix) {
			t.Fatalf("knownAgentConfigDirs()[%d].Path=%q, want %q", i, got.Path, filepath.Join(home, want.suffix))
		}
	}
}

// TestScanConfigs_ReturnsAllKnownAgentsWithExistsFlag verifies the canonical
// ScanConfigs contract: ALL known registry agents are returned, with Exists=true
// for those whose config dir is present on disk and Exists=false for those absent.
//
// This is the TUI contract: the detection screen shows "present"/"missing" for
// every known agent. The shim must enumerate all adapters from the default
// registry, not just the ones that are installed.
func TestScanConfigs_ReturnsAllKnownAgentsWithExistsFlag(t *testing.T) {
	home := t.TempDir()

	// Create only claude-code config dir — others intentionally absent.
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	configs := ScanConfigs(home)

	// Must return at least as many entries as the registry has adapters with
	// a non-empty GlobalConfigDir. Currently: claude, opencode, gemini, cursor,
	// vscode-copilot, codex = 6. Old implementation only returned 4.
	if len(configs) < 4 {
		t.Fatalf("ScanConfigs() returned %d entries, want >= 4; got %v", len(configs), configs)
	}

	// Find claude — must be Exists=true.
	var claudeState *ConfigState
	for i := range configs {
		if configs[i].Agent == "claude-code" {
			claudeState = &configs[i]
			break
		}
	}
	if claudeState == nil {
		t.Fatalf("ScanConfigs() missing claude-code entry; got %v", configs)
	}
	if !claudeState.Exists {
		t.Errorf("ScanConfigs() claude-code Exists = false, want true (dir was created)")
	}
	if !claudeState.IsDirectory {
		t.Errorf("ScanConfigs() claude-code IsDirectory = false, want true")
	}

	// All other agents must appear with Exists=false (their dirs weren't created).
	existsTrueCount := 0
	for _, c := range configs {
		if c.Exists {
			existsTrueCount++
		}
	}
	if existsTrueCount != 1 {
		t.Errorf("ScanConfigs() Exists=true count = %d, want 1 (only claude-code created); states: %v", existsTrueCount, configs)
	}
}

// TestScanConfigs_AgentFieldMatchesModelAgentID verifies that the Agent field
// in each ConfigState matches the canonical model.AgentID string values used
// by the TUI and validate.go switch statements.
func TestScanConfigs_AgentFieldMatchesModelAgentID(t *testing.T) {
	home := t.TempDir()
	configs := ScanConfigs(home)

	// These are the AgentID string values the TUI switch statements check.
	knownAgents := map[string]bool{
		"claude-code":    false,
		"opencode":       false,
		"kilocode":       false,
		"gemini-cli":     false,
		"codex":          false,
		"cursor":         false,
		"vscode-copilot": false,
		"antigravity":    false,
		"windsurf":       false,
		"qwen-code":      false,
		"kiro-ide":       false,
	}

	for _, c := range configs {
		if _, known := knownAgents[c.Agent]; known {
			knownAgents[c.Agent] = true
		}
	}

	// All known agents must appear.
	for agent, seen := range knownAgents {
		if !seen {
			t.Errorf("ScanConfigs() missing agent %q — TUI switch statements require it; got agents: %v", agent, agentNames(configs))
		}
	}
}

// TestScanConfigs_PathFieldIsNonEmpty verifies that every ConfigState has a
// non-empty Path — the TUI and validate.go use the path for display purposes.
func TestScanConfigs_PathFieldIsNonEmpty(t *testing.T) {
	home := t.TempDir()
	configs := ScanConfigs(home)

	for _, c := range configs {
		if c.Path == "" {
			t.Errorf("ScanConfigs() agent %q has empty Path — all entries must have non-empty paths", c.Agent)
		}
	}
}

// TestScanConfigs_ExistsFalseWhenDirAbsent verifies that agents whose
// GlobalConfigDir does not exist on disk have Exists=false.
func TestScanConfigs_ExistsFalseWhenDirAbsent(t *testing.T) {
	home := t.TempDir()
	// No dirs created — all agents should have Exists=false.

	configs := ScanConfigs(home)

	for _, c := range configs {
		if c.Exists {
			t.Errorf("ScanConfigs() agent %q Exists = true, want false (no dirs created)", c.Agent)
		}
	}
}

// TestScanConfigs_IsDirectorySetForExistingDirs verifies that IsDirectory is
// set correctly for existing directories.
func TestScanConfigs_IsDirectorySetForExistingDirs(t *testing.T) {
	home := t.TempDir()

	// Create two agent dirs.
	for _, rel := range []string{".claude", ".config/opencode"} {
		if err := os.MkdirAll(filepath.Join(home, rel), 0o755); err != nil {
			t.Fatalf("MkdirAll(%q): %v", rel, err)
		}
	}

	configs := ScanConfigs(home)

	claudeFound, opencodeFound := false, false
	for _, c := range configs {
		switch c.Agent {
		case "claude-code":
			claudeFound = true
			if !c.Exists || !c.IsDirectory {
				t.Errorf("claude-code: Exists=%v IsDirectory=%v, want both true", c.Exists, c.IsDirectory)
			}
		case "opencode":
			opencodeFound = true
			if !c.Exists || !c.IsDirectory {
				t.Errorf("opencode: Exists=%v IsDirectory=%v, want both true", c.Exists, c.IsDirectory)
			}
		}
	}

	if !claudeFound {
		t.Error("ScanConfigs() missing claude-code entry")
	}
	if !opencodeFound {
		t.Error("ScanConfigs() missing opencode entry")
	}
}

// agentNames extracts agent name strings for error messages.
func agentNames(configs []ConfigState) []string {
	names := make([]string, len(configs))
	for i, c := range configs {
		names[i] = c.Agent
	}
	return names
}
