package tui

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/control"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/coordinator"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/pulse"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/tui/theme"
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

	// Nothing is left to acknowledge, and Docket says so rather than commit.
	if reply := env.request(t, env.taskRequest(control.OpAcknowledge, flagged)); reply.OK {
		t.Fatalf("a settled task was acknowledged again: %+v", reply)
	}
}

func TestControlVersionReadsTheRunningBuildWhileEditing(t *testing.T) {
	env := controlEnv(t, reviewSettledTask(t, 1, "existing"))
	env.model.control.versionLine = "docket version original (commit: abc, built: then)"
	setTestPresentationPath(env.model, presentation.ViewCrossProjectOutliner, presentation.ViewNotes)
	reply := env.request(t, control.Request{Op: control.OpVersion})
	if !reply.OK || reply.Version != env.model.control.versionLine {
		t.Fatalf("running build: %+v", reply)
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
	if reply.OK || reply.Message != "Docket is busy: finish what is open in Docket first" {
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
	if reply.OK || reply.Message != "Docket is busy: finish what is open in Docket first" {
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

// TestControlPublishesEachChangedAgendaOnce: the agenda is sent when it
// changes, with each task's stored identity and its deadline as RFC 3339.
func TestControlPublishesEachChangedAgendaOnce(t *testing.T) {
	app := typedTestApp(t)
	publisher := &recordingPublisher{}
	WithControlPublisher(publisher)(app)

	due := time.Date(2026, 10, 26, 9, 30, 0, 0, time.UTC)
	app.workspaceAgenda = pulse.WorkspaceAgenda{Total: 3, Items: []pulse.AgendaItem{
		{Kind: pulse.AgendaOverdue, ProjectID: "p", TaskID: "t", Title: "Call", ProjectName: "Work", Priority: models.PriorityHigh, Due: &due, DueHasTime: true},
		{Kind: pulse.AgendaToday, ProjectID: "p", TaskID: "u", Title: "Write", ProjectName: "Work"},
	}}
	app.publishControlAgenda()
	app.publishControlAgenda()
	if len(publisher.events) != 1 {
		t.Fatalf("published %d agendas, want 1", len(publisher.events))
	}
	event := publisher.events[0]
	if event.Event != control.EventAgenda || event.Agenda == nil || event.Agenda.Total != 3 || len(event.Agenda.Items) != 2 {
		t.Fatalf("event = %+v", event)
	}
	if got := event.Agenda.Items[0]; got.Kind != control.AgendaOverdue || got.Project != "p" || got.Task != "t" ||
		got.Priority != models.PriorityHigh || got.Due != "2026-10-26T09:30:00Z" || !got.DueHasTime || got.ProjectName != "Work" {
		t.Fatalf("overdue item = %+v", got)
	}
	if got := event.Agenda.Items[1]; got.Kind != control.AgendaToday || got.Due != "" || got.Priority != 0 {
		t.Fatalf("today item = %+v", got)
	}

	app.workspaceAgenda = pulse.WorkspaceAgenda{}
	app.publishControlAgenda()
	if len(publisher.events) != 2 || publisher.events[1].Agenda == nil || publisher.events[1].Agenda.Items == nil {
		t.Fatalf("an emptied agenda was not sent as an empty list: %+v", publisher.events)
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
	if event.Event != control.EventArrivals || event.Title != "Docket · check-in" || len(event.Arrivals) != 1 {
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
// writes no machine state, so a terminal-launched Docket keeps its saved theme.
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

func TestControlThemePaletteRepaintsAndRestoresSavedTheme(t *testing.T) {
	env := controlEnv(t, reviewSettledTask(t, 1, "existing"))
	saved := env.model.domain.runtime.Machine().Theme
	savedBackground := config.DefaultTheme(saved).BackgroundColor
	t.Cleanup(func() { applySessionTheme(saved) })
	beforeUndo, beforeRedo := env.model.workspaceHistoryPreview()
	setTestPresentationPath(env.model, presentation.ViewCrossProjectOutliner, presentation.ViewNotes)
	p := theme.GhosttyPalette{
		Name: "Test Theme", Background: "#f1e8d9", Foreground: "#27213a",
		Cursor: "#543210", CursorText: "#ffffff",
		SelectionBackground: "#e2cba6", SelectionForeground: "#27213a",
		Palette: []string{"#27213a", "#9a1529", "#286536", "#856018", "#244d9a", "#753785", "#086778", "#d6d0c2",
			"#554d60", "#b93747", "#408045", "#a3782a", "#4770b1", "#98559e", "#34808e", "#fffaf0"},
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if reply := env.request(t, control.Request{Op: control.OpThemePalette, Text: string(data)}); !reply.OK || reply.Message != "Theme: Test Theme" {
		t.Fatalf("reply = %+v", reply)
	}
	if theme.ThemeBgColor != p.Background || theme.ColorNormalText != p.Foreground || theme.ActiveThemeName != "light" {
		t.Fatalf("TUI palette not applied: background %q, foreground %q, variant %q", theme.ThemeBgColor, theme.ColorNormalText, theme.ActiveThemeName)
	}
	painted := env.model.View().Content
	if !strings.Contains(painted, hexToBgEsc(p.Background)) || !strings.Contains(painted, hexToFgEsc(p.Foreground)) {
		t.Fatal("rendered view does not contain the selected theme's background and foreground")
	}
	if theme.ColorPrimary == theme.ColorError {
		t.Fatal("theme did not retain distinct semantic colors")
	}
	if got := env.model.domain.runtime.Machine().Theme; got != saved {
		t.Fatalf("machine theme = %q, want %q", got, saved)
	}
	if undo, redo := env.model.workspaceHistoryPreview(); undo != beforeUndo || redo != beforeRedo {
		t.Fatalf("theme wrote history: undo %q, redo %q", undo, redo)
	}
	if reply := env.request(t, control.Request{Op: control.OpThemePalette, Text: "bad json"}); reply.OK {
		t.Fatalf("accepted invalid palette: %+v", reply)
	}
	if theme.ThemeBgColor != p.Background {
		t.Fatal("invalid palette changed the theme")
	}
	if reply := env.request(t, control.Request{Op: control.OpThemePalette}); !reply.OK {
		t.Fatalf("clear palette reply = %+v", reply)
	}
	if theme.ThemeBgColor != savedBackground {
		t.Fatalf("clear palette background = %q, want %q", theme.ThemeBgColor, savedBackground)
	}
}

// TestControlDoneViewFiltersAllToFinishedWork: the done view is All with the
// terminal-state facets and the completed population, replacing whatever
// filters stood, and it reloads so the finished work is read in.
func TestControlDoneViewFiltersAllToFinishedWork(t *testing.T) {
	env := controlEnv(t, reviewSettledTask(t, 1, "kept"))
	env.outliner.SelectOutlinerMode(chrome.ModeInbox)
	stale, _ := models.PartitionQueryString("!1")
	env.outliner.SetFilterQuery(stale)

	replies := make(chan control.Reply, 1)
	handled, cmd := env.model.handleControlMessage(controlRequestMsg{request: control.Request{Op: control.OpView, Text: "done"}, reply: replies})
	if !handled || cmd == nil {
		t.Fatalf("done view handled = %v, reload = %v", handled, cmd != nil)
	}
	if reply := <-replies; !reply.OK || reply.Message != "Shown: All · $canceled $done ?completed" {
		t.Fatalf("reply = %+v", reply)
	}
	if env.outliner.PrimaryMode != chrome.ModeAll {
		t.Fatalf("mode = %q, want All", env.outliner.PrimaryMode)
	}
	want, _ := models.PartitionQueryString("$canceled $done ?completed")
	if got := env.outliner.FilterQuery; got.String() != want.Canonical().String() {
		t.Fatalf("filters = %q, want %q", got.String(), want.Canonical().String())
	}
	if !env.outliner.AdmitsTerminalPopulation() {
		t.Fatal("the done view does not admit finished work")
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
	if reply := env.request(t, control.Request{Op: control.OpView, Text: "inbox"}); reply.OK || reply.Message != "Docket is busy: Close search in Docket, then try again." {
		t.Fatalf("view under the ranked list answered %+v", reply)
	}
	if env.outliner.PrimaryMode != chrome.ModeDue {
		t.Fatalf("a view under the ranked list moved the mode to %q", env.outliner.PrimaryMode)
	}
	env.outliner.ClearSearch()

	setTestPresentationPath(env.model, presentation.ViewCrossProjectOutliner, presentation.ViewNotes)
	if reply := env.request(t, control.Request{Op: control.OpView, Text: "inbox"}); reply.OK || reply.Message != "Docket is busy: Close the current panel in Docket, then try again." {
		t.Fatalf("view mid-interaction answered %+v", reply)
	}
	if env.outliner.PrimaryMode != chrome.ModeDue {
		t.Fatalf("a busy view moved the mode to %q", env.outliner.PrimaryMode)
	}
}
