package engine

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/syncexchange"
)

// removeGenesis deletes the enrolling device's first manifest from the
// exchange: a history no joiner can replay from its beginning, as one sealed
// at schemas an upgraded joiner no longer reads is.
func removeGenesis(t *testing.T, env enrollEnvironment) {
	t.Helper()
	genesis := exchangeManifests(t, env.exchange, enrollingDevice)[0]
	path := filepath.Join(env.exchange, syncexchange.ExchangeRoot,
		syncexchange.ManifestPath(genesis.DeviceID, genesis.DeviceSeq, genesis.TxnID))
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

func TestJoinStartsFromCheckpointWhenHistoryCannotBeReplayed(t *testing.T) {
	env := newEnrollEnvironment(t)
	env.enroll(t)
	desk := env.open(t, env.workingCopy, enrollingDevice)
	captureIn(t, desk, "Written before the checkpoint")
	exchangeNow(t, desk)
	checkpoint, err := desk.SyncCheckpoint()
	if err != nil {
		t.Fatal(err)
	}
	report := exchangeNow(t, desk)
	if len(report.Checkpoints) != 1 || report.Checkpoints[0] != checkpoint.CheckpointID {
		t.Fatalf("checkpoint not published: %+v", report)
	}
	captureIn(t, desk, "Written after the checkpoint")
	exchangeNow(t, desk)
	removeGenesis(t, env)

	joined, joinReport, err := JoinInbox(JoinInboxConfig{DataDir: filepath.Join(env.root, "fresh-phone"),
		Exchange: env.exchange, WorkspaceID: desk.State().Catalog().WorkspaceID,
		InboxDir: filepath.Join(env.exchange, syncexchange.ExchangeRoot),
		Machine:  ExplicitMachine(env.home, "fresh-phone"), Clock: env.clock})
	if err != nil {
		t.Fatal(err)
	}
	installed, imported := false, 0
	for _, result := range joinReport.Results {
		switch {
		case result.Path == syncexchange.CheckpointPath(checkpoint.CheckpointID):
			installed = result.Outcome == syncexchange.Imported
		case result.Outcome == syncexchange.Imported:
			imported++
		}
	}
	if !installed || imported != 1 {
		t.Fatalf("join did not start from the checkpoint and import only what followed: %+v", joinReport.Results)
	}
	requireSameTree(t, "checkpoint join", syncedDigests(t, desk.DataDir()), syncedDigests(t, joined.DataDir()))

	captureIn(t, joined, "Written on the joined device")
	exchangeNow(t, joined)
	exchangeNow(t, desk)
	captureIn(t, desk, "Written on the desk afterwards")
	exchangeNow(t, desk)
	exchangeNow(t, joined)
	requireSameTree(t, "post-join sync", syncedDigests(t, desk.DataDir()), syncedDigests(t, joined.DataDir()))
	if conflicts, err := joined.SyncCommits().SyncConflicts(); err != nil || len(conflicts) != 0 {
		t.Fatalf("a clean exchange left conflicts: %v, %v", conflicts, err)
	}
}

func TestJoinExplainsAHistoryItCannotBuild(t *testing.T) {
	env := newEnrollEnvironment(t)
	env.enroll(t)
	desk := env.open(t, env.workingCopy, enrollingDevice)
	captureIn(t, desk, "Only history, no checkpoint")
	exchangeNow(t, desk)
	removeGenesis(t, env)
	data := filepath.Join(env.root, "fresh-phone")
	_, _, err := JoinInbox(JoinInboxConfig{DataDir: data,
		Exchange: env.exchange, WorkspaceID: desk.State().Catalog().WorkspaceID,
		InboxDir: filepath.Join(env.exchange, syncexchange.ExchangeRoot),
		Machine:  ExplicitMachine(env.home, "fresh-phone"), Clock: env.clock})
	if !errors.Is(err, ErrJoinNotDelivered) || !strings.Contains(err.Error(), "downloading") {
		t.Fatalf("join failure not explained: %v", err)
	}
}

func TestSyncCheckpointRefusesOpenConflicts(t *testing.T) {
	env := newEnrollEnvironment(t)
	env.enroll(t)
	desk := env.open(t, env.workingCopy, enrollingDevice)
	peer := env.join(t)
	// Both sides rewrite the same note while apart.
	note := filepath.Join(".docket", "global-notes", "shared.md")
	for _, rt := range []*Runtime{desk, peer} {
		path := filepath.Join(rt.DataDir(), note)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(desk.DataDir(), note), []byte("---\ntitle: shared\n---\n\nbase\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	exchangeNow(t, desk)
	exchangeNow(t, peer)
	for i, rt := range []*Runtime{desk, peer} {
		body := []byte("---\ntitle: shared\n---\n\nside " + string(rune('a'+i)) + "\n")
		if err := os.WriteFile(filepath.Join(rt.DataDir(), note), body, 0o600); err != nil {
			t.Fatal(err)
		}
		exchangeNow(t, rt)
	}
	exchangeNow(t, desk)
	conflicts, err := desk.SyncCommits().SyncConflicts()
	if err != nil || len(conflicts) == 0 {
		t.Fatalf("setup left no conflict: %v", err)
	}
	if _, err := desk.SyncCheckpoint(); err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("checkpoint recorded over open conflicts: %v", err)
	}
}
