package tui

import (
	"strings"
	"testing"
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

func sortPressKeys(t *testing.T, driver *UITestDriver, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if err := driver.SendKey(key); err != nil {
			t.Fatalf("key %q: %v", key, err)
		}
		driver.drain()
	}
}

func sortProjectHeaderOrder(t *testing.T, driver *UITestDriver) []string {
	t.Helper()
	outliner := driver.App().crossOutliner
	if outliner == nil {
		t.Fatal("the fixture app has no Outliner")
	}
	order := make([]string, 0, len(outliner.rows))
	for _, row := range outliner.rows {
		if row.IsProjectHeader {
			order = append(order, row.ProjectID)
		}
	}
	if len(order) == 0 {
		t.Fatal("the projection exposed no project header rows")
	}
	return order
}

func sortSectionTaskTitles(t *testing.T, driver *UITestDriver, section func(outlinerRow) bool) []string {
	t.Helper()
	outliner := driver.App().crossOutliner
	if outliner == nil {
		t.Fatal("the fixture app has no Outliner")
	}
	titles := make([]string, 0, len(outliner.rows))
	for _, row := range outliner.rows {
		if row.task != nil && section(row) {
			titles = append(titles, row.task.Title)
		}
	}
	return titles
}

// TestSortAxisInstallsAndReversesOnRepeat covers the whole axis in one pass: a
// field key installs it ascending, the same key reverses it, a different key
// switches fields back to ascending, and the clear key drops it.
func TestSortAxisInstallsAndReversesOnRepeat(t *testing.T) {
	driver := sortDriver(t, FixtureBusy)
	outliner := driver.App().crossOutliner

	for _, step := range []struct {
		keys      []string
		field     SortField
		ascending bool
	}{
		{[]string{"o", "t"}, SortByTitle, true},
		{[]string{"o", "t"}, SortByTitle, false},
		{[]string{"o", "t"}, SortByTitle, true},
		{[]string{"o", "d"}, SortByDueDate, true},
		{[]string{"o", "p"}, SortByPriority, true},
		{[]string{"o", "c"}, SortByCreatedAt, true},
		{[]string{"o", "u"}, SortByUpdatedAt, true},
		{[]string{"o", "x"}, SortByDefault, true},
	} {
		sortPressKeys(t, driver, step.keys...)
		if outliner.sort.Field != step.field {
			t.Fatalf("%s: sort field = %v, want %v", strings.Join(step.keys, ""), outliner.sort.Field, step.field)
		}
		if outliner.sort.Ascending != step.ascending {
			t.Fatalf("%s: ascending = %v, want %v", strings.Join(step.keys, ""), outliner.sort.Ascending, step.ascending)
		}
	}
}

// TestSortAxisOrdersTasksWithinTheirGroup checks the axis actually reaches the
// rows a tree-order mode owns.
func TestSortAxisOrdersTasksWithinTheirGroup(t *testing.T) {
	driver := sortDriver(t, FixtureBusy, "o", "t")
	titles := sortSectionTaskTitles(t, driver, func(row outlinerRow) bool {
		return strings.HasPrefix(row.task.Title, "Waiting")
	})
	want := []string{"Waiting on vendor response", "Waiting: production outage"}
	if !equalStringSlices(titles, want) {
		t.Fatalf("title-ascending Waiting order = %v, want %v", titles, want)
	}

	sortPressKeys(t, driver, "o", "t")
	titles = sortSectionTaskTitles(t, driver, func(row outlinerRow) bool {
		return strings.HasPrefix(row.task.Title, "Waiting")
	})
	want = []string{"Waiting: production outage", "Waiting on vendor response"}
	if !equalStringSlices(titles, want) {
		t.Fatalf("title-descending Waiting order = %v, want %v", titles, want)
	}
}

// TestSortAxisNeverReordersProjectGroups locks the ruling that the axis orders
// tasks inside a project group and never the sequence of the groups themselves.
func TestSortAxisNeverReordersProjectGroups(t *testing.T) {
	driver := sortDriver(t, FixtureMultiProject)
	before := sortProjectHeaderOrder(t, driver)

	for _, keys := range [][]string{{"o", "t"}, {"o", "t"}, {"o", "p"}, {"o", "d"}, {"o", "u"}} {
		sortPressKeys(t, driver, keys...)
		if after := sortProjectHeaderOrder(t, driver); !equalStringSlices(before, after) {
			t.Fatalf("%s reordered the project groups: %v, want %v", strings.Join(keys, ""), after, before)
		}
	}
}

// TestSortAxisNeverReachesManualOrder locks the ruling that Pinned keeps the
// order the user built by hand, whatever the axis says.
func TestSortAxisNeverReachesManualOrder(t *testing.T) {
	driver := sortDriver(t, FixtureRich, "end", "P")
	driver.Wait(defaultWaitTimeout)
	sortPressKeys(t, driver, "home", "down", "down", "down", "down", "down", "P")
	driver.Wait(defaultWaitTimeout)

	pinned := func() []string {
		return sortSectionTaskTitles(t, driver, func(row outlinerRow) bool {
			return row.IsPinnedSection
		})
	}
	before := pinned()
	want := []string{"Build login form", "Explore social login"}
	if !equalStringSlices(before, want) {
		t.Fatalf("pinned manual order = %v, want %v", before, want)
	}

	for _, keys := range [][]string{{"o", "t"}, {"o", "t"}, {"o", "p"}, {"o", "c"}} {
		sortPressKeys(t, driver, keys...)
		if after := pinned(); !equalStringSlices(before, after) {
			t.Fatalf("%s reached the manual Pinned order: %v, want %v", strings.Join(keys, ""), after, before)
		}
	}
}

// TestSortAxisRecordsTheChoiceInAFixedOrderMode covers the fixed-order branch:
// the mode keeps its own order, says so, and the recorded axis installs on the
// return to a tree-order mode.
func TestSortAxisRecordsTheChoiceInAFixedOrderMode(t *testing.T) {
	driver := sortDriver(t, FixtureRich, "g", "t", "o", "d")
	outliner := driver.App().crossOutliner

	if outliner.sort.Field != SortByDefault {
		t.Fatalf("Today installed sort field %v, want the mode's own order", outliner.sort.Field)
	}
	if outliner.treeSort.Field != SortByDueDate {
		t.Fatalf("Today recorded tree sort field %v, want %v", outliner.treeSort.Field, SortByDueDate)
	}
	if status := driver.App().statusMessage; !strings.Contains(status, "Today owns its order") {
		t.Fatalf("status %q does not explain the fixed order", status)
	}

	sortPressKeys(t, driver, "g", "a")
	if outliner.sort.Field != SortByDueDate {
		t.Fatalf("All installed sort field %v, want the recorded %v", outliner.sort.Field, SortByDueDate)
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
