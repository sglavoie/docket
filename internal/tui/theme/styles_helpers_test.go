package theme

import (
	"image/color"
	"testing"
	"time"

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

	normal := NormalStyle
	for _, priority := range []int{0, 4, 5, 6, -1, 99} {
		s := GetTitlePriorityStyle(priority)
		if s.GetBold() != normal.GetBold() || s.GetItalic() != normal.GetItalic() ||
			s.GetFaint() != normal.GetFaint() || s.GetForeground() != normal.GetForeground() {
			t.Errorf("priority %d: expected normal style, got different style", priority)
		}
	}
}

func TestTaskContextBadgeStyleUsesDeferredVariant(t *testing.T) {
	normal := TaskContextBadgeStyle(&models.Task{})
	if !normal.GetBold() || normal.GetFaint() || normal.GetForeground() != ColorTaskContext {
		t.Fatalf("normal context style bold/faint/foreground = %v/%v/%v", normal.GetBold(), normal.GetFaint(), normal.GetForeground())
	}

	future := time.Now().Add(time.Hour)
	deferred := TaskContextBadgeStyle(&models.Task{DeferredUntil: &future})
	if !deferred.GetBold() || !deferred.GetFaint() || deferred.GetForeground() != ColorTaskContextDeferred {
		t.Fatalf("deferred context style bold/faint/foreground = %v/%v/%v", deferred.GetBold(), deferred.GetFaint(), deferred.GetForeground())
	}
}

func TestPriorityIconUnsetAndInvalidAreBlank(t *testing.T) {
	for _, priority := range []int{0, 4, 5, -1, 99} {
		if got := PriorityIcon(priority); got != "" {
			t.Errorf("PriorityIcon(%d) = %q, want blank", priority, got)
		}
	}
}

func TestRenderCursorPrefixUsesMinimalMargin(t *testing.T) {
	if got, want := RenderCursorPrefix(), " "; got != want {
		t.Fatalf("RenderCursorPrefix() = %q, want minimal blank margin %q", got, want)
	}
}

// TestTaskTitleStyleFor holds the neutral title: nothing about how loudly a task
// is asking reaches its title, so priority, a deadline, an owed review, a
// semantic tag, a pin, and a deferral all render in ordinary text. Only the
// three lifecycle statements — muted, canceled, cursor-selected — remain.
func TestTaskTitleStyleFor(t *testing.T) {
	ApplyTheme(ThemeConfig{})

	taskWith := func(priority int) *models.Task {
		return &models.Task{
			Title:         "Task",
			WorkflowState: models.StateBacklog,
			Priority:      priority,
		}
	}

	overdue := time.Now().AddDate(0, 0, -3)
	today := time.Now()
	tomorrow := time.Now().AddDate(0, 0, 1)

	tests := []struct {
		name string
		task *models.Task
		opts TaskTitleStyleOptions
		want lipgloss.Style
	}{
		{"priority 1", taskWith(1), TaskTitleStyleOptions{}, NormalStyle},
		{"priority 2", taskWith(2), TaskTitleStyleOptions{}, NormalStyle},
		{"priority 3", taskWith(3), TaskTitleStyleOptions{}, NormalStyle},
		{"priority unset", taskWith(0), TaskTitleStyleOptions{}, NormalStyle},
		{"pinned is neutral", func() *models.Task {
			task := taskWith(2)
			task.Pinned = true
			return task
		}(), TaskTitleStyleOptions{}, NormalStyle},
		{"semantic tag is neutral", func() *models.Task {
			task := taskWith(1)
			task.Tags = []string{"bug"}
			return task
		}(), TaskTitleStyleOptions{}, NormalStyle},
		{"overdue is neutral", func() *models.Task {
			task := taskWith(1)
			task.DueDate = &overdue
			return task
		}(), TaskTitleStyleOptions{}, NormalStyle},
		{"due today is neutral", func() *models.Task {
			task := taskWith(3)
			task.DueDate = &today
			return task
		}(), TaskTitleStyleOptions{}, NormalStyle},
		{"review flag is neutral", func() *models.Task {
			task := taskWith(0)
			task.Review = true
			return task
		}(), TaskTitleStyleOptions{}, NormalStyle},
		{"waiting is neutral", func() *models.Task {
			task := taskWith(0)
			task.WorkflowState = models.StateWaiting
			return task
		}(), TaskTitleStyleOptions{}, NormalStyle},
		{"deferred is neutral", func() *models.Task {
			task := taskWith(1)
			task.DeferredUntil = &tomorrow
			return task
		}(), TaskTitleStyleOptions{}, NormalStyle},
		{"done is neutral", func() *models.Task {
			task := taskWith(1)
			task.WorkflowState = models.StateDone
			return task
		}(), TaskTitleStyleOptions{}, NormalStyle},
		{"cursor selected uses selected title style", taskWith(0), TaskTitleStyleOptions{CursorSelected: true}, SelectedTaskTitleStyle},
		{"cursor selected outranks priority", taskWith(1), TaskTitleStyleOptions{CursorSelected: true}, SelectedTaskTitleStyle},
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
