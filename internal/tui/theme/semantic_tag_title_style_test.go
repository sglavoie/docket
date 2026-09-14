package theme

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// TestSemanticTaskTitleStyleResolvesOneRoleAtATime holds what the accent is now
// for: one member of the closed six-name vocabulary, resolved by role. It takes
// a role rather than a task's tag list because no task title wears one — the
// `SEMANTIC` picker rows are its one consumer, and a row names exactly one tag.
func TestSemanticTaskTitleStyleResolvesOneRoleAtATime(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: "dark"})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{Name: "dark"}) })

	tests := []struct {
		role models.SemanticTaskTagRole
		want color.Color
	}{
		{models.SemanticTaskTagBug, lipgloss.Color(defaultColorSemanticBugTitle)},
		{models.SemanticTaskTagQuestion, lipgloss.Color(defaultColorSemanticQuestionTitle)},
		{models.SemanticTaskTagEpic, lipgloss.Color(defaultColorSemanticEpicTitle)},
		{models.SemanticTaskTagStory, lipgloss.Color(defaultColorSemanticStoryTitle)},
		{models.SemanticTaskTagFeature, lipgloss.Color(defaultColorSemanticFeatureTitle)},
		{models.SemanticTaskTagIdea, lipgloss.Color(defaultColorSemanticIdeaTitle)},
	}
	for _, tt := range tests {
		if got := SemanticTaskTitleStyle(tt.role, 0, false, false).GetForeground(); got != tt.want {
			t.Errorf("role %v foreground = %v, want %v", tt.role, got, tt.want)
		}
	}

	// The absent role adds no color of its own, so a non-semantic row reads as the
	// list's ordinary text rather than as a seventh member.
	if got := SemanticTaskTitleStyle(models.SemanticTaskTagNone, 0, false, false); got.GetForeground() != NormalStyle.GetForeground() {
		t.Errorf("SemanticTaskTagNone foreground = %v, want the ordinary %v", got.GetForeground(), NormalStyle.GetForeground())
	}
}

// TestTaskTitleStyleForIgnoresSemanticTags is the rule the accent no longer
// reaches. A semantic tag says what kind of work a task is, which the chip
// beside the title carries; the title itself stays ordinary text whatever the
// tag list holds, in every theme.
func TestTaskTitleStyleForIgnoresSemanticTags(t *testing.T) {
	for _, themeName := range []string{"dark", "light"} {
		t.Run(themeName, func(t *testing.T) {
			ApplyTheme(ThemeConfig{Name: themeName})
			t.Cleanup(func() { ApplyTheme(ThemeConfig{Name: "dark"}) })

			plain := TaskTitleStyleFor(&models.Task{WorkflowState: models.StateBacklog}, TaskTitleStyleOptions{})
			for _, tags := range [][]string{
				{"bug"}, {"QuEsTiOn"}, {"epic"}, {"Story"}, {"feature"}, {"iDeA"},
				{"idea", "feature", "story", "epic", "question", "bug"},
				{"type:bug"}, {"bugfix"}, {" idea "},
			} {
				task := &models.Task{WorkflowState: models.StateBacklog, Priority: models.PriorityHigh, Tags: tags}
				assertStylesEqual(t, TaskTitleStyleFor(task, TaskTitleStyleOptions{}), plain)
			}
		})
	}
}

// TestSemanticTaskTitleStyleCoversThemeSelectedAndDeferredVariants keeps every
// palette entry the vocabulary publishes under test. The accent is a published
// surface: a picker row can be focused or point at deferred work, so all three
// variants stay reachable in both themes.
func TestSemanticTaskTitleStyleCoversThemeSelectedAndDeferredVariants(t *testing.T) {
	type roleColors struct {
		role     models.SemanticTaskTagRole
		base     string
		selected string
		deferred string
	}
	themes := []struct {
		name  string
		roles []roleColors
	}{
		{name: "dark", roles: []roleColors{
			{role: models.SemanticTaskTagBug, base: "#ff4f58", selected: "#ff9ea3", deferred: "#c47074"},
			{role: models.SemanticTaskTagQuestion, base: "#f57a00", selected: "#ffa366", deferred: "#d66e29"},
			{role: models.SemanticTaskTagEpic, base: "#e17bff", selected: "#eeb6ff", deferred: "#b16fc4"},
			{role: models.SemanticTaskTagStory, base: "#3fc5ff", selected: "#95dfff", deferred: "#4891b0"},
			{role: models.SemanticTaskTagFeature, base: "#35d67f", selected: "#90e8b9", deferred: "#549572"},
			{role: models.SemanticTaskTagIdea, base: "#ffd23f", selected: "#ffe695", deferred: "#9c8740"},
		}},
		{name: "light", roles: []roleColors{
			{role: models.SemanticTaskTagBug, base: "#cc2222", selected: "#8f1818", deferred: "#b06c6c"},
			{role: models.SemanticTaskTagQuestion, base: "#c25a00", selected: "#5d3228", deferred: "#c76c38"},
			{role: models.SemanticTaskTagEpic, base: "#9d1fb8", selected: "#6e1681", deferred: "#a26aaf"},
			{role: models.SemanticTaskTagStory, base: "#0b6ba8", selected: "#084b76", deferred: "#4d84a7"},
			{role: models.SemanticTaskTagFeature, base: "#067a3f", selected: "#04552c", deferred: "#3f8d66"},
			{role: models.SemanticTaskTagIdea, base: "#8f6b00", selected: "#644b00", deferred: "#927d3c"},
		}},
	}
	for _, variant := range themes {
		t.Run(variant.name, func(t *testing.T) {
			ApplyTheme(ThemeConfig{Name: variant.name})
			t.Cleanup(func() { ApplyTheme(ThemeConfig{Name: "dark"}) })

			for _, role := range variant.roles {
				if got := SemanticTaskTitleStyle(role.role, 0, false, false).GetForeground(); got != lipgloss.Color(role.base) {
					t.Errorf("role %v base foreground = %v, want %s", role.role, got, role.base)
				}
				selected := SemanticTaskTitleStyle(role.role, 0, true, false)
				if got := selected.GetForeground(); got != lipgloss.Color(role.selected) || !selected.GetBold() {
					t.Errorf("role %v selected foreground/bold = %v/%v, want %s/true", role.role, got, selected.GetBold(), role.selected)
				}
				deferred := SemanticTaskTitleStyle(role.role, 0, false, true)
				if got := deferred.GetForeground(); got != lipgloss.Color(role.deferred) || !deferred.GetFaint() {
					t.Errorf("role %v deferred foreground/faint = %v/%v, want %s/true", role.role, got, deferred.GetFaint(), role.deferred)
				}
			}
		})
	}
}
