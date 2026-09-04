package lens

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// NamedLensSummary reads a definition alone, so it is exercised here rather than
// through the mounted picker: nothing about the shell routing or the overlay
// stack changes what one saved record spells.

func TestNamedLensSummaryNamesOnlyTheAxesTheLensSets(t *testing.T) {
	summary := NamedLensSummary(config.NamedLens{
		Mode: config.LensModeAll,
		Query: fmt.Sprintf("!%d #api $%s ?stale ?completed +#platform +parked freetext",
			models.PriorityHigh, models.StateWaiting),
		Sort: config.LensSort{Field: config.LensSortPriority, Ascending: true},
	})

	// Every clause the query names is spelled by the shared chip renderer, so the
	// summary carries the sigils rather than a second vocabulary of its own. The
	// unparseable tail follows them behind a single "/" marker.
	for _, want := range []string{
		"!1", "#api", "$waiting", "?stale", "?completed", "+#platform", "+parked",
		"/freetext", "sort: ",
	} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary %q is missing %q", summary, want)
		}
	}
	// An axis the query never names contributes nothing, so no defaulted facet
	// can appear.
	for _, unwanted := range []string{"@", "?deferred", "*pin", "+archived", "##"} {
		if strings.Contains(summary, unwanted) {
			t.Errorf("summary %q names an axis the query left out: %q", summary, unwanted)
		}
	}
}

func TestNamedLensSummaryOmitsTheDefaultSort(t *testing.T) {
	summary := NamedLensSummary(config.NamedLens{
		Mode: config.LensModeAll,
		Sort: config.LensSort{Field: config.LensSortDefault},
	})
	if strings.Contains(summary, "sort:") {
		t.Errorf("summary %q names the default sort", summary)
	}
}
