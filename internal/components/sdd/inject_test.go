package sdd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/internal/agents"
	"github.com/gentleman-programming/gentle-ai/internal/agents/claude"
	"github.com/gentleman-programming/gentle-ai/internal/agents/opencode"
	// agents/cursor, agents/gemini, agents/vscode used via agents.NewAdapter()
)

func claudeAdapter() agents.Adapter   { return claude.NewAdapter() }
func opencodeAdapter() agents.Adapter { return opencode.NewAdapter() }

func TestInjectClaudeWritesSectionMarkers(t *testing.T) {
	home := t.TempDir()

	result, err := Inject(home, claudeAdapter())
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("Inject() first changed = false")
	}

	path := filepath.Join(home, ".claude", "CLAUDE.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	text := string(content)

	if !strings.Contains(text, "<!-- gentle-ai:sdd-orchestrator -->") {
		t.Fatal("CLAUDE.md missing open marker for sdd-orchestrator")
	}
	if !strings.Contains(text, "<!-- /gentle-ai:sdd-orchestrator -->") {
		t.Fatal("CLAUDE.md missing close marker for sdd-orchestrator")
	}
	if !strings.Contains(text, "sub-agent") {
		t.Fatal("CLAUDE.md missing real SDD orchestrator content (expected 'sub-agent')")
	}
	if !strings.Contains(text, "dependency") {
		t.Fatal("CLAUDE.md missing real SDD orchestrator content (expected 'dependency')")
	}
}

func TestInjectClaudePreservesExistingSections(t *testing.T) {
	home := t.TempDir()
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	existing := "# My Config\n\nSome user content.\n"
	if err := os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte(existing), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := Inject(home, claudeAdapter())
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}

	content, err := os.ReadFile(filepath.Join(claudeDir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	text := string(content)
	if !strings.Contains(text, "Some user content.") {
		t.Fatal("Existing user content was clobbered")
	}
	if !strings.Contains(text, "<!-- gentle-ai:sdd-orchestrator -->") {
		t.Fatal("SDD section was not injected")
	}
}

func TestInjectClaudeIsIdempotent(t *testing.T) {
	home := t.TempDir()

	first, err := Inject(home, claudeAdapter())
	if err != nil {
		t.Fatalf("Inject() first error = %v", err)
	}
	if !first.Changed {
		t.Fatalf("Inject() first changed = false")
	}

	second, err := Inject(home, claudeAdapter())
	if err != nil {
		t.Fatalf("Inject() second error = %v", err)
	}
	if second.Changed {
		t.Fatalf("Inject() second changed = true")
	}
}

func TestInjectOpenCodeWritesCommandFiles(t *testing.T) {
	home := t.TempDir()

	result, err := Inject(home, opencodeAdapter())
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("Inject() first changed = false")
	}

	if len(result.Files) == 0 {
		t.Fatal("Inject() returned no files")
	}

	commandPath := filepath.Join(home, ".config", "opencode", "commands", "sdd-init.md")
	content, err := os.ReadFile(commandPath)
	if err != nil {
		t.Fatalf("ReadFile(sdd-init.md) error = %v", err)
	}

	text := string(content)
	if !strings.Contains(text, "description") {
		t.Fatal("sdd-init.md missing frontmatter description — not real content")
	}

	settingsPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	settingsContent, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile(opencode.json) error = %v", err)
	}

	settingsText := string(settingsContent)
	if !strings.Contains(settingsText, `"agent"`) {
		t.Fatal("opencode.json missing agent key for SDD commands")
	}
	if !strings.Contains(settingsText, `"sdd-orchestrator"`) {
		t.Fatal("opencode.json missing sdd-orchestrator agent")
	}

	sharedPath := filepath.Join(home, ".config", "opencode", "skills", "_shared", "persistence-contract.md")
	if _, err := os.Stat(sharedPath); err != nil {
		t.Fatalf("expected shared SDD convention file %q: %v", sharedPath, err)
	}

	skillPath := filepath.Join(home, ".config", "opencode", "skills", "sdd-init", "SKILL.md")
	skillContent, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("ReadFile(sdd-init SKILL.md) error = %v", err)
	}

	if !strings.Contains(string(skillContent), "sdd-init") {
		t.Fatal("SDD skill file missing expected content")
	}
}

func TestInjectOpenCodeIsIdempotent(t *testing.T) {
	home := t.TempDir()

	first, err := Inject(home, opencodeAdapter())
	if err != nil {
		t.Fatalf("Inject() first error = %v", err)
	}
	if !first.Changed {
		t.Fatalf("Inject() first changed = false")
	}

	second, err := Inject(home, opencodeAdapter())
	if err != nil {
		t.Fatalf("Inject() second error = %v", err)
	}
	if second.Changed {
		t.Fatalf("Inject() second changed = true")
	}
}

func TestInjectOpenCodeMigratesLegacyAgentsKey(t *testing.T) {
	home := t.TempDir()

	settingsPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	legacy := `{
  "agents": {
    "legacy-agent": {
      "mode": "all",
      "prompt": "{file:./AGENTS.md}"
    }
  }
}`
	if err := os.WriteFile(settingsPath, []byte(legacy), 0o644); err != nil {
		t.Fatalf("WriteFile(opencode.json) error = %v", err)
	}

	if _, err := Inject(home, opencodeAdapter()); err != nil {
		t.Fatalf("Inject() error = %v", err)
	}

	content, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("ReadFile(opencode.json) error = %v", err)
	}

	root := map[string]any{}
	if err := json.Unmarshal(content, &root); err != nil {
		t.Fatalf("Unmarshal(opencode.json) error = %v", err)
	}

	if _, hasLegacy := root["agents"]; hasLegacy {
		t.Fatal("opencode.json should not keep legacy agents key after migration")
	}

	agentRaw, ok := root["agent"]
	if !ok {
		t.Fatal("opencode.json missing agent key after migration")
	}

	agentMap, ok := agentRaw.(map[string]any)
	if !ok {
		t.Fatalf("opencode.json agent key has unexpected type: %T", agentRaw)
	}

	if _, ok := agentMap["legacy-agent"]; !ok {
		t.Fatal("legacy agent was not migrated under agent key")
	}
	if _, ok := agentMap["sdd-orchestrator"]; !ok {
		t.Fatal("sdd-orchestrator agent missing after merge")
	}
}

func TestInjectCursorWritesSDDOrchestratorAndSkills(t *testing.T) {
	home := t.TempDir()

	cursorAdapter, err := agents.NewAdapter("cursor")
	if err != nil {
		t.Fatalf("NewAdapter(cursor) error = %v", err)
	}

	result, injectErr := Inject(home, cursorAdapter)
	if injectErr != nil {
		t.Fatalf("Inject(cursor) error = %v", injectErr)
	}

	if !result.Changed {
		t.Fatal("Inject(cursor) changed = false")
	}

	// Should have SDD skill files AND the system prompt file.
	if len(result.Files) == 0 {
		t.Fatal("Inject(cursor) returned no files")
	}

	// Verify SDD orchestrator was injected into the system prompt file.
	promptPath := filepath.Join(home, ".cursor", "rules", "gentle-ai.mdc")
	content, readErr := os.ReadFile(promptPath)
	if readErr != nil {
		t.Fatalf("ReadFile(%q) error = %v", promptPath, readErr)
	}

	text := string(content)
	if !strings.Contains(text, "Spec-Driven Development") {
		t.Fatal("Cursor system prompt missing SDD orchestrator content")
	}
	if !strings.Contains(text, "sub-agent") {
		t.Fatal("Cursor system prompt missing SDD sub-agent references")
	}
}

func TestInjectGeminiWritesSDDOrchestratorAndSkills(t *testing.T) {
	home := t.TempDir()

	geminiAdapter, err := agents.NewAdapter("gemini-cli")
	if err != nil {
		t.Fatalf("NewAdapter(gemini-cli) error = %v", err)
	}

	result, injectErr := Inject(home, geminiAdapter)
	if injectErr != nil {
		t.Fatalf("Inject(gemini) error = %v", injectErr)
	}

	if !result.Changed {
		t.Fatal("Inject(gemini) changed = false")
	}

	// Verify SDD orchestrator was injected into GEMINI.md.
	promptPath := filepath.Join(home, ".gemini", "GEMINI.md")
	content, readErr := os.ReadFile(promptPath)
	if readErr != nil {
		t.Fatalf("ReadFile(%q) error = %v", promptPath, readErr)
	}

	text := string(content)
	if !strings.Contains(text, "Spec-Driven Development") {
		t.Fatal("Gemini system prompt missing SDD orchestrator content")
	}

	// Should also write SDD skill files.
	skillPath := filepath.Join(home, ".gemini", "skills", "sdd-init", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("expected SDD skill file %q: %v", skillPath, err)
	}
}

func TestInjectVSCodeWritesSDDOrchestratorAndSkills(t *testing.T) {
	home := t.TempDir()

	vscodeAdapter, err := agents.NewAdapter("vscode-copilot")
	if err != nil {
		t.Fatalf("NewAdapter(vscode-copilot) error = %v", err)
	}

	result, injectErr := Inject(home, vscodeAdapter)
	if injectErr != nil {
		t.Fatalf("Inject(vscode) error = %v", injectErr)
	}

	if !result.Changed {
		t.Fatal("Inject(vscode) changed = false")
	}

	// Verify SDD orchestrator was injected into the VS Code instructions file.
	promptPath := vscodeAdapter.SystemPromptFile(home)
	content, readErr := os.ReadFile(promptPath)
	if readErr != nil {
		t.Fatalf("ReadFile(%q) error = %v", promptPath, readErr)
	}

	text := string(content)
	if !strings.Contains(text, "Spec-Driven Development") {
		t.Fatal("VS Code system prompt missing SDD orchestrator content")
	}

	// Should also write SDD skill files under ~/.copilot/skills/.
	skillPath := filepath.Join(home, ".copilot", "skills", "sdd-init", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("expected SDD skill file %q: %v", skillPath, err)
	}

	sharedPath := filepath.Join(home, ".copilot", "skills", "_shared", "engram-convention.md")
	if _, err := os.Stat(sharedPath); err != nil {
		t.Fatalf("expected shared SDD convention file %q: %v", sharedPath, err)
	}
}

func TestInjectFileAppendSkipsIfAlreadyPresent(t *testing.T) {
	home := t.TempDir()

	cursorAdapter, err := agents.NewAdapter("cursor")
	if err != nil {
		t.Fatalf("NewAdapter(cursor) error = %v", err)
	}

	// First injection.
	first, firstErr := Inject(home, cursorAdapter)
	if firstErr != nil {
		t.Fatalf("Inject() first error = %v", firstErr)
	}
	if !first.Changed {
		t.Fatal("first Inject() changed = false")
	}

	// Second injection — SDD content is already there, should not duplicate.
	second, secondErr := Inject(home, cursorAdapter)
	if secondErr != nil {
		t.Fatalf("Inject() second error = %v", secondErr)
	}
	if second.Changed {
		t.Fatal("second Inject() changed = true — SDD orchestrator was duplicated")
	}
}

func TestInjectFileAppendSkipsLegacyHeading(t *testing.T) {
	home := t.TempDir()

	cursorAdapter, err := agents.NewAdapter("cursor")
	if err != nil {
		t.Fatalf("NewAdapter(cursor) error = %v", err)
	}

	promptPath := cursorAdapter.SystemPromptFile(home)
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	existing := "# Existing\n\n## Spec-Driven Development (SDD) Orchestrator\nAlready present.\n"
	if err := os.WriteFile(promptPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, injectErr := Inject(home, cursorAdapter)
	if injectErr != nil {
		t.Fatalf("Inject() error = %v", injectErr)
	}
	if len(result.Files) == 0 {
		t.Fatal("Inject() returned no files")
	}

	content, readErr := os.ReadFile(promptPath)
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}

	text := string(content)
	if strings.Count(text, "## Spec-Driven Development (SDD) Orchestrator") != 1 {
		t.Fatal("legacy SDD heading duplicated")
	}
}

func TestInjectFileAppendSkipsAgentTeamsHeading(t *testing.T) {
	home := t.TempDir()

	cursorAdapter, err := agents.NewAdapter("cursor")
	if err != nil {
		t.Fatalf("NewAdapter(cursor) error = %v", err)
	}

	promptPath := cursorAdapter.SystemPromptFile(home)
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	existing := "# Existing\n\n## Agent Teams Orchestrator\nAlready present.\n"
	if err := os.WriteFile(promptPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, injectErr := Inject(home, cursorAdapter)
	if injectErr != nil {
		t.Fatalf("Inject() error = %v", injectErr)
	}
	if len(result.Files) == 0 {
		t.Fatal("Inject() returned no files")
	}

	content, readErr := os.ReadFile(promptPath)
	if readErr != nil {
		t.Fatalf("ReadFile() error = %v", readErr)
	}

	text := string(content)
	if strings.Count(text, "## Agent Teams Orchestrator") != 1 {
		t.Fatal("agent teams heading duplicated")
	}
}
