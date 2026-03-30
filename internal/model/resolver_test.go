package model

import (
	"testing"
)

func TestResolveSkillCompatibility_NativeNoRequirements(t *testing.T) {
	// Universal skill with no requirements should always be native
	skill := SkillCapabilities{
		ExecutionPattern: PatternNative,
	}
	runtime := NewRuntimeCapabilitySet([]RuntimeCapability{CapabilityFileSystem})

	result := ResolveSkillCompatibility(skill, runtime)

	if result.Mode != ResolutionModeNative {
		t.Errorf("Mode = %v, want %v", result.Mode, ResolutionModeNative)
	}
	if result.Pattern != PatternNative {
		t.Errorf("Pattern = %v, want %v", result.Pattern, PatternNative)
	}
	if len(result.MissingCapabilities) != 0 {
		t.Errorf("MissingCapabilities = %v, want empty", result.MissingCapabilities)
	}
}

func TestResolveSkillCompatibility_NativeAllCapabilitiesPresent(t *testing.T) {
	// Multi-agent skill on multi-agent runtime
	skill := SkillCapabilities{
		RequiredCapabilities: []RuntimeCapability{CapabilitySubAgents},
		ExecutionPattern:     PatternNative,
	}
	runtime := NewRuntimeCapabilitySet([]RuntimeCapability{
		CapabilitySubAgents,
		CapabilityFileSystem,
		CapabilityShellExecution,
	})

	result := ResolveSkillCompatibility(skill, runtime)

	if result.Mode != ResolutionModeNative {
		t.Errorf("Mode = %v, want %v", result.Mode, ResolutionModeNative)
	}
	if result.Pattern != PatternNative {
		t.Errorf("Pattern = %v, want %v", result.Pattern, PatternNative)
	}
}

func TestResolveSkillCompatibility_FallbackWhenPrimaryMissing(t *testing.T) {
	// Multi-agent skill with fallback on single-agent runtime
	skill := SkillCapabilities{
		RequiredCapabilities: []RuntimeCapability{CapabilitySubAgents},
		ExecutionPattern:     PatternNative,
		FallbackPattern:      PatternSequentialFallback,
		FallbackCapabilities: []RuntimeCapability{CapabilitySequentialExecution, CapabilityFileSystem},
	}
	runtime := NewRuntimeCapabilitySet([]RuntimeCapability{
		CapabilitySequentialExecution,
		CapabilityFileSystem,
		CapabilityShellExecution,
	})

	result := ResolveSkillCompatibility(skill, runtime)

	if result.Mode != ResolutionModeFallback {
		t.Errorf("Mode = %v, want %v", result.Mode, ResolutionModeFallback)
	}
	if result.Pattern != PatternSequentialFallback {
		t.Errorf("Pattern = %v, want %v", result.Pattern, PatternSequentialFallback)
	}
	if len(result.MissingCapabilities) != 1 || result.MissingCapabilities[0] != CapabilitySubAgents {
		t.Errorf("MissingCapabilities = %v, want [sub_agents]", result.MissingCapabilities)
	}
}

func TestResolveSkillCompatibility_UnsupportedNoFallback(t *testing.T) {
	// Multi-agent only skill on single-agent runtime (no fallback defined)
	skill := SkillCapabilities{
		RequiredCapabilities: []RuntimeCapability{CapabilitySubAgents},
		ExecutionPattern:     PatternNative,
		// No fallback
	}
	runtime := NewRuntimeCapabilitySet([]RuntimeCapability{
		CapabilitySequentialExecution,
		CapabilityFileSystem,
	})

	result := ResolveSkillCompatibility(skill, runtime)

	if result.Mode != ResolutionModeUnsupported {
		t.Errorf("Mode = %v, want %v", result.Mode, ResolutionModeUnsupported)
	}
	if result.Pattern != "" {
		t.Errorf("Pattern = %v, want empty", result.Pattern)
	}
	if len(result.MissingCapabilities) != 1 || result.MissingCapabilities[0] != CapabilitySubAgents {
		t.Errorf("MissingCapabilities = %v, want [sub_agents]", result.MissingCapabilities)
	}
}

func TestResolveSkillCompatibility_UnsupportedFallbackAlsoMissing(t *testing.T) {
	// Skill has fallback but runtime doesn't support fallback either
	skill := SkillCapabilities{
		RequiredCapabilities: []RuntimeCapability{CapabilitySubAgents},
		ExecutionPattern:     PatternNative,
		FallbackPattern:      PatternSequentialFallback,
		FallbackCapabilities: []RuntimeCapability{CapabilitySequentialExecution, CapabilityFileSystem},
	}
	// Runtime has neither sub_agents nor sequential_execution
	runtime := NewRuntimeCapabilitySet([]RuntimeCapability{CapabilityShellExecution})

	result := ResolveSkillCompatibility(skill, runtime)

	if result.Mode != ResolutionModeUnsupported {
		t.Errorf("Mode = %v, want %v", result.Mode, ResolutionModeUnsupported)
	}
	// Reason should mention both primary and fallback missing
	if result.Reason == "" {
		t.Error("Reason should not be empty")
	}
}

func TestRuntimeCapabilitySet_Operations(t *testing.T) {
	set := NewRuntimeCapabilitySet([]RuntimeCapability{
		CapabilitySubAgents,
		CapabilityFileSystem,
	})

	// Has
	if !set.Has(CapabilitySubAgents) {
		t.Error("Has(sub_agents) = false, want true")
	}
	if set.Has(CapabilitySequentialExecution) {
		t.Error("Has(sequential_execution) = true, want false")
	}

	// HasAll
	if !set.HasAll([]RuntimeCapability{CapabilitySubAgents, CapabilityFileSystem}) {
		t.Error("HasAll([sub_agents, file_system]) = false, want true")
	}
	if set.HasAll([]RuntimeCapability{CapabilitySubAgents, CapabilitySequentialExecution}) {
		t.Error("HasAll([sub_agents, sequential_execution]) = true, want false")
	}

	// Missing
	missing := set.Missing([]RuntimeCapability{CapabilitySubAgents, CapabilitySequentialExecution})
	if len(missing) != 1 || missing[0] != CapabilitySequentialExecution {
		t.Errorf("Missing() = %v, want [sequential_execution]", missing)
	}

	// Empty case
	if !set.HasAll(nil) {
		t.Error("HasAll(nil) = false, want true")
	}
	if !set.HasAll([]RuntimeCapability{}) {
		t.Error("HasAll([]) = false, want true")
	}
}

func TestRuntimeCapabilitiesFromDelegationModel(t *testing.T) {
	tests := []struct {
		dm          DelegationModel
		wantSubAg   bool
		wantSeqExec bool
	}{
		{DelegationMultiAgent, true, false},
		{DelegationSingleAgent, false, true},
		{DelegationAny, false, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.dm), func(t *testing.T) {
			caps := RuntimeCapabilitiesFromDelegationModel(tt.dm)
			set := NewRuntimeCapabilitySet(caps)

			if got := set.Has(CapabilitySubAgents); got != tt.wantSubAg {
				t.Errorf("Has(sub_agents) = %v, want %v", got, tt.wantSubAg)
			}
			if got := set.Has(CapabilitySequentialExecution); got != tt.wantSeqExec {
				t.Errorf("Has(sequential_execution) = %v, want %v", got, tt.wantSeqExec)
			}
			// All should have baseline
			if !set.Has(CapabilityFileSystem) {
				t.Error("missing baseline capability: file_system")
			}
			if !set.Has(CapabilityShellExecution) {
				t.Error("missing baseline capability: shell_execution")
			}
		})
	}
}

func TestResolutionResult_String(t *testing.T) {
	native := ResolutionResult{
		Mode:    ResolutionModeNative,
		Pattern: PatternNative,
		Reason:  "test reason",
	}
	if s := native.String(); s == "" {
		t.Error("String() returned empty for native result")
	}

	unsupported := ResolutionResult{
		Mode:   ResolutionModeUnsupported,
		Reason: "missing caps",
	}
	if s := unsupported.String(); s == "" {
		t.Error("String() returned empty for unsupported result")
	}
}

func TestResolveSkillCompatibility_LegacyBackwardCompatibility(t *testing.T) {
	// Test that skills converted from DelegationModel work correctly
	// with runtimes derived from DelegationModel

	// Case 1: Multi-agent skill on multi-agent runtime
	multiSkill := SkillCapabilitiesFromDelegationModel(DelegationMultiAgent)
	multiRuntime := NewRuntimeCapabilitySet(RuntimeCapabilitiesFromDelegationModel(DelegationMultiAgent))

	result := ResolveSkillCompatibility(multiSkill, multiRuntime)
	if result.Mode != ResolutionModeNative {
		t.Errorf("multi on multi: Mode = %v, want native", result.Mode)
	}

	// Case 2: Multi-agent skill on single-agent runtime (no fallback = unsupported)
	singleRuntime := NewRuntimeCapabilitySet(RuntimeCapabilitiesFromDelegationModel(DelegationSingleAgent))
	result = ResolveSkillCompatibility(multiSkill, singleRuntime)
	if result.Mode != ResolutionModeUnsupported {
		t.Errorf("multi on single (no fallback): Mode = %v, want unsupported", result.Mode)
	}

	// Case 3: Any skill on any runtime
	anySkill := SkillCapabilitiesFromDelegationModel(DelegationAny)
	result = ResolveSkillCompatibility(anySkill, singleRuntime)
	if result.Mode != ResolutionModeNative {
		t.Errorf("any on single: Mode = %v, want native", result.Mode)
	}
}
