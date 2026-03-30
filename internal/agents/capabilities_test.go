package agents_test

import (
	"testing"

	"github.com/gentleman-programming/gentle-ai/internal/agents"
	"github.com/gentleman-programming/gentle-ai/internal/agents/claude"
	"github.com/gentleman-programming/gentle-ai/internal/agents/windsurf"
	"github.com/gentleman-programming/gentle-ai/internal/model"
)

func TestGetRuntimeCapabilities_DerivedFromDelegationModel(t *testing.T) {
	tests := []struct {
		name        string
		adapter     agents.Adapter
		wantSubAg   bool
		wantSeqExec bool
	}{
		{
			name:        "claude (multi-agent)",
			adapter:     claude.NewAdapter(),
			wantSubAg:   true,
			wantSeqExec: false,
		},
		{
			name:        "windsurf (single-agent)",
			adapter:     windsurf.NewAdapter(),
			wantSubAg:   false,
			wantSeqExec: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caps := agents.GetRuntimeCapabilities(tt.adapter)
			set := model.NewRuntimeCapabilitySet(caps)

			if got := set.Has(model.CapabilitySubAgents); got != tt.wantSubAg {
				t.Errorf("Has(sub_agents) = %v, want %v", got, tt.wantSubAg)
			}
			if got := set.Has(model.CapabilitySequentialExecution); got != tt.wantSeqExec {
				t.Errorf("Has(sequential_execution) = %v, want %v", got, tt.wantSeqExec)
			}
			// All should have baseline capabilities
			if !set.Has(model.CapabilityFileSystem) {
				t.Error("missing baseline: file_system")
			}
			if !set.Has(model.CapabilityShellExecution) {
				t.Error("missing baseline: shell_execution")
			}
		})
	}
}

func TestGetRuntimeCapabilitySet(t *testing.T) {
	adapter := claude.NewAdapter()
	set := agents.GetRuntimeCapabilitySet(adapter)

	// Should return a usable set
	if !set.Has(model.CapabilitySubAgents) {
		t.Error("claude adapter should have sub_agents capability")
	}
}
