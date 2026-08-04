package tui

import (
	"image/color"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

func TestSemanticTagTitleStylesUseExactCaseInsensitivePrecedence(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: "dark"})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{Name: "dark"}) })

	tests := []struct {
		name string
		tags []string
		want color.Color
	}{
		{name: "bug", tags: []string{"BUG"}, want: lipgloss.Color(defaultColorSemanticBugTitle)},
		{name: "epic", tags: []string{"EPIC"}, want: lipgloss.Color(defaultColorSemanticEpicTitle)},
		{name: "story", tags: []string{"Story"}, want: lipgloss.Color(defaultColorSemanticStoryTitle)},
		{name: "feature", tags: []string{"Feature"}, want: lipgloss.Color(defaultColorSemanticFeatureTitle)},
		{name: "idea", tags: []string{"iDeA"}, want: lipgloss.Color(defaultColorSemanticIdeaTitle)},
		{name: "bug precedence", tags: []string{"idea", "feature", "story", "epic", "bug"}, want: lipgloss.Color(defaultColorSemanticBugTitle)},
		{name: "epic precedence", tags: []string{"idea", "feature", "story", "epic"}, want: lipgloss.Color(defaultColorSemanticEpicTitle)},
		{name: "story precedence", tags: []string{"idea", "feature", "story"}, want: lipgloss.Color(defaultColorSemanticStoryTitle)},
		{name: "feature precedence", tags: []string{"idea", "feature"}, want: lipgloss.Color(defaultColorSemanticFeatureTitle)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &models.Task{WorkflowState: models.StateBacklog, Priority: models.PriorityMedium, Tags: tt.tags}
			if got := TaskTitleStyleFor(task, TaskTitleStyleOptions{}).GetForeground(); got != tt.want {
				t.Fatalf("foreground = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSemanticTagTitleStylesIgnoreFormerMarkersAndPartialMatches(t *testing.T) {
	ApplyTheme(ThemeConfig{Name: "dark"})
	t.Cleanup(func() { ApplyTheme(ThemeConfig{Name: "dark"}) })

	for _, tags := range [][]string{
		{"type:bug"}, {"type:epic"}, {"storytelling"}, {"bugfix"}, {"feature:ui"}, {" idea "}, {" epic "},
	} {
		task := &models.Task{WorkflowState: models.StateBacklog, Priority: models.PriorityHigh, Tags: tags}
		got := TaskTitleStyleFor(task, TaskTitleStyleOptions{})
		assertStylesEqual(t, got, GetTitlePriorityStyle(models.PriorityHigh))
	}
}

func TestSemanticTagTitleStylesCoverThemeSelectedAndSnoozedVariants(t *testing.T) {
	tomorrow := time.Now().AddDate(0, 0, 1)
	themes := []struct {
		name  string
		roles []struct {
			tag      string
			base     string
			selected string
			snoozed  string
		}
	}{
		{name: "dark", roles: []struct {
			tag      string
			base     string
			selected string
			snoozed  string
		}{
			{tag: "bug", base: "#ff9e9e", selected: "#ffd1d1", snoozed: "#c97b7b"},
			{tag: "epic", base: "#bb9af7", selected: "#dccbff", snoozed: "#8d73bd"},
			{tag: "story", base: "#7dcfff", selected: "#b6e5ff", snoozed: "#5f9bbf"},
			{tag: "feature", base: "#b9f27c", selected: "#d7f7b5", snoozed: "#8bb65f"},
			{tag: "idea", base: "#c0caf5", selected: "#e5e7eb", snoozed: "#8f98c7"},
		}},
		{name: "light", roles: []struct {
			tag      string
			base     string
			selected string
			snoozed  string
		}{
			{tag: "bug", base: "#b42318", selected: "#7f1d1d", snoozed: "#9f5656"},
			{tag: "epic", base: "#6d28d9", selected: "#4c1d95", snoozed: "#8b6bb7"},
			{tag: "story", base: "#0e7490", selected: "#155e75", snoozed: "#5f8793"},
			{tag: "feature", base: "#237a3b", selected: "#14532d", snoozed: "#5f8f6c"},
			{tag: "idea", base: "#5f6b7a", selected: "#374151", snoozed: "#7b8491"},
		}},
	}
	for _, theme := range themes {
		t.Run(theme.name, func(t *testing.T) {
			ApplyTheme(ThemeConfig{Name: theme.name})
			t.Cleanup(func() { ApplyTheme(ThemeConfig{Name: "dark"}) })

			for _, role := range theme.roles {
				t.Run(role.tag, func(t *testing.T) {
					base := &models.Task{WorkflowState: models.StateBacklog, Tags: []string{role.tag}}
					if got := TaskTitleStyleFor(base, TaskTitleStyleOptions{}).GetForeground(); got != lipgloss.Color(role.base) {
						t.Errorf("base foreground = %v, want %s", got, role.base)
					}
					selected := TaskTitleStyleFor(base, TaskTitleStyleOptions{CursorSelected: true})
					if got := selected.GetForeground(); got != lipgloss.Color(role.selected) || !selected.GetBold() {
						t.Errorf("selected style foreground/bold = %v/%v, want %s/true", got, selected.GetBold(), role.selected)
					}
					base.SnoozedUntil = &tomorrow
					snoozed := TaskTitleStyleFor(base, TaskTitleStyleOptions{})
					if got := snoozed.GetForeground(); got != lipgloss.Color(role.snoozed) || !snoozed.GetFaint() {
						t.Errorf("snoozed style foreground/faint = %v/%v, want %s/true", got, snoozed.GetFaint(), role.snoozed)
					}
				})
			}
		})
	}
}
