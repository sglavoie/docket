package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/lens"
	"github.com/sglavoie/docket/internal/tui/operation"
)

func TestLensDuplicateCopiesSavedDefinitionWithoutApplying(t *testing.T) {
	model := lensOperationApp(t)
	source := lensNamed(t, model, "Waiting")
	applyLensOrFatal(t, model.crossOutliner, source)
	model.crossOutliner.SelectOutlinerMode(chrome.ModeBacklog)
	beforeView := model.crossOutliner.CaptureLensState()
	beforeMachine := model.domain.runtime.Machine()
	result := executeLens(model, lens.OperationLensDuplicate, operation.LensEntity(source.ID), "Waiting copy", false)
	if result.Err != nil {
		t.Fatal(result.Err)
	}
	copy := lensNamed(t, model, "Waiting copy")
	if copy.ID == source.ID || config.ValidateNamedLensID(copy.ID) != nil {
		t.Fatalf("copy has no fresh identity: %+v", copy)
	}
	copy.ID, copy.Name = source.ID, source.Name
	if copy != source || lensNamed(t, model, source.Name) != source {
		t.Fatal("duplication changed the saved definition")
	}
	if !reflect.DeepEqual(beforeView, model.crossOutliner.CaptureLensState()) ||
		model.crossOutliner.Lens.Active.LensID != source.ID || !model.crossOutliner.LensModified() {
		t.Fatal("duplication changed the modified active view")
	}
	if len(beforeMachine.NamedLenses)+1 != len(machineLenses(t, model)) {
		t.Fatal("duplication mutated the old snapshot or failed to append")
	}
	if result := model.operations.Executor.Undo(); result.Err == nil {
		t.Fatal("duplication entered workspace undo history")
	}
}

func TestLensDuplicatePreservesUnavailableScope(t *testing.T) {
	model := lensOperationApp(t)
	machine := model.domain.runtime.Machine().Clone()
	source := machine.NamedLenses[0].Clone()
	source.Scope = lensTestID(99)
	source.Mode = config.LensModeDue
	source.Query = "#auth @overdue"
	source.Sort = config.LensSort{Field: config.LensSortTitle, Ascending: false}
	machine.NamedLenses[0] = source
	setMachineLenses(t, model, machine.NamedLenses)
	if result := executeLens(model, lens.OperationLensDuplicate, operation.LensEntity(source.ID), "Unavailable copy", false); result.Err != nil {
		t.Fatal(result.Err)
	}
	copy := lensNamed(t, model, "Unavailable copy")
	copy.ID, copy.Name = source.ID, source.Name
	if copy != source {
		t.Fatalf("definition changed: %+v, want %+v", copy, source)
	}
}

func TestLensDuplicateRejectsInvalidNamesWithoutMutation(t *testing.T) {
	for _, name := range []string{"   ", "waiting", "line\nbreak", strings.Repeat("x", config.MaxNamedLensNameRunes+1)} {
		t.Run(name[:min(len(name), 15)], func(t *testing.T) {
			model := lensOperationApp(t)
			before := model.domain.runtime.Machine()
			result := executeLens(model, lens.OperationLensDuplicate, operation.LensEntity(starterLensID(t)), name, false)
			if result.Err == nil || model.domain.runtime.Machine() != before {
				t.Fatalf("invalid name changed lenses: %+v", result)
			}
		})
	}
}

func TestLensDuplicatePromptAndCancellation(t *testing.T) {
	model := lensPickerApp(t)
	source := selectLensRow(t, model, "Waiting")
	pressLens(t, model, "space", "j", "j", "enter")
	if model.operations.TextPrompt == nil {
		t.Fatal("Duplicate raised no name prompt")
	}
	pressLens(t, model, "esc")
	if model.operations.ActionSheet == nil || len(machineLenses(t, model)) != 3 {
		t.Fatal("canceling duplication changed the saved list or closed the sheet")
	}
	pressLens(t, model, "enter")
	typeLens(t, model, "Waiting copy")
	pressLens(t, model, "enter")
	if model.operations.TextPrompt != nil || model.operations.ActionSheet != nil || model.lenses.Picker() == nil {
		t.Fatal("completed duplication did not return to the picker")
	}
	if got := lensNamed(t, model, "Waiting copy"); got.ID == source.ID || got.Query != source.Query {
		t.Fatalf("wrong copy: %+v", got)
	}
	if !strings.Contains(strings.Join(lensPickerRowTitles(t, model), "|"), "Waiting copy") {
		t.Fatal("picker did not refresh to include the copy")
	}
}

func TestLensDuplicatePersistsAndFailedWritePreservesState(t *testing.T) {
	model := lensOperationApp(t)
	dataDir := t.TempDir()
	project := models.Project{ID: "alpha", Name: "alpha", Slug: "alpha"}
	model.domain.runtime = testRuntimeAt(dataDir, testConfigWithProjects(&project))
	manager := config.NewManager(filepath.Join(dataDir, ".tt.json"))
	if err := manager.CommitPlanning(config.NewPlanningState()); err != nil {
		t.Fatal(err)
	}
	source := lensNamed(t, model, "Waiting")
	if result := executeLens(model, lens.OperationLensDuplicate, operation.LensEntity(source.ID), "Persisted copy", false); result.Err != nil {
		t.Fatal(result.Err)
	}
	copy := lensNamed(t, model, "Persisted copy")
	domains, err := manager.LoadDomains()
	if err != nil {
		t.Fatal(err)
	}
	if saved := domains.Machine.NamedLensByID(copy.ID); saved == nil || *saved != copy {
		t.Fatalf("copy did not survive reload: %+v", saved)
	}
	// A file in place of the machine home makes the write fail even when the
	// tests run with permissions that can bypass chmod-based write protection.
	blockedHome := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blockedHome, []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", blockedHome)
	before := model.domain.runtime.Machine()
	beforeView := model.crossOutliner.CaptureLensState()
	result := executeLens(model, lens.OperationLensDuplicate, operation.LensEntity(source.ID), "Failed copy", false)
	if result.Err == nil {
		t.Fatal("blocked write unexpectedly succeeded")
	}
	if model.domain.runtime.Machine() != before || !reflect.DeepEqual(beforeView, model.crossOutliner.CaptureLensState()) {
		t.Fatal("failed duplication published state")
	}
}
