package tui

import (
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/control"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// controlEnv is the R chain's environment with the Outliner as the live route,
// which is where a host's requests are answered.
func controlEnv(t *testing.T, tasks ...*models.Task) *reviewAsyncEnv {
	t.Helper()
	env := newReviewAsyncEnv(t, tasks)
	env.model.features = coordinator.NewFeatureCoordinators(env.env.svc)
	env.model.domain.runtime.Catalog().SystemUnfiledProjectID = env.env.projects[0].ID
	setTestPresentationPath(env.model, presentation.ViewCrossProjectOutliner)
	return env
}

// request answers one request the way a socket would: the reply that arrives
// at once, or the commit's, after the command resolves and its message is
// handed back to the shell.
func (e *reviewAsyncEnv) request(t *testing.T, request control.Request) control.Reply {
	t.Helper()
	replies := make(chan control.Reply, 1)
	handled, cmd := e.model.handleControlMessage(controlRequestMsg{request: request, reply: replies})
	if !handled {
		t.Fatal("a control request was not handled")
	}
	select {
	case reply := <-replies:
		return reply
	default:
	}
	committed, ok := findControlCommit(cmd)
	if !ok {
		t.Fatal("the request neither answered nor produced a commit")
	}
	if handled, _ := e.model.handleControlMessage(committed); !handled {
		t.Fatal("the commit was not handled")
	}
	select {
	case reply := <-replies:
		return reply
	default:
		t.Fatal("the commit resolved without answering")
		return control.Reply{}
	}
}

func findControlCommit(cmd tea.Cmd) (controlCommittedMsg, bool) {
	if cmd == nil {
		return controlCommittedMsg{}, false
	}
	switch msg := cmd().(type) {
	case controlCommittedMsg:
		return msg, true
	case tea.BatchMsg:
		for _, nested := range msg {
			if found, ok := findControlCommit(nested); ok {
				return found, true
			}
		}
	}
	return controlCommittedMsg{}, false
}

func (e *reviewAsyncEnv) taskRequest(op string, task *models.Task) control.Request {
	return control.Request{Op: op, Project: e.env.projects[0].ID, Task: task.ID}
}

func TestControlAcknowledgeReviewsTheNamedTask(t *testing.T) {
	flagged := reviewCandidateTask(t, 1, "flagged")
	env := controlEnv(t, flagged)

	reply := env.request(t, env.taskRequest(control.OpAcknowledge, flagged))
	if !reply.OK || reply.Message != "Acknowledged: flagged" {
		t.Fatalf("reply = %+v", reply)
	}
	if env.storedTask(t, 1).Review {
		t.Fatal("the review flag survived the acknowledgement")
	}

	// Nothing is left to acknowledge, and tt says so rather than commit.
	if reply := env.request(t, env.taskRequest(control.OpAcknowledge, flagged)); reply.OK {
		t.Fatalf("a settled task was acknowledged again: %+v", reply)
	}
}

func TestControlDeferDefersByMinutesFromNow(t *testing.T) {
	task := reviewSettledTask(t, 1, "later")
	env := controlEnv(t, task)

	before := time.Now()
	request := env.taskRequest(control.OpDefer, task)
	request.Minutes = 60
	if reply := env.request(t, request); !reply.OK {
		t.Fatalf("reply = %+v", reply)
	}
	stored := env.storedTask(t, 1)
	if stored.DeferredUntil == nil || stored.DeferredUntil.Before(before.Add(59*time.Minute)) {
		t.Fatalf("deferred until %v, want about an hour from %v", stored.DeferredUntil, before)
	}

	request.Minutes = 0
	if reply := env.request(t, request); reply.OK {
		t.Fatal("a zero-minute deferral was accepted")
	}
}

// TestControlDoneCommitsDoneAsWdDoes: the task is Done with its clock and
// completion stamped, and a second request refuses it by name.
func TestControlDoneCommitsDoneAsWdDoes(t *testing.T) {
	task := reviewSettledTask(t, 1, "ship it")
	env := controlEnv(t, task)

	reply := env.request(t, env.taskRequest(control.OpDone, task))
	if !reply.OK || reply.Message != "Done: ship it" {
		t.Fatalf("reply = %+v", reply)
	}
	stored := env.storedTask(t, 1)
	if stored.WorkflowState != models.StateDone || stored.CompletedAt == nil {
		t.Fatalf("stored task = %+v", stored)
	}

	if reply := env.request(t, env.taskRequest(control.OpDone, task)); reply.OK || reply.Message != "ship it is already done" {
		t.Fatalf("a done task answered %+v", reply)
	}
	if reply := env.request(t, control.Request{Op: control.OpDone, Project: "gone", Task: "gone"}); reply.OK || reply.Message != "that task is no longer in the workspace" {
		t.Fatalf("a missing task answered %+v", reply)
	}
}

func TestControlDoneRefusesWhileTheUserIsMidInteraction(t *testing.T) {
	task := reviewSettledTask(t, 1, "ship it")
	env := controlEnv(t, task)
	setTestPresentationPath(env.model, presentation.ViewCrossProjectOutliner, presentation.ViewNotes)

	reply := env.request(t, env.taskRequest(control.OpDone, task))
	if reply.OK || reply.Message != "tt is busy: finish what is open in tt first" {
		t.Fatalf("reply = %+v", reply)
	}
	if env.storedTask(t, 1).WorkflowState != models.StateBacklog {
		t.Fatal("a refused request committed")
	}
}

func TestControlCaptureCommitsThroughTheComposerGrammar(t *testing.T) {
	env := controlEnv(t, reviewSettledTask(t, 1, "existing"))

	reply := env.request(t, control.Request{Op: control.OpCapture, Text: "Buy milk #errand !1"})
	if !reply.OK || reply.Message != "Captured: Buy milk" {
		t.Fatalf("reply = %+v", reply)
	}
	var captured *models.Task
	for _, task := range env.env.reload(t, 0).Tasks {
		if task.Title == "Buy milk" {
			captured = task
		}
	}
	if captured == nil || captured.Priority != 1 || len(captured.Tags) != 1 || captured.Tags[0] != "errand" {
		t.Fatalf("captured task = %+v", captured)
	}

	if reply := env.request(t, control.Request{Op: control.OpCapture, Text: "Pay $ready"}); reply.OK || reply.Message == "" {
		t.Fatalf("a refused line answered %+v", reply)
	}
}

// TestControlCompleteOffersWhatTheComposerOffers: a completion names the token
// ending at the caret and rewrites it as accepting it in the composer would,
// and prose answers with no completion rather than a refusal.
func TestControlCompleteOffersWhatTheComposerOffers(t *testing.T) {
	env := controlEnv(t, reviewSettledTask(t, 1, "existing"))
	project := env.env.projects[0]

	reply := env.request(t, control.Request{Op: control.OpComplete, Text: "Buy milk %" + project.Name[:3]})
	if !reply.OK || reply.Completion == nil {
		t.Fatalf("reply = %+v", reply)
	}
	if got := reply.Completion.Token; got != "%"+project.Name[:3] {
		t.Fatalf("token = %q", got)
	}
	want := control.CompletionItem{Label: "%" + project.Name, Insert: "%" + project.Slug + " "}
	if !slices.Contains(reply.Completion.Items, want) {
		t.Fatalf("items = %+v, want %+v among them", reply.Completion.Items, want)
	}

	if reply := env.request(t, control.Request{Op: control.OpComplete, Text: "Buy milk"}); !reply.OK || reply.Completion != nil {
		t.Fatalf("prose answered %+v", reply)
	}
}

// TestControlCompleteAnswersWhileTheUserIsMidInteraction: a completion commits
// nothing, so the busy guard that refuses a capture does not refuse it.
func TestControlCompleteAnswersWhileTheUserIsMidInteraction(t *testing.T) {
	env := controlEnv(t, reviewSettledTask(t, 1, "existing"))
	setTestPresentationPath(env.model, presentation.ViewCrossProjectOutliner, presentation.ViewNotes)

	if reply := env.request(t, control.Request{Op: control.OpComplete, Text: "Pay !"}); !reply.OK || reply.Completion == nil {
		t.Fatalf("reply = %+v", reply)
	}
}

func TestControlPreviewUsesCaptureRoutingWithoutCommitting(t *testing.T) {
	env := controlEnv(t, reviewSettledTask(t, 1, "existing"))
	beforeUndo, beforeRedo := env.model.workspaceHistoryPreview()
	project := env.env.projects[0]
	setTestPresentationPath(env.model, presentation.ViewCrossProjectOutliner, presentation.ViewNotes)
	for _, tc := range []struct {
		text string
		want []string
	}{
		{"", []string{"Project: " + project.Name, "Status: Inbox"}},
		{"Buy milk %" + project.Slug + " $today @2099-01-02", []string{"Project: " + project.Name, "Plan: Today", "Status: Backlog", "Due: Fri Jan 2 2099"}},
		{"Buy milk @today", []string{"Plan: Today", "Status: Backlog", "Due: " + time.Now().Format("Mon Jan 2")}},
		{"Buy milk $doing", []string{"Status: Doing"}},
		{"Buy milk @@tomorrow", []string{"Status: Backlog"}},
	} {
		reply := env.request(t, control.Request{Op: control.OpPreview, Text: tc.text})
		if !reply.OK {
			t.Fatalf("preview %q refused: %+v", tc.text, reply)
		}
		for _, want := range tc.want {
			if !strings.Contains(reply.Preview, want) {
				t.Errorf("preview %q = %q, want %q", tc.text, reply.Preview, want)
			}
		}
	}
	for _, text := range []string{"Pay $ready", "Buy milk %missing-project"} {
		reply := env.request(t, control.Request{Op: control.OpPreview, Text: text})
		if reply.OK || reply.Preview != "" || reply.Message == "" {
			t.Fatalf("invalid draft %q advertised a destination: %+v", text, reply)
		}
	}
	if tasks := env.env.reload(t, 0).Tasks; len(tasks) != 1 || tasks[0].Title != "existing" {
		t.Fatalf("preview changed stored tasks: %+v", tasks)
	}
	if undo, redo := env.model.workspaceHistoryPreview(); undo != beforeUndo || redo != beforeRedo {
		t.Fatalf("preview wrote history: undo %q, redo %q", undo, redo)
	}
}

// TestControlRefusesWhileTheUserIsMidInteraction: a request never reaches a
// session whose live route is not the Outliner, and never commits there.
func TestControlRefusesWhileTheUserIsMidInteraction(t *testing.T) {
	flagged := reviewCandidateTask(t, 1, "flagged")
	env := controlEnv(t, flagged)
	setTestPresentationPath(env.model, presentation.ViewCrossProjectOutliner, presentation.ViewNotes)

	reply := env.request(t, env.taskRequest(control.OpAcknowledge, flagged))
	if reply.OK || reply.Message != "tt is busy: finish what is open in tt first" {
		t.Fatalf("reply = %+v", reply)
	}
	if !env.storedTask(t, 1).Review {
		t.Fatal("a refused request committed")
	}
}

// TestControlQuitTakesTheQuitPath: an idle session answers OK and emits the
// quit key's own message, so view and filter state are saved on the way out.
func TestControlQuitTakesTheQuitPath(t *testing.T) {
	env := controlEnv(t, reviewSettledTask(t, 1, "kept"))
	replies := make(chan control.Reply, 1)
	handled, cmd := env.model.handleControlMessage(controlRequestMsg{request: control.Request{Op: control.OpQuit}, reply: replies})
	if !handled || cmd == nil {
		t.Fatalf("quit handled=%v cmd=%v", handled, cmd)
	}
	if reply := <-replies; !reply.OK {
		t.Fatalf("reply = %+v", reply)
	}
	if _, ok := cmd().(status.QuitMsg); !ok {
		t.Fatal("quit did not emit status.QuitMsg")
	}
}

func TestControlQuitAllowsCleanNavigation(t *testing.T) {
	env := controlEnv(t, reviewSettledTask(t, 1, "kept"))
	setTestPresentationPath(env.model, presentation.ViewCrossProjectOutliner, presentation.ViewNotes)
	replies := make(chan control.Reply, 1)
	_, cmd := env.model.handleControlMessage(controlRequestMsg{request: control.Request{Op: control.OpQuit}, reply: replies})
	if reply := <-replies; !reply.OK {
		t.Fatalf("reply = %+v", reply)
	}
	if cmd == nil {
		t.Fatal("clean navigation prevented quit")
	}
}

func TestControlRefusesAnUnknownTaskOrOperation(t *testing.T) {
	env := controlEnv(t, reviewSettledTask(t, 1, "kept"))
	if reply := env.request(t, control.Request{Op: control.OpReveal, Project: "gone", Task: "gone"}); reply.OK {
		t.Fatalf("an unknown task answered %+v", reply)
	}
	if reply := env.request(t, control.Request{Op: "explode"}); reply.OK {
		t.Fatalf("an unknown operation answered %+v", reply)
	}
}

type recordingPublisher struct{ events []control.Event }

func (r *recordingPublisher) Publish(event control.Event) { r.events = append(r.events, event) }

func TestControlPublishesEachChangedPulseOnce(t *testing.T) {
	app := typedTestApp(t)
	publisher := &recordingPublisher{}
	WithControlPublisher(publisher)(app)

	app.workspacePulse = pulse.WorkspacePulse{Overdue: 2}
	app.publishControlPulse()
	app.publishControlPulse()
	app.workspacePulse = pulse.WorkspacePulse{Overdue: 1}
	app.publishControlPulse()

	if len(publisher.events) != 2 {
		t.Fatalf("published %d pulses, want 2", len(publisher.events))
	}
	if got := publisher.events[1].Pulse; got == nil || got.Overdue != 1 {
		t.Fatalf("second pulse = %+v", got)
	}
}

// TestControlArrivalsCarryIdentityWithoutNotify: a session on a control socket
// reports arrivals as events and sends no OSC 777 unless --notify asked for it.
func TestControlArrivalsCarryIdentityWithoutNotify(t *testing.T) {
	app := typedTestApp(t)
	publisher := &recordingPublisher{}
	WithControlPublisher(publisher)(app)
	app.publishControlArrivals([]pulse.Arrival{{Kind: pulse.ArrivalCheckIn, ProjectID: "p", TaskID: "t", Title: "Call", Reviewable: true}})

	if len(publisher.events) != 1 {
		t.Fatalf("events = %+v", publisher.events)
	}
	event := publisher.events[0]
	if event.Event != control.EventArrivals || event.Title != "tt · check-in" || len(event.Arrivals) != 1 {
		t.Fatalf("event = %+v", event)
	}
	if got := event.Arrivals[0]; got.Kind != control.ArrivalCheckIn || got.Task != "t" || !got.Reviewable {
		t.Fatalf("arrival = %+v", got)
	}
	if app.notifications.enabled {
		t.Fatal("a control socket turned --notify on")
	}
}

// TestFocusClearsTheBlurAndOwnCommitsDoNotReload: a commit the session made
// while blurred is recorded, so coming back does not announce a reload.
func TestFocusClearsTheBlurAndOwnCommitsDoNotReload(t *testing.T) {
	flagged := reviewCandidateTask(t, 1, "flagged")
	env := controlEnv(t, flagged)
	env.model.handleFocusMessage(tea.BlurMsg{})

	if reply := env.request(t, env.taskRequest(control.OpAcknowledge, flagged)); !reply.OK {
		t.Fatalf("reply = %+v", reply)
	}
	env.model.handleFocusMessage(tea.FocusMsg{})
	if env.model.externalReload.owed || env.model.externalReload.blurFingerprint != "" {
		t.Fatalf("own commit read as an external change: %+v", env.model.externalReload)
	}
}

func TestControlRevealSeatsTheCursorOnTheTask(t *testing.T) {
	first := reviewSettledTask(t, 1, "first")
	second := reviewSettledTask(t, 2, "second")
	env := controlEnv(t, first, second)
	cursorToTaskRow(t, env.outliner, first.ID)

	if reply := env.request(t, env.taskRequest(control.OpReveal, second)); !reply.OK || reply.Message != "Shown: second" {
		t.Fatalf("reply = %+v", reply)
	}
	requireCursorRow(t, env.outliner, second.ID)
}

// TestControlAppearanceAppliesForTheSessionAlone: a host's appearance op
// changes the theme on screen, even while the user is mid-interaction, and
// writes no machine state, so a terminal-launched tt keeps its saved theme.
func TestControlAppearanceAppliesForTheSessionAlone(t *testing.T) {
	env := controlEnv(t, reviewSettledTask(t, 1, "existing"))
	t.Cleanup(func() { applySessionTheme(env.model.domain.runtime.Machine().Theme) })
	saved := env.model.domain.runtime.Machine().Theme
	want := "light"
	if saved == "light" {
		want = "dark"
	}
	beforeUndo, beforeRedo := env.model.workspaceHistoryPreview()
	setTestPresentationPath(env.model, presentation.ViewCrossProjectOutliner, presentation.ViewNotes)

	reply := env.request(t, control.Request{Op: control.OpAppearance, Text: want})
	if !reply.OK || reply.Message != "Theme: "+want {
		t.Fatalf("reply = %+v", reply)
	}
	if theme.ActiveThemeName != want {
		t.Fatalf("active theme = %q, want %q", theme.ActiveThemeName, want)
	}
	if got := env.model.domain.runtime.Machine().Theme; got != saved {
		t.Fatalf("machine theme = %q, want the saved %q", got, saved)
	}
	if undo, redo := env.model.workspaceHistoryPreview(); undo != beforeUndo || redo != beforeRedo {
		t.Fatalf("appearance wrote history: undo %q, redo %q", undo, redo)
	}

	for _, text := range []string{"", "Dark", "solarized"} {
		reply := env.request(t, control.Request{Op: control.OpAppearance, Text: text})
		if reply.OK || reply.Message == "" {
			t.Fatalf("appearance %q answered %+v", text, reply)
		}
		if theme.ActiveThemeName != want {
			t.Fatalf("refused appearance %q changed the theme to %q", text, theme.ActiveThemeName)
		}
	}
}

// TestControlViewSwitchesModeAsTheJumpDoes: a host's view op lands the Outliner
// in the named mode, refuses an unknown name, and is refused wherever the mode
// jump is — mid-interaction or under the ranked list — without moving the mode.
func TestControlViewSwitchesModeAsTheJumpDoes(t *testing.T) {
	env := controlEnv(t, reviewSettledTask(t, 1, "kept"))
	for _, tc := range []struct {
		text string
		want chrome.OutlinerModeID
	}{{"inbox", chrome.ModeInbox}, {"today", chrome.ModeToday}, {"due", chrome.ModeDue}} {
		mode, _ := chrome.OutlinerModeByID(tc.want)
		if reply := env.request(t, control.Request{Op: control.OpView, Text: tc.text}); !reply.OK || reply.Message != "Shown: "+mode.Label {
			t.Fatalf("view %q reply = %+v", tc.text, reply)
		}
		if env.outliner.PrimaryMode != tc.want {
			t.Fatalf("view %q left mode %q", tc.text, env.outliner.PrimaryMode)
		}
	}

	for _, text := range []string{"", "all", "Inbox"} {
		if reply := env.request(t, control.Request{Op: control.OpView, Text: text}); reply.OK || reply.Message == "" {
			t.Fatalf("view %q answered %+v", text, reply)
		}
	}
	if env.outliner.PrimaryMode != chrome.ModeDue {
		t.Fatalf("a refused view moved the mode to %q", env.outliner.PrimaryMode)
	}

	searchRankedList(env.outliner, "kept")
	if reply := env.request(t, control.Request{Op: control.OpView, Text: "inbox"}); reply.OK || reply.Message != "tt is busy: Close search in tt, then try again." {
		t.Fatalf("view under the ranked list answered %+v", reply)
	}
	if env.outliner.PrimaryMode != chrome.ModeDue {
		t.Fatalf("a view under the ranked list moved the mode to %q", env.outliner.PrimaryMode)
	}
	env.outliner.ClearSearch()

	setTestPresentationPath(env.model, presentation.ViewCrossProjectOutliner, presentation.ViewNotes)
	if reply := env.request(t, control.Request{Op: control.OpView, Text: "inbox"}); reply.OK || reply.Message != "tt is busy: Close the current panel in tt, then try again." {
		t.Fatalf("view mid-interaction answered %+v", reply)
	}
	if env.outliner.PrimaryMode != chrome.ModeDue {
		t.Fatalf("a busy view moved the mode to %q", env.outliner.PrimaryMode)
	}
}

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
	if reply.OK || reply.Message != "Something newer was changed in tt; undo it there" {
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
	if reply.OK || reply.Message != "tt is busy: finish what is open in tt first" {
		t.Fatalf("undo reply = %+v", reply)
	}
	if !env.hasTitle(t, "Buy milk") {
		t.Fatal("a refused undo changed the workspace")
	}
}
