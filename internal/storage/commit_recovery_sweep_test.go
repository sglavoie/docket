package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/transaction"
)

// TestRecoverWorkspaceSweepsOrphansOncePerProcess pins what keeps a workspace
// load cheap: a load recovers once per migration step and an enrolled session
// loads one every sync pass, while the sweep walks every approved root. Only the
// first recovery of a journal and root set sweeps; every later one, from this
// service or a new one over the same paths, still completes a pending
// transaction.
func TestRecoverWorkspaceSweepsOrphansOncePerProcess(t *testing.T) {
	root := t.TempDir()
	transactionsDir := filepath.Join(root, ".tt", "transactions")
	orphan := func(id string) string {
		path := filepath.Join(root, "projects", "alpha", ".tt-stage-"+id+"-000.stage")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("staged"), FilePermissions); err != nil {
			t.Fatal(err)
		}
		return path
	}

	first := orphan("11111111-1111-1111-1111-111111111111")
	report, err := NewCommitService(transactionsDir, []string{root}).OnMachine("machine-a").RecoverWorkspace()
	if err != nil {
		t.Fatalf("first RecoverWorkspace: %v", err)
	}
	if len(report.SweptOrphans) != 1 || report.SweptOrphans[0] != first {
		t.Fatalf("first recovery swept %v, want %s", report.SweptOrphans, first)
	}

	later := orphan("22222222-2222-2222-2222-222222222222")
	target := filepath.Join(root, "projects", "alpha", "tasks.md")
	if _, prepareErr := transaction.NewJournal(transactionsDir, []string{root}, "machine-a").Prepare([]transaction.Op{{
		Kind: transaction.OpWrite, Target: target, Data: []byte("rolled forward"), Mode: FilePermissions,
	}}); prepareErr != nil {
		t.Fatalf("Prepare: %v", prepareErr)
	}
	report, err = NewCommitService(transactionsDir, []string{root}).OnMachine("machine-a").RecoverWorkspace()
	if err != nil {
		t.Fatalf("second RecoverWorkspace: %v", err)
	}
	if len(report.SweptOrphans) != 0 {
		t.Fatalf("second recovery swept %v, want the walk left to the next process", report.SweptOrphans)
	}
	if _, err := os.Stat(later); err != nil {
		t.Fatalf("unswept orphan stat = %v, want it left in place", err)
	}
	if len(report.RolledForward) != 1 {
		t.Fatalf("RolledForward = %v, want the pending transaction completed", report.RolledForward)
	}
	if got := readFile(t, target); got != "rolled forward" {
		t.Fatalf("target content = %q, want the pending transaction's bytes", got)
	}
}
