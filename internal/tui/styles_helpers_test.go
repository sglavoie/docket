package tui

import (
	"image/color"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func TestGetTitlePriorityStyle(t *testing.T) {
	// Ensure styles are built from defaults.
	theme.ApplyTheme(theme.ThemeConfig{})

	tests := []struct {
		priority    int
		wantBold    bool
		wantItalic  bool
		wantFaint   bool
		wantColorFg color.Color
	}{
		{1, true, true, false, lipgloss.Color(theme.DefaultColorPriority1Title)},
		{2, true, false, false, lipgloss.Color(theme.DefaultColorPriority2Title)},
		{3, false, false, false, lipgloss.Color(theme.DefaultColorPriority3Title)},
	}

	for _, tc := range tests {
		s := theme.GetTitlePriorityStyle(tc.priority)
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
	theme.ApplyTheme(theme.ThemeConfig{})

	normal := theme.NormalStyle
	for _, priority := range []int{0, 4, 5, 6, -1, 99} {
		s := theme.GetTitlePriorityStyle(priority)
		if s.GetBold() != normal.GetBold() || s.GetItalic() != normal.GetItalic() ||
			s.GetFaint() != normal.GetFaint() || s.GetForeground() != normal.GetForeground() {
			t.Errorf("priority %d: expected normal style, got different style", priority)
		}
	}
}

func TestTaskContextBadgeStyleUsesDeferredVariant(t *testing.T) {
	normal := theme.TaskContextBadgeStyle(&models.Task{})
	if !normal.GetBold() || normal.GetFaint() || normal.GetForeground() != theme.ColorTaskContext {
		t.Fatalf("normal context style bold/faint/foreground = %v/%v/%v", normal.GetBold(), normal.GetFaint(), normal.GetForeground())
	}

	future := time.Now().Add(time.Hour)
	deferred := theme.TaskContextBadgeStyle(&models.Task{DeferredUntil: &future})
	if !deferred.GetBold() || !deferred.GetFaint() || deferred.GetForeground() != theme.ColorTaskContextDeferred {
		t.Fatalf("deferred context style bold/faint/foreground = %v/%v/%v", deferred.GetBold(), deferred.GetFaint(), deferred.GetForeground())
	}
}

func TestPriorityIconUnsetAndInvalidAreBlank(t *testing.T) {
	for _, priority := range []int{0, 4, 5, -1, 99} {
		if got := theme.PriorityIcon(priority); got != "" {
			t.Errorf("PriorityIcon(%d) = %q, want blank", priority, got)
		}
	}
}

func TestRenderCursorPrefixUsesMinimalMargin(t *testing.T) {
	if got, want := theme.RenderCursorPrefix(), " "; got != want {
		t.Fatalf("RenderCursorPrefix() = %q, want minimal blank margin %q", got, want)
	}
}

func TestTaskTitleStyleFor(t *testing.T) {
	theme.ApplyTheme(theme.ThemeConfig{})

	taskWith := func(priority int) *models.Task {
		return &models.Task{
			Title:         "Task",
			WorkflowState: models.StateBacklog,
			Priority:      priority,
		}
	}

	tests := []struct {
		name string
		task *models.Task
		opts theme.TaskTitleStyleOptions
		want lipgloss.Style
	}{
		{"priority 1", taskWith(1), theme.TaskTitleStyleOptions{}, theme.GetTitlePriorityStyle(1)},
		{"priority 2", taskWith(2), theme.TaskTitleStyleOptions{}, theme.GetTitlePriorityStyle(2)},
		{"priority 3", taskWith(3), theme.TaskTitleStyleOptions{}, theme.GetTitlePriorityStyle(3)},
		{"priority unset", taskWith(0), theme.TaskTitleStyleOptions{}, theme.NormalStyle},
		{"plain task keeps priority title style", func() *models.Task {
			task := taskWith(1)
			return task
		}(), theme.TaskTitleStyleOptions{}, theme.GetTitlePriorityStyle(1)},
		{"pinned keeps priority title style", func() *models.Task {
			task := taskWith(2)
			task.Pinned = true
			return task
		}(), theme.TaskTitleStyleOptions{}, theme.GetTitlePriorityStyle(2)},
		{"cursor selected uses selected title style", taskWith(0), theme.TaskTitleStyleOptions{CursorSelected: true}, theme.SelectedTaskTitleStyle},
		{"canceled uses canceled style", func() *models.Task {
			task := taskWith(1)
			task.WorkflowState = models.StateCanceled
			return task
		}(), theme.TaskTitleStyleOptions{}, theme.CanceledStyle},
		{"canceled suppresses selected title style", func() *models.Task {
			task := taskWith(1)
			task.WorkflowState = models.StateCanceled
			return task
		}(), theme.TaskTitleStyleOptions{CursorSelected: true}, theme.CanceledStyle},
		{"context muted suppresses selected title style", taskWith(2), theme.TaskTitleStyleOptions{CursorSelected: true, Muted: true}, theme.MutedStyle},
		{"disabled suppresses selected title style", taskWith(3), theme.TaskTitleStyleOptions{CursorSelected: true, Disabled: true}, theme.MutedStyle},
		{"nil task fallback", nil, theme.TaskTitleStyleOptions{}, theme.NormalStyle},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := theme.TaskTitleStyleFor(tc.task, tc.opts)
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
