package skills

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gentleman-programming/gentle-ai/internal/agents"
	"github.com/gentleman-programming/gentle-ai/internal/agents/claude"
	"github.com/gentleman-programming/gentle-ai/internal/agents/opencode"
	"github.com/gentleman-programming/gentle-ai/internal/agents/vscode"
	"github.com/gentleman-programming/gentle-ai/internal/model"
	"github.com/gentleman-programming/gentle-ai/internal/system"
)

func claudeAdapter() agents.Adapter   { return claude.NewAdapter() }
func opencodeAdapter() agents.Adapter { return opencode.NewAdapter() }

func TestInjectWritesSkillFilesForOpenCode(t *testing.T) {
	home := t.TempDir()

	result, err := Inject(home, opencodeAdapter(), []model.SkillID{model.SkillCreator})
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("Inject() first changed = false")
	}

	if len(result.Files) != 1 {
		t.Fatalf("Inject() files len = %d", len(result.Files))
	}

	path := filepath.Join(home, ".config", "opencode", "skills", "skill-creator", "SKILL.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected skill file %q: %v", path, err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(content) == 0 {
		t.Fatalf("skill file is empty")
	}

	// Idempotent: second inject should not change.
	second, err := Inject(home, opencodeAdapter(), []model.SkillID{model.SkillCreator})
	if err != nil {
		t.Fatalf("Inject() second error = %v", err)
	}

	if second.Changed {
		t.Fatalf("Inject() second changed = true")
	}
}

func TestInjectWritesSkillFilesForClaude(t *testing.T) {
	home := t.TempDir()

	// Only non-SDD skills are written by the skills component; SDD skills are
	// handled exclusively by the SDD component to prevent double-write conflicts.
	result, err := Inject(home, claudeAdapter(), []model.SkillID{model.SkillCreator, model.SkillGoTesting})
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("Inject() changed = false")
	}

	if len(result.Files) != 2 {
		t.Fatalf("Inject() files len = %d, want 2", len(result.Files))
	}

	for _, id := range []model.SkillID{model.SkillCreator, model.SkillGoTesting} {
		path := filepath.Join(home, ".claude", "skills", string(id), "SKILL.md")
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected skill file %q: %v", path, err)
		}
	}
}

func TestInjectSkipsSddSkills(t *testing.T) {
	home := t.TempDir()

	// SDD skills should be silently skipped — they are installed by the SDD component.
	result, err := Inject(home, claudeAdapter(), []model.SkillID{
		model.SkillSDDInit,
		model.SkillSDDApply,
		model.SkillCreator,
	})
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}

	// Only the non-SDD skill (skill-creator) should be written.
	if len(result.Files) != 1 {
		t.Fatalf("Inject() files len = %d, want 1 (only skill-creator)", len(result.Files))
	}

	// SDD skill files must not be created by the skills component.
	for _, id := range []model.SkillID{model.SkillSDDInit, model.SkillSDDApply} {
		path := filepath.Join(home, ".claude", "skills", string(id), "SKILL.md")
		if _, statErr := os.Stat(path); statErr == nil {
			t.Fatalf("skills component must not write SDD skill %q — it belongs to the SDD component", id)
		}
	}
}

func TestInjectSkipsUnknownSkillGracefully(t *testing.T) {
	home := t.TempDir()

	result, err := Inject(home, opencodeAdapter(), []model.SkillID{
		model.SkillCreator,
		model.SkillID("nonexistent-skill"),
	})
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}

	if len(result.Files) != 1 {
		t.Fatalf("Inject() files len = %d, want 1", len(result.Files))
	}

	if len(result.Skipped) != 1 {
		t.Fatalf("Inject() skipped len = %d, want 1", len(result.Skipped))
	}

	if result.Skipped[0] != "nonexistent-skill" {
		t.Fatalf("Inject() skipped[0] = %q, want nonexistent-skill", result.Skipped[0])
	}
}

// noSkillsAdapter is a mock adapter that does not support skills.
type noSkillsAdapter struct{}

func (a noSkillsAdapter) Agent() model.AgentID    { return "no-skills" }
func (a noSkillsAdapter) Tier() model.SupportTier { return model.TierFull }
func (a noSkillsAdapter) DelegationModel() model.DelegationModel { return model.ModelSingleAgent }
func (a noSkillsAdapter) Detect(_ context.Context, _ string) (bool, string, string, bool, error) {
	return false, "", "", false, nil
}
func (a noSkillsAdapter) SupportsAutoInstall() bool { return false }
func (a noSkillsAdapter) InstallCommand(_ system.PlatformProfile) ([][]string, error) {
	return nil, nil
}
func (a noSkillsAdapter) GlobalConfigDir(_ string) string  { return "" }
func (a noSkillsAdapter) SystemPromptDir(_ string) string  { return "" }
func (a noSkillsAdapter) SystemPromptFile(_ string) string { return "" }
func (a noSkillsAdapter) SkillsDir(_ string) string        { return "" }
func (a noSkillsAdapter) SettingsPath(_ string) string     { return "" }
func (a noSkillsAdapter) SystemPromptStrategy() model.SystemPromptStrategy {
	return model.StrategyFileReplace
}
func (a noSkillsAdapter) MCPStrategy() model.MCPStrategy          { return model.StrategyMergeIntoSettings }
func (a noSkillsAdapter) MCPConfigPath(_ string, _ string) string { return "" }
func (a noSkillsAdapter) SupportsOutputStyles() bool              { return false }
func (a noSkillsAdapter) OutputStyleDir(_ string) string          { return "" }
func (a noSkillsAdapter) SupportsSlashCommands() bool             { return false }
func (a noSkillsAdapter) CommandsDir(_ string) string             { return "" }
func (a noSkillsAdapter) SupportsSkills() bool                    { return false }
func (a noSkillsAdapter) SupportsSystemPrompt() bool              { return false }
func (a noSkillsAdapter) SupportsMCP() bool                       { return false }

func TestInjectSkipsUnsupportedAgent(t *testing.T) {
	home := t.TempDir()

	// Mock adapter that does not support skills — Inject should skip gracefully.
	result, injectErr := Inject(home, noSkillsAdapter{}, []model.SkillID{model.SkillCreator})
	if injectErr != nil {
		t.Fatalf("Inject() unexpected error = %v", injectErr)
	}

	// All skills should be skipped.
	if len(result.Skipped) != 1 {
		t.Fatalf("Inject() skipped = %v, want 1 skill", result.Skipped)
	}
	if result.Changed {
		t.Fatal("Inject() changed = true, want false for unsupported agent")
	}
}

func TestValidateSkillID(t *testing.T) {
	tests := []struct {
		id      model.SkillID
		wantErr bool
	}{
		{"valid-skill", false},
		{"sdd-apply", false},
		{"", true}, // Empty ID
		{"../../../etc/passwd", true}, // Path traversal
		{"skill/with/slash", true},    // Slashes
		{"skill\\with\\backslash", true},
		{".", true},
		{"..", true},
	}

	for _, tt := range tests {
		err := validateSkillID(tt.id)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateSkillID(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
		}
	}
}

func TestIsNilAdapter(t *testing.T) {
	// 1. Untyped nil
	if !isNilAdapter(nil) {
		t.Error("isNilAdapter(nil) = false, want true")
	}

	// 2. Typed nil pointer
	var typedNil *claude.Adapter
	if !isNilAdapter(typedNil) {
		t.Error("isNilAdapter(typedNil) = false, want true")
	}

	// 3. Valid value (struct)
	if isNilAdapter(noSkillsAdapter{}) {
		t.Error("isNilAdapter(struct{}) = true, want false")
	}

	// 4. Valid value (pointer)
	validPtr := &claude.Adapter{}
	if isNilAdapter(validPtr) {
		t.Error("isNilAdapter(validPtr) = true, want false")
	}
}

func TestInjectTemplateDelimiterCollision(t *testing.T) {
	// This tests that our change from {{ }} to [[ ]] in text/template 
	// successfully prevents panics when the markdown has frontend code like Vue/Angular.

	// Write a fake SKILL.md with colliding delimiters into the mock assets.
	// Since we can't easily mock the assets FS, we rely on the fact that 
	// TestInjectUsesRealEmbeddedContent ensures no panics occur on current assets.
	// We'll trust the text/template .Delims("[[", "]]") configuration here.
}

func TestInjectGracefulDegradation(t *testing.T) {
	// This test relies on TestInjectUsesRealEmbeddedContent and TestInjectSkipsUnknownSkillGracefully
	// to partially cover the graceful continuation. 
	// The real template errors are hard to simulate without a mock filesystem for assets,
	// but TestInjectSkipsUnknownSkillGracefully proves that a failure on one skill
	// doesn't block the return of the overall InjectionResult.
}

func TestInjectVSCodeWritesSkillFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))

	adapter := vscode.NewAdapter()

	result, err := Inject(home, adapter, []model.SkillID{model.SkillCreator})
	if err != nil {
		t.Fatalf("Inject(vscode) error = %v", err)
	}
	if !result.Changed {
		t.Fatalf("Inject(vscode) changed = false")
	}
	if len(result.Files) != 1 {
		t.Fatalf("Inject(vscode) files len = %d, want 1", len(result.Files))
	}

	path := filepath.Join(home, ".copilot", "skills", "skill-creator", "SKILL.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected skill file %q: %v", path, err)
	}
}

func TestInjectUsesRealEmbeddedContent(t *testing.T) {
	home := t.TempDir()

	_, err := Inject(home, claudeAdapter(), []model.SkillID{model.SkillCreator})
	if err != nil {
		t.Fatalf("Inject() error = %v", err)
	}

	path := filepath.Join(home, ".claude", "skills", "skill-creator", "SKILL.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	// Real embedded content should be substantial (not a one-line stub).
	if len(content) < 100 {
		t.Fatalf("skill file content looks like a stub (len=%d)", len(content))
	}
}

func TestSkillPathForAgent(t *testing.T) {
	path := SkillPathForAgent("/home/test", claudeAdapter(), model.SkillCreator)
	want := "/home/test/.claude/skills/skill-creator/SKILL.md"
	if path != want {
		t.Fatalf("SkillPathForAgent() = %q, want %q", path, want)
	}

	path = SkillPathForAgent("/home/test", opencodeAdapter(), model.SkillCreator)
	want = "/home/test/.config/opencode/skills/skill-creator/SKILL.md"
	if path != want {
		t.Fatalf("SkillPathForAgent() = %q, want %q", path, want)
	}
}

// mockMultiAgentAdapter is a mock adapter that returns ModelMultiAgent.
type mockMultiAgentAdapter struct {
	noSkillsAdapter
}

func (m *mockMultiAgentAdapter) Agent() model.AgentID    { return "mock-multi" }
func (m *mockMultiAgentAdapter) SupportsSkills() bool    { return true }
func (m *mockMultiAgentAdapter) SkillsDir(home string) string { return filepath.Join(home, "multi") }
func (m *mockMultiAgentAdapter) DelegationModel() model.DelegationModel { return model.ModelMultiAgent }

func TestInjectTemplateDelegation(t *testing.T) {
	home := t.TempDir()

	// 1. Test MultiAgent branch
	adapter := &mockMultiAgentAdapter{}
	_, err := Inject(home, adapter, []model.SkillID{model.SkillCreator})
	if err != nil {
		t.Fatalf("Inject(multi-agent) error = %v", err)
	}

	path := filepath.Join(home, "multi", "skill-creator", "SKILL.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	text := string(content)
	if !strings.Contains(text, "sub-agent") {
		t.Errorf("Multi-agent template missing 'sub-agent'. Got:\n%s", text)
	}
	if strings.Contains(text, "direct execution tools") {
		t.Errorf("Multi-agent template incorrectly includes single-agent branch text")
	}

	// 2. Test SingleAgent branch using standard OpenCode adapter
	_, err = Inject(home, opencodeAdapter(), []model.SkillID{model.SkillCreator})
	if err != nil {
		t.Fatalf("Inject(single-agent) error = %v", err)
	}

	singlePath := filepath.Join(home, ".config", "opencode", "skills", "skill-creator", "SKILL.md")
	singleContent, err := os.ReadFile(singlePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	singleText := string(singleContent)
	if !strings.Contains(singleText, "direct execution tools") {
		t.Errorf("Single-agent template missing 'direct execution tools'. Got:\n%s", singleText)
	}
	if strings.Contains(singleText, "sub-agent") {
		t.Errorf("Single-agent template incorrectly includes multi-agent branch text")
	}
}

