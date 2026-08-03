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
		{name: "feature", tags: []string{"Feature"}, want: lipgloss.Color(defaultColorSemanticFeatureTitle)},
		{name: "idea", tags: []string{"iDeA"}, want: lipgloss.Color(defaultColorSemanticIdeaTitle)},
		{name: "bug precedence", tags: []string{"idea", "feature", "bug"}, want: lipgloss.Color(defaultColorSemanticBugTitle)},
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
		{"epic"}, {"story"}, {"type:bug"}, {"bugfix"}, {"feature:ui"}, {" idea "},
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
