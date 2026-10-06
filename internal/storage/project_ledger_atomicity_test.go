package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/synclog"
	"github.com/sglavoie/tt/internal/transaction"
)

// TestCommitMultiProjectAndArtifactLedgerIsOneUndo proves the production
// multi-project path publishes one transaction identity and one grouped history
// entry for both tasks.md files and their companion artifact.
func TestCommitMultiProjectAndArtifactLedgerIsOneUndo(t *testing.T) {
	dataDir := t.TempDir()
	transactionsDir := filepath.Join(dataDir, ".tt", "transactions")
	seedService := NewCommitService(transactionsDir, []string{dataDir})
	alpha := newCommitTestProject(t, dataDir, "alpha", "alpha before")
	beta := newCommitTestProject(t, dataDir, "beta", "beta before")
	seed, err := seedService.CommitMulti([]ChangeSet{{Project: alpha}, {Project: beta}})
	if err != nil {
		t.Fatalf("seed CommitMulti: %v", err)
	}

	alphaPath := tasksFilePath(alpha.Path)
	betaPath := tasksFilePath(beta.Path)
	alphaBefore := mustReadProjectLedgerFile(t, alphaPath)
	betaBefore := mustReadProjectLedgerFile(t, betaPath)
	artifactPath := filepath.Join(dataDir, ".tt", "planning.json")
	artifactBefore := []byte(`{"state":"before"}`)
	artifactAfter := []byte(`{"state":"after"}`)
	writeArtifactBytes(t, artifactPath, artifactBefore)
	alpha.Tasks[0].Title = "alpha after"
	beta.Tasks[0].Title = "beta after"

	ledger := NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	service := NewCommitServiceWithLedger(transactionsDir, []string{dataDir}, ledger)
	results, err := service.CommitMulti([]ChangeSet{
		{
			Project:          alpha,
			ExpectedRevision: seed[0].Revision,
			Artifacts:        []ArtifactChange{{Path: artifactPath, Data: artifactAfter}},
			UndoAction:       "change both projects",
		},
		{Project: beta, ExpectedRevision: seed[1].Revision},
	})
	if err != nil {
		t.Fatalf("CommitMulti: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("CommitMulti results = %#v, want two revisions", results)
	}
	assertArtifactBytes(t, artifactPath, artifactAfter)

	entry, ok, err := ledger.SelectNewestUndo(GlobalScope())
	if err != nil || !ok {
		t.Fatalf("SelectNewestUndo(global): ok=%v err=%v", ok, err)
	}
	if entry.TransactionID == "" {
		t.Fatal("ledger transaction id is empty")
	}
	for _, projectID := range []string{ledgerProjectID(alpha), ledgerProjectID(beta)} {
		selected, selectable, selectErr := ledger.SelectNewestUndo(LedgerScope{ProjectID: projectID})
		if selectErr != nil || !selectable || selected.TransactionID != entry.TransactionID {
			t.Fatalf("SelectNewestUndo(%s) = %#v ok=%v err=%v", projectID, selected, selectable, selectErr)
		}
	}

	if _, err := ledger.Undo(LedgerScope{ProjectID: ledgerProjectID(beta)}); err != nil {
		t.Fatalf("Undo(beta): %v", err)
	}
	assertArtifactBytes(t, alphaPath, alphaBefore)
	assertArtifactBytes(t, betaPath, betaBefore)
	assertArtifactBytes(t, artifactPath, artifactBefore)
	assertJournalClean(t, dataDir)
}

// TestCommitMultiLedgerPreparationFailureChangesNoUserArtifacts exercises the
// real project commit boundary with two projects and a companion artifact. A
// malformed ledger must fail during side-effect-free planning, before the journal
// stages or applies any user operation.
func TestCommitMultiLedgerPreparationFailureChangesNoUserArtifacts(t *testing.T) {
	dataDir := t.TempDir()
	transactionsDir := filepath.Join(dataDir, ".tt", "transactions")
	seedService := NewCommitService(transactionsDir, []string{dataDir})
	alpha := newCommitTestProject(t, dataDir, "alpha", "alpha before")
	beta := newCommitTestProject(t, dataDir, "beta", "beta before")
	seed, err := seedService.CommitMulti([]ChangeSet{{Project: alpha}, {Project: beta}})
	if err != nil {
		t.Fatalf("seed CommitMulti: %v", err)
	}

	alphaPath := tasksFilePath(alpha.Path)
	betaPath := tasksFilePath(beta.Path)
	alphaBefore, err := os.ReadFile(alphaPath)
	if err != nil {
		t.Fatalf("read alpha before: %v", err)
	}
	betaBefore, err := os.ReadFile(betaPath)
	if err != nil {
		t.Fatalf("read beta before: %v", err)
	}
	artifactPath := filepath.Join(dataDir, ".tt", "planning.json")
	artifactBefore := []byte(`{"state":"before"}`)
	writeArtifactBytes(t, artifactPath, artifactBefore)

	alpha.Tasks[0].Title = "alpha after"
	beta.Tasks[0].Title = "beta after"
	malformed := []byte("{not-json")
	ledger := NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	writeArtifactBytes(t, ledger.ledgerPath(), malformed)
	service := NewCommitServiceWithLedger(transactionsDir, []string{dataDir}, ledger)

	_, err = service.CommitMulti([]ChangeSet{
		{
			Project:          alpha,
			ExpectedRevision: seed[0].Revision,
			Artifacts: []ArtifactChange{{
				Path: artifactPath,
				Data: []byte(`{"state":"after"}`),
			}},
			UndoAction: "change both projects",
		},
		{Project: beta, ExpectedRevision: seed[1].Revision},
	})
	if err == nil {
		t.Fatal("CommitMulti succeeded with a malformed ledger")
	}

	assertArtifactBytes(t, alphaPath, alphaBefore)
	assertArtifactBytes(t, betaPath, betaBefore)
	assertArtifactBytes(t, artifactPath, artifactBefore)
	assertArtifactBytes(t, ledger.ledgerPath(), malformed)
	assertJournalClean(t, dataDir)
}

// TestProjectCommitLedgerRecoveryConvergesAtEveryOperation prepares the same
// combined manifest used by project commits, then simulates a process stop after
// every operation prefix. Recovery must always converge both project files, the
// companion artifact, retained undo blobs, and ledger.json to one visible state.
func TestProjectCommitLedgerRecoveryConvergesAtEveryOperation(t *testing.T) {
	prototype := prepareProjectLedgerCrashFixture(t)
	opCount := len(prototype.manifest.Ops)
	if opCount < len(prototype.userPaths)+2 {
		t.Fatalf("combined manifest has %d ops, want user writes plus undo blobs and ledger.json", opCount)
	}

	for prefix := 0; prefix <= opCount; prefix++ {
		name := "prepared"
		if prefix > 0 {
			name = fmt.Sprintf("after-op-%02d", prefix)
		}
		t.Run(name, func(t *testing.T) {
			fixture := prepareProjectLedgerCrashFixture(t)
			if len(fixture.manifest.Ops) != opCount {
				t.Fatalf("manifest op count = %d, want stable %d", len(fixture.manifest.Ops), opCount)
			}
			if prefix > 0 {
				fixture.manifest.State = transaction.StateCommitting
				writeProjectLedgerManifest(t, fixture)
				applyProjectLedgerOpPrefix(t, fixture.manifest, prefix)
			}
			assertProjectLedgerVisibility(t, fixture, prefix == opCount)
			recoverAndAssertProjectLedgerState(t, fixture, false)
		})
	}

	t.Run("committed-cleanup", func(t *testing.T) {
		fixture := prepareProjectLedgerCrashFixture(t)
		fixture.manifest.State = transaction.StateCommitting
		writeProjectLedgerManifest(t, fixture)
		applyProjectLedgerOpPrefix(t, fixture.manifest, len(fixture.manifest.Ops))
		fixture.manifest.State = transaction.StateCommitted
		writeProjectLedgerManifest(t, fixture)
		assertProjectLedgerVisibility(t, fixture, true)
		recoverAndAssertProjectLedgerState(t, fixture, true)
	})
}

type projectLedgerCrashFixture struct {
	dataDir   string
	journal   *transaction.Journal
	ledger    *WorkspaceLedger
	manifest  *transaction.Manifest
	userPaths []string
	before    map[string][]byte
	after     map[string][]byte
}

// prepareProjectLedgerCrashFixture uses CommitService.prepareTransaction — the
// production preparation seam shared by project and artifact-only commits — and
// deliberately stops after the combined manifest becomes durable.
func prepareProjectLedgerCrashFixture(t *testing.T) projectLedgerCrashFixture {
	t.Helper()
	dataDir := t.TempDir()
	transactionsDir := filepath.Join(dataDir, ".tt", "transactions")
	alpha := newCommitTestProject(t, dataDir, "alpha", "alpha before")
	beta := newCommitTestProject(t, dataDir, "beta", "beta before")
	seedService := NewCommitService(transactionsDir, []string{dataDir})
	if _, err := seedService.CommitMulti([]ChangeSet{{Project: alpha}, {Project: beta}}); err != nil {
		t.Fatalf("seed CommitMulti: %v", err)
	}

	alphaPath := tasksFilePath(alpha.Path)
	betaPath := tasksFilePath(beta.Path)
	artifactPath := filepath.Join(dataDir, ".tt", "planning.json")
	artifactBefore := []byte(`{"state":"before"}`)
	artifactAfter := []byte(`{"state":"after"}`)
	writeArtifactBytes(t, artifactPath, artifactBefore)
	before := map[string][]byte{
		alphaPath:    mustReadProjectLedgerFile(t, alphaPath),
		artifactPath: artifactBefore,
		betaPath:     mustReadProjectLedgerFile(t, betaPath),
	}

	alpha.Tasks[0].Title = "alpha after"
	beta.Tasks[0].Title = "beta after"
	alphaDoc, err := CanonicalProjectDocument(alpha, nil)
	if err != nil {
		t.Fatalf("render alpha: %v", err)
	}
	betaDoc, err := CanonicalProjectDocument(beta, nil)
	if err != nil {
		t.Fatalf("render beta: %v", err)
	}
	alphaAfter := []byte(mustWriteCurrentDocument(t, alphaDoc))
	betaAfter := []byte(mustWriteCurrentDocument(t, betaDoc))
	alphaSet := ChangeSet{
		Project: alpha,
		Artifacts: []ArtifactChange{{
			Path: artifactPath,
			Data: artifactAfter,
		}},
	}
	betaSet := ChangeSet{Project: beta}
	alphaImages, err := captureLedgerArtifacts(alphaSet, string(alphaAfter))
	if err != nil {
		t.Fatalf("capture alpha ledger images: %v", err)
	}
	betaImages, err := captureLedgerArtifacts(betaSet, string(betaAfter))
	if err != nil {
		t.Fatalf("capture beta ledger images: %v", err)
	}
	ledgerImages := append(alphaImages, betaImages...)

	userOps := []transaction.Op{
		{Kind: transaction.OpWrite, Target: alphaPath, Data: alphaAfter, Mode: FilePermissions},
		{Kind: transaction.OpWrite, Target: artifactPath, Data: artifactAfter, Mode: FilePermissions},
		{Kind: transaction.OpWrite, Target: betaPath, Data: betaAfter, Mode: FilePermissions},
	}
	ledger := NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	service := NewCommitServiceWithLedger(transactionsDir, []string{dataDir}, ledger)
	journal := transaction.NewJournal(transactionsDir, []string{dataDir}, "")
	manifest, ledgerPlan, err := service.prepareTransaction(journal, userOps, &LedgerRecord{
		Action:     "change both projects",
		ProjectIDs: []string{ledgerProjectID(alpha), ledgerProjectID(beta)},
		Artifacts:  ledgerImages,
	}, synclog.KindUser)
	if err != nil {
		t.Fatalf("prepare combined project and ledger transaction: %v", err)
	}
	if ledgerPlan == nil {
		t.Fatal("prepareTransaction returned no ledger plan")
	}
	for i := range userOps {
		if manifest.Ops[i].Target != userOps[i].Target {
			t.Fatalf("manifest op %d target = %q, want user target %q", i, manifest.Ops[i].Target, userOps[i].Target)
		}
	}
	if got := manifest.Ops[len(manifest.Ops)-1].Target; got != ledger.ledgerPath() {
		t.Fatalf("last manifest target = %q, want ledger %q", got, ledger.ledgerPath())
	}

	return projectLedgerCrashFixture{
		dataDir:   dataDir,
		journal:   journal,
		ledger:    ledger,
		manifest:  manifest,
		userPaths: []string{alphaPath, artifactPath, betaPath},
		before:    before,
		after: map[string][]byte{
			alphaPath:    alphaAfter,
			artifactPath: artifactAfter,
			betaPath:     betaAfter,
		},
	}
}

func writeProjectLedgerManifest(t *testing.T, fixture projectLedgerCrashFixture) {
	t.Helper()
	data, err := json.MarshalIndent(fixture.manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal crash manifest: %v", err)
	}
	if err := os.WriteFile(fixture.journal.ManifestPath(fixture.manifest.ID), data, FilePermissions); err != nil {
		t.Fatalf("write crash manifest: %v", err)
	}
}

func applyProjectLedgerOpPrefix(t *testing.T, manifest *transaction.Manifest, prefix int) {
	t.Helper()
	for i := 0; i < prefix; i++ {
		op := manifest.Ops[i]
		if op.Kind != transaction.OpWrite {
			t.Fatalf("manifest op %d kind = %q, recovery fixture expects writes", i, op.Kind)
		}
		if err := os.Rename(op.Staged, op.Target); err != nil {
			t.Fatalf("apply manifest op %d: %v", i, err)
		}
	}
}

func assertProjectLedgerVisibility(t *testing.T, fixture projectLedgerCrashFixture, want bool) {
	t.Helper()
	entry, ok, err := fixture.ledger.SelectNewestUndo(GlobalScope())
	if err != nil {
		t.Fatalf("SelectNewestUndo before recovery: %v", err)
	}
	if ok != want {
		t.Fatalf("ledger visibility before recovery = %v, want %v (entry=%#v)", ok, want, entry)
	}
}

func recoverAndAssertProjectLedgerState(t *testing.T, fixture projectLedgerCrashFixture, committed bool) {
	t.Helper()
	report, err := transaction.NewJournal(
		filepath.Join(fixture.dataDir, ".tt", "transactions"),
		[]string{fixture.dataDir}, "",
	).Recover()
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if report.Blocked() {
		t.Fatalf("Recover quarantined transaction: %+v", report.Quarantined)
	}
	if committed {
		if len(report.CleanedUp) != 1 || report.CleanedUp[0] != fixture.manifest.ID {
			t.Fatalf("CleanedUp = %v, want [%s]", report.CleanedUp, fixture.manifest.ID)
		}
	} else if len(report.RolledForward) != 1 || report.RolledForward[0] != fixture.manifest.ID {
		t.Fatalf("RolledForward = %v, want [%s]", report.RolledForward, fixture.manifest.ID)
	}

	for _, path := range fixture.userPaths {
		assertArtifactBytes(t, path, fixture.after[path])
	}
	entry, ok, err := fixture.ledger.SelectNewestUndo(GlobalScope())
	if err != nil || !ok {
		t.Fatalf("recovered ledger entry missing: ok=%v err=%v", ok, err)
	}
	if entry.TransactionID != fixture.manifest.ID {
		t.Fatalf("ledger transaction id = %q, want %q", entry.TransactionID, fixture.manifest.ID)
	}
	if _, err := fixture.ledger.Undo(GlobalScope()); err != nil {
		t.Fatalf("Undo recovered project transaction: %v", err)
	}
	for _, path := range fixture.userPaths {
		assertArtifactBytes(t, path, fixture.before[path])
	}
	assertJournalClean(t, fixture.dataDir)
}

func mustReadProjectLedgerFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}
