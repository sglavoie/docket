package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/outliner"
)

// uxDailyUseIntegratedReleaseKeys is the walk the release catalog is read with:
// ends, pages, and single steps, which is every way a window moves. It is one
// sequence so the pace measured below is measured over the same work at every
// size.
var uxDailyUseIntegratedReleaseKeys = []string{
	"G", "ctrl+u", "ctrl+u", "ctrl+b", "k", "k", "j",
	"g", "g", "ctrl+d", "ctrl+d", "ctrl+f", "j", "j", "k",
	"G", "g", "g",
}

// uxDailyUseIntegratedPaceBudget is the wall clock the walk above may spend. It
// is deliberately far above what the walk costs — the point is a pathological
// regression, not a benchmark — because a tight bound on a shared machine under
// the race detector would fail for the machine rather than for the product.
const uxDailyUseIntegratedPaceBudget = 30 * time.Second

// TestUXDailyUseIntegratedReleaseCatalogHoldsItsColumnAndItsPace reads the large
// catalog the plan was told to exercise, and asks the two questions that only a
// catalog this size can answer.
//
// The first is the column. `ComputeOutlinerRowLayout` measures over the whole
// projection rather than over the window, precisely so the trailing field does
// not walk left and right as a user scrolls; with 240 rows the window is a small
// fraction of the projection, so a layout that had quietly become window-scoped
// would move the column on the first page turn. The test reads the column off
// the drawn rows at every stop of the walk, which is where a user would see it.
//
// The second is pace. Row geometry became a per-frame measurement over the whole
// projection in session 2, which is a real cost on a catalog this size; the
// bound below is what separates "measurably slower" from "no longer usable".
func TestUXDailyUseIntegratedReleaseCatalogHoldsItsColumnAndItsPace(t *testing.T) {
	for _, size := range [][2]int{uxDailyUseMatrixSizes[0], uxDailyUseDailySize} {
		t.Run(uxDailyUseSizeName(size), func(t *testing.T) {
			driver, view := uxDailyUseDriver(t, FixtureRelease, size[0], size[1])
			if len(view.Rows) < 100 {
				t.Fatalf("the release projection draws %d rows, which is not the large catalog", len(view.Rows))
			}

			columns := map[int][]string{}
			started := time.Now()
			for step, key := range uxDailyUseIntegratedReleaseKeys {
				uxClarityPress(t, driver, uxClarityKeys(key)...)
				obs := driver.Observe(ObserveOptions{})
				assertUXScreenFitsItsTerminal(t, obs, size)
				where := fmt.Sprintf("step %d (%s)", step, key)
				for _, column := range uxDailyUseIntegratedProjectFieldColumns(obs.PlainScreen) {
					columns[column] = append(columns[column], where)
				}
			}
			elapsed := time.Since(started)

			if len(columns) == 0 {
				t.Fatal("no drawn row names its project, so the column was never measured")
			}
			if len(columns) > 1 {
				var report []string
				for column, where := range columns {
					report = append(report, fmt.Sprintf("column %d first at %s", column, where[0]))
				}
				t.Fatalf("the project field is drawn in %d different columns while the window scrolls: %s",
					len(columns), strings.Join(report, "; "))
			}
			if elapsed > uxDailyUseIntegratedPaceBudget {
				t.Fatalf("%d keystrokes over a %d-row projection took %s, budget %s",
					len(uxDailyUseIntegratedReleaseKeys), len(view.Rows), elapsed, uxDailyUseIntegratedPaceBudget)
			}
		})
	}
}

// uxDailyUseIntegratedProjectFieldColumns returns the display cell every drawn
// project field starts in. Reading it off the screen rather than off the layout
// struct is the point: the struct is one number by construction, and what a
// reader sees is where the bracket landed.
func uxDailyUseIntegratedProjectFieldColumns(plain string) []int {
	var columns []int
	seen := map[int]bool{}
	for _, line := range strings.Split(plain, "\n") {
		index := strings.Index(line, "[release-")
		if index < 0 {
			continue
		}
		// The pinned context row names the project after its lane, so its
		// bracket follows the lane's name and is no row's project field.
		if strings.Contains(line, " › [release-") {
			continue
		}
		column := ansi.StringWidth(line[:index])
		if !seen[column] {
			seen[column] = true
			columns = append(columns, column)
		}
	}
	return columns
}

// uxDailyUseIntegratedRequireTaskCursor is the launch rule read wherever the
// loop rests: the cursor names work, so the next key is an act.
func uxDailyUseIntegratedRequireTaskCursor(t *testing.T, view *outliner.OutlinerModel, where string) {
	t.Helper()
	if view.Cursor < 0 || view.Cursor >= len(view.Rows) {
		t.Fatalf("at %s the cursor is at index %d of %d rows", where, view.Cursor, len(view.Rows))
	}
	if row := view.Rows[view.Cursor]; !row.IsTaskRow() {
		t.Fatalf("at %s the cursor rests on a %s rather than on a task row", where, uxDailyUseIntegratedRowKind(row))
	}
}

// uxDailyUseIntegratedRowKind names what a row is, for a failure message.
func uxDailyUseIntegratedRowKind(row outliner.OutlinerRow) string {
	switch {
	case row.IsProjectHeader || row.IsFocusedProjectHeader:
		return "project header"
	case row.IsSectionHeader:
		return "section header"
	case row.IsChecklistItem:
		return "checklist row"
	default:
		return "non-task row"
	}
}

// uxDailyUseIntegratedRequireLeadingActs reads the head of the one footer ladder
// wherever a task row is resting.
func uxDailyUseIntegratedRequireLeadingActs(t *testing.T, driver *UITestDriver, where string) {
	t.Helper()
	footer := uxClarityFooter(driver)
	if !strings.Contains(footer, uxDailyUseIntegratedLeadingActs) {
		t.Fatalf("at %s the footer does not lead with %q:\n%s", where, uxDailyUseIntegratedLeadingActs, footer)
	}
}

// uxDailyUseIntegratedRequireRow fails unless the projection draws a task row
// with this title, which is how the loop checks that a commit reached the list
// rather than only the store.
func uxDailyUseIntegratedRequireRow(t *testing.T, driver *UITestDriver, view *outliner.OutlinerModel, title string) {
	t.Helper()
	for _, row := range view.Rows {
		if row.IsTaskRow() && row.Task != nil && row.Task.Title == title {
			return
		}
	}
	t.Fatalf("the projection draws no row titled %q\n%s", title, driver.Observe(ObserveOptions{}).PlainScreen)
}

// uxDailyUseIntegratedRequireStored fails unless a task with this title is in the
// loaded population, which is the half a drawn row cannot prove on its own.
func uxDailyUseIntegratedRequireStored(t *testing.T, view *outliner.OutlinerModel, title string) {
	t.Helper()
	uxDailyUseIntegratedStoredTask(t, view, title)
}

// uxDailyUseIntegratedStoredTask resolves one task out of the loaded population
// by title. The population is what a reload rebuilt, so a task found here is a
// task the transaction wrote rather than one the projection is remembering.
func uxDailyUseIntegratedStoredTask(t *testing.T, view *outliner.OutlinerModel, title string) *models.Task {
	t.Helper()
	for _, project := range view.Population.Projects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task != nil && task.Title == title {
				return task
			}
		}
	}
	t.Fatalf("no stored task is titled %q", title)
	return nil
}

// uxDailyUseIntegratedRequireState reads a task's stored lifecycle state, which
// is the only honest way to ask what a workflow key committed: the lane a row is
// drawn in is a projection of that state and not a second copy of it.
func uxDailyUseIntegratedRequireState(t *testing.T, view *outliner.OutlinerModel, title, want string) {
	t.Helper()
	task := uxDailyUseIntegratedStoredTask(t, view, title)
	if got := string(task.State()); got != want {
		t.Fatalf("%q is stored in %q, want %q", title, got, want)
	}
}

// uxDailyUseIntegratedSettle drains the commands a committed transaction leaves
// behind — the reload, the result envelope, and the deferred note published
// beside it — so the next assertion reads settled state rather than a frame in
// flight.
func uxDailyUseIntegratedSettle(t *testing.T, driver *UITestDriver) {
	t.Helper()
	uxClarityPress(t, driver, UITestAction{Type: UITestActionWait})
}

// uxDailyUseIntegratedRowsContaining counts the screen rows holding needle,
// which is how "stated once" is asked of a rendered frame.
func uxDailyUseIntegratedRowsContaining(plain, needle string) int {
	var rows int
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, needle) {
			rows++
		}
	}
	return rows
}
