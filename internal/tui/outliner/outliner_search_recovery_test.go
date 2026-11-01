package outliner

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/query"
)

func recoveryOutliner() *OutlinerModel {
	now, later := time.Now(), time.Now().Add(24*time.Hour)
	open := primaryRowsTask("open", models.StateBacklog)
	open.Title, open.Tags = "Needle open", []string{"keep"}
	done := primaryRowsTask("done", models.StateDone)
	done.Title, done.Tags, done.CompletedAt = "Needle finished", []string{"keep"}, &now
	deferred := primaryRowsTask("deferred", models.StateBacklog)
	deferred.Title, deferred.Tags, deferred.DeferredUntil = "Needle deferred", []string{"keep"}, &later
	hidden := primaryRowsTask("hidden", models.StateDone)
	hidden.Title, hidden.Tags, hidden.CompletedAt, hidden.Hidden = "Needle hidden", []string{"keep"}, &now, true
	unmatched := primaryRowsTask("other-tag", models.StateDone)
	unmatched.Title, unmatched.Tags, unmatched.CompletedAt = "Needle excluded", []string{"other"}, &now
	other := primaryRowsTask("other-project", models.StateDone)
	other.Title, other.Tags, other.CompletedAt = "Needle outside scope", []string{"keep"}, &now
	m := scopeTestOutliner(primaryRowsProject("alpha", open, done, deferred, hidden, unmatched), primaryRowsProject("beta", other))
	m.Scope.ProjectID = "alpha"
	m.SetSize(60, 18)
	m.SelectOutlinerMode(chrome.ModeInbox)
	return m
}

func TestSearchRecoveryUsesTheExistingPopulationGrammarAndRestoresOrigin(t *testing.T) {
	for _, tc := range []struct {
		id          operation.OperationID
		token, task string
	}{
		{operation.OperationCoreSearchFinished, "?completed", "done"},
		{operation.OperationCoreSearchDeferred, "?deferred", "deferred"},
	} {
		t.Run(tc.token, func(t *testing.T) {
			m := recoveryOutliner()
			before := m.capturePresentationSnapshot()
			m.ActivateSearch()
			m.SetSearchQuery("Needle #keep")
			ctx := m.OperationCommandContext()
			if ctx.Search.Completed != 1 || ctx.Search.Deferred != 1 {
				t.Fatalf("counts escaped scope, tags, or hidden exclusions: %+v", ctx.Search)
			}
			cmd, err := m.ApplySearchRecovery(tc.id, ctx)
			if err != nil || cmd != nil {
				t.Fatalf("small recovery: cmd=%v err=%v", cmd != nil, err)
			}
			if got := m.SearchQuery(); got != "Needle #keep "+tc.token || m.SearchInputValue() != got || m.IsSearchMode() {
				t.Fatalf("query/input were not applied together: %q / %q", got, m.SearchInputValue())
			}
			if got := searchVisibleTaskIDs(m); !reflect.DeepEqual(got, []string{tc.task}) {
				t.Fatalf("population = %v, want %s only", got, tc.task)
			}
			m.ClearSearch()
			if after := m.capturePresentationSnapshot(); !reflect.DeepEqual(after, before) || m.Scope.ProjectID != "alpha" {
				t.Fatalf("recovery replaced the original return frame:\nbefore=%+v\nafter=%+v", before, after)
			}
		})
	}
}

func TestSearchRecoveryRefusesChangedQueryFiltersAndScope(t *testing.T) {
	for _, change := range []string{"query", "filters", "scope", "presentation"} {
		t.Run(change, func(t *testing.T) {
			m := recoveryOutliner()
			searchRankedList(m, "Needle")
			ctx := m.OperationCommandContext()
			switch change {
			case "query":
				m.SetSearchQuery("Needle finished")
			case "filters":
				edit, err := m.RowFilterClauseEdit(operation.OperationInvocation{Descriptor: operation.OperationDescriptor{ID: operation.OperationFilterPinned}})
				if err != nil {
					t.Fatal(err)
				}
				m.ReplaceFilterClauses(FilterOperationFields(operation.OperationFilterPinned), edit.Clauses)
			case "scope":
				m.Scope.ProjectID = "beta"
			case "presentation":
				m.ToggleRankedList()
			}
			before := m.SearchQuery()
			if _, err := m.ApplySearchRecovery(operation.OperationCoreSearchFinished, ctx); err == nil || !strings.Contains(err.Error(), "search changed") {
				t.Fatalf("changed search was accepted: %v", err)
			}
			if m.SearchQuery() != before {
				t.Fatal("refusal changed the query")
			}
		})
	}
}

func TestSearchRecoveryNeverOffersStaleBackgroundCounts(t *testing.T) {
	m := backgroundSearchFixture(1000)
	m.BaseSearchableView.SetSearchQuery("release")
	cmd := m.rebuildLiveSearch()
	m.Search.withheld = searchWithheldMatches{completed: 8, deferred: 3}
	ctx := m.SearchRecoveryContext()
	if !ctx.Pending || ctx.Completed != 0 || ctx.Deferred != 0 || m.withheldSearchHint() != "" {
		t.Fatalf("pending query advertised stale counts: %+v", ctx)
	}
	m.Update(cmd())
	if m.SearchRecoveryContext().Pending {
		t.Fatal("accepted results remained pending")
	}
	m.ClearSearch()
	if m.SearchRecoveryContext() != (operation.SearchContext{}) {
		t.Fatal("recovery outlived its search")
	}
}

func TestSearchRecoveryKeepsFocusAndRestoresASuspendedLens(t *testing.T) {
	t.Run("Focus", func(t *testing.T) {
		m := recoveryOutliner()
		alpha := m.Population.Projects[0]
		root := primaryRowsTask("root", models.StateBacklog)
		alpha.Tasks = append(alpha.Tasks, root)
		for _, id := range []string{"done", "deferred"} {
			alpha.GetTaskByID(id).ParentID = &root.ID
		}
		alpha.BuildTaskTree()
		m.Focus.CrossStack = []CrossProjectFocusEntry{{TaskID: root.ID, ProjectID: alpha.ID}}
		m.SelectOutlinerMode(chrome.ModeAll)
		m.Population.Invalidate()
		m.RebuildRows()
		before := m.capturePresentationSnapshot()
		searchRankedList(m, "Needle")
		ctx := m.OperationCommandContext()
		if ctx.Search.Completed != 1 || ctx.Search.Deferred != 1 {
			t.Fatalf("Focus counts: %+v", ctx.Search)
		}
		if _, err := m.ApplySearchRecovery(operation.OperationCoreSearchFinished, ctx); err != nil {
			t.Fatal(err)
		}
		if got := searchVisibleTaskIDs(m); !reflect.DeepEqual(got, []string{"done"}) {
			t.Fatalf("escaped Focus: %v", got)
		}
		m.ClearSearch()
		if !reflect.DeepEqual(before, m.capturePresentationSnapshot()) {
			t.Fatal("recovery lost the Focus return frame")
		}
	})
	t.Run("lens", func(t *testing.T) {
		m := recoveryOutliner()
		m.SetPersistActiveLens(func(string) error { return nil })
		state := LensState{Mode: chrome.ModeAll, Query: "#keep", Sort: query.NewSortState()}
		saved := state.NamedLens("6f1a0c4e-0b2f-4a7d-9b3c-5d1e8a2f6d02", "Keep")
		if cmd, err := m.ApplyNamedLens(saved); err != nil || cmd != nil {
			t.Fatalf("apply lens: %v", err)
		}
		before := m.CaptureOutlinerPresentationCheckpoint()
		m.ActivateSearch()
		m.SetSearchQuery("Needle #keep")
		if !m.HasSuspendedLens() {
			t.Fatal("search did not suspend the lens")
		}
		if _, err := m.ApplySearchRecovery(operation.OperationCoreSearchFinished, m.OperationCommandContext()); err != nil {
			t.Fatal(err)
		}
		m.ClearSearch()
		if !m.HasActiveLens() || !reflect.DeepEqual(before, m.CaptureOutlinerPresentationCheckpoint()) {
			t.Fatal("recovery failed to restore the exact lens presentation")
		}
	})
}

func TestSearchRecoveryNeverTruncatesQueryTermsAtTheInputLimit(t *testing.T) {
	for _, n := range []int{80, 100} {
		t.Run(fmt.Sprintf("%d_runes", n), func(t *testing.T) {
			text := strings.Repeat("界", n)
			done := primaryRowsTask("done", models.StateDone)
			now := time.Now()
			done.Title, done.CompletedAt = text, &now
			m := scopeTestOutliner(primaryRowsProject("alpha", done))
			m.ActivateSearch()
			m.SetSearchInputValue(text)
			m.SetSearchQuery(text)
			m.SetSearchInputCursor(17)
			_, err := m.ApplySearchRecovery(operation.OperationCoreSearchFinished, m.OperationCommandContext())
			if n == 100 {
				if err == nil || !strings.Contains(err.Error(), "shorten the query") {
					t.Fatalf("overlong recovery: %v", err)
				}
				if m.SearchQuery() != text || m.SearchInputValue() != text || m.SearchInputCursor() != 17 || !m.IsSearchMode() {
					t.Fatal("refusal lost input content or caret")
				}
			} else if err != nil || m.SearchQuery() != text+" ?completed" {
				t.Fatalf("valid Unicode recovery was shortened or refused: %v", err)
			}
		})
	}
}
