package model

import (
	"fmt"
	"strings"
)

// RuntimeCapability represents a capability that an agent runtime provides.
// Skills declare which capabilities they require; the system matches skills
// to agents based on capability availability.
type RuntimeCapability string

const (
	// CapabilitySubAgents indicates the runtime can spawn and coordinate
	// multiple sub-agents in parallel (e.g., Claude Code with Task tool).
	CapabilitySubAgents RuntimeCapability = "sub_agents"

	// CapabilitySequentialExecution indicates the runtime can execute
	// tasks sequentially with artifact passing between steps.
	CapabilitySequentialExecution RuntimeCapability = "sequential_execution"

	// CapabilityFileSystem indicates the runtime can read/write files.
	CapabilityFileSystem RuntimeCapability = "file_system"

	// CapabilityShellExecution indicates the runtime can execute shell commands.
	CapabilityShellExecution RuntimeCapability = "shell_execution"
)

// knownCapabilities is the authoritative list of valid capabilities.
var knownCapabilities = map[RuntimeCapability]bool{
	CapabilitySubAgents:           true,
	CapabilitySequentialExecution: true,
	CapabilityFileSystem:          true,
	CapabilityShellExecution:      true,
}

// IsValidCapability reports whether c is a known runtime capability.
func IsValidCapability(c RuntimeCapability) bool {
	return knownCapabilities[c]
}

// ExecutionPattern defines HOW a skill executes its logic.
type ExecutionPattern string

const (
	// PatternNative means the skill uses the runtime's native capabilities
	// directly (e.g., parallel sub-agents on Claude Code).
	PatternNative ExecutionPattern = "native"

	// PatternSequentialFallback means the skill adapts to single-agent
	// runtimes by executing steps sequentially with artifact handoff.
	PatternSequentialFallback ExecutionPattern = "sequential_fallback"
)

// IsValidExecutionPattern reports whether p is a known execution pattern.
func IsValidExecutionPattern(p ExecutionPattern) bool {
	switch p {
	case PatternNative, PatternSequentialFallback:
		return true
	default:
		return false
	}
}

// SkillCapabilities defines the capability requirements and execution
// patterns for a skill. This is the new model that will eventually
// replace the binary DelegationModel.
type SkillCapabilities struct {
	// RequiredCapabilities lists capabilities the skill needs.
	// For PatternNative, all must be present.
	// For PatternSequentialFallback, the fallback requirements apply.
	RequiredCapabilities []RuntimeCapability

	// ExecutionPattern defines the primary execution mode.
	ExecutionPattern ExecutionPattern

	// FallbackPattern defines an alternative execution mode when
	// RequiredCapabilities aren't fully available. Empty means no fallback.
	FallbackPattern ExecutionPattern

	// FallbackCapabilities lists the minimum capabilities needed
	// for the fallback pattern. Only valid when FallbackPattern is set.
	FallbackCapabilities []RuntimeCapability
}

// CapabilityValidationError represents a validation failure for skill capabilities.
type CapabilityValidationError struct {
	Field   string
	Message string
}

func (e CapabilityValidationError) Error() string {
	return fmt.Sprintf("capability validation: %s: %s", e.Field, e.Message)
}

// Validate checks that the SkillCapabilities configuration is internally consistent.
// Returns nil if valid, or a CapabilityValidationError describing the issue.
func (sc SkillCapabilities) Validate() error {
	// 1. Validate ExecutionPattern
	if !IsValidExecutionPattern(sc.ExecutionPattern) {
		return CapabilityValidationError{
			Field:   "ExecutionPattern",
			Message: fmt.Sprintf("unknown pattern %q", sc.ExecutionPattern),
		}
	}

	// 2. Validate RequiredCapabilities
	for _, cap := range sc.RequiredCapabilities {
		if !IsValidCapability(cap) {
			return CapabilityValidationError{
				Field:   "RequiredCapabilities",
				Message: fmt.Sprintf("unknown capability %q", cap),
			}
		}
	}

	// 3. Validate FallbackPattern if present
	if sc.FallbackPattern != "" {
		if !IsValidExecutionPattern(sc.FallbackPattern) {
			return CapabilityValidationError{
				Field:   "FallbackPattern",
				Message: fmt.Sprintf("unknown pattern %q", sc.FallbackPattern),
			}
		}

		// FallbackPattern must differ from ExecutionPattern
		if sc.FallbackPattern == sc.ExecutionPattern {
			return CapabilityValidationError{
				Field:   "FallbackPattern",
				Message: "fallback pattern must differ from primary execution pattern",
			}
		}

		// FallbackCapabilities must be present when FallbackPattern is set
		if len(sc.FallbackCapabilities) == 0 {
			return CapabilityValidationError{
				Field:   "FallbackCapabilities",
				Message: "required when FallbackPattern is set",
			}
		}
	}

	// 4. Validate FallbackCapabilities
	for _, cap := range sc.FallbackCapabilities {
		if !IsValidCapability(cap) {
			return CapabilityValidationError{
				Field:   "FallbackCapabilities",
				Message: fmt.Sprintf("unknown capability %q", cap),
			}
		}
	}

	// 5. FallbackCapabilities without FallbackPattern is invalid
	if len(sc.FallbackCapabilities) > 0 && sc.FallbackPattern == "" {
		return CapabilityValidationError{
			Field:   "FallbackCapabilities",
			Message: "cannot specify fallback capabilities without fallback pattern",
		}
	}

	return nil
}

// HasCapability reports whether cap is in the required capabilities list.
func (sc SkillCapabilities) HasCapability(cap RuntimeCapability) bool {
	for _, c := range sc.RequiredCapabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// RequiresSubAgents is a convenience method for the common check.
func (sc SkillCapabilities) RequiresSubAgents() bool {
	return sc.HasCapability(CapabilitySubAgents)
}

// CanFallback reports whether this skill has a defined fallback pattern.
func (sc SkillCapabilities) CanFallback() bool {
	return sc.FallbackPattern != ""
}

// String returns a human-readable summary of the capabilities.
func (sc SkillCapabilities) String() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("pattern=%s", sc.ExecutionPattern))

	if len(sc.RequiredCapabilities) > 0 {
		caps := make([]string, len(sc.RequiredCapabilities))
		for i, c := range sc.RequiredCapabilities {
			caps[i] = string(c)
		}
		parts = append(parts, fmt.Sprintf("requires=[%s]", strings.Join(caps, ",")))
	}

	if sc.FallbackPattern != "" {
		parts = append(parts, fmt.Sprintf("fallback=%s", sc.FallbackPattern))
	}

	return strings.Join(parts, " ")
}

// ToDelegationModel converts SkillCapabilities to the legacy DelegationModel
// for backward compatibility during the migration period.
//
// Mapping rules:
//   - RequiresSubAgents + no fallback → DelegationMultiAgent
//   - RequiresSubAgents + has fallback → DelegationAny
//   - No sub-agent requirement → DelegationAny
func (sc SkillCapabilities) ToDelegationModel() DelegationModel {
	if sc.RequiresSubAgents() {
		if sc.CanFallback() {
			return DelegationAny
		}
		return DelegationMultiAgent
	}
	return DelegationAny
}

// SkillCapabilitiesFromDelegationModel creates a SkillCapabilities from a
// legacy DelegationModel for backward compatibility.
//
// Mapping rules:
//   - DelegationMultiAgent → requires sub_agents, native pattern, no fallback
//   - DelegationSingleAgent → requires sequential_execution, native pattern
//   - DelegationAny → no specific requirements, native pattern
func SkillCapabilitiesFromDelegationModel(dm DelegationModel) SkillCapabilities {
	switch dm {
	case DelegationMultiAgent:
		return SkillCapabilities{
			RequiredCapabilities: []RuntimeCapability{CapabilitySubAgents},
			ExecutionPattern:     PatternNative,
		}
	case DelegationSingleAgent:
		return SkillCapabilities{
			RequiredCapabilities: []RuntimeCapability{CapabilitySequentialExecution},
			ExecutionPattern:     PatternNative,
		}
	default: // DelegationAny
		return SkillCapabilities{
			ExecutionPattern: PatternNative,
		}
	}
}
