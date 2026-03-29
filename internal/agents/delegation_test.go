package agents_test

import (
	"testing"

	"github.com/gentleman-programming/gentle-ai/internal/agents"
	"github.com/gentleman-programming/gentle-ai/internal/model"
)

func TestDelegationModels(t *testing.T) {
	tests := []struct {
		agent model.AgentID
		want  model.DelegationModel
	}{
		{model.AgentClaudeCode, model.ModelSingleAgent},
		{model.AgentOpenCode, model.ModelSingleAgent},
		{model.AgentGeminiCLI, model.ModelSingleAgent},
		{model.AgentCursor, model.ModelSingleAgent},
		{model.AgentVSCodeCopilot, model.ModelSingleAgent},
		{model.AgentCodex, model.ModelSingleAgent},
		{model.AgentAntigravity, model.ModelMultiAgent},
		{model.AgentWindsurf, model.ModelMultiAgent},
	}

	for _, tt := range tests {
		t.Run(string(tt.agent), func(t *testing.T) {
			adapter, err := agents.NewAdapter(tt.agent)
			if err != nil {
				t.Fatalf("failed to create adapter for %s: %v", tt.agent, err)
			}
			
			got := adapter.DelegationModel()
			if got != tt.want {
				t.Errorf("DelegationModel() for %s = %v, want %v", tt.agent, got, tt.want)
			}
		})
	}
}
