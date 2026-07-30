package tui

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

func TestGetTitlePriorityStyle(t *testing.T) {
	// Ensure styles are built from defaults.
	ApplyTheme(ThemeConfig{})

	tests := []struct {
		priority    int
		wantBold    bool
		wantItalic  bool
		wantFaint   bool
		wantColorFg color.Color
	}{
		{1, true, true, false, lipgloss.Color(defaultColorPriority1Title)},
		{2, true, false, false, lipgloss.Color(defaultColorPriority2Title)},
		{3, false, false, false, lipgloss.Color(defaultColorPriority3Title)},
		{4, false, false, false, lipgloss.Color(defaultColorPriority4Title)},
		{5, false, false, false, lipgloss.Color(defaultColorPriority5Title)},
	}

	for _, tc := range tests {
		s := GetTitlePriorityStyle(tc.priority)
		if s.GetBold() != tc.wantBold {
			t.Errorf("priority %d: Bold = %v, want %v", tc.priority, s.GetBold(), tc.wantBold)
		}
		if s.GetItalic() != tc.wantItalic {
			t.Errorf("priority %d: Italic = %v, want %v", tc.priority, s.GetItalic(), tc.wantItalic)
		}
		if s.GetFaint() != tc.wantFaint {
			t.Errorf("priority %d: Faint = %v, want %v", tc.priority, s.GetFaint(), tc.wantFaint)
		}
		if s.GetForeground() != tc.wantColorFg {
			t.Errorf("priority %d: Foreground = %v, want %v", tc.priority, s.GetForeground(), tc.wantColorFg)
		}
	}
}

func TestGetTitlePriorityStyleOutOfRange(t *testing.T) {
	ApplyTheme(ThemeConfig{})

	p3 := GetTitlePriorityStyle(3)
	for _, priority := range []int{0, 6, -1, 99} {
		s := GetTitlePriorityStyle(priority)
		if s.GetBold() != p3.GetBold() || s.GetItalic() != p3.GetItalic() ||
			s.GetFaint() != p3.GetFaint() || s.GetForeground() != p3.GetForeground() {
			t.Errorf("priority %d: expected P3 style, got different style", priority)
		}
	}
}

func TestRenderCursorPrefixDoesNotShowPinnedMarker(t *testing.T) {
	if got, want := RenderCursorPrefix(), "   "; got != want {
		t.Fatalf("RenderCursorPrefix() = %q, want fixed-width blank gutter %q", got, want)
	}
}

func TestTaskTitleStyleFor(t *testing.T) {
	ApplyTheme(ThemeConfig{})

	taskWith := func(priority int) *models.Task {
		return &models.Task{
			Title:         "Task",
			WorkflowState: models.StateReady,
			Priority:      priority,
		}
	}

	tests := []struct {
		name string
		task *models.Task
		opts TaskTitleStyleOptions
		want lipgloss.Style
	}{
		{"priority 1", taskWith(1), TaskTitleStyleOptions{}, GetTitlePriorityStyle(1)},
		{"priority 2", taskWith(2), TaskTitleStyleOptions{}, GetTitlePriorityStyle(2)},
		{"priority 3", taskWith(3), TaskTitleStyleOptions{}, GetTitlePriorityStyle(3)},
		{"priority 4", taskWith(4), TaskTitleStyleOptions{}, GetTitlePriorityStyle(4)},
		{"priority 5", taskWith(5), TaskTitleStyleOptions{}, GetTitlePriorityStyle(5)},
		{"plain task keeps priority title style", func() *models.Task {
			task := taskWith(1)
			return task
		}(), TaskTitleStyleOptions{}, GetTitlePriorityStyle(1)},
		{"pinned keeps priority title style", func() *models.Task {
			task := taskWith(2)
			task.Pinned = true
			return task
		}(), TaskTitleStyleOptions{}, GetTitlePriorityStyle(2)},
		{"bulk selected keeps priority title style", taskWith(1), TaskTitleStyleOptions{BulkSelected: true}, GetTitlePriorityStyle(1)},
		{"cursor selected uses selected title style", taskWith(4), TaskTitleStyleOptions{CursorSelected: true}, SelectedTaskTitleStyle},
		{"canceled uses canceled style", func() *models.Task {
			task := taskWith(1)
			task.WorkflowState = models.StateCanceled
			return task
		}(), TaskTitleStyleOptions{}, CanceledStyle},
		{"canceled suppresses selected title style", func() *models.Task {
			task := taskWith(1)
			task.WorkflowState = models.StateCanceled
			return task
		}(), TaskTitleStyleOptions{CursorSelected: true}, CanceledStyle},
		{"context muted suppresses selected title style", taskWith(2), TaskTitleStyleOptions{CursorSelected: true, Muted: true}, MutedStyle},
		{"disabled suppresses selected title style", taskWith(3), TaskTitleStyleOptions{CursorSelected: true, Disabled: true}, MutedStyle},
		{"nil task fallback", nil, TaskTitleStyleOptions{}, NormalStyle},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := TaskTitleStyleFor(tc.task, tc.opts)
			assertStylesEqual(t, got, tc.want)
		})
	}
}

func assertStylesEqual(t *testing.T, got, want lipgloss.Style) {
	t.Helper()

	if got.GetBold() != want.GetBold() {
		t.Errorf("Bold = %v, want %v", got.GetBold(), want.GetBold())
	}
	if got.GetItalic() != want.GetItalic() {
		t.Errorf("Italic = %v, want %v", got.GetItalic(), want.GetItalic())
	}
	if got.GetFaint() != want.GetFaint() {
		t.Errorf("Faint = %v, want %v", got.GetFaint(), want.GetFaint())
	}
	if got.GetStrikethrough() != want.GetStrikethrough() {
		t.Errorf("Strikethrough = %v, want %v", got.GetStrikethrough(), want.GetStrikethrough())
	}
	if got.GetForeground() != want.GetForeground() {
		t.Errorf("Foreground = %v, want %v", got.GetForeground(), want.GetForeground())
	}
}
