package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/status"
)

// TestLifecycleControllerArmsOnlyWhatCompositionOwed pins that the startup
// message is optional: a shell assembled without one owes the first frame
// nothing, and a nil command is how that is spelled.
func TestLifecycleControllerArmsOnlyWhatCompositionOwed(t *testing.T) {
	t.Parallel()

	var absent *lifecycleController
	if absent.startupCmd() != nil {
		t.Error("a nil cluster owes the first frame nothing")
	}

	empty := &lifecycleController{}
	if empty.startupCmd() != nil {
		t.Error("an unarmed cluster owes the first frame nothing")
	}

	armed := &lifecycleController{startup: launchOutlinerMsg{}}
	cmd := armed.startupCmd()
	if cmd == nil {
		t.Fatal("an armed cluster owes the first frame its startup message")
	}
	if _, ok := cmd().(launchOutlinerMsg); !ok {
		t.Fatalf("startup command delivered %T, want launchOutlinerMsg", cmd())
	}
}

// TestLifecycleControllerStartupCommandKeepsItsPromise pins the capture: the
// command delivers the message that was armed when it was built, so a later write
// to the field cannot change what an already-returned command promised.
func TestLifecycleControllerStartupCommandKeepsItsPromise(t *testing.T) {
	t.Parallel()

	controller := &lifecycleController{startup: launchOutlinerMsg{}}
	cmd := controller.startupCmd()
	controller.startup = status.PeriodicSaveMsg{}

	if _, ok := cmd().(launchOutlinerMsg); !ok {
		t.Fatalf("startup command delivered %T, want the message armed when it was built", cmd())
	}
}

// TestLifecycleControllerSchedulesThePeriodicSaveThroughTheTickSeam pins the one
// recurring app timer to status.TickCmd rather than tea.Tick, which is what keeps
// it visible to the scenario harness's virtual clock, and pins its cadence to the
// single periodicSaveInterval both arming sites read.
func TestLifecycleControllerSchedulesThePeriodicSaveThroughTheTickSeam(t *testing.T) {
	var scheduled time.Duration
	var fired tea.Msg

	previous := status.TickScheduler
	status.TickScheduler = func(delay time.Duration, fire func(time.Time) tea.Msg) tea.Cmd {
		scheduled = delay
		return func() tea.Msg { return fire(time.Time{}) }
	}
	defer func() { status.TickScheduler = previous }()

	var controller lifecycleController
	cmd := controller.periodicSaveCmd()
	if cmd == nil {
		t.Fatal("the periodic save is always scheduled")
	}
	fired = cmd()

	if scheduled != periodicSaveInterval {
		t.Errorf("periodic save delay = %v, want %v", scheduled, periodicSaveInterval)
	}
	if _, ok := fired.(status.PeriodicSaveMsg); !ok {
		t.Fatalf("periodic save fired %T, want status.PeriodicSaveMsg", fired)
	}
}

// TestLifecycleClusterHasOneHome pins the lifecycle cluster to its single named
// AppModel field. What the session still owes the first frame, what it still
// owes the cursor, and whether it is on the way out are the whole of a session's
// retained lifecycle state; a second home is how a quit would come to be
// observed by one reader and not another, or a launch focus honored twice.
func TestLifecycleClusterHasOneHome(t *testing.T) {
	t.Parallel()

	clustered := map[string]bool{
		"startupMessage": true,
		"quitting":       true,
		"launchFocus":    true,
	}
	assertAppModelClusterHasOneHome(t, "lifecycle", clustered)
}

// TestLifecycleControllerLaunchFocusIsOneShot pins the intent's whole lifetime:
// it is owed only once armed, it is not owed by default, and both the honoring
// drop and the newer-navigation drop spend the same flag, so a startup that
// completes after a keystroke finds nothing to apply.
func TestLifecycleControllerLaunchFocusIsOneShot(t *testing.T) {
	t.Parallel()

	var absent *lifecycleController
	if absent.launchFocusArmed() {
		t.Error("a nil cluster owes the cursor nothing")
	}
	absent.armLaunchFocus()
	absent.dropLaunchFocus()

	controller := &lifecycleController{}
	if controller.launchFocusArmed() {
		t.Error("an unarmed cluster owes the cursor nothing")
	}
	controller.armLaunchFocus()
	if !controller.launchFocusArmed() {
		t.Fatal("arming left the launch focus unowed")
	}
	controller.dropLaunchFocus()
	if controller.launchFocusArmed() {
		t.Fatal("the launch focus survived being dropped")
	}
}
