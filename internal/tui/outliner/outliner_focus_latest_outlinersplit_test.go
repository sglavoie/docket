package outliner

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

func TestFocusLatestActivityLandsOnTheNewestTaskNotTheFirstRow(t *testing.T) {
	parent := primaryRowsTask("parent", models.StateBacklog)
	child := primaryRowsTask("child", models.StateBacklog)
	child.ParentID = &parent.ID
	// Anchor both timestamps to today's calendar day instead of subtracting a
	// fixed span from now: within two hours of local midnight that span dropped
	// the parent into yesterday's Recent lane and split the nesting.
	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	parent.UpdatedAt = startOfToday
	child.UpdatedAt = startOfToday.Add(time.Minute)
	project := primaryRowsProject("alpha", parent, child)
	outliner, _ := primaryRowsOutliner(project)

	outliner.FocusLatestActivity()

	// Both tasks land in Today, where grouping nests the newer child under the
	// older parent, so the first row is not the newest task.
	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{parent.ID, child.ID}) {
		t.Fatalf("Recent rows = %v, want the child nested under its parent", got)
	}
	if got := cursorTaskID(outliner); got != child.ID {
		t.Fatalf("cursor task = %q, want %q", got, child.ID)
	}
}

func TestFocusLatestActivityReadsTheViewportFromTheTop(t *testing.T) {
	newest := recentTask("newest", models.StateBacklog, 0)
	project := primaryRowsProject("alpha", newest)
	outliner, _ := primaryRowsOutliner(project)
	outliner.ViewScrollOffset = 12

	outliner.FocusLatestActivity()

	if outliner.ViewScrollOffset != 0 {
		t.Fatalf("scroll offset = %d, want 0 so the lead's headers stay on screen", outliner.ViewScrollOffset)
	}
	if got := cursorTaskID(outliner); got != newest.ID {
		t.Fatalf("cursor task = %q, want %q", got, newest.ID)
	}
}

func TestFocusLatestActivityRereadsActivityOnEveryPress(t *testing.T) {
	older := recentTask("older", models.StateBacklog, 3)
	newer := recentTask("newer", models.StateBacklog, 2)
	project := primaryRowsProject("alpha", older, newer)
	outliner, _ := primaryRowsOutliner(project)

	outliner.FocusLatestActivity()
	if got := cursorTaskID(outliner); got != newer.ID {
		t.Fatalf("cursor task = %q, want %q", got, newer.ID)
	}

	older.UpdatedAt = time.Now()
	outliner.FocusLatestActivity()
	if got := cursorTaskID(outliner); got != older.ID {
		t.Fatalf("cursor task after re-read = %q, want %q", got, older.ID)
	}
}

func TestFocusLatestActivityOpensOnlyTheFoldsHidingTheTarget(t *testing.T) {
	today := recentTask("today", models.StateBacklog, 0)
	yesterday := recentTask("yesterday", models.StateBacklog, 1)
	project := primaryRowsProject("alpha", today, yesterday)
	outliner, _ := primaryRowsOutliner(project)
	outliner.SelectOutlinerMode(chrome.ModeRecent)

	todayKey := canonicalModeCollapseKey(chrome.ModeRecent, "today")
	yesterdayKey := canonicalModeCollapseKey(chrome.ModeRecent, "yesterday")
	outliner.Folds.CollapsedSections[todayKey] = true
	outliner.Folds.CollapsedSections[yesterdayKey] = true

	outliner.FocusLatestActivity()

	if got := cursorTaskID(outliner); got != today.ID {
		t.Fatalf("cursor task = %q, want %q", got, today.ID)
	}
	if outliner.Folds.CollapsedSections[todayKey] {
		t.Error("the section hiding the newest task stayed collapsed")
	}
	if !outliner.Folds.CollapsedSections[yesterdayKey] {
		t.Error("a fold that hid nothing was opened")
	}
}

func TestFocusLatestActivityReportsAnEmptyRecent(t *testing.T) {
	stale := recentTask("stale", models.StateBacklog, recentModeCutoffDays+1)
	project := primaryRowsProject("alpha", stale)
	outliner, _ := primaryRowsOutliner(project)

	outliner.FocusLatestActivity()

	if got := visibleTaskIDs(outliner); len(got) != 0 {
		t.Fatalf("Recent rows = %v, want none past the cutoff", got)
	}
	if !strings.Contains(outliner.StatusMessage, "No activity") {
		t.Fatalf("status = %q, want it to report an empty Recent", outliner.StatusMessage)
	}
}
