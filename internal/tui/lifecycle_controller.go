package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/status"
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
	// timed deadline passes, or a deferral ends. See app_notifications.go.
	Notify bool
}

// RunApp starts the TUI application.
func RunApp(runtime *config.RuntimeState, manager *config.Manager, runOpts RunOptions, opts ...AppOption) error {
	if runOpts.HideAdvisories {
		opts = append(opts, WithHiddenAdvisories())
	}
	if runOpts.Notify {
		opts = append(opts, WithNotifications())
	}
	if runOpts.DisableColor {
		opts = append(opts, WithDisabledColor())
	}
	if runtime == nil {
		return fmt.Errorf("runtime state is required")
	}
	var err error
	runtime, err = recoverTUIWorkspace(runtime)
	if err != nil {
		return err
	}
	dataDir := runtime.DataDir()
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

	model, err := NewAppModel(runtime, manager, opts...)
	if err != nil {
		return err
	}

	p := tea.NewProgram(model, tea.WithEnvironment(tuiEnvironment(os.Environ(), runOpts.DisableColor)))

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
	// before the first blur, which is what keeps a launch from reloading.
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
		if before != "" && before != m.workspaceFingerprint() {
			m.externalReload.owed = true
		}
		return true, m.adoptExternalWorkspaceChange()
	default:
		return false, nil
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
	if !m.notifications.enabled || m.domain.runtime == nil {
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
