package tui

import (
	"slices"
	"testing"
)

func TestNotesOperationCatalogMatchesDirectActions(t *testing.T) {
	app := parityAuditApp(t)
	// Delete carries no key: it is destructive, so the Space sheet is its one
	// path and the empty spelling is what the one-path rule demands.
	wantKeys := map[OperationID]string{
		operationNoteOpen:      "enter",
		operationNoteRename:    "e",
		operationNoteMove:      "%",
		operationNoteCopyTitle: "y",
		operationNoteCopyPath:  "Y",
		operationNoteDelete:    "",
	}
	descriptors := noteOperationDescriptors()
	if len(descriptors) != len(wantKeys) {
		t.Fatalf("note operation count = %d, want %d", len(descriptors), len(wantKeys))
	}
	handled := (&NotesViewModel{}).HandledKeys()
	for _, descriptor := range descriptors {
		key, ok := wantKeys[descriptor.ID]
		if !ok {
			t.Errorf("unexpected note operation %s", descriptor.ID)
			continue
		}
		switch {
		case key == "" && !slices.Contains(notesActionSheetOperations, descriptor.ID):
			t.Errorf("note operation %s has no direct key and no Space sheet row", descriptor.ID)
		case key != "" && !slices.Contains(handled, key):
			t.Errorf("note operation %s has no Notes direct key %q", descriptor.ID, key)
		}
		registered, ok := app.GetOperationRegistry().Descriptor(descriptor.ID)
		if !ok || registered.Label != descriptor.Label {
			t.Errorf("note operation %s registry descriptor = %#v, want label %q", descriptor.ID, registered, descriptor.Label)
		}
		if len(app.operationExecutor.bindings[descriptor.ID]) == 0 {
			t.Errorf("note operation %s has no executable binding", descriptor.ID)
		}
	}
}

func TestSpaceProjectionUsesTheDeclaredTaskManifest(t *testing.T) {
	app := parityAuditApp(t)
	registry := app.GetOperationRegistry()
	executor := app.operationExecutor

	sheetContext := taskActionParityContext(t, app)
	descriptors := TaskActionSheetDescriptors(registry, sheetContext)
	if len(descriptors) == 0 {
		t.Fatal("the Space sheet offered no action for a live task")
	}
	seen := make(map[OperationID]bool)
	for _, descriptor := range descriptors {
		if seen[descriptor.ID] {
			t.Errorf("Space sheet lists %s twice", descriptor.ID)
		}
		seen[descriptor.ID] = true
		if len(executor.bindings[descriptor.ID]) == 0 {
			t.Errorf("Space sheet lists unexecutable operation %s", descriptor.ID)
		}
		entry, manifested := taskOperationManifestEntryFor(descriptor.ID)
		if !manifested {
			t.Errorf("Space sheet lists %s, which the task manifest does not own", descriptor.ID)
			continue
		}
		if entry.DirectOnly {
			t.Errorf("Space sheet repeats direct-only operation %s", descriptor.ID)
		}
		// Manifest actions reach users through the row they act on.
	}

	// The menu catalog excludes direct-only entries whose key the footer already
	// advertises.
	rowActions := taskOperationManifestIDs()
	for _, id := range rowActions {
		if _, ok := registry.Descriptor(id); !ok {
			t.Errorf("task manifest advertises unregistered operation %s", id)
		}
	}
	strategy, ok := registry.InputStrategy(InputStrategySimplified)
	if !ok {
		t.Fatal("simplified input strategy is not registered")
	}
	for _, entry := range taskOperationManifest() {
		if !entry.DirectOnly {
			continue
		}
		if slices.Contains(rowActions, entry.ID) {
			t.Errorf("direct-only operation %s is still a row action", entry.ID)
		}
		// A removed projection has to leave a key behind, or the operation becomes
		// unreachable instead of faster.
		if displayKeysForOperation(strategy, entry.ID) == "" {
			t.Errorf("direct-only operation %s has no key in the input strategy", entry.ID)
		}
	}
}
