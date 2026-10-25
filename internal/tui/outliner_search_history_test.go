package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/outliner"
)

// Recall binds no root key: it is claimed inside the input, which the shell
// hands every key while search mode is open, and the strategy's own table still
// knows Up and Down only as the cursor keys.
func TestSearchHistoryBindsNoRootKey(t *testing.T) {
	outlinerView := searchHistoryOutliner()
	if !outlinerView.SimplifiedInlineControlOwnsInput() {
		t.Fatal("search mode does not own the input, so recall would reach root dispatch")
	}

	for _, binding := range SimplifiedInputStrategy().Bindings {
		for _, key := range binding.Keys {
			if key != "up" && key != "down" {
				continue
			}
			if binding.OperationID != operation.OperationCoreCursorUp && binding.OperationID != operation.OperationCoreCursorDown {
				t.Fatalf("root %q is bound to %q, want the cursor operations alone", key, binding.OperationID)
			}
		}
	}
}

// searchHistoryOutliner builds a scoped Outliner with the slash input open,
// which is the only state the walk exists in.
func searchHistoryOutliner() *outliner.OutlinerModel {
	outlinerView := scopeTestOutliner(scopeTestProject("work", "Work", "alpha", "beta"))
	outlinerView.SetSize(120, 40)
	outlinerView.ActivateSearch()
	return outlinerView
}

func guidanceHintLabels(spec chrome.GuidanceSpec) string {
	labels := make([]string, 0, len(spec.Hints))
	for _, hint := range spec.Hints {
		labels = append(labels, strings.TrimSpace(hint.Key+" "+hint.Label))
	}
	return strings.Join(labels, " | ")
}
