package catalog

import "github.com/gentleman-programming/gentle-ai/internal/model"

// MVPSkills delegates to model.MVPSkills() to avoid import cycles.
// The catalog package exists for organizational purposes but the canonical
// skill definitions live in model/types.go.
func MVPSkills() []model.Skill {
	return model.MVPSkills()
}
