package skills

import (
	"bytes"
	"fmt"
	"log"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"

	"github.com/gentleman-programming/gentle-ai/internal/agents"
	"github.com/gentleman-programming/gentle-ai/internal/assets"
	"github.com/gentleman-programming/gentle-ai/internal/components/filemerge"
	"github.com/gentleman-programming/gentle-ai/internal/model"
)

// isSDDSkill reports whether a skill ID belongs to the SDD orchestrator suite.
// SDD skills are installed by the SDD component; the skills component skips
// them to prevent duplicate writes when both components are selected.
func isSDDSkill(id model.SkillID) bool {
	return strings.HasPrefix(string(id), "sdd-")
}

// validateSkillID prevents path traversal attacks by ensuring the ID is safe.
func validateSkillID(id model.SkillID) error {
	strID := string(id)
	if strID == "" {
		return fmt.Errorf("skill ID cannot be empty")
	}
	if strings.Contains(strID, "/") || strings.Contains(strID, "\\") {
		return fmt.Errorf("skill ID contains invalid characters (slashes)")
	}
	if strID == "." || strID == ".." {
		return fmt.Errorf("skill ID cannot be '.' or '..'")
	}
	if strID != filepath.Clean(strID) {
		return fmt.Errorf("skill ID is not clean")
	}
	return nil
}

type InjectionResult struct {
	Changed bool
	Files   []string
	Skipped []model.SkillID
}

// SkillTemplateData provides context to the skill Markdown templates.
type SkillTemplateData struct {
	DelegationModel model.DelegationModel
}

// Inject writes the embedded SKILL.md files for each requested skill
// to the correct directory for the given agent adapter.
//
// The skills directory is determined by adapter.SkillsDir(), removing
// the need for any agent-specific switch statements.
//
// SDD skills (those whose IDs begin with "sdd-") are intentionally skipped
// here because the SDD component installs them as part of its own injection.
// This prevents a write conflict when both components are selected together.
//
// Individual skill failures (e.g., missing embedded asset) are logged
// and skipped rather than aborting the entire operation.

func isNilAdapter(adapter agents.Adapter) bool {
	if adapter == nil {
		return true
	}
	v := reflect.ValueOf(adapter)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return v.IsNil()
	}
	return false
}

func Inject(homeDir string, adapter agents.Adapter, skillIDs []model.SkillID) (InjectionResult, error) {
	if isNilAdapter(adapter) {
		return InjectionResult{}, fmt.Errorf("adapter is nil")
	}

	if !adapter.SupportsSkills() {
		return InjectionResult{Skipped: skillIDs}, nil
	}

	skillDir := adapter.SkillsDir(homeDir)
	if skillDir == "" {
		return InjectionResult{Skipped: skillIDs}, nil
	}

	paths := make([]string, 0, len(skillIDs))
	skipped := make([]model.SkillID, 0)
	changed := false

	templateData := SkillTemplateData{
		DelegationModel: adapter.DelegationModel(),
	}

	for _, id := range skillIDs {
		// SDD skills are written by the SDD component — skip to avoid conflicts.
		if isSDDSkill(id) {
			continue
		}

		if err := validateSkillID(id); err != nil {
			log.Printf("skills: skipping %q — invalid ID: %v", id, err)
			skipped = append(skipped, id)
			continue
		}

		assetPath := "skills/" + string(id) + "/SKILL.md"
		content, readErr := assets.Read(assetPath)
		if readErr != nil {
			log.Printf("skills: skipping %q — embedded asset not found: %v", id, readErr)
			skipped = append(skipped, id)
			continue
		}
		if len(content) == 0 {
			return InjectionResult{}, fmt.Errorf("skill %q: embedded asset exists but is empty — build may be corrupt", id)
		}

		tmpl, err := template.New(string(id)).Delims("[[", "]]").Parse(string(content))
		if err != nil {
			log.Printf("skills: skipping %q — failed to parse template: %v", id, err)
			skipped = append(skipped, id)
			continue
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, templateData); err != nil {
			log.Printf("skills: skipping %q — failed to render template: %v", id, err)
			skipped = append(skipped, id)
			continue
		}

		path := filepath.Join(skillDir, string(id), "SKILL.md")
		writeResult, writeErr := filemerge.WriteFileAtomic(path, buf.Bytes(), 0o644)
		if writeErr != nil {
			return InjectionResult{}, fmt.Errorf("skill %q: write failed: %w", id, writeErr)
		}

		changed = changed || writeResult.Changed
		paths = append(paths, path)
	}

	return InjectionResult{Changed: changed, Files: paths, Skipped: skipped}, nil
}

// SkillPathForAgent returns the filesystem path where a skill file would be written.
func SkillPathForAgent(homeDir string, adapter agents.Adapter, id model.SkillID) string {
	if isNilAdapter(adapter) {
		return ""
	}
	if err := validateSkillID(id); err != nil {
		return ""
	}
	skillDir := adapter.SkillsDir(homeDir)
	if skillDir == "" {
		return ""
	}
	return filepath.Join(skillDir, string(id), "SKILL.md")
}
