package theme

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestTaskStateBadgeUsesSemanticAccentWithoutChangingState(t *testing.T) {
	t.Cleanup(func() { ApplyTheme(ThemeConfig{Name: "dark"}) })
	for _, themeName := range []string{"dark", "light"} {
		ApplyTheme(ThemeConfig{Name: themeName})
		for _, entry := range models.SemanticTaskTags {
			for _, state := range models.AllWorkflowStates() {
				t.Run(themeName+"/"+entry.Name+"/"+state.String(), func(t *testing.T) {
					task := &models.Task{WorkflowState: state, Tags: []string{entry.Name}}
					base := stateBadgeStyle(state)
					want := base.Foreground(SemanticTaskTitleStyle(entry.Role, 0, false, false).GetForeground())
					got := taskStateBadgeStyle(task)
					assertStylesEqual(t, got, want)
					if got.GetBackground() != base.GetBackground() {
						t.Fatal("semantic accent changed the workflow badge background")
					}
					badge := GetTaskStateBadge(task)
					if badge != want.Render(state.Icon()) || ansi.Strip(badge) != ansi.Strip(GetStateBadge(state)) {
						t.Fatalf("badge %q changed the workflow symbol or lost its semantic accent", badge)
					}
					if lipgloss.Width(badge) != lipgloss.Width(GetStateBadge(state)) {
						t.Fatal("semantic accent changed the workflow badge width")
					}
				})
			}
		}
	}
}

func TestTaskStateBadgeResolvesSemanticPrecedence(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: "dark"})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{Name: "dark"}) })
	for i, entry := range models.SemanticTaskTags {
		// Reverse tag order so storage order cannot masquerade as precedence.
		tags := []string{"ordinary"}
		for j := len(models.SemanticTaskTags) - 1; j >= i; j-- {
			tags = append(tags, models.SemanticTaskTags[j].Name)
		}
		task := &models.Task{WorkflowState: models.StateBacklog, Tags: tags}
		want := SemanticTaskTitleStyle(entry.Role, 0, false, false).GetForeground()
		if got := taskStateBadgeStyle(task).GetForeground(); got != want {
			t.Errorf("tags %v foreground = %v, want %s's %v", tags, got, entry.Name, want)
		}
	}
	task := &models.Task{WorkflowState: models.StateBacklog, Tags: []string{"feature", "BUG", "bug"}}
	if got := taskStateBadgeStyle(task).GetForeground(); got != ColorSemanticBugTitle {
		t.Errorf("mixed-case semantic tags foreground = %v, want bug's %v", got, ColorSemanticBugTitle)
	}
}

func TestTaskStateBadgeWithoutSemanticTagsKeepsWorkflowColors(t *testing.T) {
	t.Cleanup(func() { ApplyTheme(ThemeConfig{Name: "dark"}) })
	for _, themeName := range []string{"dark", "light"} {
		ApplyTheme(ThemeConfig{Name: themeName})
		for _, state := range models.AllWorkflowStates() {
			for _, tags := range [][]string{nil, {"ordinary", "type:bug", "bugfix", " idea ", "#bug"}} {
				task := &models.Task{WorkflowState: state, Tags: tags}
				if got, want := GetTaskStateBadge(task), GetStateBadge(state); got != want {
					t.Errorf("%s/%s tags %v badge = %q, want %q", themeName, state, tags, got, want)
				}
			}
		}
	}
}
