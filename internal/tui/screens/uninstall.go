package screens

import (
	"fmt"
	"strings"

	"github.com/gentleman-programming/gentle-ai/internal/catalog"
	componentuninstall "github.com/gentleman-programming/gentle-ai/internal/components/uninstall"
	"github.com/gentleman-programming/gentle-ai/internal/model"
	"github.com/gentleman-programming/gentle-ai/internal/tui/styles"
)

func UninstallAgentOptions() []catalog.Agent {
	return catalog.AllAgents()
}

func UninstallComponentOptions() []catalog.Component {
	return catalog.MVPComponents()
}

func RenderUninstall(selected []model.AgentID, cursor int) string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Uninstall Managed Configs"))
	b.WriteString("\n\n")
	b.WriteString(styles.HelpStyle.Render("Use j/k to move, space to toggle, enter to continue."))
	b.WriteString("\n\n")
	b.WriteString(styles.SubtextStyle.Render("Select the agents whose gentle-ai managed configuration should be removed."))
	b.WriteString("\n\n")

	selectedSet := make(map[model.AgentID]struct{}, len(selected))
	for _, agent := range selected {
		selectedSet[agent] = struct{}{}
	}

	for idx, agent := range UninstallAgentOptions() {
		_, checked := selectedSet[agent.ID]
		focused := idx == cursor
		b.WriteString(renderCheckbox(agent.Name, checked, focused))
	}

	b.WriteString("\n")
	b.WriteString(renderOptions([]string{"Continue", "Back"}, cursor-len(UninstallAgentOptions())))
	b.WriteString("\n")
	b.WriteString(styles.HelpStyle.Render("space: toggle • enter: confirm • esc: back"))

	return b.String()
}

func RenderUninstallComponents(selected []model.ComponentID, cursor int) string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Uninstall Managed Components"))
	b.WriteString("\n\n")
	b.WriteString(styles.HelpStyle.Render("Use j/k to move, space to toggle, enter to continue."))
	b.WriteString("\n\n")
	b.WriteString(styles.SubtextStyle.Render("Select which gentle-ai managed components should be removed from the selected agents."))
	b.WriteString("\n\n")

	selectedSet := make(map[model.ComponentID]struct{}, len(selected))
	for _, component := range selected {
		selectedSet[component] = struct{}{}
	}

	for idx, component := range UninstallComponentOptions() {
		_, checked := selectedSet[component.ID]
		focused := idx == cursor
		b.WriteString(renderCheckbox(component.Name, checked, focused))
		b.WriteString(styles.SubtextStyle.Render("    "+component.Description) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(renderOptions([]string{"Continue", "Back"}, cursor-len(UninstallComponentOptions())))
	b.WriteString("\n")
	b.WriteString(styles.HelpStyle.Render("space: toggle • enter: continue • esc: back"))

	return b.String()
}

func RenderUninstallConfirm(selected []model.AgentID, components []model.ComponentID, cursor int, operationRunning bool, spinnerFrame int) string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Confirm Uninstall"))
	b.WriteString("\n\n")

	if operationRunning {
		b.WriteString(styles.WarningStyle.Render(SpinnerChar(spinnerFrame) + "  Removing managed configuration..."))
		b.WriteString("\n\n")
		b.WriteString(styles.HelpStyle.Render("Please wait..."))
		return b.String()
	}

	if len(selected) == 0 {
		b.WriteString(styles.WarningStyle.Render("No agents selected."))
		b.WriteString("\n\n")
		b.WriteString(styles.HelpStyle.Render("enter: back • esc: back"))
		return b.String()
	}

	b.WriteString(styles.SubtextStyle.Render("Agents:"))
	b.WriteString("\n")
	for _, label := range uninstallAgentLabels(selected) {
		b.WriteString(styles.UnselectedStyle.Render("  • " + label))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(styles.SubtextStyle.Render("Components:"))
	b.WriteString("\n")
	for _, label := range uninstallComponentLabels(components) {
		b.WriteString(styles.UnselectedStyle.Render("  • " + label))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(styles.WarningStyle.Render("This removes only gentle-ai managed content and creates a backup snapshot first."))
	b.WriteString("\n\n")
	b.WriteString(renderOptions([]string{"Uninstall", "Cancel"}, cursor))
	b.WriteString("\n")
	b.WriteString(styles.HelpStyle.Render("j/k: navigate • enter: select • esc: back"))

	return b.String()
}

func RenderUninstallResult(result componentuninstall.Result, err error) string {
	var b strings.Builder

	b.WriteString(styles.TitleStyle.Render("Uninstall Result"))
	b.WriteString("\n\n")

	if err != nil {
		b.WriteString(styles.ErrorStyle.Render("✗ Uninstall failed"))
		b.WriteString("\n\n")
		b.WriteString(styles.HeadingStyle.Render("Error:"))
		b.WriteString("\n")
		b.WriteString(styles.ErrorStyle.Render("  " + err.Error()))
		b.WriteString("\n\n")
		if result.Manifest.ID != "" {
			b.WriteString(styles.SubtextStyle.Render("Backup created before failure: "))
			b.WriteString(styles.SelectedStyle.Render(result.Manifest.ID))
			b.WriteString("\n")
			b.WriteString(styles.SubtextStyle.Render(result.Manifest.DisplayLabel()))
		}
	} else {
		b.WriteString(styles.SuccessStyle.Render("✓ Uninstall complete"))
		b.WriteString("\n\n")
		if result.Manifest.ID != "" {
			b.WriteString(styles.SubtextStyle.Render("Backup: "))
			b.WriteString(styles.SelectedStyle.Render(result.Manifest.ID))
			b.WriteString("\n")
			b.WriteString(styles.SubtextStyle.Render(result.Manifest.DisplayLabel()))
			b.WriteString("\n\n")
		}
		b.WriteString(styles.UnselectedStyle.Render(fmt.Sprintf("Rewritten files: %d", len(result.ChangedFiles))))
		b.WriteString("\n")
		b.WriteString(styles.UnselectedStyle.Render(fmt.Sprintf("Deleted files: %d", len(result.RemovedFiles))))
		b.WriteString("\n")
		b.WriteString(styles.UnselectedStyle.Render(fmt.Sprintf("Deleted directories: %d", len(result.RemovedDirectories))))
		if len(result.AgentsRemovedFromState) > 0 {
			b.WriteString("\n")
			b.WriteString(styles.UnselectedStyle.Render("Updated state.json: " + strings.Join(uninstallAgentLabels(result.AgentsRemovedFromState), ", ")))
		}
	}

	b.WriteString("\n\n")
	b.WriteString(styles.HelpStyle.Render("enter: return • esc: back • q: quit"))
	return b.String()
}

func uninstallAgentLabels(agentIDs []model.AgentID) []string {
	labels := make([]string, 0, len(agentIDs))
	for _, selected := range agentIDs {
		labels = append(labels, uninstallAgentLabel(selected))
	}
	return labels
}

func uninstallAgentLabel(agentID model.AgentID) string {
	for _, agent := range UninstallAgentOptions() {
		if agent.ID == agentID {
			return agent.Name
		}
	}
	return string(agentID)
}

func uninstallComponentLabels(componentIDs []model.ComponentID) []string {
	labels := make([]string, 0, len(componentIDs))
	for _, selected := range componentIDs {
		labels = append(labels, uninstallComponentLabel(selected))
	}
	return labels
}

func uninstallComponentLabel(componentID model.ComponentID) string {
	for _, component := range UninstallComponentOptions() {
		if component.ID == componentID {
			return component.Name
		}
	}
	return string(componentID)
}
