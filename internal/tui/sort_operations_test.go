package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/query"
)

// sortDriver builds a fixture-backed driver and presses a key path, so the
// assertions below read the same rows the renderer would.
func sortDriver(t *testing.T, fixture string, keys ...string) *UITestDriver {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(fixture, 100, 40)
	if err != nil {
		t.Fatalf("fixture %s: %v", fixture, err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	sortPressKeys(t, driver, keys...)
	return driver
}

func sortSectionTaskTitles(t *testing.T, driver *UITestDriver, section func(outliner.OutlinerRow) bool) []string {
	t.Helper()
	outlinerView := driver.App().crossOutliner
	if outlinerView == nil {
		t.Fatal("the fixture app has no Outliner")
	}
	titles := make([]string, 0, len(outlinerView.Rows))
	for _, row := range outlinerView.Rows {
		if row.Task != nil && section(row) {
			titles = append(titles, row.Task.Title)
		}
	}
	return titles
}

// TestSortAxisInstallsAndReversesOnRepeat covers the whole axis in one pass: a
// field key installs it ascending, the same key reverses it, a different key
// switches fields back to ascending, and the clear key drops it.
func TestSortAxisInstallsAndReversesOnRepeat(t *testing.T) {
	driver := sortDriver(t, FixtureBusy)
	outlinerView := driver.App().crossOutliner

	for _, step := range []struct {
		keys      []string
		field     query.SortField
		ascending bool
	}{
		{[]string{"o", "t"}, query.SortByTitle, true},
		{[]string{"o", "t"}, query.SortByTitle, false},
		{[]string{"o", "t"}, query.SortByTitle, true},
		{[]string{"o", "@"}, query.SortByDueDate, true},
		{[]string{"o", "!"}, query.SortByPriority, true},
		{[]string{"o", "c"}, query.SortByCreatedAt, true},
		{[]string{"o", "u"}, query.SortByUpdatedAt, true},
		{[]string{"o", "x"}, query.SortByDefault, true},
	} {
		sortPressKeys(t, driver, step.keys...)
		if outlinerView.Sort.Field != step.field {
			t.Fatalf("%s: sort field = %v, want %v", strings.Join(step.keys, ""), outlinerView.Sort.Field, step.field)
		}
		if outlinerView.Sort.Ascending != step.ascending {
			t.Fatalf("%s: ascending = %v, want %v", strings.Join(step.keys, ""), outlinerView.Sort.Ascending, step.ascending)
		}
	}
}

// TestSortAxisOrdersTasksWithinTheirGroup checks the axis actually reaches the
// rows a tree-order mode owns.
func TestSortAxisOrdersTasksWithinTheirGroup(t *testing.T) {
	driver := sortDriver(t, FixtureBusy, "o", "t")
	titles := sortSectionTaskTitles(t, driver, func(row outliner.OutlinerRow) bool {
		return strings.HasPrefix(row.Task.Title, "Waiting")
	})
	want := []string{"Waiting on vendor response", "Waiting: production outage"}
	if !equalStringSlices(titles, want) {
		t.Fatalf("title-ascending Waiting order = %v, want %v", titles, want)
	}

	sortPressKeys(t, driver, "o", "t")
	titles = sortSectionTaskTitles(t, driver, func(row outliner.OutlinerRow) bool {
		return strings.HasPrefix(row.Task.Title, "Waiting")
	})
	want = []string{"Waiting: production outage", "Waiting on vendor response"}
	if !equalStringSlices(titles, want) {
		t.Fatalf("title-descending Waiting order = %v, want %v", titles, want)
	}
}

// TestSortAxisOrdersPinnedWithPinOrderBreakingTies locks the ruling that
// Pinned follows the axis like every other section, and that clearing it gives
// the order the user built by hand back.
func TestSortAxisOrdersPinnedWithPinOrderBreakingTies(t *testing.T) {
	driver := sortDriver(t, FixtureRich, "end", "l", "end", "*")
	driver.Wait(defaultWaitTimeout)
	// Three rows below the top: the Pinned lane header, the one row it holds,
	// and the Doing lane header, then Doing's one task. Both lanes hold a single
	// task, so each project group merges into that row and draws no heading.
	sortPressKeys(t, driver, "home", "down", "down", "down", "*")
	driver.Wait(defaultWaitTimeout)

	pinned := func() []string {
		return sortSectionTaskTitles(t, driver, func(row outliner.OutlinerRow) bool {
			return row.IsPinnedSection
		})
	}
	manual := []string{"Build login form", "Explore social login"}
	if got := pinned(); !equalStringSlices(got, manual) {
		t.Fatalf("pinned default order = %v, want the newest pin first %v", got, manual)
	}

	for _, step := range []struct {
		keys []string
		want []string
	}{
		{[]string{"o", "t"}, []string{"Build login form", "Explore social login"}},
		{[]string{"o", "t"}, []string{"Explore social login", "Build login form"}},
		{[]string{"o", "x"}, manual},
	} {
		sortPressKeys(t, driver, step.keys...)
		if got := pinned(); !equalStringSlices(got, step.want) {
			t.Fatalf("%s: pinned order = %v, want %v", strings.Join(step.keys, ""), got, step.want)
		}
	}
}

// TestSortAxisRecordsTheChoiceInAFixedOrderMode covers the fixed-order branch:
// the mode keeps its own order, says so, and the recorded axis installs on the
// return to a tree-order mode.
func TestSortAxisRecordsTheChoiceInAFixedOrderMode(t *testing.T) {
	driver := sortDriver(t, FixtureRich, "g", "t", "o", "@")
	outlinerView := driver.App().crossOutliner

	if outlinerView.Sort.Field != query.SortByDefault {
		t.Fatalf("Today installed sort field %v, want the mode's own order", outlinerView.Sort.Field)
	}
	if outlinerView.TreeSort.Field != query.SortByDueDate {
		t.Fatalf("Today recorded tree sort field %v, want %v", outlinerView.TreeSort.Field, query.SortByDueDate)
	}
	if status := driver.App().feedback.message; !strings.Contains(status, "Today owns its order") {
		t.Fatalf("status %q does not explain the fixed order", status)
	}

	sortPressKeys(t, driver, "g", "a")
	if outlinerView.Sort.Field != query.SortByDueDate {
		t.Fatalf("All installed sort field %v, want the recorded %v", outlinerView.Sort.Field, query.SortByDueDate)
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
