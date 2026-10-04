package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/lens"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/query"
)

func TestLensChangesExplainModifiedReadoutAndRebase(t *testing.T) {
	model := lensOperationApp(t)
	out := model.crossOutliner
	saved := config.StarterNamedLenses()[0]
	applyLensOrFatal(t, out, saved)
	if len(out.LensChangedAxes()) != 0 {
		t.Fatal("fresh lens reports changes")
	}
	out.TreeSort = query.SortState{Field: query.SortByTitle, Ascending: true}
	if got := out.LensChangedAxes(); !slices.Equal(got, []string{"sort"}) {
		t.Fatalf("changed axes = %v", got)
	}
	out.ToggleQueryReadout()
	if body := out.QueryReadoutBody(100); !strings.Contains(body, "Changed: sort") {
		t.Fatal(body)
	}
	result := executeLens(model, lens.OperationLensReplace, operation.LensEntity(saved.ID), "", false)
	if result.ConfirmationRequired == nil {
		t.Fatalf("missing confirmation: %+v", result)
	}
	model.handleOperationOverlayMessage(*result.ConfirmationRequired)
	if !strings.Contains(model.operations.Confirm.Prompt(), "Changed: sort") {
		t.Fatal(model.operations.Confirm.Prompt())
	}
	if result := executeLens(model, lens.OperationLensReplace, operation.LensEntity(saved.ID), "", true); result.Err != nil {
		t.Fatal(result.Err)
	}
	if out.LensModified() || len(out.LensChangedAxes()) != 0 {
		t.Fatal("replacement left stale change explanation")
	}
}

func TestLensChangesCompareEverySavedAxis(t *testing.T) {
	base := outliner.LensState{Mode: chrome.ModeAll}
	for _, tc := range []struct {
		want   string
		change func(*outliner.LensState)
	}{
		{"mode", func(s *outliner.LensState) { s.Mode = chrome.ModeToday }},
		{"query / filters", func(s *outliner.LensState) { s.Query = "needle" }},
		{"query / filters", func(s *outliner.LensState) { s.Facets, _ = models.PartitionQueryString("#work") }},
		{"sort", func(s *outliner.LensState) { s.Sort.Ascending = true }},
		{"project scope", func(s *outliner.LensState) { s.ScopeProjectID = "project" }},
	} {
		t.Run(tc.want, func(t *testing.T) {
			changed := base
			tc.change(&changed)
			if got := changed.ChangedAxes(base); !slices.Equal(got, []string{tc.want}) {
				t.Fatalf("changed = %v", got)
			}
		})
	}
}

func TestLensReplacementExplainsInactiveSelectedLens(t *testing.T) {
	model := lensOperationApp(t)
	saved := config.StarterNamedLenses()[0]
	if model.crossOutliner.HasActiveLens() {
		t.Fatal("expected no active lens")
	}
	result := executeLens(model, lens.OperationLensReplace, operation.LensEntity(saved.ID), "", false)
	if result.ConfirmationRequired == nil {
		t.Fatalf("missing confirmation: %+v", result)
	}
	model.handleOperationOverlayMessage(*result.ConfirmationRequired)
	prompt := model.operations.Confirm.Prompt()
	if !strings.Contains(prompt, saved.Name) || !strings.Contains(prompt, "Changed:") {
		t.Fatal(prompt)
	}
	for _, width := range []int{60, 80, 120} {
		body, ok := model.operations.ConfirmOverlay(width)
		if !ok || !strings.Contains(body, "Changed:") {
			t.Fatal("replacement detail missing from confirmation")
		}
	}
}
