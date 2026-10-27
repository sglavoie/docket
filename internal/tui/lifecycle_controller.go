package tui

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/control"
	"github.com/sglavoie/docket/internal/engine"
	"github.com/sglavoie/docket/internal/logging"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/pulse"
	"github.com/sglavoie/docket/internal/tui/status"
	"github.com/sglavoie/docket/internal/util"
	"github.com/sglavoie/docket/internal/workspace"
)

// periodicSaveInterval is the cadence of the recurring maintenance timer. It is the
// span a scenario advances to observe something only the tick can produce, such
// as an outstanding-conflict count another machine's sync wrote.
const periodicSaveInterval = 60 * time.Second

// lifecycleController is the shell's session-lifecycle cluster: the one message
// the composition root arms for the first frame, and the flag a quit sets so the
// final paint renders nothing.
//
// It also retains negotiated keyboard capabilities for accurate shortcut hints.
// RunApp owns the process lock and panic recovery; teardown releases the lock
// beside the Bubble Tea program.
//
// It also owns the scheduling of the recurring maintenance timer, because the two places
// that arm it — Init and the handler that re-arms it on each fire — must arm the
// same one. App timers go through status.TickCmd and never tea.Tick, so the tick
// stays virtualizable by the scenario harness; keeping the construction here is
// what stops a second, differently-scheduled periodic save from appearing.
type lifecycleController struct {
	// keyDisambiguation is negotiated by Bubble Tea; until then use legacy-safe hints.
	keyDisambiguation bool

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

	// colorDisabled is a --no-color run. The terminal background Docket announces
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

// periodicSaveCmd schedules the next periodic maintenance tick. It is the single
// construction of that timer: Init arms it, and the handler
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
	VersionLine  string
	DisableColor bool
	// HideAdvisories suppresses every ambient advisory for this run without
	// touching the machine-local setting. See advisories_shell.go.
	HideAdvisories bool
	// DisableMouse leaves mouse reporting off for this run without touching the
	// machine-local setting. See WithDisabledMouse in app_advisories.go.
	DisableMouse bool
	// Notify sends a desktop notification escape when a check-in arrives, a
	// timed deadline passes, or a deferral ends. See notifyArrivals below.
	Notify bool
	// ControlSocket serves internal/control's socket at this path for the life
	// of the session, so a host such as Docket.app can ask it to act and hear its
	// pulse and arrivals. See the control-socket section below.
	ControlSocket string
	// Headless runs the session with no terminal: no input, no renderer, and a
	// fixed size, answering only ControlSocket. Docket.app runs one while its
	// window is closed, so the pulse, the schedule, sync, and the requests a
	// widget or a shortcut makes keep being answered by this same Update. It
	// takes the same process lock, so it never runs beside a windowed session.
	Headless bool
}

// headlessSize is the terminal a headless session pretends to have, so every
// view lays out as it would in an ordinary window.
const (
	headlessWidth  = 120
	headlessHeight = 40
)

// RunApp starts the TUI application over an opened runtime. engine.Open has
// already recovered and loaded the workspace, so the shell takes the process
// lock and builds its presentation over that one composition.
func RunApp(rt *engine.Runtime, runOpts RunOptions, opts ...AppOption) error {
	if runOpts.HideAdvisories {
		opts = append(opts, WithHiddenAdvisories())
	}
	if runOpts.DisableMouse {
		opts = append(opts, WithDisabledMouse())
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
	model.control.versionLine = runOpts.VersionLine

	if runOpts.Headless && runOpts.ControlSocket == "" {
		return fmt.Errorf("a headless session needs a control socket")
	}
	programOpts := []tea.ProgramOption{tea.WithEnvironment(tuiEnvironment(os.Environ(), runOpts.DisableColor))}
	if runOpts.Headless {
		programOpts = append(programOpts,
			tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(),
			tea.WithWindowSize(headlessWidth, headlessHeight))
	}
	p := tea.NewProgram(model, programOpts...)

	if runOpts.ControlSocket != "" {
		server, listenErr := control.Listen(runOpts.ControlSocket, controlDispatch(p.Send))
		if listenErr != nil {
			return listenErr
		}
		defer func() { _ = server.Close() }()
		WithControlPublisher(server)(model)
	}
	if runOpts.Headless {
		// Nobody is looking at a headless session, so it starts blurred: every
		// request it answers first adopts what changed on disk since, as a
		// windowed session's would while its window is hidden.
		go p.Send(tea.BlurMsg{})
		stop := make(chan struct{})
		defer close(stop)
		go quitWhenOrphaned(p, os.Getppid(), headlessHostPoll, stop)
	}

	// Recover from panics to restore terminal state before crashing.
	defer func() {
		if r := recover(); r != nil {
			p.Kill()
			_ = p.ReleaseTerminal()
			fmt.Fprintf(os.Stderr, "\ntt: unexpected panic: %v\n", r)
			fmt.Fprintf(os.Stderr, "Please report this at https://github.com/sglavoie/docket/issues\n")
			os.Exit(1)
		}
	}()

	_, err = p.Run()
	return err
}

// headlessHostPoll is how often a headless session checks that the host that
// started it is still there.
const headlessHostPoll = 2 * time.Second

// quitWhenOrphaned ends a headless session whose host went away without ending
// it, as a crashed Docket.app would: nothing could reach it any more, and it
// would hold the process lock the next host's Docket needs. Reparenting is how
// the host's death shows, since the session's parent changes when it exits.
func quitWhenOrphaned(p *tea.Program, host int, every time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if os.Getppid() != host {
				logging.Warn("headless: host %d is gone; quitting", host)
				p.Quit()
				return
			}
		}
	}
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
// terminal that honors the escape shows it whether or not Docket is in view.
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
	projects, failed := outliner.LoadCrossScopeSelection(m.domain.runtime.TaskWorkspaceProjects(), m.ensurePulseLoader())
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
