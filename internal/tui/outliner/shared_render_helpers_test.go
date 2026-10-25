package outliner

import (
	"image/color"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/notes"
	"github.com/sglavoie/docket/internal/tui/theme"
)

func guidanceHintLabels(spec chrome.GuidanceSpec) string {
	labels := make([]string, 0, len(spec.Hints))
	for _, hint := range spec.Hints {
		labels = append(labels, strings.TrimSpace(hint.Key+" "+hint.Label))
	}
	return strings.Join(labels, " | ")
}

func noteIndexRow(location app.NoteLocation, label, title, filename string) notes.LocatedNoteRow {
	return notes.LocatedNoteRow{
		Note:       &models.Note{Title: title, Filename: filename},
		Location:   location,
		OwnerLabel: label,
	}
}

// recentTask seeds a task whose last activity is dayOffset days in the past.
func recentTask(id string, state models.WorkflowState, dayOffset int) *models.Task {
	task := primaryRowsTask(id, state)
	task.UpdatedAt = time.Now().AddDate(0, 0, -dayOffset)
	return task
}

func renderFixtureProject(t *testing.T, id, slug, name string, tasks []*models.Task) *models.Project {
	t.Helper()
	project, err := models.NewProject(id, name, t.TempDir())
	if err != nil {
		t.Fatalf("NewProject(%q): %v", id, err)
	}
	project.Slug = slug
	project.Tasks = tasks
	project.BuildTaskTree()
	return project
}

func renderFixtureTask(
	t *testing.T,
	id string,
	shortID int,
	title string,
	status models.WorkflowState,
	priority int,
	section models.WorkflowState,
) *models.Task {
	t.Helper()
	task, err := models.NewGenericTask(id, shortID, title)
	if err != nil {
		t.Fatalf("NewTask(%q): %v", id, err)
	}
	task.WorkflowState = legacyStateForStatusSection(status, string(section))
	task.Priority = priority
	return task
}

// backgroundParams returns the SGR parameters lipgloss emits for a background
// color. A gutter piece merges its own foreground into one sequence, so a test
// asks for the parameters rather than a standalone escape.
func backgroundParams(t *testing.T, c color.Color) string {
	t.Helper()
	rendered := lipgloss.NewStyle().Background(c).Render(" ")
	open, _, ok := strings.Cut(rendered, "m")
	if !ok {
		t.Fatalf("background render carried no SGR sequence: %q", rendered)
	}
	return strings.TrimPrefix(open, "\x1b[")
}

// withoutTheCursorBand strips the band's own arming sequences off a marked row,
// leaving the bytes the row drew. PaintBackground re-arms the band after every
// reset the content carries, so a segment assembled from several styles — a due
// chip is a padded run of them — stops being one contiguous string on the line
// even though each of its pieces still carries its own style whole. Removing the
// arms is how a test asks about the segment instead of about the band.
func withoutTheCursorBand(t *testing.T, line string) string {
	t.Helper()
	for _, bg := range []color.Color{theme.ColorCursorRowBg, theme.ColorFlashDimBg} {
		line = strings.ReplaceAll(line, "\x1b["+backgroundParams(t, bg)+"m", "")
	}
	return line
}

func cursorHighlightOutliner(t *testing.T, tasks ...*models.Task) (*OutlinerModel, *models.Project) {
	t.Helper()
	return cursorHighlightOutlinerTheme(t, "dark", tasks...)
}

func cursorHighlightOutlinerTheme(t *testing.T, themeName string, tasks ...*models.Task) (*OutlinerModel, *models.Project) {
	t.Helper()
	theme.ApplyTheme(theme.ThemeConfig{Name: themeName})
	t.Cleanup(func() { theme.ApplyTheme(theme.ThemeConfig{}) })

	project := renderFixtureProject(t, "project", "project", "Project", tasks)
	return NewOutlinerModel(project, nil, newTestConfig()), project
}

// visibleTaskIDs returns the task IDs of the outliner's current task rows in
// render order, ignoring headers and separators.

// visibleTaskIDs returns the task IDs of the outliner's current task rows in
// render order, ignoring headers and separators.
func visibleTaskIDs(m *OutlinerModel) []string {
	var ids []string
	for _, row := range m.Rows {
		if row.IsTaskRow() && row.Task != nil {
			ids = append(ids, row.Task.ID)
		}
	}
	return ids
}

// runBatchResult executes a coordinator command and asserts it produced a
// committed batch-transaction message, returning it.

func testWorkflowStateSet(states ...models.WorkflowState) models.WorkflowStateSet {
	set, err := models.NewWorkflowStateSet(states...)
	if err != nil {
		panic(err)
	}
	return set
}

// testPeekWindowRows is the window a document-level test measures against, and
// testPeekWindow the geometry carrying it. The window is the frame's answer
// rather than the document's, so a test that builds a document without a frame
// has to say which window it wants; this is that declaration, and it is one
// declaration so every one of those tests reads the same height.
const testPeekWindowRows = 5

func testPeekWindow() peekDockGeometry {
	return peekDockGeometry{bodyRows: testPeekWindowRows}
}
