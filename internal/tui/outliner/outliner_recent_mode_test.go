package outliner

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

func recentSectionNames(sections []OutlinerModeSection) []string {
	names := make([]string, 0, len(sections))
	for _, section := range sections {
		names = append(names, section.Name)
	}
	return names
}

func recentSectionTaskIDs(sections []OutlinerModeSection, name string) []string {
	for _, section := range sections {
		if section.Name != name {
			continue
		}
		ids := make([]string, 0, len(section.Rows))
		for _, row := range section.Rows {
			ids = append(ids, row.Task.ID)
		}
		return ids
	}
	return nil
}

func TestBuildRecentModeSectionsBucketsByCalendarDayAndDropsStaleActivity(t *testing.T) {
	today := recentTask("today", models.StateBacklog, 0)
	yesterday := recentTask("yesterday", models.StateBacklog, 1)
	twoDays := recentTask("two-days", models.StateBacklog, 2)
	sevenDays := recentTask("seven-days", models.StateBacklog, 7)
	eightDays := recentTask("eight-days", models.StateBacklog, 8)
	thirtyDays := recentTask("thirty-days", models.StateBacklog, 30)
	thirtyOneDays := recentTask("thirty-one-days", models.StateBacklog, 31)
	project := primaryRowsProject("alpha",
		today, yesterday, twoDays, sevenDays, eightDays, thirtyDays, thirtyOneDays)

	sections := BuildRecentModeSections(OutlinerModeInput{
		Projects:        []*models.Project{project},
		IncludeTerminal: true,
		IncludeDeferred: true,
	})

	if got := recentSectionNames(sections); !slices.Equal(got, []string{
		"Today", "Yesterday", "Last 7 Days", "Last 30 Days",
	}) {
		t.Fatalf("section names = %v, want the four day buckets", got)
	}
	for _, want := range []struct {
		section string
		ids     []string
	}{
		{"Today", []string{today.ID}},
		{"Yesterday", []string{yesterday.ID}},
		{"Last 7 Days", []string{twoDays.ID, sevenDays.ID}},
		{"Last 30 Days", []string{eightDays.ID, thirtyDays.ID}},
	} {
		if got := recentSectionTaskIDs(sections, want.section); !slices.Equal(got, want.ids) {
			t.Errorf("%s rows = %v, want %v", want.section, got, want.ids)
		}
	}
	for _, section := range sections {
		for _, row := range section.Rows {
			if row.Task.ID == thirtyOneDays.ID {
				t.Fatalf("activity past the %d-day cutoff remains in %s", recentModeCutoffDays, section.Name)
			}
		}
	}
}

// TestBuildRecentModeSectionsReadsStoredInstantsInLocalDays pins the day a
// UTC-loaded touch belongs to: 01:00 UTC is still the previous evening west of
// Greenwich, so it is Yesterday there and never Today.
func TestBuildRecentModeSectionsReadsStoredInstantsInLocalDays(t *testing.T) {
	west := time.FixedZone("UTC-4", -4*60*60)
	now := time.Date(2026, time.September, 26, 10, 0, 0, 0, west)
	lastEvening := recentTask("last-evening", models.StateBacklog, 0)
	lastEvening.UpdatedAt = time.Date(2026, time.September, 26, 1, 0, 0, 0, time.UTC)
	thisMorning := recentTask("this-morning", models.StateBacklog, 0)
	thisMorning.UpdatedAt = time.Date(2026, time.September, 26, 13, 0, 0, 0, time.UTC)
	project := primaryRowsProject("alpha", lastEvening, thisMorning)

	sections := BuildRecentModeSections(OutlinerModeInput{
		Projects: []*models.Project{project},
		Now:      now,
	})

	if got := recentSectionTaskIDs(sections, "Today"); !slices.Equal(got, []string{thisMorning.ID}) {
		t.Errorf("Today rows = %v, want only %q", got, thisMorning.ID)
	}
	if got := recentSectionTaskIDs(sections, "Yesterday"); !slices.Equal(got, []string{lastEvening.ID}) {
		t.Errorf("Yesterday rows = %v, want %q", got, lastEvening.ID)
	}
}

func TestBuildRecentModeSectionsAdmitsTerminalAndDeferredWork(t *testing.T) {
	open := recentTask("open", models.StateBacklog, 0)
	done := recentTask("done", models.StateDone, 0)
	canceled := recentTask("canceled", models.StateCanceled, 0)
	deferred := recentTask("deferred", models.StateBacklog, 0)
	future := time.Now().AddDate(0, 0, 3)
	deferred.DeferredUntil = &future
	// One shared key so the assertion reads document order, not creation jitter.
	touched := time.Now()
	for _, task := range []*models.Task{open, done, canceled, deferred} {
		task.UpdatedAt = touched
	}
	project := primaryRowsProject("alpha", open, done, canceled, deferred)

	admitted := BuildRecentModeSections(OutlinerModeInput{
		Projects:        []*models.Project{project},
		IncludeTerminal: true,
		IncludeDeferred: true,
	})
	if got := recentSectionTaskIDs(admitted, "Today"); !slices.Equal(got, []string{
		open.ID, done.ID, canceled.ID, deferred.ID,
	}) {
		t.Fatalf("Recent rows = %v, want terminal and deferred work included", got)
	}

	excluded := BuildRecentModeSections(OutlinerModeInput{Projects: []*models.Project{project}})
	if got := recentSectionTaskIDs(excluded, "Today"); !slices.Equal(got, []string{open.ID}) {
		t.Fatalf("gated Recent rows = %v, want open work only", got)
	}
}

func TestBuildRecentModeSectionsHoldsFrozenOrderAgainstLiveActivity(t *testing.T) {
	older := recentTask("older", models.StateBacklog, 3)
	newer := recentTask("newer", models.StateBacklog, 2)
	project := primaryRowsProject("alpha", older, newer)
	frozen := map[taskref.TaskIdentity]time.Time{
		{ProjectID: project.ID, TaskID: older.ID}: older.UpdatedAt,
		{ProjectID: project.ID, TaskID: newer.ID}: newer.UpdatedAt,
	}

	// Triaging older bumps its live timestamp to now; its frozen key must keep it
	// in the bucket and slot the user is reading.
	older.UpdatedAt = time.Now()

	sections := BuildRecentModeSections(OutlinerModeInput{
		Projects:    []*models.Project{project},
		RecencyKeys: frozen,
	})
	if got := recentSectionTaskIDs(sections, "Last 7 Days"); !slices.Equal(got, []string{
		newer.ID, older.ID,
	}) {
		t.Fatalf("frozen Recent rows = %v, want the pre-triage order", got)
	}
	if ids := recentSectionTaskIDs(sections, "Today"); len(ids) > 0 {
		t.Fatalf("triaged task moved to Today: %v", ids)
	}
}

func TestRecentModeFreezesOrderAcrossRebuildsAndRefreshesOnReentry(t *testing.T) {
	older := recentTask("older", models.StateBacklog, 3)
	newer := recentTask("newer", models.StateBacklog, 2)
	project := primaryRowsProject("alpha", older, newer)
	outliner, _ := primaryRowsOutliner(project)

	outliner.SelectOutlinerMode(chrome.ModeRecent)
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{newer.ID, older.ID}) {
		t.Fatalf("Recent rows = %v, want newest activity first", got)
	}

	older.UpdatedAt = time.Now()
	outliner.RebuildRows()
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{newer.ID, older.ID}) {
		t.Fatalf("rows after triage = %v, want the frozen order", got)
	}

	outliner.SelectOutlinerMode(chrome.ModeRecent)
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{older.ID, newer.ID}) {
		t.Fatalf("rows after re-entry = %v, want re-read activity order", got)
	}
}
