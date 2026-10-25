package outliner

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

func TestEmptySearchExplainsOnlyProvenSingleFilterMatches(t *testing.T) {
	for _, tc := range []struct {
		name, text            string
		state                 models.WorkflowState
		extraFilter, deferred bool
		want                  string
	}{
		{name: "priority", text: "needle", state: models.StateBacklog, want: "1 match hidden by your priority filter"},
		{name: "case retry", text: "NEEDLE", state: models.StateBacklog, want: "1 match hidden by your priority filter"},
		{name: "unmatched text", text: "unrelated", state: models.StateBacklog},
		{name: "sigil still narrows", text: "needle #absent", state: models.StateBacklog},
		{name: "invalid query", text: "needle !9", state: models.StateBacklog},
		{name: "completed excluded", text: "needle", state: models.StateDone},
		{name: "deferred excluded", text: "needle", state: models.StateBacklog, deferred: true},
		{name: "several filters", text: "needle", state: models.StateBacklog, extraFilter: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := primaryRowsTask("task", tc.state)
			task.Title, task.Priority = "needle", models.PriorityLow
			if tc.deferred {
				future := time.Now().Add(24 * time.Hour)
				task.DeferredUntil = &future
			}
			out := scopeTestOutliner(primaryRowsProject("alpha", task))
			out.Filter.Priority = models.PriorityHigh
			if tc.extraFilter {
				out.Filter.Tags = map[string]bool{"missing": true}
			}
			adoptFilterCarriers(out)
			before := out.FilterQuery.Fingerprint()
			searchRankedList(out, tc.text)
			if len(searchVisibleTaskIDs(out)) != 0 {
				t.Fatal("diagnostic changed visible results")
			}
			message := out.OutlinerEmptyMessage()
			if tc.want != "" {
				if !strings.Contains(message, tc.want) || !strings.Contains(message, "fx to clear filters") {
					t.Fatalf("missing explanation: %s", message)
				}
			} else if strings.Contains(message, "hidden by your") {
				t.Fatalf("unproven explanation: %s", message)
			}
			if out.FilterQuery.Fingerprint() != before {
				t.Fatal("diagnostic mutated filters")
			}
		})
	}
}

func TestEmptySearchFilterHintPreservesFocusAndUpdatesAfterClearing(t *testing.T) {
	parent := primaryRowsTask("parent", models.StateBacklog)
	child := primaryRowsTask("child", models.StateBacklog)
	child.Title, child.Priority, child.ParentID = "needle", models.PriorityLow, &parent.ID
	outside := primaryRowsTask("outside", models.StateBacklog)
	outside.Title, outside.Priority = "needle", models.PriorityLow
	project := primaryRowsProject("alpha", parent, child, outside)
	out := scopeTestOutliner(project)
	out.pushFocusFrame(taskref.TaskIdentity{ProjectID: project.ID, TaskID: parent.ID}, 0, 0)
	out.Filter.Priority = models.PriorityHigh
	adoptFilterCarriers(out)
	searchRankedList(out, "needle")
	if !strings.Contains(out.OutlinerEmptyMessage(), "1 match hidden") {
		t.Fatalf("count escaped Focus: %s", out.OutlinerEmptyMessage())
	}
	out.Filter.Priority = 0
	adoptFilterCarriers(out)
	out.CrossProjectRebuildRows()
	if out.Search.hiddenFilterHint != "" || len(searchVisibleTaskIDs(out)) != 1 {
		t.Fatal("clearing filter did not reveal exactly the focused match")
	}
}

func TestEmptySearchExplainsIndependentFilterRemovals(t *testing.T) {
	for _, tc := range []struct {
		name   string
		text   string
		tagged bool
		high   bool
		want   string
	}{
		{"priority alone obstructs", "needle", true, false, "Clearing your priority filter reveals 1 match."},
		{"tag alone obstructs", "needle", false, true, "Clearing your tag filter reveals 1 match."},
		{"both obstruct", "needle", false, false, ""},
		{"typed gate retained", "needle #absent", true, false, ""},
		{"case retry", "NEEDLE", true, false, "Clearing your priority filter reveals 1 match."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := primaryRowsTask("task", models.StateBacklog)
			task.Title, task.Priority = "needle", models.PriorityLow
			if tc.tagged {
				task.Tags = []string{"keep"}
			}
			if tc.high {
				task.Priority = models.PriorityHigh
			}
			out := scopeTestOutliner(primaryRowsProject("alpha", task))
			out.Filter.Priority = models.PriorityHigh
			out.Filter.Tags = map[string]bool{"keep": true}
			adoptFilterCarriers(out)
			before := out.FilterQuery.Fingerprint()
			searchRankedList(out, tc.text)
			if out.Search.hiddenFilterHint != tc.want {
				t.Fatalf("hint = %q, want %q", out.Search.hiddenFilterHint, tc.want)
			}
			if len(searchVisibleTaskIDs(out)) != 0 || before != out.FilterQuery.Fingerprint() {
				t.Fatal("diagnostic changed the projection or filters")
			}
		})
	}
}

func TestEmptySearchFilterRemovalRetainsHierarchyAndListsAlternatives(t *testing.T) {
	low := primaryRowsTask("low", models.StateBacklog)
	low.Title, low.Priority, low.Tags = "needle", models.PriorityLow, []string{"keep"}
	high := primaryRowsTask("high", models.StateBacklog)
	high.Title, high.Priority = "needle", models.PriorityHigh
	out := scopeTestOutliner(primaryRowsProject("alpha", low, high))
	out.Filter.Priority, out.Filter.Tags = models.PriorityHigh, map[string]bool{"keep": true}
	adoptFilterCarriers(out)
	searchRankedList(out, "needle")
	for _, field := range []string{"priority", "tag"} {
		if !strings.Contains(out.Search.hiddenFilterHint, "Clearing your "+field+" filter reveals 1 match.") {
			t.Fatal(out.Search.hiddenFilterHint)
		}
	}
	out.Filter.Hierarchy = query.HierarchyChild
	adoptFilterCarriers(out)
	out.CrossProjectRebuildRows()
	if out.Search.hiddenFilterHint != "" {
		t.Fatal("diagnostic ignored hierarchy: " + out.Search.hiddenFilterHint)
	}
}
