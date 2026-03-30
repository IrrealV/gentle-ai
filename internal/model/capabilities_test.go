package model

import (
	"testing"
)

func TestIsValidCapability(t *testing.T) {
	tests := []struct {
		cap  RuntimeCapability
		want bool
	}{
		{CapabilitySubAgents, true},
		{CapabilitySequentialExecution, true},
		{CapabilityFileSystem, true},
		{CapabilityShellExecution, true},
		{"unknown_capability", false},
		{"", false},
		{"sub-agents", false}, // wrong format (hyphen vs underscore)
	}

	for _, tt := range tests {
		t.Run(string(tt.cap), func(t *testing.T) {
			got := IsValidCapability(tt.cap)
			if got != tt.want {
				t.Errorf("IsValidCapability(%q) = %v, want %v", tt.cap, got, tt.want)
			}
		})
	}
}

func TestIsValidExecutionPattern(t *testing.T) {
	tests := []struct {
		pattern ExecutionPattern
		want    bool
	}{
		{PatternNative, true},
		{PatternSequentialFallback, true},
		{"unknown", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.pattern), func(t *testing.T) {
			got := IsValidExecutionPattern(tt.pattern)
			if got != tt.want {
				t.Errorf("IsValidExecutionPattern(%q) = %v, want %v", tt.pattern, got, tt.want)
			}
		})
	}
}

func TestSkillCapabilities_Validate_ValidMultiOnly(t *testing.T) {
	// Case: skill that ONLY works on multi-agent runtimes (no fallback)
	sc := SkillCapabilities{
		RequiredCapabilities: []RuntimeCapability{CapabilitySubAgents},
		ExecutionPattern:     PatternNative,
	}

	if err := sc.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil for valid multi-only config", err)
	}

	// Should map to DelegationMultiAgent
	if dm := sc.ToDelegationModel(); dm != DelegationMultiAgent {
		t.Errorf("ToDelegationModel() = %v, want %v", dm, DelegationMultiAgent)
	}
}

func TestSkillCapabilities_Validate_ValidAdaptableToSingle(t *testing.T) {
	// Case: skill that prefers multi-agent but can adapt to single-agent
	sc := SkillCapabilities{
		RequiredCapabilities: []RuntimeCapability{CapabilitySubAgents},
		ExecutionPattern:     PatternNative,
		FallbackPattern:      PatternSequentialFallback,
		FallbackCapabilities: []RuntimeCapability{CapabilitySequentialExecution, CapabilityFileSystem},
	}

	if err := sc.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil for valid adaptable config", err)
	}

	// Should map to DelegationAny (can work on both)
	if dm := sc.ToDelegationModel(); dm != DelegationAny {
		t.Errorf("ToDelegationModel() = %v, want %v", dm, DelegationAny)
	}

	// Verify helper methods
	if !sc.RequiresSubAgents() {
		t.Error("RequiresSubAgents() = false, want true")
	}
	if !sc.CanFallback() {
		t.Error("CanFallback() = false, want true")
	}
}

func TestSkillCapabilities_Validate_InvalidUnknownCapability(t *testing.T) {
	sc := SkillCapabilities{
		RequiredCapabilities: []RuntimeCapability{"unknown_cap"},
		ExecutionPattern:     PatternNative,
	}

	err := sc.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want error for unknown capability")
	}

	capErr, ok := err.(CapabilityValidationError)
	if !ok {
		t.Fatalf("error type = %T, want CapabilityValidationError", err)
	}
	if capErr.Field != "RequiredCapabilities" {
		t.Errorf("error.Field = %q, want RequiredCapabilities", capErr.Field)
	}
}

func TestSkillCapabilities_Validate_InvalidFallbackInconsistent(t *testing.T) {
	tests := []struct {
		name      string
		sc        SkillCapabilities
		wantField string
	}{
		{
			name: "fallback pattern same as primary",
			sc: SkillCapabilities{
				RequiredCapabilities: []RuntimeCapability{CapabilitySubAgents},
				ExecutionPattern:     PatternNative,
				FallbackPattern:      PatternNative, // same as primary
				FallbackCapabilities: []RuntimeCapability{CapabilityFileSystem},
			},
			wantField: "FallbackPattern",
		},
		{
			name: "fallback pattern without capabilities",
			sc: SkillCapabilities{
				RequiredCapabilities: []RuntimeCapability{CapabilitySubAgents},
				ExecutionPattern:     PatternNative,
				FallbackPattern:      PatternSequentialFallback,
				// Missing FallbackCapabilities
			},
			wantField: "FallbackCapabilities",
		},
		{
			name: "fallback capabilities without pattern",
			sc: SkillCapabilities{
				RequiredCapabilities: []RuntimeCapability{CapabilitySubAgents},
				ExecutionPattern:     PatternNative,
				// Missing FallbackPattern
				FallbackCapabilities: []RuntimeCapability{CapabilityFileSystem},
			},
			wantField: "FallbackCapabilities",
		},
		{
			name: "unknown fallback pattern",
			sc: SkillCapabilities{
				RequiredCapabilities: []RuntimeCapability{CapabilitySubAgents},
				ExecutionPattern:     PatternNative,
				FallbackPattern:      "unknown_pattern",
				FallbackCapabilities: []RuntimeCapability{CapabilityFileSystem},
			},
			wantField: "FallbackPattern",
		},
		{
			name: "unknown fallback capability",
			sc: SkillCapabilities{
				RequiredCapabilities: []RuntimeCapability{CapabilitySubAgents},
				ExecutionPattern:     PatternNative,
				FallbackPattern:      PatternSequentialFallback,
				FallbackCapabilities: []RuntimeCapability{"bad_cap"},
			},
			wantField: "FallbackCapabilities",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.sc.Validate()
			if err == nil {
				t.Fatal("Validate() = nil, want error")
			}

			capErr, ok := err.(CapabilityValidationError)
			if !ok {
				t.Fatalf("error type = %T, want CapabilityValidationError", err)
			}
			if capErr.Field != tt.wantField {
				t.Errorf("error.Field = %q, want %q", capErr.Field, tt.wantField)
			}
		})
	}
}

func TestSkillCapabilities_Validate_InvalidExecutionPattern(t *testing.T) {
	sc := SkillCapabilities{
		ExecutionPattern: "invalid_pattern",
	}

	err := sc.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want error for invalid execution pattern")
	}

	capErr, ok := err.(CapabilityValidationError)
	if !ok {
		t.Fatalf("error type = %T, want CapabilityValidationError", err)
	}
	if capErr.Field != "ExecutionPattern" {
		t.Errorf("error.Field = %q, want ExecutionPattern", capErr.Field)
	}
}

func TestSkillCapabilities_String(t *testing.T) {
	sc := SkillCapabilities{
		RequiredCapabilities: []RuntimeCapability{CapabilitySubAgents, CapabilityFileSystem},
		ExecutionPattern:     PatternNative,
		FallbackPattern:      PatternSequentialFallback,
		FallbackCapabilities: []RuntimeCapability{CapabilitySequentialExecution},
	}

	s := sc.String()
	if s == "" {
		t.Error("String() returned empty string")
	}
	// Should contain key information
	if !contains(s, "native") {
		t.Errorf("String() = %q, missing 'native'", s)
	}
	if !contains(s, "sub_agents") {
		t.Errorf("String() = %q, missing 'sub_agents'", s)
	}
	if !contains(s, "sequential_fallback") {
		t.Errorf("String() = %q, missing 'sequential_fallback'", s)
	}
}

func TestSkillCapabilitiesFromDelegationModel(t *testing.T) {
	tests := []struct {
		dm             DelegationModel
		wantSubAgents  bool
		wantCanFallbck bool
	}{
		{DelegationMultiAgent, true, false},
		{DelegationSingleAgent, false, false},
		{DelegationAny, false, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.dm), func(t *testing.T) {
			sc := SkillCapabilitiesFromDelegationModel(tt.dm)

			// Must be valid
			if err := sc.Validate(); err != nil {
				t.Errorf("Validate() error = %v for converted %s", err, tt.dm)
			}

			if got := sc.RequiresSubAgents(); got != tt.wantSubAgents {
				t.Errorf("RequiresSubAgents() = %v, want %v", got, tt.wantSubAgents)
			}

			if got := sc.CanFallback(); got != tt.wantCanFallbck {
				t.Errorf("CanFallback() = %v, want %v", got, tt.wantCanFallbck)
			}
		})
	}
}

func TestSkillCapabilities_ToDelegationModel_Roundtrip(t *testing.T) {
	// Multi-only should stay multi
	multiOnly := SkillCapabilities{
		RequiredCapabilities: []RuntimeCapability{CapabilitySubAgents},
		ExecutionPattern:     PatternNative,
	}
	if dm := multiOnly.ToDelegationModel(); dm != DelegationMultiAgent {
		t.Errorf("multi-only ToDelegationModel() = %v, want %v", dm, DelegationMultiAgent)
	}

	// Multi with fallback should become Any
	multiWithFallback := SkillCapabilities{
		RequiredCapabilities: []RuntimeCapability{CapabilitySubAgents},
		ExecutionPattern:     PatternNative,
		FallbackPattern:      PatternSequentialFallback,
		FallbackCapabilities: []RuntimeCapability{CapabilitySequentialExecution},
	}
	if dm := multiWithFallback.ToDelegationModel(); dm != DelegationAny {
		t.Errorf("multi-with-fallback ToDelegationModel() = %v, want %v", dm, DelegationAny)
	}

	// No sub-agent requirement should be Any
	noSubAgents := SkillCapabilities{
		RequiredCapabilities: []RuntimeCapability{CapabilityFileSystem},
		ExecutionPattern:     PatternNative,
	}
	if dm := noSubAgents.ToDelegationModel(); dm != DelegationAny {
		t.Errorf("no-sub-agents ToDelegationModel() = %v, want %v", dm, DelegationAny)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
