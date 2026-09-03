package tui

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

// flatGroupingModes are the modes that gained project grouping; All, Today, and
// Next already had it and are covered by outliner_planning_headers_test.go.
var flatGroupingModes = []chrome.OutlinerModeID{chrome.ModeInbox, chrome.ModeBacklog, chrome.ModeDue, chrome.ModeRecent}

// projectGroupHeaders returns the project sub-headers in render order plus each
// one's declared count.
func projectGroupHeaders(m *OutlinerModel) ([]string, map[string]int) {
	var ids []string
	counts := make(map[string]int)
	for _, row := range m.rows {
		if row.IsProjectHeader {
			ids = append(ids, row.ProjectID)
			counts[row.ProjectID] = row.SectionCount
		}
	}
	return ids, counts
}

// projectGroupCollapseKey returns the fold key one project's group declares.
func projectGroupCollapseKey(m *OutlinerModel, projectID string) string {
	for _, row := range m.rows {
		if row.IsProjectHeader && row.ProjectID == projectID {
			return row.effectiveCollapseKey()
		}
	}
	return ""
}

// groupingFixture seeds two projects, alpha with two rows and beta with one, all
// visible in the named mode and ordered alpha-one, alpha-two, beta-one.
type groupingFixture struct {
	alpha *models.Project
	beta  *models.Project
	tasks []string
}

// groupingDueTask seeds an open task due dayOffset days from now.
func groupingDueTask(id string, dayOffset int) *models.Task {
	task := primaryRowsTask(id, models.StateBacklog)
	due := time.Now().AddDate(0, 0, dayOffset)
	task.DueDate = &due
	return task
}

func newGroupingFixture(t *testing.T, mode chrome.OutlinerModeID) groupingFixture {
	t.Helper()
	var alphaOne, alphaTwo, betaOne *models.Task
	switch mode {
	case chrome.ModeInbox:
		alphaOne = primaryRowsTask("alpha-one", models.StateInbox)
		alphaTwo = primaryRowsTask("alpha-two", models.StateInbox)
		betaOne = primaryRowsTask("beta-one", models.StateInbox)
	case chrome.ModeBacklog:
		alphaOne = primaryRowsTask("alpha-one", models.StateBacklog)
		alphaTwo = primaryRowsTask("alpha-two", models.StateBacklog)
		betaOne = primaryRowsTask("beta-one", models.StateBacklog)
	case chrome.ModeDue:
		alphaOne = groupingDueTask("alpha-one", -3)
		alphaTwo = groupingDueTask("alpha-two", -2)
		betaOne = groupingDueTask("beta-one", -1)
	case chrome.ModeRecent:
		alphaOne = recentTask("alpha-one", models.StateBacklog, 3)
		alphaTwo = recentTask("alpha-two", models.StateBacklog, 4)
		betaOne = recentTask("beta-one", models.StateBacklog, 5)
	default:
		t.Fatalf("mode %q already grouped before this contract; no fixture defined", mode)
		return groupingFixture{}
	}
	return groupingFixture{
		alpha: primaryRowsProject("alpha", alphaOne, alphaTwo),
		beta:  primaryRowsProject("beta", betaOne),
		tasks: []string{alphaOne.ID, alphaTwo.ID, betaOne.ID},
	}
}

func TestFlatModesGroupCrossProjectRowsByProject(t *testing.T) {
	for _, mode := range flatGroupingModes {
		t.Run(string(mode), func(t *testing.T) {
			fixture := newGroupingFixture(t, mode)
			outliner, _ := primaryRowsOutliner(fixture.alpha, fixture.beta)
			outliner.selectOutlinerMode(mode)

			if got := visibleTaskIDs(outliner); !slices.Equal(got, fixture.tasks) {
				t.Fatalf("%s rows = %v, want %v", mode, got, fixture.tasks)
			}
			headers, counts := projectGroupHeaders(outliner)
			if !slices.Equal(headers, []string{fixture.alpha.ID, fixture.beta.ID}) {
				t.Fatalf("%s project headers = %v, want one per project in alphabetical order",
					mode, headers)
			}
			if counts[fixture.alpha.ID] != 2 || counts[fixture.beta.ID] != 1 {
				t.Fatalf("%s project counts = %v, want alpha=2 and beta=1", mode, counts)
			}
		})
	}
}

func TestFlatModesPlacePinnedProjectGroupsFirst(t *testing.T) {
	for _, mode := range flatGroupingModes {
		t.Run(string(mode), func(t *testing.T) {
			fixture := newGroupingFixture(t, mode)
			fixture.beta.Pinned = true
			outliner, _ := primaryRowsOutliner(fixture.alpha, fixture.beta)
			outliner.selectOutlinerMode(mode)

			headers, _ := projectGroupHeaders(outliner)
			if !slices.Equal(headers, []string{fixture.beta.ID, fixture.alpha.ID}) {
				t.Fatalf("%s project headers = %v, want pinned projects first", mode, headers)
			}
		})
	}
}

func TestFlatModeProjectHeadersDisappearInSingleProjectScopes(t *testing.T) {
	for _, mode := range flatGroupingModes {
		t.Run(string(mode), func(t *testing.T) {
			fixture := newGroupingFixture(t, mode)

			scoped, _ := primaryRowsOutliner(fixture.alpha, fixture.beta)
			scoped.pushProjectScope(fixture.alpha.ID)
			scoped.selectOutlinerMode(mode)
			if len(visibleTaskIDs(scoped)) == 0 {
				t.Fatalf("%s focused on one project rendered no tasks", mode)
			}
			if headers, _ := projectGroupHeaders(scoped); len(headers) != 0 {
				t.Fatalf("%s focused on one project rendered redundant headers %v", mode, headers)
			}

			single := NewOutlinerModel(fixture.alpha, nil, nil)
			single.selectOutlinerMode(mode)
			if len(visibleTaskIDs(single)) == 0 {
				t.Fatalf("%s single-project Outliner rendered no tasks", mode)
			}
			if headers, _ := projectGroupHeaders(single); len(headers) != 0 {
				t.Fatalf("%s single-project Outliner rendered redundant headers %v", mode, headers)
			}
		})
	}
}

func TestDueProjectGroupsUseAlphabeticalProjectHeaders(t *testing.T) {
	alphaTask := groupingDueTask("alpha-task", -1)
	betaTask := groupingDueTask("beta-task", -3)
	alpha := primaryRowsProject("alpha", alphaTask)
	beta := primaryRowsProject("beta", betaTask)

	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.selectOutlinerMode(chrome.ModeDue)

	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{alphaTask.ID, betaTask.ID}) {
		t.Fatalf("Due rows = %v, want rows grouped by alphabetical project headers", got)
	}
	if headers, _ := projectGroupHeaders(outliner); !slices.Equal(headers, []string{alpha.ID, beta.ID}) {
		t.Fatalf("Due project headers = %v, want alphabetical order", headers)
	}
}

func TestRecentProjectHeadersStayAlphabeticalAcrossTriageAndReentry(t *testing.T) {
	alphaTask := recentTask("alpha-task", models.StateBacklog, 4)
	betaTask := recentTask("beta-task", models.StateBacklog, 3)
	alpha := primaryRowsProject("alpha", alphaTask)
	beta := primaryRowsProject("beta", betaTask)

	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.selectOutlinerMode(chrome.ModeRecent)
	if headers, _ := projectGroupHeaders(outliner); !slices.Equal(headers, []string{alpha.ID, beta.ID}) {
		t.Fatalf("Recent project headers = %v, want alphabetical order", headers)
	}

	alphaTask.UpdatedAt = time.Now()
	outliner.rebuildRows()
	if headers, _ := projectGroupHeaders(outliner); !slices.Equal(headers, []string{alpha.ID, beta.ID}) {
		t.Fatalf("Recent project headers after triage = %v, want the same alphabetical order", headers)
	}

	outliner.selectOutlinerMode(chrome.ModeRecent)
	if headers, _ := projectGroupHeaders(outliner); !slices.Equal(headers, []string{alpha.ID, beta.ID}) {
		t.Fatalf("Recent project headers after re-entry = %v, want alphabetical order", headers)
	}
}

// TestBacklogModeAndAllModeBacklogGroupFoldIndependently pins the disjoint
// collapse-key namespaces. collapsedSections survives every mode change, and the
// two groups hold different task sets, so one fold must never bind to the other.
func TestBacklogModeAndAllModeBacklogGroupFoldIndependently(t *testing.T) {
	newOutliner := func() (*OutlinerModel, *models.Project, *models.Task, *models.Task) {
		alphaTask := primaryRowsTask("alpha-task", models.StateBacklog)
		betaTask := primaryRowsTask("beta-task", models.StateBacklog)
		alpha := primaryRowsProject("alpha", alphaTask)
		beta := primaryRowsProject("beta", betaTask)
		outliner, _ := primaryRowsOutliner(alpha, beta)
		return outliner, alpha, alphaTask, betaTask
	}

	t.Run("backlog-mode-fold-leaves-all-mode-expanded", func(t *testing.T) {
		outliner, alpha, alphaTask, betaTask := newOutliner()
		outliner.selectOutlinerMode(chrome.ModeBacklog)
		key := projectGroupCollapseKey(outliner, alpha.ID)
		if want := modeProjectCollapseKey(chrome.ModeBacklog, alpha.ID); key != want {
			t.Fatalf("Backlog project fold key = %q, want %q", key, want)
		}
		outliner.collapsedSections[key] = true
		outliner.rebuildRows()
		if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{betaTask.ID}) {
			t.Fatalf("folded Backlog rows = %v, want beta only", got)
		}

		outliner.selectOutlinerMode(chrome.ModeAll)
		if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{alphaTask.ID, betaTask.ID}) {
			t.Fatalf("All rows = %v, want Backlog mode's fold to stay behind", got)
		}
	})

	t.Run("all-mode-fold-leaves-backlog-mode-expanded", func(t *testing.T) {
		outliner, alpha, alphaTask, betaTask := newOutliner()
		outliner.selectOutlinerMode(chrome.ModeAll)
		key := projectGroupCollapseKey(outliner, alpha.ID)
		if want := projectSectionCollapseKey(alpha.ID, models.StateGroupBacklog); key != want {
			t.Fatalf("All mode project fold key = %q, want %q", key, want)
		}
		outliner.collapsedSections[key] = true
		outliner.rebuildRows()
		if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{betaTask.ID}) {
			t.Fatalf("folded All rows = %v, want beta only", got)
		}

		outliner.selectOutlinerMode(chrome.ModeBacklog)
		if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{alphaTask.ID, betaTask.ID}) {
			t.Fatalf("Backlog rows = %v, want All mode's fold to stay behind", got)
		}
	})
}
