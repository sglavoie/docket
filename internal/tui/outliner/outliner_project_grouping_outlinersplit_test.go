package outliner

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

// projectGroupCollapseKey returns the fold key one project's group declares. It
// reads the group off the rows, because the heading that used to spell that key
// is drawn only while the fold it names is already closed.
func projectGroupCollapseKey(m *OutlinerModel, projectID string) string {
	for _, group := range m.VisibleProjectGroups() {
		if group.ProjectID == projectID {
			return group.ProjectKey
		}
	}
	return ""
}

func TestFlatModesGroupCrossProjectRowsByProject(t *testing.T) {
	for _, mode := range flatGroupingModes {
		t.Run(string(mode), func(t *testing.T) {
			fixture := newGroupingFixture(t, mode)
			outliner, _ := primaryRowsOutliner(fixture.alpha, fixture.beta)
			outliner.SelectOutlinerMode(mode)

			if got := visibleTaskIDs(outliner); !slices.Equal(got, fixture.tasks) {
				t.Fatalf("%s rows = %v, want %v", mode, got, fixture.tasks)
			}
			groups := projectGroupOrder(outliner)
			if !slices.Equal(groups, []string{fixture.alpha.ID, fixture.beta.ID}) {
				t.Fatalf("%s project groups = %v, want one per project in alphabetical order",
					mode, groups)
			}
			counts := projectGroupTaskCounts(outliner)
			if counts[fixture.alpha.ID] != 2 || counts[fixture.beta.ID] != 1 {
				t.Fatalf("%s project counts = %v, want alpha=2 and beta=1", mode, counts)
			}
		})
	}
}

// TestFlatModesBreakProjectGroupTiesPinnedFirst holds the tiebreak under the
// block ranking: two groups whose best rows tie put the pinned project first.
func TestFlatModesBreakProjectGroupTiesPinnedFirst(t *testing.T) {
	for _, mode := range flatGroupingModes {
		t.Run(string(mode), func(t *testing.T) {
			fixture := newGroupingFixture(t, mode)
			fixture.beta.Pinned = true
			alphaLead, betaLead := fixture.alpha.Tasks[0], fixture.beta.Tasks[0]
			betaLead.DueDate = alphaLead.DueDate
			betaLead.UpdatedAt = alphaLead.UpdatedAt
			outliner, _ := primaryRowsOutliner(fixture.alpha, fixture.beta)
			outliner.SelectOutlinerMode(mode)

			groups := projectGroupOrder(outliner)
			if !slices.Equal(groups, []string{fixture.beta.ID, fixture.alpha.ID}) {
				t.Fatalf("%s project groups = %v, want the pinned project first on a tie", mode, groups)
			}
		})
	}
}

// TestFlatModeProjectGroupsGiveWayToOneFocusedHeader locks both halves of the
// single-project rule at once: the project axis disappears, taking the per-row
// labels with it, and the one focused-project header leads the projection
// instead. Both single-project forms are covered, because an explicit scope
// and a direct project model reach that header by different routes.
func TestFlatModeProjectGroupsGiveWayToOneFocusedHeader(t *testing.T) {
	for _, mode := range flatGroupingModes {
		t.Run(string(mode), func(t *testing.T) {
			fixture := newGroupingFixture(t, mode)

			scoped, _ := primaryRowsOutliner(fixture.alpha, fixture.beta)
			scoped.PushProjectScope(fixture.alpha.ID)
			scoped.SelectOutlinerMode(mode)
			if len(visibleTaskIDs(scoped)) == 0 {
				t.Fatalf("%s focused on one project rendered no tasks", mode)
			}
			if groups := projectGroupOrder(scoped); len(groups) != 0 {
				t.Fatalf("%s focused on one project rendered redundant groups %v", mode, groups)
			}
			assertSoleFocusedProjectHeader(t, scoped, fixture.alpha.ID, mode)

			single := NewOutlinerModel(fixture.alpha, nil, nil)
			single.SelectOutlinerMode(mode)
			if len(visibleTaskIDs(single)) == 0 {
				t.Fatalf("%s single-project Outliner rendered no tasks", mode)
			}
			if groups := projectGroupOrder(single); len(groups) != 0 {
				t.Fatalf("%s single-project Outliner rendered redundant groups %v", mode, groups)
			}
			assertSoleFocusedProjectHeader(t, single, fixture.alpha.ID, mode)
		})
	}
}

// TestDueProjectGroupsRankByTheirEarliestDeadline holds that a Due bucket's
// blocks follow the bucket's own order rather than the project names.
func TestDueProjectGroupsRankByTheirEarliestDeadline(t *testing.T) {
	alphaTask := groupingDueTask("alpha-task", -1)
	betaTask := groupingDueTask("beta-task", -3)
	alpha := primaryRowsProject("alpha", alphaTask)
	beta := primaryRowsProject("beta", betaTask)

	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.SelectOutlinerMode(chrome.ModeDue)

	if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{betaTask.ID, alphaTask.ID}) {
		t.Fatalf("Due rows = %v, want the older deadline's group first", got)
	}
	if groups := projectGroupOrder(outliner); !slices.Equal(groups, []string{beta.ID, alpha.ID}) {
		t.Fatalf("Due project groups = %v, want the older deadline's group first", groups)
	}
}

// TestRecentProjectGroupsHoldTheirOrderAcrossTriage holds that Recent's blocks
// rank by their newest touch, and that the frozen recency keys keep a triage
// touch from lifting a block until the mode is entered again.
func TestRecentProjectGroupsHoldTheirOrderAcrossTriage(t *testing.T) {
	alphaTask := recentTask("alpha-task", models.StateBacklog, 4)
	betaTask := recentTask("beta-task", models.StateBacklog, 3)
	alpha := primaryRowsProject("alpha", alphaTask)
	beta := primaryRowsProject("beta", betaTask)

	outliner, _ := primaryRowsOutliner(alpha, beta)
	outliner.SelectOutlinerMode(chrome.ModeRecent)
	if groups := projectGroupOrder(outliner); !slices.Equal(groups, []string{beta.ID, alpha.ID}) {
		t.Fatalf("Recent project groups = %v, want the newest touch first", groups)
	}

	alphaTask.UpdatedAt = time.Now()
	outliner.RebuildRows()
	if groups := projectGroupOrder(outliner); !slices.Equal(groups, []string{beta.ID, alpha.ID}) {
		t.Fatalf("Recent project groups after triage = %v, want the frozen order", groups)
	}

	outliner.SelectOutlinerMode(chrome.ModeRecent)
	if groups := projectGroupOrder(outliner); !slices.Equal(groups, []string{alpha.ID, beta.ID}) {
		t.Fatalf("Recent project groups after re-entry = %v, want the fresh touch first", groups)
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
		outliner.SelectOutlinerMode(chrome.ModeBacklog)
		key := projectGroupCollapseKey(outliner, alpha.ID)
		if want := modeProjectCollapseKey(chrome.ModeBacklog, alpha.ID); key != want {
			t.Fatalf("Backlog project fold key = %q, want %q", key, want)
		}
		outliner.Folds.CollapsedSections[key] = true
		outliner.RebuildRows()
		if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{betaTask.ID}) {
			t.Fatalf("folded Backlog rows = %v, want beta only", got)
		}

		outliner.SelectOutlinerMode(chrome.ModeAll)
		if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{alphaTask.ID, betaTask.ID}) {
			t.Fatalf("All rows = %v, want Backlog mode's fold to stay behind", got)
		}
	})

	t.Run("all-mode-fold-leaves-backlog-mode-expanded", func(t *testing.T) {
		outliner, alpha, alphaTask, betaTask := newOutliner()
		outliner.SelectOutlinerMode(chrome.ModeAll)
		key := projectGroupCollapseKey(outliner, alpha.ID)
		if want := projectSectionCollapseKey(alpha.ID, models.StateGroupBacklog); key != want {
			t.Fatalf("All mode project fold key = %q, want %q", key, want)
		}
		outliner.Folds.CollapsedSections[key] = true
		outliner.RebuildRows()
		if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{betaTask.ID}) {
			t.Fatalf("folded All rows = %v, want beta only", got)
		}

		outliner.SelectOutlinerMode(chrome.ModeBacklog)
		if got := visibleTaskIDs(outliner); !slices.Equal(got, []string{alphaTask.ID, betaTask.ID}) {
			t.Fatalf("Backlog rows = %v, want All mode's fold to stay behind", got)
		}
	})
}
