package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/status"
)

// A committed rename replaces the picker with one built from the renamed
// workspace, and that replacement is constructed seeded from the facet that is
// still narrowing the view. The pending multi-select is therefore the only
// carrier of what the reader actually chose, and an empty one is a choice like
// any other: unchecking every row and then renaming must not hand the active
// facet back as if the unchecking never happened. Both filter pickers are driven
// here through real keys, because the seed and the remap are shared code and a
// regression in either would be invisible on the other.

// vocabularyRenamePendingEnv drives the rich fixture with one extra context
// seeded on disk, so the fc facet has two vocabulary rows for the same reason f#
// already does: one row to check and clear, and a different one to rename.
func vocabularyRenamePendingEnv(t *testing.T) *UITestDriver {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatalf("fixture %s: %v", FixtureRich, err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	project := projectByFixtureName(driver.App().domain.runtime.TaskWorkspaceProjects(), "Rich Project")
	if project == nil {
		t.Fatal("the rich fixture no longer holds its project")
	}
	setFixtureTaskContextOnDisk(t, project, "Fix redirect loop after logout", "ops")

	driver.dispatch(status.ViewDidAppearMsg{})
	driver.drain()
	return driver
}

// setFixtureTaskContextOnDisk writes one context onto one task's durable bytes,
// through the same load/save pair the fixtures use, so the running app picks it
// up as an ordinary external edit.
func setFixtureTaskContextOnDisk(t *testing.T, project *models.Project, title, context string) {
	t.Helper()
	loaded := diskCopyOfProject(t, project)
	for _, task := range loaded.Tasks {
		if task.Title != title {
			continue
		}
		task.Context = context
		if err := storage.SaveProject(loaded); err != nil {
			t.Fatalf("SaveProject(%q): %v", project.Name, err)
		}
		return
	}
	t.Fatalf("project %q has no task %q to give a context", project.Name, title)
}

// movePickerCursorToID walks the open picker's cursor with the browse key a
// reader would use, so the row a rename starts from is reached the way the
// workflow is actually entered rather than by writing the cursor.
func movePickerCursorToID(t *testing.T, driver *UITestDriver, id listsurface.ListItemID) {
	t.Helper()
	for step := 0; step <= len(openPickerSurface(t, driver).Items()); step++ {
		if item, ok := openPickerSurface(t, driver).CurrentItem(); ok && item.ID == id {
			return
		}
		sendKeys(t, driver, "down")
	}
	t.Fatalf("the open picker never reached row %s", id)
}

func openPickerSurface(t *testing.T, driver *UITestDriver) *listsurface.ListSurfaceModel {
	t.Helper()
	picker := driver.App().operations.ValuePicker
	if picker == nil {
		t.Fatal("no value picker is open")
	}
	return picker.Surface()
}

func assertPendingSelection(t *testing.T, driver *UITestDriver, want []listsurface.ListItemID, when string) {
	t.Helper()
	got := openPickerSurface(t, driver).SelectedIDs()
	if !slices.Equal(got, want) {
		t.Fatalf("%s: pending selection = %v, want %v", when, got, want)
	}
}

// TestVocabularyRenameKeepsAClearedPendingSelectionCleared is the tag half. The
// seed is real — the facet is still narrowing the view throughout — so the
// assertion that nothing is pending after the rename is a statement about the
// clearing surviving the rebuild rather than about there having been no seed.
func TestVocabularyRenameKeepsAClearedPendingSelectionCleared(t *testing.T) {
	driver := vocabularyRenamePendingEnv(t)

	sendKeys(t, driver, "f", "#")
	movePickerCursorToID(t, driver, "tag:backend")
	sendKeys(t, driver, "space", "enter")
	if tags := driver.App().crossOutliner.Filter.Tags; !tags["backend"] {
		t.Fatalf("active tag filter = %v, want #backend applied", tags)
	}

	sendKeys(t, driver, "f", "#")
	assertPendingSelection(t, driver, []listsurface.ListItemID{"tag:backend"}, "reopened on the active facet")
	movePickerCursorToID(t, driver, "tag:backend")
	sendKeys(t, driver, "space")
	assertPendingSelection(t, driver, nil, "after unchecking the seeded row")

	movePickerCursorToID(t, driver, "tag:security")
	sendKeys(t, driver, "e")
	sendText(driver, "-renamed")
	sendKeys(t, driver, "enter", "y")

	assertPendingSelection(t, driver, nil, "after the rename rebuilt the picker")
	// The rename writes tasks, never the facet: unchecking is an intention the
	// ordinary Enter applies, so until then the view is narrowed exactly as before.
	if tags := driver.App().crossOutliner.Filter.Tags; !tags["backend"] || len(tags) != 1 {
		t.Fatalf("active tag filter = %v, want #backend still applied", tags)
	}
	if item, ok := openPickerSurface(t, driver).CurrentItem(); !ok || item.ID != "tag:security-renamed" {
		t.Fatalf("cursor row = %+v, want the row the rename produced", item)
	}

	// Enter is what spends the cleared choice, and it drops the filter rather than
	// reapplying the seed the rebuild would have restored.
	sendKeys(t, driver, "esc", "enter")
	if tags := driver.App().crossOutliner.Filter.Tags; len(tags) != 0 {
		t.Fatalf("active tag filter after applying the cleared selection = %v, want none", tags)
	}
}

// TestVocabularyRenameKeepsAClearedPendingContextSelectionCleared is the context
// half of the same claim: the two pickers share the host, the seed and the
// remap, so the fix has to hold on the axis the finding did not name.
func TestVocabularyRenameKeepsAClearedPendingContextSelectionCleared(t *testing.T) {
	driver := vocabularyRenamePendingEnv(t)

	sendKeys(t, driver, "f", "c")
	movePickerCursorToID(t, driver, "filter-context:ops")
	sendKeys(t, driver, "space", "enter")
	if contexts := driver.App().crossOutliner.Filter.Contexts; !contexts["ops"] {
		t.Fatalf("active context filter = %v, want ##ops applied", contexts)
	}

	sendKeys(t, driver, "f", "c")
	assertPendingSelection(t, driver, []listsurface.ListItemID{"filter-context:ops"}, "reopened on the active facet")
	movePickerCursorToID(t, driver, "filter-context:ops")
	sendKeys(t, driver, "space")
	assertPendingSelection(t, driver, nil, "after unchecking the seeded row")

	movePickerCursorToID(t, driver, "filter-context:platform")
	sendKeys(t, driver, "e")
	sendText(driver, "-renamed")
	sendKeys(t, driver, "enter", "y")

	assertPendingSelection(t, driver, nil, "after the rename rebuilt the picker")
	if contexts := driver.App().crossOutliner.Filter.Contexts; !contexts["ops"] || len(contexts) != 1 {
		t.Fatalf("active context filter = %v, want ##ops still applied", contexts)
	}
	if item, ok := openPickerSurface(t, driver).CurrentItem(); !ok || item.ID != "filter-context:platform-renamed" {
		t.Fatalf("cursor row = %+v, want the row the rename produced", item)
	}
}

// TestVocabularyRenameStillRestoresANonemptyPendingSelection is the other side of
// the unconditional write: making the empty set load-bearing must not cost the
// remap that carries a checked source row onto the destination it became.
func TestVocabularyRenameStillRestoresANonemptyPendingSelection(t *testing.T) {
	driver := vocabularyRenamePendingEnv(t)

	sendKeys(t, driver, "f", "#")
	movePickerCursorToID(t, driver, "tag:backend")
	sendKeys(t, driver, "space")
	movePickerCursorToID(t, driver, "tag:security")
	sendKeys(t, driver, "space")
	assertPendingSelection(t, driver, []listsurface.ListItemID{"tag:backend", "tag:security"}, "before the rename")

	sendKeys(t, driver, "e")
	sendText(driver, "-renamed")
	sendKeys(t, driver, "enter", "y")

	assertPendingSelection(
		t, driver,
		[]listsurface.ListItemID{"tag:backend", "tag:security-renamed"},
		"after the rename rebuilt the picker",
	)
}
