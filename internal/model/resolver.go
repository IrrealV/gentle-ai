package model

import (
	"fmt"
	"strings"
)

// ResolutionMode indicates how a skill should execute on a given runtime.
type ResolutionMode string

const (
	// ResolutionModeNative means the skill can use its primary execution pattern.
	ResolutionModeNative ResolutionMode = "native"

	// ResolutionModeFallback means the skill must use its fallback pattern.
	ResolutionModeFallback ResolutionMode = "fallback"

	// ResolutionModeUnsupported means the skill cannot run on this runtime.
	ResolutionModeUnsupported ResolutionMode = "unsupported"
)

// ResolutionResult is the output of the capability resolver.
// It contains the decision and the reasoning for traceability.
type ResolutionResult struct {
	// Mode is the resolved execution mode.
	Mode ResolutionMode

	// Pattern is the actual ExecutionPattern to use (native or fallback).
	// Empty when Mode is Unsupported.
	Pattern ExecutionPattern

	// Reason explains why this mode was chosen (for logging/debugging).
	Reason string

	// MissingCapabilities lists capabilities that were required but not available.
	// Populated when Mode is Fallback or Unsupported.
	MissingCapabilities []RuntimeCapability
}

// String returns a human-readable summary of the resolution.
func (r ResolutionResult) String() string {
	if r.Mode == ResolutionModeUnsupported {
		return fmt.Sprintf("mode=%s reason=%q", r.Mode, r.Reason)
	}
	return fmt.Sprintf("mode=%s pattern=%s reason=%q", r.Mode, r.Pattern, r.Reason)
}

// RuntimeCapabilitySet represents the capabilities available in a runtime.
// Using a set for O(1) lookups.
type RuntimeCapabilitySet map[RuntimeCapability]bool

// NewRuntimeCapabilitySet creates a set from a slice of capabilities.
func NewRuntimeCapabilitySet(caps []RuntimeCapability) RuntimeCapabilitySet {
	set := make(RuntimeCapabilitySet, len(caps))
	for _, c := range caps {
		set[c] = true
	}
	return set
}

// Has reports whether the set contains the given capability.
func (s RuntimeCapabilitySet) Has(cap RuntimeCapability) bool {
	return s[cap]
}

// HasAll reports whether the set contains all given capabilities.
func (s RuntimeCapabilitySet) HasAll(caps []RuntimeCapability) bool {
	for _, c := range caps {
		if !s[c] {
			return false
		}
	}
	return true
}

// Missing returns capabilities from the input that are not in the set.
func (s RuntimeCapabilitySet) Missing(caps []RuntimeCapability) []RuntimeCapability {
	var missing []RuntimeCapability
	for _, c := range caps {
		if !s[c] {
			missing = append(missing, c)
		}
	}
	return missing
}

// Slice returns the capabilities as a slice.
func (s RuntimeCapabilitySet) Slice() []RuntimeCapability {
	caps := make([]RuntimeCapability, 0, len(s))
	for c := range s {
		caps = append(caps, c)
	}
	return caps
}

// ResolveSkillCompatibility determines how a skill should execute given
// the available runtime capabilities.
//
// This is a pure function with no side effects, making it easy to test.
//
// Decision logic:
//  1. If skill has no required capabilities → Native (universal skill)
//  2. If runtime has all required capabilities → Native
//  3. If skill has fallback AND runtime has fallback capabilities → Fallback
//  4. Otherwise → Unsupported
func ResolveSkillCompatibility(skill SkillCapabilities, runtime RuntimeCapabilitySet) ResolutionResult {
	// Case 1: No requirements = universal skill
	if len(skill.RequiredCapabilities) == 0 {
		return ResolutionResult{
			Mode:    ResolutionModeNative,
			Pattern: skill.ExecutionPattern,
			Reason:  "skill has no capability requirements",
		}
	}

	// Case 2: Runtime satisfies all requirements
	if runtime.HasAll(skill.RequiredCapabilities) {
		return ResolutionResult{
			Mode:    ResolutionModeNative,
			Pattern: skill.ExecutionPattern,
			Reason:  "runtime provides all required capabilities",
		}
	}

	// At this point, some capabilities are missing
	missing := runtime.Missing(skill.RequiredCapabilities)

	// Case 3: Skill has fallback and runtime can support it
	if skill.CanFallback() && runtime.HasAll(skill.FallbackCapabilities) {
		return ResolutionResult{
			Mode:                ResolutionModeFallback,
			Pattern:             skill.FallbackPattern,
			Reason:              fmt.Sprintf("using fallback; missing primary capabilities: %s", formatCaps(missing)),
			MissingCapabilities: missing,
		}
	}

	// Case 4: Unsupported
	reason := fmt.Sprintf("missing capabilities: %s", formatCaps(missing))
	if skill.CanFallback() {
		fallbackMissing := runtime.Missing(skill.FallbackCapabilities)
		reason = fmt.Sprintf("missing primary capabilities: %s; fallback also unavailable (missing: %s)",
			formatCaps(missing), formatCaps(fallbackMissing))
	}

	return ResolutionResult{
		Mode:                ResolutionModeUnsupported,
		Reason:              reason,
		MissingCapabilities: missing,
	}
}

// formatCaps formats a slice of capabilities for display.
func formatCaps(caps []RuntimeCapability) string {
	if len(caps) == 0 {
		return "none"
	}
	strs := make([]string, len(caps))
	for i, c := range caps {
		strs[i] = string(c)
	}
	return strings.Join(strs, ", ")
}

// RuntimeCapabilitiesFromDelegationModel derives runtime capabilities from
// the legacy DelegationModel. This enables backward compatibility: adapters
// that only implement DelegationModel() can still participate in capability
// resolution.
//
// Mapping:
//   - DelegationMultiAgent → sub_agents, file_system, shell_execution
//   - DelegationSingleAgent → sequential_execution, file_system, shell_execution
//   - DelegationAny → file_system, shell_execution (baseline)
func RuntimeCapabilitiesFromDelegationModel(dm DelegationModel) []RuntimeCapability {
	// Baseline capabilities all agents have
	baseline := []RuntimeCapability{CapabilityFileSystem, CapabilityShellExecution}

	switch dm {
	case DelegationMultiAgent:
		return append(baseline, CapabilitySubAgents)
	case DelegationSingleAgent:
		return append(baseline, CapabilitySequentialExecution)
	default: // DelegationAny - shouldn't happen for adapters, but handle gracefully
		return baseline
	}
}
