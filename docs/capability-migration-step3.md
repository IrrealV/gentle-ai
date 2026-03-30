# Capability Migration - Step 3 Summary

## Overview

Step 3 implemented the capability-aware skill resolution system, allowing skills to declare explicit capabilities and execute differently based on runtime capabilities (native vs fallback).

## Changes Made

### New Files

| File | Purpose |
|------|---------|
| `internal/model/capabilities.go` | Defines `RuntimeCapability`, `ExecutionPattern`, `SkillCapabilities` types and validation |
| `internal/model/capabilities_test.go` | Unit tests for capabilities |
| `internal/model/resolver.go` | Defines `ResolutionMode`, `ResolutionResult`, `RuntimeCapabilitySet` and the `ResolveSkillCompatibility` function |
| `internal/model/resolver_test.go` | Unit tests for resolver |
| `internal/agents/capabilities.go` | Defines `RuntimeCapabilitiesProvider` interface and helper functions |
| `internal/agents/capabilities_test.go` | Unit tests for agent capabilities |

### Modified Files

| File | Changes |
|------|---------|
| `internal/model/types.go` | Added `Capabilities *SkillCapabilities` field to `Skill` struct, `GetCapabilities()` method, and explicit capabilities for `judgment-day` |
| `internal/components/skills/inject.go` | Extended `InjectionResult` with resolution results, `SkillTemplateData` with `ResolutionMode`/`ExecutionPattern`, updated `Inject()` to use capability-aware resolver |
| `internal/components/skills/inject_test.go` | Added tests for resolution modes, backward compatibility, and template comparator regression |
| `internal/assets/skills/sdd-apply/SKILL.md` | Fixed incorrect comparator `"multi-agent"` → `"multi"` |
| `internal/assets/skills/skill-creator/SKILL.md` | Migrated to use `ResolutionMode` with `DelegationModel` fallback |

## Key Decisions

### 1. Capability Declaration

Skills can now declare explicit capabilities:

```go
Capabilities: &SkillCapabilities{
    RequiredCapabilities: []RuntimeCapability{CapabilitySubAgents},
    ExecutionPattern:     PatternNative,
    FallbackPattern:      PatternSequentialFallback,
    FallbackCapabilities: []RuntimeCapability{CapabilitySequentialExecution, CapabilityFileSystem},
},
```

### 2. Resolution Logic

The resolver is a pure function with clear decision logic:
1. No requirements → Native (universal skill)
2. Runtime has all required capabilities → Native
3. Skill has fallback AND runtime has fallback capabilities → Fallback
4. Otherwise → Unsupported

### 3. Backward Compatibility

- Skills without explicit `Capabilities` get capabilities inferred from `DelegationModel`
- Templates can use both `ResolutionMode` and `DelegationModel` for gradual migration
- Existing tests continue to pass

## Known Limitations

### SDD Skills Cannot Use ResolutionMode Templates

**Issue**: The SDD component writes skill files directly from embedded assets without template processing. This means SDD skills (`sdd-*`, `judgment-day`) cannot have conditional content based on `ResolutionMode`.

**Impact**: `judgment-day` and other SDD skills will have the same content regardless of whether they're running in native or fallback mode.

**Workaround**: The capability resolution still works correctly (the skill is marked as native/fallback in `InjectionResult.Resolutions`), but the template content is static.

**Resolution for Step 4**: Modify the SDD component to process templates with `SkillTemplateData`, or move SDD skills to the skills component.

### Template Variable Availability

Templates can use:
- `[[.DelegationModel]]` - "single" or "multi" (legacy)
- `[[.ResolutionMode]]` - "native", "fallback", or "unsupported" (new)
- `[[.ExecutionPattern]]` - "native" or "sequential_fallback" (new)

Non-SDD skills (like `skill-creator`) can use all three. SDD skills can only use `DelegationModel` until Step 4.

## Risks for Step 4

### 1. SDD Component Template Processing

Adding template processing to the SDD component requires:
- Passing `SkillTemplateData` to the skill writing logic
- Ensuring the adapter's capabilities are available during SDD injection
- Testing idempotency with rendered templates

### 2. Migration of Remaining Skills

Skills that need capability-aware behavior:
- `judgment-day` - Parallel vs sequential judge execution
- `sdd-apply` - Delegation vs direct execution (already has `DelegationModel` check)
- Other SDD skills may benefit from similar patterns

### 3. Agent Capability Declaration

Currently, capabilities are inferred from `DelegationModel`. Step 4 should:
- Add explicit `RuntimeCapabilities()` method to adapters
- Define capabilities for each agent (Claude, OpenCode, Windsurf, etc.)
- Consider capabilities beyond delegation (MCP support, file system access, etc.)

## Test Coverage

| Test | Purpose |
|------|---------|
| `TestInjectResolutionResult_NativeMode` | Multi-agent adapter gets native resolution |
| `TestInjectResolutionResult_SkipsMultiOnlyOnSingleAgent` | Single-agent adapter gets fallback for judgment-day |
| `TestInjectResolutionResult_LegacySkillBackwardCompatible` | Skills without Capabilities work via DelegationModel |
| `TestInjectTemplateComparator_Regression` | Ensures correct comparator values in templates |
| `TestInjectJudgmentDay_Idempotent` | Verifies idempotency of skill injection |

## Files to Review

Before merging, review:
1. `internal/model/capabilities.go` - Core capability types
2. `internal/model/resolver.go` - Resolution logic
3. `internal/components/skills/inject.go` - Integration with injection
4. `internal/model/types.go` - Skill metadata changes
