package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/transaction"
)

func TestWithWorkspaceCommitLocksRecoversEveryJournalBeforeCallback(t *testing.T) {
	rootA := filepath.Join(t.TempDir(), "a")
	rootB := filepath.Join(t.TempDir(), "b")
	for _, root := range []string{rootA, rootB} {
		if err := os.MkdirAll(root, DirPermissions); err != nil {
			t.Fatalf("mkdir %s: %v", root, err)
		}
	}

	targetA := filepath.Join(rootA, "recovered.txt")
	targetB := filepath.Join(rootB, "recovered.txt")
	journalA := transaction.NewJournal(filepath.Join(rootA, ".tt", "transactions"), []string{rootA}, "")
	journalB := transaction.NewJournal(filepath.Join(rootB, ".tt", "transactions"), []string{rootB}, "")
	manifestA, err := journalA.Prepare([]transaction.Op{{Kind: transaction.OpWrite, Target: targetA, Data: []byte("a")}})
	if err != nil {
		t.Fatalf("prepare journal A: %v", err)
	}
	manifestB, err := journalB.Prepare([]transaction.Op{{Kind: transaction.OpWrite, Target: targetB, Data: []byte("b")}})
	if err != nil {
		t.Fatalf("prepare journal B: %v", err)
	}

	serviceA := NewCommitService(filepath.Join(rootA, ".tt", "transactions"), []string{rootA})
	serviceB := NewCommitService(filepath.Join(rootB, ".tt", "transactions"), []string{rootB})
	if err := WithWorkspaceCommitLocks([]*CommitService{serviceB, serviceA, serviceA}, func() error {
		for path, want := range map[string]string{targetA: "a", targetB: "b"} {
			got, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("callback read %s: %v", path, readErr)
			}
			if string(got) != want {
				t.Fatalf("callback content at %s = %q, want %q", path, got, want)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("WithWorkspaceCommitLocks: %v", err)
	}

	for _, path := range []string{journalA.ManifestPath(manifestA.ID), journalB.ManifestPath(manifestB.ID)} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("recovered manifest still exists at %s: %v", path, err)
		}
	}
}

func TestWithWorkspaceCommitLocksOppositeDirectionsSerializeWithoutDeadlock(t *testing.T) {
	rootA := filepath.Join(t.TempDir(), "a")
	rootB := filepath.Join(t.TempDir(), "b")
	serviceA := NewCommitService(filepath.Join(rootA, ".tt", "transactions"), []string{rootA})
	serviceB := NewCommitService(filepath.Join(rootB, ".tt", "transactions"), []string{rootB})

	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- WithWorkspaceCommitLocks([]*CommitService{serviceA, serviceB}, func() error {
			close(firstEntered)
			<-releaseFirst
			return nil
		})
	}()

	select {
	case <-firstEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("first opposite-direction request did not acquire both locks")
	}

	secondStarted := make(chan struct{})
	secondEntered := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		close(secondStarted)
		secondDone <- WithWorkspaceCommitLocks([]*CommitService{serviceB, serviceA}, func() error {
			close(secondEntered)
			return nil
		})
	}()
	<-secondStarted

	select {
	case <-secondEntered:
		t.Fatal("opposite-direction request entered while the first held both locks")
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseFirst)
	for name, done := range map[string]<-chan error{"first": firstDone, "second": secondDone} {
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("%s request: %v", name, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%s opposite-direction request deadlocked", name)
		}
	}
}
