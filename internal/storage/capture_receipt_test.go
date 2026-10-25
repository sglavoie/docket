package storage

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/uuid"
)

func testCaptureReceipt() *CaptureReceipt {
	return &CaptureReceipt{CaptureSubmission: CaptureSubmission{ID: uuid.NewString(), Digest: fmt.Sprintf("%x", sha256.Sum256([]byte("capture draft")))}, Titles: []string{"Captured task"}}
}

func TestCaptureReceiptSurvivesRestartUndoAndRepeatedDelivery(t *testing.T) {
	w := newSyncWorkspace(t)
	receipt := testCaptureReceipt()
	w.alpha.Tasks[0].Title = "Captured task"
	result, err := w.service.Commit(ChangeSet{Project: w.alpha, Capture: receipt, UndoAction: "Add Task"})
	if err != nil {
		t.Fatal(err)
	}
	// The same capture works with sync enabled; its private receipt must not be
	// classified as a user artifact, exported, or placed in undo images.
	restarted := NewCommitService(w.transactionsDir, []string{w.root})
	got, err := restarted.LookupCaptureReceipt(receipt.CaptureSubmission)
	if err != nil || got == nil || got.TransactionID != result.TransactionID {
		t.Fatalf("receipt=%+v err=%v", got, err)
	}
	if _, err = w.ledger.Undo(LedgerScope{}); err != nil {
		t.Fatal(err)
	}
	got, err = restarted.LookupCaptureReceipt(receipt.CaptureSubmission)
	if err != nil || got == nil {
		t.Fatalf("undo removed receipt: %+v %v", got, err)
	}
	_, err = restarted.Commit(ChangeSet{Project: w.alpha, Capture: receipt})
	var duplicate *CaptureAlreadyCommitted
	if !errors.As(err, &duplicate) || duplicate.Receipt.TransactionID != result.TransactionID {
		t.Fatalf("retry: %v", err)
	}
	receipt.Digest = fmt.Sprintf("%x", sha256.Sum256([]byte("different draft")))
	if _, err = restarted.LookupCaptureReceipt(receipt.CaptureSubmission); err == nil {
		t.Fatal("accepted identity reuse with different text")
	}
}

func TestCaptureReceiptConcurrentRetriesCommitOnce(t *testing.T) {
	svc, root := newCommitTestService(t)
	project := newCommitTestProject(t, root, "capture", "Captured task")
	receipt := testCaptureReceipt()
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Commit(ChangeSet{Project: project, Capture: receipt})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	successes, duplicates := 0, 0
	for err := range errs {
		var duplicate *CaptureAlreadyCommitted
		switch {
		case err == nil:
			successes++
		case errors.As(err, &duplicate):
			duplicates++
		default:
			t.Fatal(err)
		}
	}
	if successes != 1 || duplicates != 7 {
		t.Fatalf("commits=%d duplicates=%d", successes, duplicates)
	}
}

func TestCaptureFailureDoesNotPublishReceipt(t *testing.T) {
	svc, root := newCommitTestService(t)
	project := newCommitTestProject(t, root, "capture", "Captured task")
	receipt := testCaptureReceipt()
	_, err := svc.Commit(ChangeSet{Project: project, Capture: receipt, ExpectedRevision: "stale"})
	if err == nil {
		t.Fatal("expected refusal")
	}
	got, err := svc.LookupCaptureReceipt(receipt.CaptureSubmission)
	if err != nil || got != nil {
		t.Fatalf("failed capture recorded: %+v %v", got, err)
	}
	if _, err = os.Stat(filepath.Join(root, "projects", "capture", "tasks.md")); !os.IsNotExist(err) {
		t.Fatalf("failed capture wrote tasks: %v", err)
	}
}

func TestCaptureReceiptRecoversAtEveryCommitBoundary(t *testing.T) {
	// Task write plus receipt: both must recover together even when all writes
	// landed but the committing manifest was not yet removed.
	for _, prefix := range []int{0, 1, 2} {
		t.Run(fmt.Sprint(prefix), func(t *testing.T) {
			svc, root := newCommitTestService(t)
			project := newCommitTestProject(t, root, "capture", "Captured task")
			receipt := testCaptureReceipt()
			manifest := crashDuring(t, func() error { _, err := svc.Commit(ChangeSet{Project: project, Capture: receipt}); return err })
			if len(manifest.Ops) != 2 {
				t.Fatalf("unexpected operation count: %d", len(manifest.Ops))
			}
			simulateCrashAfter(t, svc.transactionsDir, manifest, prefix)
			restarted := NewCommitService(svc.transactionsDir, []string{root})
			got, err := restarted.LookupCaptureReceipt(receipt.CaptureSubmission)
			if err != nil || got == nil || got.TransactionID != manifest.ID {
				t.Fatalf("recovered receipt=%+v err=%v", got, err)
			}
			if _, err = os.Stat(TasksFilePath(project.Path)); err != nil {
				t.Fatalf("receipt without task: %v", err)
			}
		})
	}
}

func TestCaptureReceiptsRefuseCorruptionAndPathEscapes(t *testing.T) {
	svc, root := newCommitTestService(t)
	receipt := testCaptureReceipt()
	path := captureReceiptPath(svc.transactionsDir, receipt.ID)
	if err := os.MkdirAll(filepath.Dir(path), DirPermissions); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("broken"), FilePermissions); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.LookupCaptureReceipt(receipt.CaptureSubmission); err == nil {
		t.Fatal("corrupt receipt was treated as an unsubmitted draft")
	}
	if err := os.RemoveAll(filepath.Join(root, ".docket", "control-captures")); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".docket", "control-captures")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.LookupCaptureReceipt(receipt.CaptureSubmission); err == nil {
		t.Fatal("followed a receipt outside the workspace")
	}
}
