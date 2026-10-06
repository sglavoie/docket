package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/control"
	"github.com/sglavoie/tt/internal/engine"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/grammar"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskform"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/util"
	"github.com/sglavoie/tt/internal/workspace"
)

// periodicSaveInterval is the cadence of the one recurring app timer. It is the
// span a scenario advances to observe something only the tick can produce, such
// as an outstanding-conflict count another machine's sync wrote.
const periodicSaveInterval = 60 * time.Second

// lifecycleController is the shell's session-lifecycle cluster: the one message
// the composition root arms for the first frame, and the flag a quit sets so the
// final paint renders nothing.
//
// It is the smallest cluster on AppModel deliberately. A session has exactly two
// pieces of state between "constructed" and "gone": what it still owes the first
// frame, and whether it is on the way out. Everything else about the lifecycle is
// behavior, not state — RunApp acquires the process lock and owns the panic
// recovery (app_run.go), NewAppModel runs the fifteen-step composition
// (app_init.go), and teardown is the deferred lock release beside the Bubble Tea
// program. None of that is retained, so none of it belongs to a field.
//
// It also owns the scheduling of the one recurring timer, because the two places
// that arm it — Init and the handler that re-arms it on each fire — must arm the
// same one. App timers go through status.TickCmd and never tea.Tick, so the tick
// stays virtualizable by the scenario harness; keeping the construction here is
// what stops a second, differently-scheduled periodic save from appearing.
type lifecycleController struct {
	// startup is the message the composition root owes the first frame. It is
	// held rather than sent because NewAppModel returns a model, not a command:
	// Init is the first place a tea.Cmd can exist.
	startup tea.Msg

	// quitting records that the session is on the way out, so the final View
	// paints an empty frame instead of a torn one over the restored terminal.
	quitting bool

	// launchFocus is the one-shot intent to seat the cursor where a fresh launch
	// begins. It belongs to the session rather than to the Outliner because only
	// the shell knows which act armed it: the launch message arms it, and the
	// clean-root reset that message shares with every later reload does not, so
	// the rule cannot leak into a refresh, a mode change, an Esc restoration, or
	// the reset itself.
	//
	// It is spent by the first settled population — after any saved lens has
	// published, since an application still reading its own projects leaves the
	// population loading — and dropped by the first keystroke, because a user who
	// has already pressed a key owns the cursor and a load still on its way may
	// not move it.
	launchFocus bool

	// colorDisabled is a --no-color run. The terminal background tt announces
	// is written outside the renderer's color profile, so that run announces
	// none rather than color the terminal behind a colorless frame.
	colorDisabled bool
}

// WithDisabledColor marks a --no-color run. RunApp translates the flag into
// this option; the renderer's own profile already strips the cells.
func WithDisabledColor() AppOption {
	return func(m *AppModel) { m.lifecycle.colorDisabled = true }
}

// armLaunchFocus declares that this session still owes the cursor its starting
// position.
func (c *lifecycleController) armLaunchFocus() {
	if c != nil {
		c.launchFocus = true
	}
}

// dropLaunchFocus abandons the intent without honoring it. It is what newer
// navigation costs a launch that has not landed yet.
func (c *lifecycleController) dropLaunchFocus() {
	if c != nil {
		c.launchFocus = false
	}
}

// launchFocusArmed reports whether the starting position is still owed.
func (c *lifecycleController) launchFocusArmed() bool {
	return c != nil && c.launchFocus
}

// startupCmd returns the command that delivers the armed startup message, or nil
// when nothing was armed. The message is captured by value so a later write to
// the field cannot change what this command already promised to deliver.
func (c *lifecycleController) startupCmd() tea.Cmd {
	if c == nil || c.startup == nil {
		return nil
	}
	msg := c.startup
	return func() tea.Msg { return msg }
}

// periodicSaveCmd schedules the next periodic tick. It is the single
// construction of the app's one recurring timer: Init arms it, and the handler
// for each fire re-arms it through this same call.
func (c *lifecycleController) periodicSaveCmd() tea.Cmd {
	return status.TickCmd(periodicSaveInterval, func(at time.Time) tea.Msg {
		return status.PeriodicSaveMsg{At: at}
	})
}

// AppVersion is the application version string used when writing theme files.
// Set this from the cmd package before calling RunApp.
var AppVersion = "dev"

// RunOptions configures process-level TUI behavior independently from the
// AppModel options used by tests and dependency injection.
type RunOptions struct {
	DisableColor bool
	// HideAdvisories suppresses every ambient advisory for this run without
	// touching the machine-local setting. See advisories_shell.go.
	HideAdvisories bool
	// Notify sends a desktop notification escape when a check-in arrives, a
	// timed deadline passes, or a deferral ends. See notifyArrivals below.
	Notify bool
	// ControlSocket serves internal/control's socket at this path for the life
	// of the session, so a host such as tt.app can ask it to act and hear its
	// pulse and arrivals. See the control-socket section below.
	ControlSocket string
}

// RunApp starts the TUI application over an opened runtime. engine.Open has
// already recovered and loaded the workspace, so the shell takes the process
// lock and builds its presentation over that one composition.
func RunApp(rt *engine.Runtime, runOpts RunOptions, opts ...AppOption) error {
	if runOpts.HideAdvisories {
		opts = append(opts, WithHiddenAdvisories())
	}
	if runOpts.Notify {
		opts = append(opts, WithNotifications())
	}
	if runOpts.DisableColor {
		opts = append(opts, WithDisabledColor())
	}
	if rt == nil {
		return fmt.Errorf("runtime is required")
	}
	dataDir := rt.DataDir()
	// Keep the TUI PID lock machine-local: when the durable data dir lives on
	// iCloud Drive it must never sync, so route it through the local runtime dir.
	lockPath := config.TUIProcessLockPath(dataDir)
	if mkdirErr := os.MkdirAll(filepath.Dir(lockPath), 0700); mkdirErr != nil {
		return fmt.Errorf("create runtime dir: %w", mkdirErr)
	}
	lock, err := util.AcquirePIDLock(lockPath)
	if err != nil {
		return err
	}
	defer lock.Release()

	model, err := NewAppModel(rt, opts...)
	if err != nil {
		return err
	}

	p := tea.NewProgram(model, tea.WithEnvironment(tuiEnvironment(os.Environ(), runOpts.DisableColor)))

	if runOpts.ControlSocket != "" {
		server, listenErr := control.Listen(runOpts.ControlSocket, controlDispatch(p.Send))
		if listenErr != nil {
			return listenErr
		}
		defer func() { _ = server.Close() }()
		WithControlPublisher(server)(model)
	}

	// Recover from panics to restore terminal state before crashing.
	defer func() {
		if r := recover(); r != nil {
			p.Kill()
			_ = p.ReleaseTerminal()
			fmt.Fprintf(os.Stderr, "\ntt: unexpected panic: %v\n", r)
			fmt.Fprintf(os.Stderr, "Please report this at https://github.com/sglavoie/tt/issues\n")
			os.Exit(1)
		}
	}()

	_, err = p.Run()
	return err
}

// tuiEnvironment ignores an inherited NO_COLOR for the interactive TUI while
// preserving every other terminal capability hint. The explicit --no-color
// flag sets DisableColor, in which case cmd has already normalized NO_COLOR=1
// and the renderer receives the process environment unchanged.
func tuiEnvironment(environ []string, disableColor bool) []string {
	result := make([]string, 0, len(environ))
	for _, entry := range environ {
		key, _, _ := strings.Cut(entry, "=")
		if !disableColor && strings.EqualFold(key, "NO_COLOR") {
			continue
		}
		result = append(result, entry)
	}
	return result
}

// syncedDomainArtifacts renders the two explicit synced configuration domains
// into revision-checked artifacts for a transaction that intentionally spans
// catalog and planning state.
//
// The candidates were derived from a runtime snapshot, so they are stamped with
// the revisions that snapshot observed. Reloading the domains here to obtain a
// fresher baseline would silently re-parent stale in-memory intent onto a
// concurrent writer's state and overwrite it; the snapshot's own revision is
// what makes an intervening write fail closed.
func syncedDomainArtifacts(
	runtime *config.RuntimeState,
	catalog *config.WorkspaceCatalog,
	planning *config.PlanningState,
) ([]storage.ArtifactChange, error) {
	domainArts, err := runtime.CheckedSyncedArtifacts(&config.Domains{Catalog: catalog, Planning: planning})
	if err != nil {
		return nil, err
	}
	arts := make([]storage.ArtifactChange, 0, len(domainArts))
	for _, da := range domainArts {
		arts = append(arts, da.StorageChange())
	}
	return arts, nil
}

func machineRepairWarning(action string, err error) string {
	if err == nil {
		return action
	}
	logging.Warn("%s; machine-state cleanup will retry: %v", action, err)
	return fmt.Sprintf("%s — machine-state cleanup incomplete; retrying", action)
}

func workspaceLedgerFromConfig(runtime *config.RuntimeState) *storage.WorkspaceLedger {
	dataDir := ""
	if runtime != nil {
		dataDir = runtime.DataDir()
	}
	return storage.NewWorkspaceLedger(config.UndoLedgerDir(dataDir), []string{dataDir})
}

// pipelineNextHint returns a contextual "next step" hint string for a view.
// Returns "" for views that should show the default help text.
func pipelineNextHint(vt presentation.PresentationID) string {
	switch vt {
	case presentation.ViewTaskComposer:
		return "Next: Inbox triage | Tab cycles modes"
	case presentation.ViewCrossProjectOutliner:
		return "Next: Tab cycles modes | wt plans Today"
	default:
	}
	return ""
}

// =============================================================================
// Attention while in the background: focus reload and --notify
// =============================================================================

// externalReloadState is the shell's memory of what the workspace looked like
// on disk when the terminal last lost focus. A long-lived session — tt.app hides
// its window rather than ending tt — otherwise never re-reads a workspace that
// a synced machine changed underneath it, and learns of that change only when
// its own next commit records a conflict.
//
// It needs a terminal that reports focus (mode 1004). Without one neither
// message arrives and nothing here runs.
type externalReloadState struct {
	// blurFingerprint is the workspace fingerprint taken at the last blur; empty
	// while focused and before the first blur, which is what keeps a launch from
	// reloading. A control-socket commit made while blurred advances it, since
	// that change is tt's own.
	blurFingerprint string
	// owed records a detected change the shell could not yet adopt because the
	// user was mid-interaction. The periodic tick and the next focus retry it.
	owed bool
}

// workspaceFingerprint is workspace.Fingerprint over the runtime's data dir
// and every catalog project, parked and archived included, since a reload
// rereads them all.
func (m *AppModel) workspaceFingerprint() string {
	rs := m.domain.runtime
	if rs == nil {
		return ""
	}
	return workspace.Fingerprint(rs.DataDir(), rs.AllProjects())
}

// handleFocusMessage answers the terminal's focus reports. A blur records the
// workspace as it stands; a focus compares against it and adopts whatever
// changed on disk meanwhile.
func (m *AppModel) handleFocusMessage(msg tea.Msg) (bool, tea.Cmd) {
	switch msg.(type) {
	case tea.BlurMsg:
		m.externalReload.blurFingerprint = m.workspaceFingerprint()
		return true, nil
	case tea.FocusMsg:
		before := m.externalReload.blurFingerprint
		m.externalReload.blurFingerprint = ""
		if before != "" && before != m.workspaceFingerprint() {
			m.externalReload.owed = true
		}
		return true, m.adoptExternalWorkspaceChange()
	default:
		return false, nil
	}
}

// adoptBlurredWorkspaceChange is the focus check for a session acting while
// blurred, as a control-socket request makes it: a change made elsewhere since
// the blur is adopted before acting, so the act commits against a fresh read.
func (m *AppModel) adoptBlurredWorkspaceChange() tea.Cmd {
	before := m.externalReload.blurFingerprint
	if before == "" {
		return nil
	}
	if current := m.workspaceFingerprint(); before != current {
		m.externalReload.owed = true
		m.externalReload.blurFingerprint = current
	}
	return m.adoptExternalWorkspaceChange()
}

// recordBlurredWorkspace advances a blurred session's fingerprint over a commit
// of its own.
func (m *AppModel) recordBlurredWorkspace() {
	if m.externalReload.blurFingerprint != "" {
		m.externalReload.blurFingerprint = m.workspaceFingerprint()
	}
}

// safeToAdoptExternalChange is the time-driven invalidation's own guard plus
// the Outliner being the live route: a reload rebinds every project pointer,
// so it waits for the user to put down a picker, an editor, a selection, or a
// child view rather than renumber what they are acting on.
func (m *AppModel) safeToAdoptExternalChange() bool {
	return m.currentView() == presentation.ViewCrossProjectOutliner &&
		m.activeOutlinerRoute() &&
		m.safeToInvalidateTimeDrivenRows()
}

// adoptExternalWorkspaceChange reloads the workspace from disk when a change is
// owed and the shell can take it. It is the ledger restore's reload without the
// restore: catalog and planning first, so a project another machine created is
// present, then every project's tasks, then the planning collections resolved
// against them. It writes nothing; reconciling planning is a commit's job.
func (m *AppModel) adoptExternalWorkspaceChange() tea.Cmd {
	if !m.externalReload.owed || !m.safeToAdoptExternalChange() {
		return nil
	}
	m.externalReload.owed = false
	m.refreshRuntime()
	for id := range m.domain.projects {
		m.reloadProjectByID(id)
	}
	m.rebuildPlanningFromRuntime()
	m.refreshLoadedTaskWorkspace()
	m.refreshOutstandingConflicts()
	cmds := m.refreshTaskViews(true, true)
	m.setInfoStatus("Reloaded: the workspace changed on disk")
	cmds = append(cmds, m.statusReadingTimeout())
	return tea.Batch(cmds...)
}

// notificationState is a --notify run's memory of the last instant it
// reported on. Each periodic tick reports what arrived in (since, now] and
// advances it, so a launch reports nothing already standing and a machine
// waking from sleep reports the whole gap once.
type notificationState struct {
	enabled bool
	since   time.Time
}

// WithNotifications marks a --notify run: the session sends a desktop
// notification escape when work arrives while it runs. It is opt-in because a
// terminal that honors the escape shows it whether or not tt is in view.
func WithNotifications() AppOption {
	return func(m *AppModel) {
		m.notifications.enabled = true
		m.notifications.since = time.Now()
	}
}

// notifyArrivals answers one periodic tick for a --notify run. It reads the
// base task workspace through the pulse's own loader, which reparses only
// files that changed, so a quiet tick costs a stat per project.
func (m *AppModel) notifyArrivals(now time.Time) tea.Cmd {
	if (!m.notifications.enabled && !m.control.enabled()) || m.domain.runtime == nil {
		return nil
	}
	since := m.notifications.since
	m.notifications.since = now
	if since.IsZero() || !now.After(since) {
		return nil
	}
	_, projects, failed := outliner.LoadCrossScopeSelection(m.domain.runtime.TaskWorkspaceProjects(), m.ensurePulseLoader())
	for _, failure := range failed {
		logging.Warn("notifications: %s", failure.Detail)
	}
	arrivals := pulse.Arrivals(projects, since, now)
	if len(arrivals) == 0 {
		return nil
	}
	m.publishControlArrivals(arrivals)
	if !m.notifications.enabled {
		return nil
	}
	return tea.Raw(pulse.DesktopNotification(pulse.ArrivalNotification(arrivals)))
}

// ensurePulseLoader returns the shell's retained snapshot loader, creating it
// on first use. Both readers run inside Update, never in a command goroutine.
func (m *AppModel) ensurePulseLoader() *workspace.Loader {
	if m.pulseLoader == nil {
		m.pulseLoader = workspace.NewLoader()
	}
	return m.pulseLoader
}

// =============================================================================
// The control socket: a host asking the running session to act
// =============================================================================
//
// `tt --control PATH` serves internal/control's socket for a host such as
// tt.app. Every request arrives here as a controlRequestMsg and is answered
// from inside Update, so it reads the same state a keystroke would and commits
// through the same coordinator, transaction, and undo ledger. A request is
// refused, never queued, while the user is mid-interaction: the guard is the
// external reload's, since a commit reloads the projects a picker, an editor, or
// a selection is acting on.

// controlPublisher is the socket's outbound half, as the shell sees it.
type controlPublisher interface {
	Publish(control.Event)
}

// controlState is the shell's link to the socket, nil-safe when tt runs
// without one.
type controlState struct {
	publisher controlPublisher
	// published is the last pulse sent, so an unchanged pulse is never resent.
	published *pulse.WorkspacePulse
}

// WithControlPublisher connects the session to a control socket's events.
// Arrivals are reported from launch on, as --notify reports them.
func WithControlPublisher(publisher controlPublisher) AppOption {
	return func(m *AppModel) {
		m.control.publisher = publisher
		if m.notifications.since.IsZero() {
			m.notifications.since = time.Now()
		}
	}
}

func (c *controlState) enabled() bool { return c != nil && c.publisher != nil }

// controlRequestMsg carries one request and the channel its reply goes to.
// The channel is buffered, so answering never blocks Update.
type controlRequestMsg struct {
	request control.Request
	reply   chan<- control.Reply
}

// controlDispatch is the socket's Dispatch for a running program: it hands the
// request to Update and returns where the answer will arrive.
func controlDispatch(send func(tea.Msg)) control.Dispatch {
	return func(request control.Request) <-chan control.Reply {
		replies := make(chan control.Reply, 1)
		send(controlRequestMsg{request: request, reply: replies})
		return replies
	}
}

// controlCommittedMsg is a socket request's resolved commit: the coordinator's
// own message, handled as the save pipeline handles it, plus the undo toast a
// capture owes. A capture travels as TasksCapturedMsg rather than
// TaskCreatedMsg, because that one pops the live view and the user was not in
// a composer.
type controlCommittedMsg struct {
	inner    tea.Msg
	feedback string
	txID     string
}

func answer(reply chan<- control.Reply, ok bool, message string) {
	reply <- control.Reply{OK: ok, Message: message}
}

// handleControlMessage answers the socket's requests and its own capture
// results.
func (m *AppModel) handleControlMessage(msg tea.Msg) (bool, tea.Cmd) {
	switch msg := msg.(type) {
	case controlRequestMsg:
		return true, m.answerControlRequest(msg.request, msg.reply)
	case controlCommittedMsg:
		return true, m.adoptControlCommit(msg)
	default:
		return false, nil
	}
}

func (m *AppModel) answerControlRequest(request control.Request, reply chan<- control.Reply) tea.Cmd {
	switch request.Op {
	case control.OpComplete:
		// Ahead of the busy guard: a completion reads the workspace and commits
		// nothing, so there is nothing for an open picker or editor to lose.
		m.controlComplete(request.Text, reply)
		return nil
	case control.OpPreview:
		m.controlPreview(request.Text, reply)
		return nil
	case control.OpReveal, control.OpAcknowledge, control.OpDefer, control.OpCapture:
	default:
		answer(reply, false, fmt.Sprintf("unknown operation %q", request.Op))
		return nil
	}
	if !m.safeToAdoptExternalChange() {
		answer(reply, false, "tt is busy: finish what is open in tt first")
		return nil
	}
	// A blurred session may be holding a workspace another machine changed;
	// acting on it would commit against a stale read and record a conflict.
	reload := m.adoptBlurredWorkspaceChange()

	var cmd tea.Cmd
	switch request.Op {
	case control.OpCapture:
		cmd = m.controlCapture(request.Text, reply)
	default:
		identity := taskref.TaskIdentity{ProjectID: request.Project, TaskID: request.Task}
		task, project := m.taskAndProjectForIdentity(identity)
		if task == nil || project == nil {
			answer(reply, false, "that task is no longer in the workspace")
			return reload
		}
		switch request.Op {
		case control.OpReveal:
			cmd = m.controlReveal(task, project, reply)
		case control.OpAcknowledge:
			cmd = m.controlAcknowledge(task, project, reply)
		case control.OpDefer:
			cmd = m.controlDefer(task, project, request.Minutes, reply)
		}
	}
	return tea.Batch(reload, cmd)
}

// controlReveal seats the cursor on the task: in place when the current view
// can show it, otherwise through the same scope-and-All recipe `gp` uses, so
// Esc returns to the view the user left.
func (m *AppModel) controlReveal(task *models.Task, project *models.Project, reply chan<- control.Reply) tea.Cmd {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil {
		answer(reply, false, "the Outliner is not open")
		return nil
	}
	// The user asked for this task; a launch still settling may not move it.
	m.lifecycle.dropLaunchFocus()
	identity := taskref.For(task, project)
	previous := out.Cursor
	out.NavigateToTaskInProject(task, project)
	answer(reply, true, "Shown: "+task.Title)
	if _, found := out.RowIndexForTask(identity); found {
		return listsurface.FlashIfJumped(previous, out.Cursor, &out.CursorFlash)
	}
	targets := operation.ResolvedOperationTargets{Targets: []operation.OperationTarget{{Task: identity, ProjectID: project.ID}}}
	if result, ok := m.coreGotoTaskProject(targets).Message.(operation.OperationCommandResultMsg); ok {
		return result.Cmd
	}
	return nil
}

// controlAcknowledge reviews one task as R does, without the advance: the
// cursor is not where the request came from.
func (m *AppModel) controlAcknowledge(task *models.Task, project *models.Project, reply chan<- control.Reply) tea.Cmd {
	if !models.TaskAllowsExplicitReview(task, time.Now()) {
		answer(reply, false, "task has no review to acknowledge")
		return nil
	}
	identity := taskref.For(task, project)
	capture := m.captureOutlinerRelocationForLocators([]app.TaskLocator{identity.Locator()})
	cmd := withOutlinerRelocationCapture(m.secondaryFeatures.TaskItem.MarkReviewed(identity), capture)
	return replyWhenCommitted(cmd, reply, "Acknowledged: "+task.Title)
}

// controlDefer defers one task by minutes from now, through the request the
// date picker's Defer section builds.
func (m *AppModel) controlDefer(task *models.Task, project *models.Project, minutes int, reply chan<- control.Reply) tea.Cmd {
	if minutes <= 0 {
		answer(reply, false, "a deferral needs a positive number of minutes")
		return nil
	}
	if refusal := app.DeferralRefusesState(task.WorkflowState); refusal != "" {
		answer(reply, false, refusal)
		return nil
	}
	until := time.Now().Add(time.Duration(minutes) * time.Minute)
	locator := taskref.For(task, project).Locator()
	request := app.TaskOperationRequest{
		Operation: app.TaskOperationSetDefer,
		Targets:   []app.TaskLocator{locator},
		Date:      &until,
	}
	capture := m.captureOutlinerRelocationForLocators([]app.TaskLocator{locator})
	cmd := withOutlinerRelocationCapture(m.secondaryFeatures.TaskItem.ExecuteOperation(request, "Set Defer", ""), capture)
	return replyWhenCommitted(cmd, reply, "Deferred until "+until.Format("15:04")+": "+task.Title)
}

// controlCapture reads text as the global `a` composer would and commits it as
// one capture transaction.
func (m *AppModel) controlCapture(text string, reply chan<- control.Reply) tea.Cmd {
	preset, ok := m.captureComposerPreset(nil, nil)
	if !ok {
		answer(reply, false, "the Unfiled project is unavailable")
		return nil
	}
	reqs, lineErrors := taskform.ParseCaptureText(preset, m.domain.runtime, text)
	if len(lineErrors) > 0 {
		answer(reply, false, controlLineErrors(lineErrors, strings.Count(strings.TrimSpace(text), "\n") > 0))
		return nil
	}
	cmd := m.features.Capture.CaptureBulk(reqs, false)
	return func() tea.Msg {
		msg := cmd()
		captured, ok := msg.(coordinator.TasksCapturedMsg)
		if !ok || len(captured.Results) == 0 {
			answer(reply, false, commitRefusal(msg))
			return controlCommittedMsg{inner: msg}
		}
		summary := controlCapturedSummary(captured.Results)
		answer(reply, true, summary)
		return controlCommittedMsg{inner: captured, feedback: summary, txID: captured.Results[0].TransactionID}
	}
}

// controlComplete answers what the global `a` composer would suggest for the
// draft up to the host's caret. No suggestion is still a success: it is the
// answer for a caret standing on prose.
func (m *AppModel) controlComplete(text string, reply chan<- control.Reply) {
	preset, ok := m.captureComposerPreset(nil, nil)
	if !ok {
		answer(reply, false, "the Unfiled project is unavailable")
		return
	}
	set := taskform.CaptureCompletions(preset, m.domain.runtime, text)
	result := control.Reply{OK: true}
	if set.Token != "" {
		completion := &control.Completion{Token: set.Token, Exact: set.Exact, Items: make([]control.CompletionItem, 0, len(set.Items))}
		for _, suggestion := range set.Items {
			completion.Items = append(completion.Items, control.CompletionItem{Label: suggestion.Label, Insert: suggestion.Insert})
		}
		result.Completion = completion
	}
	reply <- result
}

func (m *AppModel) controlPreview(text string, reply chan<- control.Reply) {
	preset, ok := m.captureComposerPreset(nil, nil)
	if !ok {
		answer(reply, false, "the Unfiled project is unavailable")
		return
	}
	preview, lineErrors := taskform.CapturePreviewText(preset, m.domain.runtime, text)
	if len(lineErrors) > 0 {
		answer(reply, false, controlLineErrors(lineErrors, strings.Contains(strings.TrimSpace(text), "\n")))
		return
	}
	reply <- control.Reply{OK: true, Preview: preview}
}

// adoptControlCommit reloads what a socket request committed through the save
// pipeline's own handler, then records the workspace a blurred session now
// holds, so the next focus does not mistake tt's own commit for another
// machine's.
func (m *AppModel) adoptControlCommit(msg controlCommittedMsg) tea.Cmd {
	_, refresh := m.handleGlobalSaveMessage(msg.inner)
	m.recordBlurredWorkspace()
	if msg.feedback == "" {
		return refresh
	}
	return tea.Batch(refresh, m.setRecoveryFeedback(msg.feedback, msg.txID, false))
}

func controlCapturedSummary(results []app.TaskResult) string {
	if len(results) == 1 {
		return "Captured: " + results[0].Title
	}
	return "Captured " + util.Count(len(results), "task")
}

func controlLineErrors(lineErrors []grammar.LineError, multiline bool) string {
	parts := make([]string, 0, len(lineErrors))
	for _, lineError := range lineErrors {
		if multiline {
			parts = append(parts, fmt.Sprintf("line %d: %s", lineError.Line, lineError.Message))
		} else {
			parts = append(parts, lineError.Message)
		}
	}
	return strings.Join(parts, "; ")
}

// replyWhenCommitted answers the socket once a coordinator command's commit
// has resolved, and hands its message back to Update.
func replyWhenCommitted(cmd tea.Cmd, reply chan<- control.Reply, success string) tea.Cmd {
	if cmd == nil {
		answer(reply, false, "nothing to do")
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		if refusal := commitRefusal(msg); refusal != "" {
			answer(reply, false, refusal)
		} else {
			answer(reply, true, success)
		}
		return controlCommittedMsg{inner: msg}
	}
}

// commitRefusal is the error a coordinator's message carries, or "" when it
// committed.
func commitRefusal(msg tea.Msg) string {
	var err error
	switch msg := msg.(type) {
	case coordinator.TransactionErrorMsg:
		err = msg.Err
	case status.ErrorMsg:
		err = msg.Err
	case coordinator.TransactionResultMsg, coordinator.BatchTransactionResultMsg, coordinator.TasksCapturedMsg:
		return ""
	default:
		err = errors.New("tt did not commit the change")
	}
	if err == nil {
		return "tt did not commit the change"
	}
	return err.Error()
}

// =============================================================================
// Events: the pulse and arrivals
// =============================================================================

// publishControlPulse sends the pulse when it changed since the last one sent.
// It runs in Update's one deferred closure, after every refresh a message did.
func (m *AppModel) publishControlPulse() {
	if !m.control.enabled() {
		return
	}
	current := m.workspacePulse
	if m.control.published != nil && *m.control.published == current {
		return
	}
	m.control.published = &current
	m.control.publisher.Publish(control.Event{
		Event: control.EventPulse,
		Pulse: &control.Pulse{Inbox: current.Inbox, Today: current.Today, Overdue: current.Overdue, Done: current.Done},
	})
}

// publishControlArrivals sends one tick's arrivals with each task's identity,
// so the host can offer to act on it.
func (m *AppModel) publishControlArrivals(arrivals []pulse.Arrival) {
	if !m.control.enabled() || len(arrivals) == 0 {
		return
	}
	title, body := pulse.ArrivalNotification(arrivals)
	event := control.Event{Event: control.EventArrivals, Title: title, Body: body}
	for _, arrival := range arrivals {
		event.Arrivals = append(event.Arrivals, control.Arrival{
			Kind:       controlArrivalKind(arrival.Kind),
			Project:    arrival.ProjectID,
			Task:       arrival.TaskID,
			Title:      arrival.Title,
			Reviewable: arrival.Reviewable,
		})
	}
	m.control.publisher.Publish(event)
}

func controlArrivalKind(kind pulse.ArrivalKind) string {
	switch kind {
	case pulse.ArrivalCheckIn:
		return control.ArrivalCheckIn
	case pulse.ArrivalDeadline:
		return control.ArrivalDeadline
	default:
		return control.ArrivalWake
	}
}
