package tui

import (
	"image/color"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func TestSemanticTagTitleStylesUseExactCaseInsensitivePrecedence(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: "dark"})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{Name: "dark"}) })

	tests := []struct {
		name string
		tags []string
		want color.Color
	}{
		{name: "bug", tags: []string{"BUG"}, want: lipgloss.Color(theme.DefaultColorSemanticBugTitle)},
		{name: "question", tags: []string{"QuEsTiOn"}, want: lipgloss.Color(theme.DefaultColorSemanticQuestionTitle)},
		{name: "epic", tags: []string{"EPIC"}, want: lipgloss.Color(theme.DefaultColorSemanticEpicTitle)},
		{name: "story", tags: []string{"Story"}, want: lipgloss.Color(theme.DefaultColorSemanticStoryTitle)},
		{name: "feature", tags: []string{"Feature"}, want: lipgloss.Color(theme.DefaultColorSemanticFeatureTitle)},
		{name: "idea", tags: []string{"iDeA"}, want: lipgloss.Color(theme.DefaultColorSemanticIdeaTitle)},
		{name: "bug precedence", tags: []string{"idea", "feature", "story", "epic", "question", "bug"}, want: lipgloss.Color(theme.DefaultColorSemanticBugTitle)},
		{name: "question precedence", tags: []string{"idea", "feature", "story", "epic", "question"}, want: lipgloss.Color(theme.DefaultColorSemanticQuestionTitle)},
		{name: "epic precedence", tags: []string{"idea", "feature", "story", "epic"}, want: lipgloss.Color(theme.DefaultColorSemanticEpicTitle)},
		{name: "story precedence", tags: []string{"idea", "feature", "story"}, want: lipgloss.Color(theme.DefaultColorSemanticStoryTitle)},
		{name: "feature precedence", tags: []string{"idea", "feature"}, want: lipgloss.Color(theme.DefaultColorSemanticFeatureTitle)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &models.Task{WorkflowState: models.StateBacklog, Priority: models.PriorityMedium, Tags: tt.tags}
			if got := theme.TaskTitleStyleFor(task, theme.TaskTitleStyleOptions{}).GetForeground(); got != tt.want {
				t.Fatalf("foreground = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIdeaTagColorsOtherwiseUncoloredTaskTitles(t *testing.T) {
	for _, themeName := range []string{"dark", "light"} {
		t.Run(themeName, func(t *testing.T) {
			theme.ApplyTheme(theme.ThemeConfig{Name: themeName})
			t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{Name: "dark"}) })

			plain := &models.Task{WorkflowState: models.StateBacklog}
			idea := &models.Task{WorkflowState: models.StateBacklog, Tags: []string{"idea"}}
			plainForeground := theme.TaskTitleStyleFor(plain, theme.TaskTitleStyleOptions{}).GetForeground()
			ideaForeground := theme.TaskTitleStyleFor(idea, theme.TaskTitleStyleOptions{}).GetForeground()
			if ideaForeground == plainForeground {
				t.Fatalf("idea foreground = ordinary task foreground %v", plainForeground)
			}
		})
	}
}

func TestSemanticTagTitleStylesIgnoreFormerMarkersAndPartialMatches(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{Name: "dark"})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{Name: "dark"}) })

	for _, tags := range [][]string{
		{"type:bug"}, {"type:epic"}, {"storytelling"}, {"bugfix"}, {"feature:ui"}, {" idea "}, {" epic "},
	} {
		task := &models.Task{WorkflowState: models.StateBacklog, Priority: models.PriorityHigh, Tags: tags}
		got := theme.TaskTitleStyleFor(task, theme.TaskTitleStyleOptions{})
		assertStylesEqual(t, got, theme.GetTitlePriorityStyle(models.PriorityHigh))
	}
}

func TestSemanticTagTitleStylesCoverThemeSelectedAndDeferredVariants(t *testing.T) {
	tomorrow := time.Now().AddDate(0, 0, 1)
	themes := []struct {
		name  string
		roles []struct {
			tag      string
			base     string
			selected string
			deferred string
		}
	}{
		{name: "dark", roles: []struct {
			tag      string
			base     string
			selected string
			deferred string
		}{
			{tag: "bug", base: "#ff4f58", selected: "#ff9ea3", deferred: "#c47074"},
			{tag: "question", base: "#f57a00", selected: "#ffa366", deferred: "#d66e29"},
			{tag: "epic", base: "#e17bff", selected: "#eeb6ff", deferred: "#b16fc4"},
			{tag: "story", base: "#3fc5ff", selected: "#95dfff", deferred: "#4891b0"},
			{tag: "feature", base: "#35d67f", selected: "#90e8b9", deferred: "#549572"},
			{tag: "idea", base: "#ffd23f", selected: "#ffe695", deferred: "#9c8740"},
		}},
		{name: "light", roles: []struct {
			tag      string
			base     string
			selected string
			deferred string
		}{
			{tag: "bug", base: "#cc2222", selected: "#8f1818", deferred: "#b06c6c"},
			{tag: "question", base: "#c25a00", selected: "#5d3228", deferred: "#c76c38"},
			{tag: "epic", base: "#9d1fb8", selected: "#6e1681", deferred: "#a26aaf"},
			{tag: "story", base: "#0b6ba8", selected: "#084b76", deferred: "#4d84a7"},
			{tag: "feature", base: "#067a3f", selected: "#04552c", deferred: "#3f8d66"},
			{tag: "idea", base: "#8f6b00", selected: "#644b00", deferred: "#927d3c"},
		}},
	}
	for _, variant := range themes {
		t.Run(variant.name, func(t *testing.T) {
			theme.ApplyTheme(theme.ThemeConfig{Name: variant.name})
			t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{Name: "dark"}) })

			for _, role := range variant.roles {
				t.Run(role.tag, func(t *testing.T) {
					base := &models.Task{WorkflowState: models.StateBacklog, Tags: []string{role.tag}}
					if got := theme.TaskTitleStyleFor(base, theme.TaskTitleStyleOptions{}).GetForeground(); got != lipgloss.Color(role.base) {
						t.Errorf("base foreground = %v, want %s", got, role.base)
					}
					selected := theme.TaskTitleStyleFor(base, theme.TaskTitleStyleOptions{CursorSelected: true})
					if got := selected.GetForeground(); got != lipgloss.Color(role.selected) || !selected.GetBold() {
						t.Errorf("selected style foreground/bold = %v/%v, want %s/true", got, selected.GetBold(), role.selected)
					}
					base.DeferredUntil = &tomorrow
					deferred := theme.TaskTitleStyleFor(base, theme.TaskTitleStyleOptions{})
					if got := deferred.GetForeground(); got != lipgloss.Color(role.deferred) || !deferred.GetFaint() {
						t.Errorf("deferred style foreground/faint = %v/%v, want %s/true", got, deferred.GetFaint(), role.deferred)
					}
				})
			}
		})
	}
}
