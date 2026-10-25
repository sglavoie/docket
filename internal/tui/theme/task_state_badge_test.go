package theme

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
)

func TestTaskStateBadgeUsesSemanticAccentWithoutChangingState(t *testing.T) {
	t.Cleanup(func() { ApplyTheme(ThemeConfig{Name: "dark"}) })
	lightAccents := map[models.SemanticTaskTagRole]string{
		models.SemanticTaskTagBug:      "#f02318",
		models.SemanticTaskTagQuestion: "#e06400",
		models.SemanticTaskTagEpic:     "#a020d0",
		models.SemanticTaskTagStory:    "#087de0",
		models.SemanticTaskTagFeature:  "#00a632",
		models.SemanticTaskTagIdea:     "#bc8e00",
	}
	for _, themeName := range []string{"dark", "light"} {
		ApplyTheme(ThemeConfig{Name: themeName})
		for _, entry := range models.SemanticTaskTags {
			for _, state := range models.AllWorkflowStates() {
				t.Run(themeName+"/"+entry.Name+"/"+state.String(), func(t *testing.T) {
					task := &models.Task{WorkflowState: state, Tags: []string{entry.Name}}
					base := stateBadgeStyle(state)
					want := base.Foreground(SemanticTaskTitleStyle(entry.Role, 0, false, false).GetForeground())
					if themeName == "light" {
						want = want.UnsetBackground().Foreground(lipgloss.Color(lightAccents[entry.Role])).Bold(true)
					}
					got := taskStateBadgeStyle(task)
					assertStylesEqual(t, got, want)
					if themeName == "dark" && got.GetBackground() != base.GetBackground() {
						t.Fatal("semantic accent changed the workflow badge background")
					}
					badge := GetTaskStateBadge(task)
					icon := state.Icon()
					if state == models.StateBacklog {
						icon = "🞑"
					}
					if badge != want.Render(icon) || ansi.Strip(badge) != ansi.Strip(GetStateBadge(state)) {
						t.Fatalf("badge %q changed the workflow symbol or lost its semantic accent", badge)
					}
					if lipgloss.Width(badge) != lipgloss.Width(state.Icon()) {
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
