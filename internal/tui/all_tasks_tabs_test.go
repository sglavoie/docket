package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestAllTasksBuiltInTabOrderIncludesParked(t *testing.T) {
	cfg := testutil.NewTestConfig()
	m := newAllTasksTabsTestModel(cfg)

	m.buildTabEntries()

	wantNames := []string{"All", "Active", "Parked", "Due Soon", "High Pri", "Blocked", "By Score"}
	assertTabNames(t, m.allTabEntries, wantNames)
	assertTabNames(t, m.tabEntries, wantNames)

	assertBuiltInID(t, m.allTabEntries, "Active", builtInTabActive)
	assertBuiltInID(t, m.allTabEntries, "Parked", builtInTabParked)
	assertBuiltInID(t, m.allTabEntries, "Due Soon", builtInTabDueSoon)
	assertBuiltInID(t, m.allTabEntries, "High Pri", builtInTabHighPri)
	assertBuiltInID(t, m.allTabEntries, "Blocked", builtInTabBlocked)
	assertBuiltInID(t, m.allTabEntries, "By Score", builtInTabByScore)
}

func TestAllTasksOldTabOrderAppendsParkedBuiltIn(t *testing.T) {
	cfg := testutil.NewTestConfig()
	cfg.AllTasksTabOrder = []string{"All", "Active", "Due Soon", "High Pri", "Blocked", "By Score"}
	m := newAllTasksTabsTestModel(cfg)

	m.buildTabEntries()

	assertTabNames(t, m.allTabEntries, []string{"All", "Active", "Due Soon", "High Pri", "Blocked", "By Score", "Parked"})
}

func TestAllTasksSavedFilterOverflowWithSevenBuiltIns(t *testing.T) {
	cfg := testutil.NewTestConfig()
	cfg.SaveFilter(config.SavedFilter{ID: "triage", Name: "Triage", ShowAsTab: true})
	m := newAllTasksTabsTestModel(cfg)

	m.buildTabEntries()

	if got, want := len(m.allTabEntries), 8; got != want {
		t.Fatalf("allTabEntries len = %d, want %d", got, want)
	}
	if got, want := len(m.tabEntries), overflowTabCap; got != want {
		t.Fatalf("visible tabEntries len = %d, want %d", got, want)
	}
	if !m.tabEntries[len(m.tabEntries)-1].IsOverflow {
		t.Fatalf("last visible tab should be overflow placeholder, got %#v", m.tabEntries[len(m.tabEntries)-1])
	}
	assertTabNames(t, m.tabEntries[:len(m.tabEntries)-1], []string{"All", "Active", "Parked", "Due Soon", "High Pri", "Blocked"})
	assertTabNames(t, m.allTabEntries[overflowTabCap-1:], []string{"By Score", "Triage"})
}

func TestAllTasksParkedTabFiltersParkedTasks(t *testing.T) {
	cfg := testutil.NewTestConfig()
	proj := testutil.NewTestProject("alpha")
	m := newAllTasksTabsTestModel(cfg)
	m.groupBy = ATGroupByFlat
	m.buildTabEntries()

	parkedByNote := testutil.NewTestTask("parked by note", models.TaskTypeTask)
	parkedByNote.ParkedNote = "resume after review"

	parkedByDate := testutil.NewTestTask("parked by date", models.TaskTypeTask)
	parkedAt := time.Now()
	parkedByDate.ParkedAt = &parkedAt

	plain := testutil.NewTestTask("plain", models.TaskTypeTask)

	m.allTasks = []CrossProjectTask{
		{Task: parkedByNote, Project: proj},
		{Task: plain, Project: proj},
		{Task: parkedByDate, Project: proj},
	}
	m.activeTabIdx = tabIndexByName(t, m.allTabEntries, "Parked")
	m.applyFiltersAndGroup()

	if got, want := m.totalVisibleTaskCount(), 2; got != want {
		t.Fatalf("visible parked task count = %d, want %d", got, want)
	}
	if got := collectTaskTitles(m.groups); !sameStrings(got, []string{"parked by note", "parked by date"}) {
		t.Fatalf("parked tab tasks = %#v, want parked tasks only", got)
	}
}

func newAllTasksTabsTestModel(cfg *config.Config) *AllTasksViewModel {
	m := newMinimalAllTasksVM()
	m.ViewBundle = ViewBundle{config: cfg}
	m.FilterMixin = FilterMixin{filter: newFilterState()}
	m.projectFilter = make(map[string]bool)
	m.sort = NewSortState()
	return m
}

func assertTabNames(t *testing.T, entries []TabEntry, want []string) {
	t.Helper()
	got := make([]string, len(entries))
	for i, entry := range entries {
		got[i] = entry.Name
	}
	if !sameStrings(got, want) {
		t.Fatalf("tab names = %#v, want %#v", got, want)
	}
}

func assertBuiltInID(t *testing.T, entries []TabEntry, name string, want int) {
	t.Helper()
	idx := tabIndexByName(t, entries, name)
	if !entries[idx].IsBuiltIn {
		t.Fatalf("%q should be built in", name)
	}
	if got := entries[idx].BuiltInID; got != want {
		t.Fatalf("%q BuiltInID = %d, want %d", name, got, want)
	}
}

func tabIndexByName(t *testing.T, entries []TabEntry, name string) int {
	t.Helper()
	for i, entry := range entries {
		if entry.Name == name {
			return i
		}
	}
	t.Fatalf("tab %q not found in %#v", name, entries)
	return -1
}

func collectTaskTitles(groups []AllTasksGroup) []string {
	var titles []string
	for _, group := range groups {
		for _, item := range group.Tasks {
			titles = append(titles, item.Task.Title)
		}
	}
	return titles
}

func sameStrings(a, b []string) bool {
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
