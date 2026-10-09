package engine

import (
	"fmt"
	"testing"

	"github.com/sglavoie/tt/internal/app"
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
