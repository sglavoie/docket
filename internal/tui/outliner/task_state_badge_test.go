package outliner

import (
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// The task's state glyph carries its strongest semantic tag's accent.
func TestTaskRowStatusIconUsesSemanticAccent(t *testing.T) {
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{Name: "dark"}) })
	for _, themeName := range []string{"dark", "light"} {
		theme.ApplyTheme(theme.ThemeConfig{Name: themeName})
		for _, entry := range models.SemanticTaskTags {
			task := &models.Task{ID: "task", Title: "Task title", WorkflowState: models.StateBacklog, Tags: []string{entry.Name}}
			outliner := &OutlinerModel{}
			for _, selected := range []bool{false, true} {
				segments := outliner.buildTaskRowSegments(OutlinerRow{Task: task}, 60, nil, outliner.cursorPaintFor(selected))
				want := theme.TodoBadgeStyle.Foreground(theme.SemanticTaskTitleStyle(entry.Role, 0, false, false).GetForeground()).Render(task.State().Icon())
				if segments.statusIcon != want {
					t.Errorf("%s/%s selected=%v status icon = %q, want %q", themeName, entry.Name, selected, segments.statusIcon, want)
				}
				if got := ansi.Strip(segments.statusIcon); got != task.State().Icon() {
					t.Errorf("status symbol = %q, want %q", got, task.State().Icon())
				}
			}
		}
	}
}
