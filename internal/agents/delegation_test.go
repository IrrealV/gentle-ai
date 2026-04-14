package agents

import (
	"testing"

	"github.com/gentleman-programming/gentle-ai/internal/model"
)

type delegationModeler interface {
	DelegationModel() model.DelegationModel
}

func TestDelegationModels_NewAgents(t *testing.T) {
	tests := []struct {
		agent model.AgentID
		want  model.DelegationModel
	}{
		{agent: model.AgentKilocode, want: model.DelegationMultiAgent},
		{agent: model.AgentKiroIDE, want: model.DelegationMultiAgent},
		{agent: model.AgentQwenCode, want: model.DelegationSingleAgent},
	}

	for _, tt := range tests {
		t.Run(string(tt.agent), func(t *testing.T) {
			adapter, err := NewAdapter(tt.agent)
			if err != nil {
				t.Fatalf("NewAdapter(%q) error = %v", tt.agent, err)
			}

			d, ok := adapter.(delegationModeler)
			if !ok {
				t.Fatalf("adapter %q does not implement DelegationModel()", tt.agent)
			}

			if got := d.DelegationModel(); got != tt.want {
				t.Fatalf("DelegationModel() for %q = %q, want %q", tt.agent, got, tt.want)
			}
		})
	}
}
