package engine

import (
	"fmt"
	"testing"

	"github.com/sglavoie/docket/internal/app"
)

func TestBackgroundSyncKeepsFrontendSnapshotIndependent(t *testing.T) {
	env := newEnrollEnvironment(t)
	env.enroll(t)
	desk := env.open(t, env.workingCopy, enrollingDevice)
	peer := env.join(t)
	manual := env.open(t, env.workingCopy, enrollingDevice)
	captured, err := peer.Service().Capture(app.CaptureRequest{
		ProjectID: peer.State().SystemUnfiledProject().ID, Title: "Incoming Today task",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := peer.Service().ApplyPlanning(app.PlanningRequest{
		Locator: app.TaskLocator{ProjectID: captured.ProjectID, TaskID: captured.TaskID},
		Action:  app.PlanningAddToday,
	}); err != nil {
		t.Fatal(err)
	}
	exchangeNow(t, peer)
	worker := desk.BackgroundSyncExchanger(SyncExchangeOptions{AppVersion: "test"})
	if worker == nil {
		t.Fatal("enrolled runtime has no worker")
	}
	// Two background requests and a manual exchange contend for the same
	// device inbox. Concurrent frontend reads must remain on the old snapshot.
	results := make(chan error, 3)
	for range 2 {
		go func() {
			_, err := worker()
			results <- err
		}()
	}
	go func() {
		_, err := manual.SyncExchange(SyncExchangeOptions{AppVersion: "test"})
		results <- err
	}()
	for i := range 3 {
		// Local saves can also commit while transport passes are in progress.
		captureIn(t, desk, fmt.Sprintf("Saved during exchange %d", i))
		if len(desk.State().Planning().Today) != 0 {
			t.Fatal("background exchange mutated the frontend planning snapshot")
		}
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if len(desk.State().Planning().Today) != 0 {
		t.Fatal("frontend snapshot changed before explicit adoption")
	}
	if err := desk.Refresh(); err != nil {
		t.Fatal(err)
	}
	if got := len(desk.State().Planning().Today); got != 1 {
		t.Fatalf("adopted planning has %d Today tasks, want 1", got)
	}
	if _, err := worker(); err != nil {
		t.Fatal(err)
	}
	exchangeNow(t, peer)
	requireSameTree(t, "after concurrent saves and exchanges", syncedDigests(t, desk.DataDir()), syncedDigests(t, peer.DataDir()))
}

func TestSyncLocalSequenceDistinguishesSavesFromExchange(t *testing.T) {
	env := newEnrollEnvironment(t)
	env.enroll(t)
	desk := env.open(t, env.workingCopy, enrollingDevice)
	peer := env.join(t)
	read := desk.SyncLocalSequenceReader()
	sequence := func() uint64 {
		t.Helper()
		value, err := read()
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	initial := sequence()
	captureIn(t, desk, "Local save")
	if sequence() != initial+1 {
		t.Fatal("local save did not advance the sequence")
	}
	exchangeNow(t, desk)
	if sequence() != initial+1 {
		t.Fatal("publication looked like another local save")
	}
	exchangeNow(t, peer)
	captureIn(t, peer, "Incoming save")
	exchangeNow(t, peer)
	exchangeNow(t, desk)
	if sequence() != initial+1 {
		t.Fatal("incoming transaction looked like a local save")
	}
	if _, err := desk.Service().Capture(app.CaptureRequest{ProjectID: "missing", Title: "Rejected"}); err == nil {
		t.Fatal("invalid capture succeeded")
	}
	if sequence() != initial+1 {
		t.Fatal("failed mutation advanced the sequence")
	}
	if env.open(t, env.original, "plain").SyncLocalSequenceReader() != nil {
		t.Fatal("unenrolled workspace installed a commit observer")
	}
}
