package theme

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// A semantic tag reaches Peek as a chip and the SEMANTIC picker as its accent,
// never the state glyph: that glyph sits beside the priority badge, where a tag
// accent read as a second priority.
func TestTaskStateBadgeIgnoresSemanticTags(t *testing.T) {
	t.Cleanup(func() { ApplyTheme(ThemeConfig{Name: "dark"}) })
	for _, themeName := range []string{"dark", "light"} {
		ApplyTheme(ThemeConfig{Name: themeName})
		for _, entry := range models.SemanticTaskTags {
			for _, state := range models.AllWorkflowStates() {
				t.Run(themeName+"/"+entry.Name+"/"+state.String(), func(t *testing.T) {
					task := &models.Task{WorkflowState: state, Tags: []string{entry.Name}}
					assertStylesEqual(t, taskStateBadgeStyle(task), stateBadgeStyle(state))
					if got, want := GetTaskStateBadge(task), GetStateBadge(state); got != want {
						t.Fatalf("badge = %q, want the plain state badge %q", got, want)
					}
				})
			}
		}
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
