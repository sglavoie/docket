package outliner

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

func TestSearchNamesWithheldMatchesBesideVisibleResults(t *testing.T) {
	now := time.Now()
	later := now.Add(24 * time.Hour)
	open := primaryRowsTask("open", models.StateBacklog)
	open.Title = "Needle open"
	done := primaryRowsTask("done", models.StateDone)
	done.Title, done.CompletedAt = "Needle finished", &now
	deferred := primaryRowsTask("deferred", models.StateBacklog)
	deferred.Title, deferred.DeferredUntil = "Needle deferred", &later
	model := scopeTestOutliner(primaryRowsProject("alpha", open, done, deferred))

	searchRankedList(model, "needle")
	if got := searchVisibleTaskIDs(model); len(got) != 1 || got[0] != open.ID {
		t.Fatalf("visible tasks = %v, want only open task", got)
	}
	for _, width := range []int{100, 60} {
		strip := strings.Join(strings.Fields(ansi.Strip(model.StatusStripBody(width))), " ")
		for _, want := range []string{"Also matches:", "1 finished task", "?completed", "1 deferred task", "?deferred"} {
			if !strings.Contains(strip, want) {
				t.Fatalf("strip at %d columns lacks %q: %s", width, want, strip)
			}
		}
	}

	searchRankedList(model, "needle ?completed")
	if hint := model.withheldSearchHint(); hint != "" {
		t.Fatalf("completed-only search promised excluded open/deferred work: %s", hint)
	}
	searchRankedList(model, "needle ?deferred")
	if hint := model.withheldSearchHint(); hint != "" {
		t.Fatalf("deferred-only search promised excluded completed work: %s", hint)
	}
	searchRankedList(model, "nothing matches")
	if strip := model.StatusStripBody(100); strings.Contains(strip, "Also matches:") {
		t.Fatalf("stale hidden-match counts survived a query change: %s", strip)
	}
}
