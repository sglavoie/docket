package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/control"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/presentation"
)

func (e *reviewAsyncEnv) hasTitle(t *testing.T, title string) bool {
	t.Helper()
	for _, task := range e.env.reload(t, 0).Tasks {
		if task.Title == title {
			return true
		}
	}
	return false
}

// TestControlUndoTakesBackTheNewestCapture: the capture reply names its
// transaction, and naming it while it is still the newest undo entry undoes it
// through the u key's own ledger path.
func TestControlUndoTakesBackTheNewestCapture(t *testing.T) {
	env := controlEnv(t, reviewSettledTask(t, 1, "existing"))

	captured := env.request(t, control.Request{Op: control.OpCapture, Text: "Buy milk"})
	if !captured.OK || captured.Transaction == "" {
		t.Fatalf("capture reply = %+v", captured)
	}
	reply := env.request(t, control.Request{Op: control.OpUndo, Text: captured.Transaction})
	if !reply.OK || !strings.HasPrefix(reply.Message, "Undid: ") {
		t.Fatalf("undo reply = %+v", reply)
	}
	if env.hasTitle(t, "Buy milk") {
		t.Fatal("the undone capture is still stored")
	}

	// Undone, it is no longer the newest undo entry.
	if reply := env.request(t, control.Request{Op: control.OpUndo, Text: captured.Transaction}); reply.OK || reply.Message != controlUndoSuperseded {
		t.Fatalf("a second undo answered %+v", reply)
	}
}

// TestControlUndoRefusesWhenSomethingNewerChanged: a capture superseded by a
// newer entry is refused, and the newer change is left alone.
func TestControlUndoRefusesWhenSomethingNewerChanged(t *testing.T) {
	env := controlEnv(t, reviewSettledTask(t, 1, "existing"))

	first := env.request(t, control.Request{Op: control.OpCapture, Text: "First"})
	if second := env.request(t, control.Request{Op: control.OpCapture, Text: "Second"}); !second.OK {
		t.Fatalf("capture reply = %+v", second)
	}
	reply := env.request(t, control.Request{Op: control.OpUndo, Text: first.Transaction})
	if reply.OK || reply.Message != "Something newer was changed in Docket; undo it there" {
		t.Fatalf("undo reply = %+v", reply)
	}
	if !env.hasTitle(t, "First") || !env.hasTitle(t, "Second") {
		t.Fatal("a refused undo changed the workspace")
	}
}

// TestControlUndoRefusesConcurrentNewerTransaction: a same-machine commit that
// lands after the unlocked preview of the named capture, but before the guarded
// restore takes the ledger lock, is not undone in its place. The restore
// refuses, and both captures and both stacks stay as they were.
func TestControlUndoRefusesConcurrentNewerTransaction(t *testing.T) {
	env := controlEnv(t, reviewSettledTask(t, 1, "existing"))

	first := env.request(t, control.Request{Op: control.OpCapture, Text: "First"})
	if !first.OK || first.Transaction == "" {
		t.Fatalf("capture reply = %+v", first)
	}
	ledger := workspaceLedgerFromConfig(env.model.domain.runtime)
	undosBefore, err := ledger.UndoCount(storage.GlobalScope())
	if err != nil {
		t.Fatalf("undo count: %v", err)
	}
	var competing string
	controlUndoAfterPreview = func() {
		replies := make(chan control.Reply, 1)
		cmd := env.model.controlCapture("Second", replies)
		if cmd == nil {
			t.Fatal("the competing capture produced no commit")
		}
		cmd()
		reply := <-replies
		if !reply.OK || reply.Transaction == "" {
			t.Fatalf("competing capture reply = %+v", reply)
		}
		competing = reply.Transaction
	}
	t.Cleanup(func() { controlUndoAfterPreview = nil })

	reply := env.request(t, control.Request{Op: control.OpUndo, Text: first.Transaction})
	controlUndoAfterPreview = nil
	if competing == "" {
		t.Fatal("the competing commit never ran between preview and restore")
	}
	if reply.OK || reply.Message != controlUndoSuperseded {
		t.Fatalf("undo reply = %+v", reply)
	}
	if !env.hasTitle(t, "First") || !env.hasTitle(t, "Second") {
		t.Fatal("a refused undo changed the workspace")
	}
	newest, ok, err := ledger.SelectNewestUndo(storage.GlobalScope())
	if err != nil || !ok || newest.TransactionID != competing {
		t.Fatalf("newest undo entry = %+v, %v, %v; want the competing commit %s", newest, ok, err, competing)
	}
	undos, err := ledger.UndoCount(storage.GlobalScope())
	if err != nil || undos != undosBefore+1 {
		t.Fatalf("undo count = %d, %v; want %d", undos, err, undosBefore+1)
	}
	redos, err := ledger.RedoCount(storage.GlobalScope())
	if err != nil || redos != 0 {
		t.Fatalf("redo count = %d, %v; want 0", redos, err)
	}
}

func TestControlUndoRefusesWhileTheUserIsMidInteraction(t *testing.T) {
	env := controlEnv(t, reviewSettledTask(t, 1, "existing"))

	captured := env.request(t, control.Request{Op: control.OpCapture, Text: "Buy milk"})
	setTestPresentationPath(env.model, presentation.ViewCrossProjectOutliner, presentation.ViewNotes)
	reply := env.request(t, control.Request{Op: control.OpUndo, Text: captured.Transaction})
	if reply.OK || reply.Message != "Docket is busy: finish what is open in Docket first" {
		t.Fatalf("undo reply = %+v", reply)
	}
	if !env.hasTitle(t, "Buy milk") {
		t.Fatal("a refused undo changed the workspace")
	}
}
