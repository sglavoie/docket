package tui

import (
	"testing"

	"github.com/sglavoie/docket/internal/tui/notes"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/route"
)

// noteTierFixtureApp searches the rich fixture for a query both a task and a
// note answer and parks the cursor on the first note row, which is the only
// cursor Open Note is applicable over.
func noteTierFixtureApp(t *testing.T) (*AppModel, notes.NoteRowIdentity) {
	t.Helper()
	app := headerTargetAuditApp(t)
	outlinerView := app.crossOutliner
	if outlinerView == nil {
		t.Fatal("fixture app has no Outliner")
	}
	searchRankedList(outlinerView, "launch")
	for index, row := range outlinerView.Rows {
		if !row.IsNoteRow {
			continue
		}
		outlinerView.Cursor = index
		return app, row.NoteRow
	}
	t.Fatal("the fixture search returned no note row")
	return nil, notes.NoteRowIdentity{}
}

// TestOpenNoteIsTheFourthEnterActOnANoteRow drives the descriptor the way the
// shell does: a note-row cursor resolves Enter to Open Note rather than to one
// of the three acts that read an entity, and dispatch answers with the message
// that both opens the manager and positions it on the row the cursor named.
func TestOpenNoteIsTheFourthEnterActOnANoteRow(t *testing.T) {
	model, identity := noteTierFixtureApp(t)
	ctx := model.operationCommandContext()
	ctx.View = presentation.ViewCrossProjectOutliner
	if !ctx.Entity.HasNoteCursor {
		t.Fatal("a note-row cursor did not report HasNoteCursor")
	}

	registry := model.GetOperationRegistry()
	binding, _, ok := registry.InputBindingForKey(operation.InputStrategySimplified, "enter", ctx)
	if !ok || binding.OperationID != operation.OperationCoreOpenNote {
		t.Fatalf("note row Enter = %s available=%t, want %s",
			binding.OperationID, ok, operation.OperationCoreOpenNote)
	}
	// The three acts that read an entity must all refuse, or the shared key
	// would resolve by declaration order rather than by cursor.
	for _, id := range []operation.OperationID{
		operation.OperationCoreFocusSubtree, operation.OperationCoreZoomSection, operation.OperationCoreActivateGroup,
	} {
		descriptor, found := registry.Descriptor(id)
		if !found {
			t.Fatalf("operation %s is not registered", id)
		}
		if descriptor.Applicability != nil && descriptor.Applicability(ctx) {
			t.Fatalf("%s stayed applicable over a note row", id)
		}
	}

	descriptor, found := registry.Descriptor(operation.OperationCoreOpenNote)
	if !found {
		t.Fatal("Open Note is not registered")
	}
	result := model.executeCoreInputOperation(operation.OperationInvocation{Descriptor: descriptor, Context: ctx})
	if result.Err != nil {
		t.Fatalf("Open Note refused a note row: %v", result.Err)
	}
	message, isShow := result.Message.(notes.ShowNotesManagerMsg)
	if !isShow {
		t.Fatalf("Open Note emitted %T, want notes.ShowNotesManagerMsg", result.Message)
	}
	if message.Scope != route.AllNotesScope() {
		t.Fatalf("Open Note opened scope %+v, want every note reachable", message.Scope)
	}
	if message.Focus != identity {
		t.Fatalf("Open Note focused %+v, want the row the cursor named %+v",
			message.Focus, identity)
	}

	// Off a note row the act is gone entirely, which is what keeps the footer
	// from advertising it over the ordinary projection.
	model.crossOutliner.ClearSearch()
	plain := model.operationCommandContext()
	plain.View = presentation.ViewCrossProjectOutliner
	if plain.Entity.HasNoteCursor {
		t.Fatal("a cleared search still reported a note cursor")
	}
	if descriptor.Applicability(plain) {
		t.Fatal("Open Note stayed applicable with no note row in sight")
	}
}
