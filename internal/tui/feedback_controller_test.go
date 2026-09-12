package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

// TestFeedbackClusterHasOneHome pins the status and feedback cluster to its
// single named AppModel field. The line, the error beside it, the undo callback,
// the transaction correlation, the flash and the two ambient status-strip inputs
// are one answer to "what is the shell saying right now"; a second home on the
// god struct is how two of them would come to disagree.
func TestFeedbackClusterHasOneHome(t *testing.T) {
	t.Parallel()

	clustered := map[string]bool{
		"err":                    true,
		"statusMessage":          true,
		"statusIsInfo":           true,
		"statusUpdatedAt":        true,
		"statusUndoCb":           true,
		"statusExpiresAt":        true,
		"statusTransactionID":    true,
		"statusRecoveryFallback": true,
		"statusSeq":              true,
		"statusSeverity":         true,
		"footerFlash":            true,
		"footerFlashSeq":         true,
		"outstandingConflicts":   true,
		"hideAdvisories":         true,
	}
	assertAppModelClusterHasOneHome(t, "feedback", clustered)
}

// TestFeedbackControllerKeepsTheShellStatusNamespaceDistinct is the rename guard
// the extraction is held to. InteractiveViewState and BaseView declare
// statusMessage, err, width and height of their own, and merging the two
// namespaces would be a behavior change: a view owns a status line the shell
// never writes, and footerFlashProbe compares the two independently. The
// resolution is qualification, not renaming, so the cluster must still declare
// its own message and err rather than borrowing a view's.
func TestFeedbackControllerKeepsTheShellStatusNamespaceDistinct(t *testing.T) {
	t.Parallel()

	fields := feedbackControllerFields(t)
	for _, required := range []string{"message", "err"} {
		if !fields[required] {
			t.Errorf("feedbackController must declare %s: the shell's status line is qualified through the cluster, never merged into a view's", required)
		}
	}
}

func feedbackControllerFields(t *testing.T) map[string]bool {
	t.Helper()

	parsed, err := parser.ParseFile(token.NewFileSet(), shellSourceFile(t, "feedback_controller.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse feedback_controller.go: %v", err)
	}

	fields := map[string]bool{}
	ast.Inspect(parsed, func(node ast.Node) bool {
		spec, isType := node.(*ast.TypeSpec)
		if !isType || spec.Name.Name != "feedbackController" {
			return true
		}
		controller, isStruct := spec.Type.(*ast.StructType)
		if !isStruct {
			return true
		}
		for _, field := range controller.Fields.List {
			for _, name := range field.Names {
				fields[name.Name] = true
			}
		}
		return false
	})
	if len(fields) == 0 {
		t.Fatal("feedbackController declares no fields")
	}
	return fields
}

// TestFeedbackControllerAnnounceReleasesThePreviousUndo pins why a plain status
// may not inherit an armed callback: a later `u` would silently invoke the
// previous destructive action's undo while unrelated text is on screen.
func TestFeedbackControllerAnnounceReleasesThePreviousUndo(t *testing.T) {
	t.Parallel()

	now := time.Now()
	var controller feedbackController
	controller.announce("Task deleted · u undo", true, now)
	controller.armUndo(func() tea.Cmd { return nil }, now.Add(UndoToastTimeout))
	controller.correlate("txn-1", "Task deleted", "")
	if !controller.undoLive(now) {
		t.Fatal("an armed toast is live inside its window")
	}

	controller.announce("Sorted by due date", true, now)
	if controller.undoLive(now) {
		t.Fatal("announce must release the previous undo callback")
	}
	if controller.transactionID != "" || controller.recoveryFallback != "" {
		t.Fatalf("announce must release the correlation, got %q/%q", controller.transactionID, controller.recoveryFallback)
	}
}

// TestFeedbackControllerCorrelatesOnlyTheLiveToast pins the delayed
// origin-history guard: a result may decorate the toast it was recorded for and
// no other, and only while that toast is still offered.
func TestFeedbackControllerCorrelatesOnlyTheLiveToast(t *testing.T) {
	t.Parallel()

	now := time.Now()
	var controller feedbackController
	controller.announce("Deleted 3 tasks · u undo", true, now)
	controller.armUndo(func() tea.Cmd { return nil }, now.Add(UndoToastTimeout))
	controller.correlate("txn-1", "Deleted 3 tasks", "")

	if !controller.correlates("txn-1", now) {
		t.Fatal("the live toast correlates with the transaction it was recorded for")
	}
	if controller.correlates("txn-2", now) {
		t.Fatal("a different transaction must not decorate this toast")
	}
	if controller.correlates("", now) {
		t.Fatal("an absent transaction correlates with nothing")
	}
	if controller.correlates("txn-1", now.Add(2*UndoToastTimeout)) {
		t.Fatal("a lapsed toast correlates with nothing")
	}
}

// TestFeedbackControllerSeparatesUndoLiveFromUndoLapsed pins that the two
// answers are complements over an armed toast and both false without one, which
// is what lets a timeout tick clear the toast it was scheduled for while leaving
// a newer one alone.
func TestFeedbackControllerSeparatesUndoLiveFromUndoLapsed(t *testing.T) {
	t.Parallel()

	now := time.Now()
	var controller feedbackController
	if controller.undoLive(now) || controller.undoLapsed(now) {
		t.Fatal("an unarmed toast is neither live nor lapsed")
	}

	controller.armUndo(func() tea.Cmd { return nil }, now.Add(UndoToastTimeout))
	if !controller.undoLive(now) || controller.undoLapsed(now) {
		t.Fatal("an armed toast inside its window is live and not lapsed")
	}

	later := now.Add(2 * UndoToastTimeout)
	if controller.undoLive(later) || !controller.undoLapsed(later) {
		t.Fatal("an armed toast past its deadline is lapsed and not live")
	}
}

// TestFeedbackControllerClearArmsNothing pins the flash rule from the cluster's
// side: only the setters advance the sequence, so clearing text arms no flash.
func TestFeedbackControllerClearArmsNothing(t *testing.T) {
	t.Parallel()

	var controller feedbackController
	controller.announce("Task started", false, time.Now())
	controller.recordSeverity(flashSuccess)
	armed := controller.seq

	controller.clear()
	if controller.seq != armed {
		t.Fatalf("clear advanced the status sequence to %d, so cleared text would arm a flash", controller.seq)
	}
	if controller.severity != flashNone || controller.message != "" || controller.err != nil {
		t.Fatalf("clear must drop the text, the severity and the error, got %+v", controller)
	}
}

// TestFeedbackControllerStaleGuardsAScheduledClear pins that a clear scheduled
// before newer text no longer applies, while a zero deadline stays
// unconditional.
func TestFeedbackControllerStaleGuardsAScheduledClear(t *testing.T) {
	t.Parallel()

	scheduled := time.Now()
	var controller feedbackController
	controller.announce("Older", true, scheduled.Add(-time.Second))
	if controller.stale(scheduled) {
		t.Fatal("text older than the scheduled clear is not stale")
	}

	controller.announce("Newer", true, scheduled.Add(time.Second))
	if !controller.stale(scheduled) {
		t.Fatal("text newer than the scheduled clear overtakes it")
	}
	if controller.stale(time.Time{}) {
		t.Fatal("a zero deadline is an unconditional clear")
	}
}

// TestFeedbackControllerAppendDetailKeepsRecoveryControlsTrailing pins the
// canonical order and the no-op: unverified feedback must not come to resemble a
// correlated recovery toast.
func TestFeedbackControllerAppendDetailKeepsRecoveryControlsTrailing(t *testing.T) {
	t.Parallel()

	var plain feedbackController
	plain.announce("Archived Website", false, time.Now())
	plain.appendDetail("3 tasks parked")
	if plain.message != "Archived Website" {
		t.Fatalf("an unarmed status must not gain recovery detail, got %q", plain.message)
	}

	controller := feedbackController{}
	controller.announce("Archive: Website · u undo · Ctrl+O origin", true, time.Now())
	controller.armUndo(func() tea.Cmd { return nil }, time.Now().Add(UndoToastTimeout))
	controller.appendDetail("3 tasks parked")
	const want = "Archive: Website · 3 tasks parked · u undo · Ctrl+O origin"
	if controller.message != want {
		t.Fatalf("message = %q, want %q", controller.message, want)
	}
}

// TestFeedbackControllerFlashSequenceRejectsASupersededTick pins that a tick
// belongs to the flash that armed it, and that the last frame retires the
// animation rather than scheduling past it.
func TestFeedbackControllerFlashSequenceRejectsASupersededTick(t *testing.T) {
	t.Parallel()

	var controller feedbackController
	first := controller.armFlash(flashInfo)
	second := controller.armFlash(flashError)
	if second == first {
		t.Fatalf("armFlash reused generation %d, so a stale tick stays acceptable", second)
	}
	if controller.advanceFlash(first) {
		t.Fatal("a tick from a superseded flash advances nothing")
	}

	for frame := 1; frame < footerFlashFrames; frame++ {
		if !controller.advanceFlash(second) {
			t.Fatalf("frame %d owes another tick", frame)
		}
	}
	if controller.advanceFlash(second) {
		t.Fatal("the last frame retires the flash rather than scheduling past it")
	}
	if controller.flash.active() {
		t.Fatal("a retired flash is inactive")
	}
}

// TestFeedbackControllerAmbientAccessorsAreNilSafe pins the two injected
// accessors: a nil cluster reads as showing and as zero, so nothing here can
// turn a missing shell into a silent one or assert a count it cannot prove.
func TestFeedbackControllerAmbientAccessorsAreNilSafe(t *testing.T) {
	t.Parallel()

	var absent *feedbackController
	if absent.advisoriesHidden() {
		t.Fatal("a nil cluster reads as showing advisories")
	}
	if absent.outstandingConflictCount() != 0 {
		t.Fatal("a nil cluster reports no outstanding conflicts")
	}

	controller := &feedbackController{hideAdvisories: true, outstandingConflicts: 2}
	if !controller.advisoriesHidden() || controller.outstandingConflictCount() != 2 {
		t.Fatalf("the accessors report the live setting, got %+v", *controller)
	}
}

// TestFooterFlashArmingStaysInTheOneDeferredClosure pins the arming site rather
// than the arming rule. The cluster now advances the sequence and holds the
// frames, which is exactly the seam a second arming site could be added at:
// AppModel.Update must keep exactly one defer and armFooterFlashSince must be
// called from nowhere else, since a second defer would run first under LIFO and
// see neither the recovery nor the assigned command.
func TestFooterFlashArmingStaysInTheOneDeferredClosure(t *testing.T) {
	t.Parallel()

	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, shellSourceFile(t, "app_update.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse app_update.go: %v", err)
	}

	defers := 0
	ast.Inspect(parsed, func(node ast.Node) bool {
		function, isFunction := node.(*ast.FuncDecl)
		if !isFunction || function.Name.Name != "Update" || function.Recv == nil {
			return true
		}
		ast.Inspect(function.Body, func(inner ast.Node) bool {
			if _, isDefer := inner.(*ast.DeferStmt); isDefer {
				defers++
			}
			return true
		})
		return false
	})
	if defers != 1 {
		t.Errorf("AppModel.Update has %d defer statements, want exactly 1", defers)
	}

	packageDir := filepath.Dir(shellSourceFile(t, "app_update.go"))
	entries, err := os.ReadDir(packageDir)
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, err := os.ReadFile(filepath.Join(packageDir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if !strings.Contains(string(source), "armFooterFlashSince") {
			continue
		}
		if name != "app_update.go" && name != "app_view_status.go" {
			t.Errorf("%s names armFooterFlashSince; arming stays in the one deferred closure in AppModel.Update", name)
		}
	}
}
