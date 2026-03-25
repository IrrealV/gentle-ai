package sdd

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gentleman-programming/gentle-ai/internal/agents"
	"github.com/gentleman-programming/gentle-ai/internal/assets"
	"github.com/gentleman-programming/gentle-ai/internal/components/filemerge"
	"github.com/gentleman-programming/gentle-ai/internal/model"
)

type InjectionResult struct {
	Changed bool
	Files   []string
}

type InjectOptions struct {
	OpenCodeModelAssignments map[string]model.ModelAssignment
	ClaudeModelAssignments   map[string]model.ClaudeModelAlias

	// WorkspaceDir is the root of the current workspace (e.g. os.Getwd()).
	// When non-empty and the adapter implements workflowInjector, native
	// workflow files are copied to <workspaceDir>/.windsurf/workflows/.
	WorkspaceDir string
}

// workflowInjector is an optional adapter capability: if an adapter
// implements this interface, sdd.Inject will copy the embedded workflow
// assets into the workspace directory provided via InjectOptions.WorkspaceDir.
// This intentionally does NOT extend agents.Adapter to avoid requiring all
// adapters to implement no-op stubs.
type workflowInjector interface {
	SupportsWorkflows() bool
	// WorkflowsDir returns the target filesystem directory where workflow files
	// should be written (e.g. <workspaceDir>/.windsurf/workflows/).
	WorkflowsDir(workspaceDir string) string
	// EmbeddedWorkflowsDir returns the path inside the embedded assets FS where
	// this adapter's workflow sources live (e.g. "windsurf/workflows").
	// This removes the hardcoded agent name from the injection step, making
	// the workflowInjector pattern reusable for future agents.
	EmbeddedWorkflowsDir() string
}

// projectMarkers are filenames/dirs whose presence signals a valid project root.
// We verify at least one exists before writing workspace-scoped files (like
// native Windsurf workflows) to prevent accidentally polluting the user's
// home directory when gentle-ai is invoked from ~.
var projectMarkers = []string{
	".git",
	"go.mod",
	"package.json",
	"Cargo.toml",
	"pyproject.toml",
	"pom.xml",
	"build.gradle",
}

// maxAncestorDepth is the maximum number of parent directories findProjectRoot
// will traverse before giving up. This prevents infinite loops on deeply-nested
// trees and ensures we stop well before reaching the filesystem root.
const maxAncestorDepth = 10

// findProjectRoot walks upward from dir, checking each level for a known
// project marker (.git, go.mod, package.json, …). It returns the first
// matching directory and true. If no root is found within maxAncestorDepth
// levels, or dir is empty, it returns ("", false).
//
// Walking upward means users can run gentle-ai from any subdirectory of their
// project (e.g. repo/internal/foo) and still get workflow files written to
// the correct workspace root.
func findProjectRoot(dir string) (string, bool) {
	if dir == "" {
		return "", false
	}
	current := filepath.Clean(dir)
	for i := 0; i < maxAncestorDepth; i++ {
		for _, marker := range projectMarkers {
			if _, err := os.Stat(filepath.Join(current, marker)); err == nil {
				return current, true
			}
		}
		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root ("/" on Unix, "C:\" on Windows).
			return "", false
		}
		current = parent
	}
	return "", false
}

var (
	npmLookPath = exec.LookPath
	npmRun      = func(dir string, args ...string) ([]byte, error) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		// CombinedOutput captures stdout+stderr so we can surface actionable
		// error messages on failure. Do not set Stdout/Stderr separately.
		return cmd.CombinedOutput()
	}
)

// overlayAssetPath returns the embedded asset path for the SDD agent overlay
// based on the selected SDD mode. Empty or SDDModeSingle uses the single
// orchestrator overlay; SDDModeMulti uses the multi-agent overlay.
func overlayAssetPath(sddMode model.SDDModeID) string {
	if sddMode == model.SDDModeMulti {
		return "opencode/sdd-overlay-multi.json"
	}
	return "opencode/sdd-overlay-single.json"
}

func Inject(homeDir string, adapter agents.Adapter, sddMode model.SDDModeID, options ...InjectOptions) (InjectionResult, error) {
	if !adapter.SupportsSystemPrompt() {
		return InjectionResult{}, nil
	}

	var opts InjectOptions
	if len(options) > 0 {
		opts = options[0]
	}

	files := make([]string, 0)
	changed := false

	// 1. Inject SDD orchestrator into the global system prompt for agents that
	// rely on prompt files. OpenCode is handled differently: its orchestrator
	// instructions must be scoped to the sdd-orchestrator agent only, otherwise
	// the SDD phase sub-agents inherit coordinator-only delegation rules.
	if adapter.Agent() != model.AgentOpenCode {
		switch adapter.SystemPromptStrategy() {
		case model.StrategyMarkdownSections:
			result, err := injectMarkdownSections(homeDir, adapter, opts.ClaudeModelAssignments)
			if err != nil {
				return InjectionResult{}, err
			}
			changed = changed || result.Changed
			files = append(files, result.Files...)

		case model.StrategyFileReplace, model.StrategyAppendToFile, model.StrategyInstructionsFile:
			// For FileReplace/AppendToFile agents, the SDD orchestrator is included
			// in the generic persona asset. However, if the user chose neutral or
			// custom persona, the SDD content must still be injected. We append the
			// SDD orchestrator section to the existing system prompt file so it is
			// always present regardless of persona choice.
			result, err := injectFileAppend(homeDir, adapter)
			if err != nil {
				return InjectionResult{}, err
			}
			changed = changed || result.Changed
			files = append(files, result.Files...)
		}
	}

	// 2. Write slash commands (if the agent supports them).
	if adapter.SupportsSlashCommands() {
		commandsDir := adapter.CommandsDir(homeDir)
		if commandsDir != "" {
			commandEntries, err := fs.ReadDir(assets.FS, "opencode/commands")
			if err != nil {
				return InjectionResult{}, fmt.Errorf("read embedded opencode/commands: %w", err)
			}

			for _, entry := range commandEntries {
				if entry.IsDir() {
					continue
				}

				content := assets.MustRead("opencode/commands/" + entry.Name())
				path := filepath.Join(commandsDir, entry.Name())
				writeResult, err := filemerge.WriteFileAtomic(path, []byte(content), 0o644)
				if err != nil {
					return InjectionResult{}, err
				}

				changed = changed || writeResult.Changed
				files = append(files, path)
			}
		}
	}

	// 2b. OpenCode /sdd-* commands reference agent: sdd-orchestrator.
	// Ensure that agent is present even when persona component is not installed.
	//
	// mergedSettingsBytes holds the final merged opencode.json bytes produced by
	// mergeJSONFile. We keep them in memory so the post-check (step 4) can validate
	// the merge result without re-reading from disk — on Windows/WSL2, the atomic
	// rename (temp → target) may not be immediately visible to a subsequent
	// os.ReadFile call due to VFS/NTFS metadata caching, which caused the spurious
	// "post-check: .../opencode.json missing sdd-apply sub-agent" error.
	var mergedSettingsBytes []byte
	if adapter.Agent() == model.AgentOpenCode {
		settingsPath := adapter.SettingsPath(homeDir)
		if settingsPath != "" {
			overlayContent, err := assets.Read(overlayAssetPath(sddMode))
			if err != nil {
				return InjectionResult{}, fmt.Errorf("read SDD overlay asset: %w", err)
			}

			// Inject model assignments into the overlay before merging.
			overlayBytes := []byte(overlayContent)
			overlayBytes, err = inlineOpenCodeSDDPrompts(overlayBytes)
			if err != nil {
				return InjectionResult{}, fmt.Errorf("inline OpenCode SDD prompts: %w", err)
			}
			assignments := opts.OpenCodeModelAssignments
			if sddMode != model.SDDModeMulti {
				assignments = nil
			}

			rootModelID, err := readOpenCodeRootModel(settingsPath)
			if err != nil {
				return InjectionResult{}, err
			}
			if rootModelID != "" || len(assignments) > 0 {
				overlayBytes, err = injectModelAssignments(overlayBytes, assignments, rootModelID)
				if err != nil {
					return InjectionResult{}, fmt.Errorf("inject model assignments: %w", err)
				}
			}

			agentResult, err := mergeJSONFile(settingsPath, overlayBytes)
			if err != nil {
				return InjectionResult{}, err
			}
			changed = changed || agentResult.writeResult.Changed
			files = append(files, settingsPath)
			mergedSettingsBytes = agentResult.merged

			// Install OpenCode plugins (all SDD modes).
			pluginResult, err := installOpenCodePlugins(homeDir)
			if err != nil {
				return InjectionResult{}, err
			}
			changed = changed || pluginResult.Changed
			files = append(files, pluginResult.Files...)
		}
	}

	// 3. Write SDD skill files (if the agent supports skills).
	if adapter.SupportsSkills() {
		skillDir := adapter.SkillsDir(homeDir)
		if skillDir != "" {
			sharedFiles := []string{
				"persistence-contract.md",
				"engram-convention.md",
				"openspec-convention.md",
				"sdd-phase-common.md",
			}

			for _, fileName := range sharedFiles {
				assetPath := "skills/_shared/" + fileName
				content, readErr := assets.Read(assetPath)
				if readErr != nil {
					return InjectionResult{}, fmt.Errorf("required SDD shared file %q: embedded asset not found: %w", fileName, readErr)
				}
				if len(content) == 0 {
					return InjectionResult{}, fmt.Errorf("required SDD shared file %q: embedded asset is empty", fileName)
				}

				path := filepath.Join(skillDir, "_shared", fileName)
				writeResult, err := filemerge.WriteFileAtomic(path, []byte(content), 0o644)
				if err != nil {
					return InjectionResult{}, err
				}

				changed = changed || writeResult.Changed
				files = append(files, path)
			}

			sddSkills := []string{
				"sdd-init", "sdd-explore", "sdd-propose", "sdd-spec",
				"sdd-design", "sdd-tasks", "sdd-apply", "sdd-verify", "sdd-archive",
			}

			for _, skill := range sddSkills {
				assetPath := "skills/" + skill + "/SKILL.md"
				content, readErr := assets.Read(assetPath)
				if readErr != nil {
					return InjectionResult{}, fmt.Errorf("required SDD skill %q: embedded asset not found: %w", skill, readErr)
				}
				if len(content) == 0 {
					return InjectionResult{}, fmt.Errorf("required SDD skill %q: embedded asset is empty", skill)
				}

				path := filepath.Join(skillDir, skill, "SKILL.md")
				writeResult, err := filemerge.WriteFileAtomic(path, []byte(content), 0o644)
				if err != nil {
					return InjectionResult{}, err
				}

				changed = changed || writeResult.Changed
				files = append(files, path)
			}
		}
	}

	// 3b. Write native workflow files (Windsurf Hybrid-First, and any future
	// agent that implements the workflowInjector optional interface).
	// findProjectRoot walks upward from WorkspaceDir so gentle-ai can be
	// invoked from any subdirectory (e.g. repo/internal/foo) and still inject
	// workflows at the real project root. Skips silently if no root is found
	// (e.g. running from home dir without a project).
	if wi, ok := adapter.(workflowInjector); ok && wi.SupportsWorkflows() {
		if projectRoot, found := findProjectRoot(opts.WorkspaceDir); found {
			workflowsDir := wi.WorkflowsDir(projectRoot)
			embedDir := wi.EmbeddedWorkflowsDir()
			entries, readErr := fs.ReadDir(assets.FS, embedDir)
			if readErr != nil {
				return InjectionResult{}, fmt.Errorf("read embedded %s: %w", embedDir, readErr)
			}

			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				content, readErr := assets.Read(embedDir + "/" + entry.Name())
				if readErr != nil {
					return InjectionResult{}, fmt.Errorf("read embedded workflow %q: %w", entry.Name(), readErr)
				}
				path := filepath.Join(workflowsDir, entry.Name())
				writeResult, err := filemerge.WriteFileAtomic(path, []byte(content), 0o644)
				if err != nil {
					return InjectionResult{}, fmt.Errorf("write workflow %q: %w", path, err)
				}
				changed = changed || writeResult.Changed
				files = append(files, path)
			}
		}
	}

	// 4. Post-injection verification — catch silent failures.
	// Validate against the in-memory merged bytes rather than re-reading from
	// disk to avoid false negatives on Windows/WSL2 where a freshly-renamed
	// file may not be immediately readable via os.ReadFile.
	if adapter.Agent() == model.AgentOpenCode && len(mergedSettingsBytes) > 0 {
		settingsText := string(mergedSettingsBytes)
		if !strings.Contains(settingsText, `"sdd-orchestrator"`) {
			settingsPath := adapter.SettingsPath(homeDir)
			return InjectionResult{}, fmt.Errorf("post-check: %q missing sdd-orchestrator agent definition — OpenCode /sdd-* commands will fail", settingsPath)
		}
		if sddMode == model.SDDModeMulti && !strings.Contains(settingsText, `"sdd-apply"`) {
			settingsPath := adapter.SettingsPath(homeDir)
			return InjectionResult{}, fmt.Errorf("post-check: %q missing sdd-apply sub-agent — multi-mode overlay was not injected correctly", settingsPath)
		}
	}

	if adapter.SupportsSkills() {
		skillDir := adapter.SkillsDir(homeDir)
		if skillDir != "" {
			for _, skill := range []string{"sdd-init", "sdd-apply", "sdd-verify"} {
				path := filepath.Join(skillDir, skill, "SKILL.md")
				info, err := os.Stat(path)
				if err != nil {
					return InjectionResult{}, fmt.Errorf("post-check: SDD skill %q not found on disk: %w", skill, err)
				}
				if info.Size() < 100 {
					return InjectionResult{}, fmt.Errorf("post-check: SDD skill %q is too small (%d bytes) — content may be empty or corrupt", skill, info.Size())
				}
			}
		}
	}

	return InjectionResult{Changed: changed, Files: files}, nil
}

func inlineOpenCodeSDDPrompts(overlayBytes []byte) ([]byte, error) {
	var overlay map[string]any
	if err := json.Unmarshal(overlayBytes, &overlay); err != nil {
		return nil, fmt.Errorf("unmarshal OpenCode SDD overlay: %w", err)
	}

	agentsRaw, ok := overlay["agent"]
	if !ok {
		return overlayBytes, nil
	}
	agentsMap, ok := agentsRaw.(map[string]any)
	if !ok {
		return overlayBytes, nil
	}

	orchestratorRaw, ok := agentsMap["sdd-orchestrator"]
	if !ok {
		return overlayBytes, nil
	}
	orchestratorMap, ok := orchestratorRaw.(map[string]any)
	if !ok {
		return overlayBytes, nil
	}

	orchestratorMap["prompt"] = assets.MustRead("generic/sdd-orchestrator.md")

	result, err := json.MarshalIndent(overlay, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal OpenCode SDD overlay: %w", err)
	}

	return append(result, '\n'), nil
}

// installOpenCodePlugins copies the background-agents plugin and installs its
// npm/bun dependency into ~/.config/opencode/. Returns an error with an
// actionable message if the package manager is present but the install fails.
// If no package manager is available, the install is skipped (soft failure).
func installOpenCodePlugins(homeDir string) (InjectionResult, error) {
	opencodeDir := filepath.Join(homeDir, ".config", "opencode")
	pluginsDir := filepath.Join(opencodeDir, "plugins")

	if err := os.MkdirAll(pluginsDir, 0o755); err != nil {
		return InjectionResult{}, fmt.Errorf("create plugins dir: %w", err)
	}

	content := assets.MustRead("opencode/plugins/background-agents.ts")
	pluginPath := filepath.Join(pluginsDir, "background-agents.ts")

	writeResult, err := filemerge.WriteFileAtomic(pluginPath, []byte(content), 0o644)
	if err != nil {
		return InjectionResult{}, fmt.Errorf("write plugin: %w", err)
	}

	files := []string{pluginPath}
	changed := writeResult.Changed

	// Install dependency — prefer bun (OpenCode uses it), fall back to npm.
	// If neither is available, skip with a soft no-op (npm/bun not installed).
	// If a package manager IS found and the install fails, surface the error.
	depPkg := "unique-names-generator"
	nmPath := filepath.Join(opencodeDir, "node_modules", depPkg)

	// Only run the install if the package is not already present.
	pkgMissing := false
	pkgMgrRan := false
	if _, statErr := os.Stat(nmPath); os.IsNotExist(statErr) {
		pkgMissing = true
		var installErr error
		pkgMgrRan, installErr = runPkgInstall(opencodeDir, depPkg)
		if installErr != nil {
			return InjectionResult{}, installErr
		}
	}

	// Post-install validation: if a package manager ran and claimed success,
	// confirm the package actually landed on disk.
	if pkgMissing && pkgMgrRan {
		if _, statErr := os.Stat(nmPath); os.IsNotExist(statErr) {
			// Package manager reported success but the package still isn't there.
			// This is unusual (e.g. bun wrote to a different location). Surface it.
			return InjectionResult{}, fmt.Errorf(
				"post-install check: %q was not found after install in %q — "+
					"the background-agents plugin will fail to load.\n"+
					"Fix: run `cd %s && bun add %s` (or npm install %s) manually",
				depPkg, nmPath, opencodeDir, depPkg, depPkg,
			)
		}
	}

	return InjectionResult{Changed: changed, Files: files}, nil
}

// runPkgInstall installs a node package in the given directory using bun (if
// available) or npm. Returns (true, nil) on success, (false, nil) if no
// package manager is found (soft skip), or (true, error) with a descriptive,
// actionable message if a package manager was found but the install failed.
func runPkgInstall(dir, pkg string) (ran bool, err error) {
	// Prefer bun — OpenCode ships with bun.lock and recommends bun.
	if bunPath, lookErr := npmLookPath("bun"); lookErr == nil {
		out, runErr := npmRun(dir, bunPath, "add", pkg)
		if runErr != nil {
			return true, fmt.Errorf(
				"bun add %s failed in %s: %w\nOutput: %s\nFix: run `cd %s && bun add %s` manually",
				pkg, dir, runErr, strings.TrimSpace(string(out)), dir, pkg,
			)
		}
		return true, nil
	}

	// Fall back to npm.
	if npmPath, lookErr := npmLookPath("npm"); lookErr == nil {
		out, runErr := npmRun(dir, npmPath, "install", "--save", pkg)
		if runErr != nil {
			return true, fmt.Errorf(
				"npm install %s failed in %s: %w\nOutput: %s\nFix: run `cd %s && npm install %s` manually",
				pkg, dir, runErr, strings.TrimSpace(string(out)), dir, pkg,
			)
		}
		return true, nil
	}

	// No package manager available — soft skip.
	return false, nil
}

type mergeJSONResult struct {
	writeResult filemerge.WriteResult
	// merged holds the final JSON bytes that were written to disk.
	// Callers should validate against this in-memory copy instead of
	// re-reading the file from disk — on Windows/WSL2, the atomic rename
	// (temp → target) may not be immediately visible to a subsequent
	// os.ReadFile call due to VFS/NTFS metadata caching.
	merged []byte
}

func mergeJSONFile(path string, overlay []byte) (mergeJSONResult, error) {
	baseJSON, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return mergeJSONResult{}, fmt.Errorf("read json file %q: %w", path, err)
		}
		baseJSON = nil
	}

	baseJSON, err = migrateLegacyOpenCodeAgentsKey(baseJSON)
	if err != nil {
		return mergeJSONResult{}, fmt.Errorf("migrate opencode agents key: %w", err)
	}

	merged, err := filemerge.MergeJSONObjects(baseJSON, overlay)
	if err != nil {
		return mergeJSONResult{}, err
	}

	writeResult, err := filemerge.WriteFileAtomic(path, merged, 0o644)
	if err != nil {
		return mergeJSONResult{}, err
	}

	return mergeJSONResult{writeResult: writeResult, merged: merged}, nil
}

// migrateLegacyOpenCodeAgentsKey normalizes old OpenCode schema that used
// "agents" to the current "agent" key. It keeps existing agent entries and
// merges legacy ones without overriding current definitions.
func migrateLegacyOpenCodeAgentsKey(baseJSON []byte) ([]byte, error) {
	if len(strings.TrimSpace(string(baseJSON))) == 0 {
		return baseJSON, nil
	}

	root := map[string]any{}
	if err := json.Unmarshal(baseJSON, &root); err != nil {
		// Preserve prior behavior for non-JSON/non-parseable inputs.
		return baseJSON, nil
	}

	legacyRaw, hasLegacy := root["agents"]
	if !hasLegacy {
		return baseJSON, nil
	}

	legacy, ok := legacyRaw.(map[string]any)
	if !ok {
		delete(root, "agents")
		encoded, err := json.MarshalIndent(root, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(encoded, '\n'), nil
	}

	current := map[string]any{}
	if currentRaw, hasCurrent := root["agent"]; hasCurrent {
		if parsedCurrent, ok := currentRaw.(map[string]any); ok {
			current = parsedCurrent
		}
	}

	for key, value := range legacy {
		if _, exists := current[key]; !exists {
			current[key] = value
		}
	}

	root["agent"] = current
	delete(root, "agents")

	encoded, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, err
	}

	return append(encoded, '\n'), nil
}

// sddOrchestratorMarkers are used to detect if SDD content was already injected
// (e.g., via a persona file or a previous SDD injection). Keep legacy and
// current headings to remain backward compatible across upstream syncs.
var sddOrchestratorMarkers = []string{
	"## Agent Teams Orchestrator",
	"## Spec-Driven Development (SDD) Orchestrator",
	"## Spec-Driven Development (SDD)",
	"# SDD Orchestrator for Cascade",
}

func hasSDDOrchestrator(content string) bool {
	for _, marker := range sddOrchestratorMarkers {
		if strings.Contains(content, marker) {
			return true
		}
	}
	return false
}

// sddOrchestratorAsset returns the embedded asset path for the SDD orchestrator
// content based on the agent. Agent-specific assets take priority; generic is fallback.
func sddOrchestratorAsset(agent model.AgentID) string {
	switch agent {
	case model.AgentGeminiCLI:
		return "gemini/sdd-orchestrator.md"
	case model.AgentCodex:
		return "codex/sdd-orchestrator.md"
	case model.AgentWindsurf:
		return "windsurf/sdd-orchestrator.md"
	default:
		return "generic/sdd-orchestrator.md"
	}
}

func injectFileAppend(homeDir string, adapter agents.Adapter) (InjectionResult, error) {
	promptPath := adapter.SystemPromptFile(homeDir)

	existing, err := readFileOrEmpty(promptPath)
	if err != nil {
		return InjectionResult{}, err
	}

	// If the SDD orchestrator section is already present (e.g., from the
	// gentleman persona asset which includes it), skip to avoid duplication.
	if hasSDDOrchestrator(existing) {
		return InjectionResult{Files: []string{promptPath}}, nil
	}

	if adapter.SystemPromptStrategy() == model.StrategyInstructionsFile && strings.TrimSpace(existing) == "" {
		existing = instructionsFrontmatter
	}

	// Use agent-specific SDD orchestrator content when available; fall back to generic.
	content := assets.MustRead(sddOrchestratorAsset(adapter.Agent()))

	updated := existing
	if len(updated) > 0 && !strings.HasSuffix(updated, "\n") {
		updated += "\n"
	}
	if len(updated) > 0 {
		updated += "\n"
	}
	updated += content

	writeResult, err := filemerge.WriteFileAtomic(promptPath, []byte(updated), 0o644)
	if err != nil {
		return InjectionResult{}, err
	}

	return InjectionResult{Changed: writeResult.Changed, Files: []string{promptPath}}, nil
}

const instructionsFrontmatter = "---\n" +
	"name: Gentle AI Persona\n" +
	"description: Gentleman persona with SDD orchestration and Engram protocol\n" +
	"applyTo: \"**\"\n" +
	"---\n"

// stripBareOrchestratorSection removes an un-marked "## Agent Teams Orchestrator"
// (or legacy equivalent) block from content. It finds the first matching heading
// and removes everything from that line to the next same-level (##) heading or
// the end of file. This is used to migrate files that contain bare orchestrator
// content (e.g. copied from docs) before injecting the canonical marker-based version.
func stripBareOrchestratorSection(content string) string {
	lines := strings.Split(content, "\n")

	startLine := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		for _, marker := range sddOrchestratorMarkers {
			if trimmed == marker {
				startLine = i
				break
			}
		}
		if startLine >= 0 {
			break
		}
	}

	if startLine < 0 {
		return content
	}

	// Find end: next ## heading (same or higher level) after startLine, or EOF.
	endLine := len(lines)
	for i := startLine + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "## ") {
			endLine = i
			break
		}
	}

	// Rebuild: keep lines before startLine and lines from endLine onward.
	before := lines[:startLine]
	after := lines[endLine:]

	// Trim trailing blank lines from the before section to avoid double newlines.
	for len(before) > 0 && strings.TrimSpace(before[len(before)-1]) == "" {
		before = before[:len(before)-1]
	}

	var parts []string
	if len(before) > 0 {
		parts = append(parts, strings.Join(before, "\n"))
	}
	if len(after) > 0 {
		afterStr := strings.Join(after, "\n")
		// Trim leading blank lines from the after section.
		afterStr = strings.TrimLeft(afterStr, "\n")
		if afterStr != "" {
			parts = append(parts, afterStr)
		}
	}

	result := strings.Join(parts, "\n\n")
	if result != "" && !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	return result
}

func injectMarkdownSections(homeDir string, adapter agents.Adapter, assignments map[string]model.ClaudeModelAlias) (InjectionResult, error) {
	promptPath := adapter.SystemPromptFile(homeDir)
	content := assets.MustRead("claude/sdd-orchestrator.md")
	if len(assignments) > 0 {
		var err error
		content, err = injectClaudeModelAssignments(content, assignments)
		if err != nil {
			return InjectionResult{}, err
		}
	}

	existing, err := readFileOrEmpty(promptPath)
	if err != nil {
		return InjectionResult{}, err
	}

	// If bare (un-marked) orchestrator content exists but the HTML markers are
	// not present, strip the bare block first. This migrates legacy files to the
	// canonical marker-based state without duplicating the section.
	if hasSDDOrchestrator(existing) && !strings.Contains(existing, "<!-- gentle-ai:sdd-orchestrator -->") {
		existing = stripBareOrchestratorSection(existing)
	}

	updated := filemerge.InjectMarkdownSection(existing, "sdd-orchestrator", content)

	writeResult, err := filemerge.WriteFileAtomic(promptPath, []byte(updated), 0o644)
	if err != nil {
		return InjectionResult{}, err
	}

	return InjectionResult{Changed: writeResult.Changed, Files: []string{promptPath}}, nil
}

var claudeModelAssignmentRowOrder = []string{
	"orchestrator",
	"sdd-explore",
	"sdd-propose",
	"sdd-spec",
	"sdd-design",
	"sdd-tasks",
	"sdd-apply",
	"sdd-verify",
	"sdd-archive",
	"default",
}

var claudeModelAssignmentReasons = map[string]string{
	"orchestrator": "Coordinates, makes decisions",
	"sdd-explore":  "Reads code, structural - not architectural",
	"sdd-propose":  "Architectural decisions",
	"sdd-spec":     "Structured writing",
	"sdd-design":   "Architecture decisions",
	"sdd-tasks":    "Mechanical breakdown",
	"sdd-apply":    "Implementation",
	"sdd-verify":   "Validation against spec",
	"sdd-archive":  "Copy and close",
	"default":      "Non-SDD general delegation",
}

func injectClaudeModelAssignments(content string, assignments map[string]model.ClaudeModelAlias) (string, error) {
	const openMarker = "<!-- gentle-ai:sdd-model-assignments -->"
	const closeMarker = "<!-- /gentle-ai:sdd-model-assignments -->"

	start := strings.Index(content, openMarker)
	end := strings.Index(content, closeMarker)
	if start == -1 || end == -1 || end < start {
		return "", fmt.Errorf("sdd orchestrator asset missing model assignment markers")
	}

	merged := model.ClaudeModelPresetBalanced()
	for key, alias := range assignments {
		if alias.Valid() {
			merged[key] = alias
		}
	}

	replacement := renderClaudeModelAssignmentsSection(merged)
	start += len(openMarker)
	return content[:start] + "\n" + replacement + content[end:], nil
}

func renderClaudeModelAssignmentsSection(assignments map[string]model.ClaudeModelAlias) string {
	var b strings.Builder
	b.WriteString("## Model Assignments\n\n")
	b.WriteString("Read this table at session start (or before first delegation), cache it for the session, and pass the mapped alias in every Agent tool call via the `model` parameter. If a phase is missing, use the `default` row. If you do not have access to the assigned model (for example, no Opus access), substitute `sonnet` and continue.\n\n")
	b.WriteString("| Phase | Default Model | Reason |\n")
	b.WriteString("|-------|---------------|--------|\n")
	for _, key := range claudeModelAssignmentRowOrder {
		alias := assignments[key]
		if !alias.Valid() {
			alias = model.ClaudeModelSonnet
		}
		b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", key, alias, claudeModelAssignmentReasons[key]))
	}
	b.WriteString("\n")
	return b.String()
}

// injectModelAssignments injects "model" fields into sub-agent definitions
// within the overlay JSON before it is merged into the settings file.
func injectModelAssignments(overlayBytes []byte, assignments map[string]model.ModelAssignment, rootModelID string) ([]byte, error) {
	var overlay map[string]any
	if err := json.Unmarshal(overlayBytes, &overlay); err != nil {
		return nil, fmt.Errorf("unmarshal overlay for model injection: %w", err)
	}

	agentsRaw, ok := overlay["agent"]
	if !ok {
		return overlayBytes, nil
	}
	agents, ok := agentsRaw.(map[string]any)
	if !ok {
		return overlayBytes, nil
	}

	for phase, agentDef := range agents {
		agentMap, ok := agentDef.(map[string]any)
		if !ok {
			continue
		}

		assignment, hasExplicitAssignment := assignments[phase]
		switch {
		case hasExplicitAssignment && assignment.ProviderID != "" && assignment.ModelID != "":
			agentMap["model"] = assignment.FullID()
		case rootModelID != "":
			agentMap["model"] = rootModelID
		}
	}

	result, err := json.MarshalIndent(overlay, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal overlay after model injection: %w", err)
	}
	return append(result, '\n'), nil
}

func readOpenCodeRootModel(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read opencode root model from %q: %w", path, err)
	}

	root := map[string]any{}
	if err := json.Unmarshal(data, &root); err != nil {
		return "", nil
	}

	rootModelID, _ := root["model"].(string)
	return rootModelID, nil
}

func readFileOrEmpty(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read file %q: %w", path, err)
	}
	return string(data), nil
}
