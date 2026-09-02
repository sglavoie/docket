package tui

import (
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// --- an added template is an ordinary record ---------------------------------

// templateRowReason reads the gallery's own verdict on one template against the
// machine as it stands, which is the sentence a user sees on the greyed row.
func templateRowReason(t *testing.T, model *AppModel, id string) string {
	t.Helper()
	picker := openLensTemplateGallery(t, model)
	reason := galleryRow(t, picker, id).DisabledReason
	model.operationValuePicker = nil
	return reason
}

// TestAnAddedTemplateIsAnOrdinaryRecordAcrossRenameReplaceAndDelete follows one
// added template through the whole lens lifecycle. Nothing distinguishes it from
// a user-created record: it renames, it is replaced by the current view, and it
// deletes. The only thing the catalog keeps is the stable ID, which is what the
// gallery reads to say a template is already added however far its record has
// since traveled from the definition it started as.
func TestAnAddedTemplateIsAnOrdinaryRecordAcrossRenameReplaceAndDelete(t *testing.T) {
	model := lensOperationApp(t)
	out := model.crossOutliner
	template := addableTemplate(t, model)

	if reason := templateRowReason(t, model, template.ID); reason != "" {
		t.Fatalf("an unadded template is disabled: %q", reason)
	}

	addTemplateAndSettle(t, model, template)
	if reason := templateRowReason(t, model, template.ID); reason != `Already added as "`+template.Name+`"` {
		t.Fatalf("row reason = %q, want the already-added sentence", reason)
	}

	if result := executeLens(
		model, operationLensRename, LensEntity(template.ID), "Due, mine", false,
	); result.Err != nil {
		t.Fatalf("rename the added template: %v", result.Err)
	}
	renamed := model.runtime.Machine().NamedLensByID(template.ID)
	if renamed == nil {
		t.Fatal("the rename moved the record off its stable ID")
	}
	if renamed.Name != "Due, mine" {
		t.Fatalf("renamed record = %q, want the new name", renamed.Name)
	}
	if renamed.Mode != template.Mode || renamed.Query != template.Query {
		t.Fatal("the rename rewrote the projection it was only supposed to relabel")
	}
	if reason := templateRowReason(t, model, template.ID); reason != `Already added as "Due, mine"` {
		t.Fatalf("row reason = %q, want the current label rather than the catalog one", reason)
	}

	// Replace overwrites the definition under the same ID with whatever the view
	// currently is, which is the furthest an added record can travel from its
	// template and still be it.
	out.selectOutlinerMode(ModeInbox)
	out.treeSort = SortState{Field: SortByTitle, Ascending: true}
	if result := executeLens(
		model, operationLensReplace, LensEntity(template.ID), "", true,
	); result.Err != nil {
		t.Fatalf("replace the added template: %v", result.Err)
	}
	replaced := model.runtime.Machine().NamedLensByID(template.ID)
	if replaced == nil {
		t.Fatal("the replace moved the record off its stable ID")
	}
	if replaced.Name != "Due, mine" {
		t.Fatalf("replace renamed the record to %q", replaced.Name)
	}
	if replaced.Mode == template.Mode && replaced.Query == template.Query {
		t.Fatal("the replace did not overwrite the definition with the current view")
	}
	if reason := templateRowReason(t, model, template.ID); reason != `Already added as "Due, mine"` {
		t.Fatalf("row reason = %q, want the replaced record still holding the ID", reason)
	}

	if result := executeLens(
		model, operationLensDelete, LensEntity(template.ID), "", true,
	); result.Err != nil {
		t.Fatalf("delete the added template: %v", result.Err)
	}
	if reason := templateRowReason(t, model, template.ID); reason != "" {
		t.Fatalf("row reason after delete = %q, want the template available again", reason)
	}

	// Re-adding reads the catalog, never the record that was deleted: the
	// definition that comes back is the original one, not the replaced one.
	addTemplateAndSettle(t, model, template)
	readded := model.runtime.Machine().NamedLensByID(template.ID)
	if readded == nil {
		t.Fatal("the re-add saved nothing under the template ID")
	}
	if !reflect.DeepEqual(*readded, template) {
		t.Fatalf("re-added record = %+v, want the original catalog definition %+v", *readded, template)
	}
}

// --- failure phases as the picker reports them -------------------------------

// lensPickerAppInMode opens the lens picker over a named mode, so a refusal can
// be asserted to have left the view underneath it alone.
func lensPickerAppInMode(t *testing.T, mode OutlinerModeID) *AppModel {
	t.Helper()
	model := lensOperationApp(t)
	model.crossOutliner.selectOutlinerMode(mode)
	if cmd := model.openNamedLensPicker(); cmd != nil {
		drainLensPickerCmd(t, model, cmd)
	}
	if model.lensPicker == nil {
		t.Fatalf("openNamedLensPicker left no picker mounted: %v", model.err)
	}
	return model
}

// TestAddTemplateFailuresRetainThePickerAndChangeNothing drives every way an add
// can be refused after the gallery has already drawn the row available. The
// gallery decides availability against the snapshot it renders, and the commit
// decides it again against the snapshot it writes; everything in this table
// happens in between. Each phase must leave the same four things: no gallery, a
// main picker carrying the reason, the durable list untouched, and the view the
// user was looking at unchanged.
func TestAddTemplateFailuresRetainThePickerAndChangeNothing(t *testing.T) {
	for _, phase := range []struct {
		name string
		// arm mutates the world after the gallery is open and the row is selected.
		arm func(t *testing.T, model *AppModel, template config.NamedLens)
		// want is the fragment the retained notice must carry.
		want string
		// rowReason is the gallery's verdict when the same row is revisited.
		rowReason string
	}{
		{
			name: "another lens claims the stable ID",
			arm: func(t *testing.T, model *AppModel, template config.NamedLens) {
				t.Helper()
				claimant := template.Clone()
				claimant.Name = "Claimed by hand"
				setMachineLenses(t, model, append(machineLenses(t, model).Clone(), claimant))
			},
			want:      `already added as "Claimed by hand"`,
			rowReason: `Already added as "Claimed by hand"`,
		},
		{
			name: "another lens claims the name",
			arm: func(t *testing.T, model *AppModel, template config.NamedLens) {
				t.Helper()
				stranger := template.Clone()
				stranger.ID = lensTemplateForeignID
				setMachineLenses(t, model, append(machineLenses(t, model).Clone(), stranger))
			},
			want:      "already exists",
			rowReason: `A lens named "` + "Overdue" + `" already exists`,
		},
		{
			name: "the Outliner is unavailable",
			arm: func(t *testing.T, model *AppModel, _ config.NamedLens) {
				t.Helper()
				model.crossOutliner = nil
			},
			want: "Outliner is unavailable",
			// Nothing durable moved, so the row is exactly as available as it was.
			rowReason: "",
		},
	} {
		t.Run(phase.name, func(t *testing.T) {
			model := lensPickerAppInMode(t, ModeInbox)
			template := addableTemplate(t, model)
			if template.Name != "Overdue" {
				t.Fatalf("the first addable template is %q; the name-collision row is written for Overdue", template.Name)
			}
			openAddTemplateGalleryFromPicker(t, model)
			selectGalleryRow(t, model, template.ID)
			phase.arm(t, model, template)
			before := lensNames(machineLenses(t, model))

			pressLens(t, model, "enter")

			if model.operationValuePicker != nil {
				t.Error("a refused add left the gallery mounted")
			}
			if model.lensPicker == nil {
				t.Fatal("a refused add closed the main picker")
			}
			if notice := model.lensPicker.Notice(); !strings.Contains(notice, phase.want) {
				t.Errorf("picker notice = %q, want it to name %q", notice, phase.want)
			}
			if got := lensNames(machineLenses(t, model)); !slices.Equal(got, before) {
				t.Errorf("durable list = %v, want the untouched %v", got, before)
			}
			if out := model.crossOutliner; out != nil {
				if out.hasActiveLens() {
					t.Error("a refused add applied a lens")
				}
				if out.pendingLens != nil {
					t.Error("a refused add left a population load armed")
				}
				if out.primaryMode != ModeInbox {
					t.Errorf("current view = %v, want the untouched %v", out.primaryMode, ModeInbox)
				}
			}

			// The reason is retained on the surface that raised it, and the gallery
			// the same row reopens now reports the state the commit refused against.
			pressLens(t, model, "enter")
			if model.operationValuePicker == nil {
				t.Fatal("retrying the add raised no gallery")
			}
			if got := galleryRow(t, model.operationValuePicker, template.ID).DisabledReason; got != phase.rowReason {
				t.Errorf("revisited row reason = %q, want %q", got, phase.rowReason)
			}
		})
	}
}

// TestAddTemplateDiskCommitFailureRetainsThePicker is the durable half of the
// same rule. The commit is what publishes the record and the application runs
// only after it returns, so a write that cannot land leaves the machine, the
// cached snapshot, and the current view exactly as they were — and says so on the
// picker rather than anywhere else.
func TestAddTemplateDiskCommitFailureRetainsThePicker(t *testing.T) {
	model := lensPickerAppInMode(t, ModeInbox)
	dataDir := t.TempDir()
	project := models.Project{ID: "alpha", Name: "alpha", Slug: "alpha"}
	model.runtime = testRuntimeAt(dataDir, testConfigWithProjects(&project))
	template := addableTemplate(t, model)
	openAddTemplateGalleryFromPicker(t, model)
	selectGalleryRow(t, model, template.ID)

	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.Chmod(home, 0o500); err != nil {
		t.Skipf("cannot make the runtime root read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(home, 0o700) })

	before := model.runtime.Machine()
	pressLens(t, model, "enter")

	if model.runtime.Machine() != before {
		t.Skip("the runtime root stayed writable; the failure path is untested here")
	}
	if model.operationValuePicker != nil {
		t.Error("a failed commit left the gallery mounted")
	}
	if model.lensPicker == nil {
		t.Fatal("a failed commit closed the main picker")
	}
	if model.lensPicker.Notice() == "" {
		t.Error("a failed commit reported nothing on the picker")
	}
	if before.NamedLensByID(template.ID) != nil {
		t.Error("a failed commit added the template to the cached snapshot")
	}
	if out := model.crossOutliner; out.hasActiveLens() || out.primaryMode != ModeInbox {
		t.Error("a failed commit applied the template anyway")
	}
}
