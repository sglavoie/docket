package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// This file builds the reliable data surfaces the LLM-driven review harness
// drives against. Two shapes are supported:
//
//   - Built-in fixtures construct a fresh
//     isolated temp data dir, write projects through storage, and launch the
//     driver from that copied config.
//   - Copied-config workspaces load a real user config, copy its data dir into
//     temp storage, assign the copy a fresh workspace identity, and launch the
//     driver from there — never mutating the source config, machine state, or
//     task files.
//
// Fixture builders live beside the driver in internal/tui so committed
// scenarios exercise production model construction without importing unrelated
// test helpers.

// Built-in fixture names accepted by NewFixtureDriver and the committed
// test-only scenario harness.
const (
	FixtureEmpty        = "empty"
	FixtureFresh        = "fresh"
	FixtureRich         = "rich"
	FixtureBusy         = "busy"
	FixtureWorkflow     = "workflow"
	FixtureNarrow       = "narrow"
	FixtureMultiProject = "multi-project"
	FixtureInboxTriage  = "inbox-triage"
	FixtureRelease      = "release"
	FixtureParked       = "parked"
	FixtureProjectMix   = "project-mix"
	FixtureSignals      = "signals"
	FixtureStale        = "stale"
	FixtureChecklist    = "checklist"
)

// Checklist fixture identities. Scenarios assert on these titles and item texts,
// so the builder and the committed JSON agree through these constants.
const (
	checklistFixtureProjectName = "Checklist Delivery"
	checklistFixtureFoldTitle   = "Draft the launch runbook"
	checklistFixtureParentTitle = "Coordinate the rollout window"
	checklistFixturePlainTitle  = "Retire the legacy runbook"
	checklistFixtureFirstItem   = "Outline the rollback steps"
	checklistFixtureSecondItem  = "Name the on-call owner"
	checklistFixtureThirdItem   = "Attach the comms draft"
)

// Signals fixture identities. Scenarios assert on the project slug and on task
// titles, so the builder and the committed JSON agree through these constants.
const (
	signalsFixtureProjectName = "Signal Delivery Coordination Track"
	signalsFixtureOtherName   = "Signal Beta"
)

// Inbox-triage fixture identities. The advance scenarios assert on these exact
// titles, and no title may be a substring of another so a `contains` check can
// only be answered by the row it names.
const (
	inboxTriageAlphaName         = "Triage Alpha"
	inboxTriageBetaName          = "Triage Beta"
	inboxTriageAlphaLeadTitle    = "Alpha intake ready for triage"
	inboxTriageAlphaFinalTitle   = "Alpha intake closing the group"
	inboxTriageBetaParentTitle   = "Beta intake parent awaiting triage"
	inboxTriageBetaChildTitle    = "Beta intake child promoted on departure"
	inboxTriageBetaTrailingTitle = "Beta intake trailing remainder"

	inboxTriageAlphaLeadDetail  = "Detail line belonging to the departing intake."
	inboxTriageAlphaFinalDetail = "Detail line belonging to the successor intake."
)

// Parked fixture identities. Scenarios assert on these strings, and
// models.NewProject stores a name verbatim, so the constants are the single
// source both the builder and the committed JSON agree on.
const (
	parkedFixtureWorkspaceName = "Active Delivery"
	parkedFixtureParkedName    = "Parked Work"
)

// Project-mix fixture identities. The Select Project task filter asserts on
// presence and absence of these exact names, so no name may be a substring of
// another.
const (
	projectMixActiveName   = "Mix Active Delivery"
	projectMixEmptyName    = "Mix Quiet Planning"
	projectMixFinishedName = "Mix Finished Cleanup"
	projectMixParkedName   = "Mix Parked Migration"
	projectMixArchivedName = "Mix Retired Pilot"

	// Each project also carries a one-line description and a descriptive tag
	// whose words appear nowhere else in the fixture — not in another project's
	// metadata and not in any task — so a Select Project search by description or
	// tag can only be answered by the metadata aliases, and a search by task
	// wording can only be answered by a leak.
	projectMixActiveDescription   = "Coordinate the storefront launch calendar."
	projectMixEmptyDescription    = "Sketch the charter before anyone commits."
	projectMixFinishedDescription = "Retire the legacy warehouse scripts."
	projectMixParkedDescription   = "Resume the database cutover when staffing returns."
	projectMixArchivedDescription = "Shelved the kiosk trial after the field study."
)

// builtinFixtures maps each fixture name to the builder that populates its
// workspace. The workspace is already created (empty temp data dir + config)
// when the builder runs; the builder only adds projects and sets config state.
var builtinFixtures = map[string]func(*uitestWorkspace) error{
	FixtureEmpty:        buildEmptyFixture,
	FixtureFresh:        nil,
	FixtureRich:         buildRichFixture,
	FixtureBusy:         buildBusyFixture,
	FixtureWorkflow:     buildWorkflowFixture,
	FixtureNarrow:       buildNarrowFixture,
	FixtureMultiProject: buildMultiProjectFixture,
	FixtureInboxTriage:  buildInboxTriageFixture,
	FixtureRelease:      buildReleaseFixture,
	FixtureParked:       buildParkedFixture,
	FixtureProjectMix:   buildProjectMixFixture,
	FixtureSignals:      buildSignalsFixture,
	FixtureStale:        buildStaleFixture,
	FixtureChecklist:    buildChecklistFixture,
}

// BuiltinFixtureNames returns the sorted list of built-in fixture names.
func BuiltinFixtureNames() []string {
	names := make([]string, 0, len(builtinFixtures))
	for name := range builtinFixtures {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// uitestWorkspace is an isolated temp data directory plus its config/manager.
// A driver launched from a workspace reads and writes only within the temp
// tree, so review scenarios never touch real user data.
type uitestWorkspace struct {
	// Root is the temp directory removed by Cleanup. DataDir and ConfigPath
	// live inside it.
	Root       string
	DataDir    string
	ConfigPath string
	Runtime    *config.RuntimeState
	Manager    *config.Manager
	// currentProjectID is fixture-only session state. Runtime configuration no
	// longer persists a current project, but scenarios may explicitly choose one.
	currentProjectID string
	// startAtDefault asks launch to apply the same startup navigation selected by
	// AppModel.Init, without scheduling the real program's periodic timers.
	startAtDefault bool
	// noteSeeds are initial fixture artifacts, committed without an undo ledger
	// before the app launches. Scenarios then observe them as pre-existing
	// workspace content rather than as user mutations available to undo.
	noteSeeds []fixtureNoteSeed
	// homeBeforeSwitch is the $HOME this workspace found, recorded the first
	// time switchMachineHome moves it and put back by Cleanup. Machine identity
	// is exactly the home directory, so a switch is process-wide and has to be
	// undone for the tests that follow.
	homeBeforeSwitch string
	// machineSeq numbers the anonymous machine homes handed out by
	// switchMachineHome, so each unnamed switch lands on a directory no earlier
	// machine ever wrote to.
	machineSeq int
	// machineHomes maps a scenario's machine name to the home it was given, so a
	// named switch returns to that machine's own state instead of a fresh one.
	// Without it "A, then B, then back to A" is unexpressible: every switch would
	// look like a machine that has never seen this workspace.
	machineHomes map[string]string
}

type fixtureNoteSeed struct {
	dir  string
	note *models.Note
}

// Cleanup removes the workspace's temp directory. It is safe to call more than
// once and on a zero-value/partial workspace.
func (w *uitestWorkspace) Cleanup() error {
	if w == nil || w.Root == "" {
		return nil
	}
	if w.homeBeforeSwitch != "" {
		// A scenario that switched machines moved $HOME for the whole process.
		// Put it back before the temp tree the switch pointed into is removed,
		// so the next test resolves machine-local state where TestMain put it.
		_ = os.Setenv("HOME", w.homeBeforeSwitch)
		w.homeBeforeSwitch = ""
	}
	return os.RemoveAll(w.Root)
}

// switchMachineHome points machine-local state at a home inside this
// workspace's temp tree. Machine identity is the home directory — machine.json
// lives at <$HOME>/.tt/runtime/<workspace-id>/machine.json — so moving it is
// what makes the same synced workspace look like it was opened elsewhere. The
// synced directory is not touched.
//
// An empty name is the historical behavior: a fresh, numbered home no earlier
// machine wrote to, which is the "opened on a machine that has never seen this
// workspace" case. A name is a durable machine identity within this workspace:
// the first switch to it mints a home, every later switch returns to that same
// home with whatever that machine committed still in it.
func (w *uitestWorkspace) switchMachineHome(machine string) error {
	if w.homeBeforeSwitch == "" {
		w.homeBeforeSwitch = os.Getenv("HOME")
	}
	machine = strings.TrimSpace(machine)
	var home string
	switch existing, named := w.machineHomes[machine]; {
	case machine == "":
		w.machineSeq++
		home = filepath.Join(w.Root, fmt.Sprintf("machine-%d", w.machineSeq))
	case named:
		home = existing
	default:
		if w.machineHomes == nil {
			w.machineHomes = make(map[string]string)
		}
		home = filepath.Join(w.Root, "machine-"+machineHomeSlug(machine))
		w.machineHomes[machine] = home
	}
	if err := os.MkdirAll(home, 0o755); err != nil {
		return fmt.Errorf("failed to create machine home: %w", err)
	}
	if err := os.Setenv("HOME", home); err != nil {
		return fmt.Errorf("failed to switch machine home: %w", err)
	}
	return nil
}

// machineHomeSlug reduces a scenario's machine name to a directory-safe token so
// the home a failure names is recognizable rather than a bare counter.
func machineHomeSlug(machine string) string {
	var b strings.Builder
	for _, r := range machine {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r - 'A' + 'a')
		default:
			b.WriteRune('-')
		}
	}
	return b.String()
}

// secondMachineDriver opens this same synced workspace as a different machine
// and returns a driver over it. It is the two-machine counterpart to launch:
// the data directory is untouched, only machine identity moves, so whatever the
// first machine committed to the synced files is exactly what this one reads.
//
// The returned driver replaces the caller's: uitestWorkspace holds one Runtime,
// and this repoints it at the new machine's load. Switch back by name to return
// to the first machine.
func (w *uitestWorkspace) secondMachineDriver(machine string, width, height int) (*UITestDriver, error) {
	if err := w.switchMachineHome(machine); err != nil {
		return nil, err
	}
	app, err := w.relaunchApp()
	if err != nil {
		return nil, fmt.Errorf("failed to open workspace as machine %q: %w", machine, err)
	}
	driver := NewUITestDriver(app, width, height)
	driver.relaunch = w.relaunchApp
	driver.moveToFreshMachine = w.switchMachineHome
	if app.startupMessage != nil {
		driver.dispatch(app.startupMessage)
		driver.drain()
	}
	driver.bootNoteSearchIndex()
	return driver, nil
}

// newUITestWorkspace creates an empty isolated workspace: a temp root with a
// data dir and a fresh config wired to it. External tool integrations (fzf,
// glow) are disabled so the driver stays fully headless.
func newUITestWorkspace() (*uitestWorkspace, error) {
	root, err := os.MkdirTemp("", "tt-uitest-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace temp dir: %w", err)
	}

	dataDir := filepath.Join(root, "data")
	runtime := config.EmptyRuntimeState(dataDir)
	if err := runtime.EnsureDataDir(); err != nil {
		_ = os.RemoveAll(root)
		return nil, fmt.Errorf("failed to create workspace data dir: %w", err)
	}

	// Persistence is the decomposed domain files, whose location derives from the
	// workspace root (the config path's parent). Keep the config inside the data
	// dir so the workspace root equals DataDir — the invariant a real install
	// holds and NewCopiedWorkspaceDriver relies on when it reroots the copy onto
	// its own root. A config path in `root` (a level above DataDir) would make the
	// domain files and derived project paths resolve to the wrong subtree after a
	// copy+reload.
	configPath := filepath.Join(dataDir, ".tt.json")
	return &uitestWorkspace{
		Root:       root,
		DataDir:    dataDir,
		ConfigPath: configPath,
		Runtime:    runtime,
		Manager:    config.NewManager(configPath),
	}, nil
}

// addProject creates a project rooted under the workspace data dir, lets the
// caller populate it, rebuilds its task tree, registers it in the config, and
// writes its tasks.md through storage. The persisted project is returned.
func (w *uitestWorkspace) addProject(name string, build func(*models.Project)) (*models.Project, error) {
	project, err := models.NewProject(uuid.New().String(), name, "")
	if err != nil {
		return nil, err
	}
	project.Path = w.Runtime.ProjectPath(project.Slug)
	if build != nil {
		build(project)
	}
	project.BuildTaskTree()

	catalog := cloneWorkspaceCatalog(w.Runtime.Catalog())
	if err := catalog.UpsertProject(project); err != nil {
		return nil, fmt.Errorf("failed to catalog project %q: %w", name, err)
	}
	w.Runtime.SetCatalogInMemory(catalog)
	if err := storage.SaveProject(project); err != nil {
		return nil, fmt.Errorf("failed to write project %q: %w", name, err)
	}
	return project, nil
}

// launch persists the catalog and planning domains and builds a driver over the existing
// AppModel. NewAppModel reloads each active project's tasks from disk, so the
// driver observes exactly what storage round-tripped.
func (w *uitestWorkspace) launch(width, height int) (*UITestDriver, error) {
	if err := w.persistSyncedDomains(); err != nil {
		return nil, fmt.Errorf("failed to write workspace domains: %w", err)
	}
	if err := w.adoptPortableLenses(); err != nil {
		return nil, err
	}
	// Reload the synced files so RuntimeState holds the revisions just written;
	// bootstrap commits must compare against those baselines. Preserve the
	// intentionally in-memory machine fixture. A newly minted fixture identity
	// gets a canonical default machine file only to prevent legacy-path import;
	// fixture-specific machine values remain in memory until the app commits one.
	machine := cloneMachineState(w.Runtime.Machine())
	if err := w.Runtime.Reload(); err != nil {
		return nil, fmt.Errorf("failed to reload workspace domains: %w", err)
	}
	// Adoption is the one thing that wrote machine state before the app ran, so
	// its two fields are carried across the preserved fixture values rather than
	// dropped. Without the marker a lens write would publish nowhere portable.
	machine.LensHomeAdopted = w.Runtime.Machine().LensHomeAdopted
	machine.NamedLenses = w.Runtime.Machine().NamedLenses.Clone()
	w.Runtime.SetMachineInMemory(machine)
	if err := w.persistNoteSeeds(); err != nil {
		return nil, err
	}
	app, err := NewAppModel(w.Runtime, w.Manager)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize app model: %w", err)
	}
	// The Outliner is the sole root and always boots into a loaded global All.
	// Dispatch the startup message through the driver and drain it so the
	// cross-project cache is populated before the scenario observes the screen —
	// mirroring the real app rather than a cold, unrendered root.
	driver := NewUITestDriver(app, width, height)
	driver.relaunch = w.relaunchApp
	driver.moveToFreshMachine = w.switchMachineHome
	if app.startupMessage != nil {
		driver.dispatch(app.startupMessage)
		driver.drain()
	}
	driver.bootNoteSearchIndex()
	if w.currentProjectID != "" {
		for _, project := range app.crossOutliner.loadedProjects {
			if project != nil && project.ID == w.currentProjectID {
				app.currentProject = project
				break
			}
		}
		if app.currentProject == nil {
			app.currentProject = app.projectByID(w.currentProjectID)
		}
	}
	return driver, nil
}

// relaunchApp rebuilds the app the way relaunching `tt` against this workspace
// does: the runtime is re-read from disk, so the in-memory machine fixture
// launch preserved is deliberately not carried over. Whatever the session
// committed — the theme, the saved lenses, the active-lens pointer — is exactly
// what the new app sees, and whatever it only held in memory is gone.
func (w *uitestWorkspace) relaunchApp() (*AppModel, error) {
	if err := w.adoptPortableLenses(); err != nil {
		return nil, err
	}
	runtime, err := config.LoadRuntimeState(w.DataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to reload workspace for restart: %w", err)
	}
	w.Runtime = runtime
	app, err := NewAppModel(runtime, w.Manager)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize app model after restart: %w", err)
	}
	return app, nil
}

// adoptPortableLenses runs the one-time-per-machine merge a real launch runs
// between loading the domains and handing them to the TUI. It is a no-op once
// this machine's marker is set, and it is what publishes `.tt/lenses.json`, so
// a fixture workspace keeps its lens definitions where a synced workspace keeps
// them rather than where an unadopted machine would.
func (w *uitestWorkspace) adoptPortableLenses() error {
	if err := w.Manager.AdoptPortableLenses(); err != nil {
		return fmt.Errorf("failed to adopt portable lenses: %w", err)
	}
	return nil
}

func (w *uitestWorkspace) persistNoteSeeds() error {
	if len(w.noteSeeds) == 0 {
		return nil
	}
	artifacts := make([]storage.ArtifactChange, 0, len(w.noteSeeds))
	for _, seed := range w.noteSeeds {
		if seed.note == nil {
			return fmt.Errorf("fixture note seed is nil")
		}
		path, err := storage.NoteFilePathIn(seed.dir, seed.note.Filename)
		if err != nil {
			return fmt.Errorf("resolve fixture note %q: %w", seed.note.Title, err)
		}
		content, err := storage.RenderNoteContent(seed.note)
		if err != nil {
			return fmt.Errorf("render fixture note %q: %w", seed.note.Title, err)
		}
		artifacts = append(artifacts, storage.ArtifactChange{
			Path: path,
			Data: []byte(content),
		})
	}
	committer := storage.NewCommitService(
		filepath.Join(w.DataDir, ".tt", "transactions"),
		[]string{w.DataDir},
	)
	if _, err := committer.CommitArtifacts(artifacts); err != nil {
		return fmt.Errorf("commit fixture notes: %w", err)
	}
	return nil
}

func (w *uitestWorkspace) persistSyncedDomains() error {
	store := config.NewDomainFileStore(w.DataDir)
	catalog := cloneWorkspaceCatalog(w.Runtime.Catalog())
	// A copied workspace already carries the fresh identity assigned by
	// forkCopiedWorkspaceIdentity. Built-in fixtures start blank and mint once
	// here before the remaining domains save.
	freshIdentity := catalog.WorkspaceID == ""
	if freshIdentity {
		catalog.WorkspaceID = store.EnsureWorkspaceID()
	}
	if err := store.SaveCatalog(catalog); err != nil {
		return err
	}
	w.Runtime.SetCatalogInMemory(catalog)
	if err := store.SavePlanning(w.Runtime.Planning()); err != nil {
		return err
	}
	if freshIdentity {
		return store.SaveMachine(config.NewMachineState())
	}
	return nil
}

func (w *uitestWorkspace) setCurrentProject(projectID string) {
	w.currentProjectID = projectID
}

// NewFixtureDriver builds an isolated workspace preloaded with the named
// built-in fixture and returns a driver over it, the owning workspace (call
// Cleanup when done), and any error. On error the workspace is already cleaned
// up and a nil workspace is returned.
func NewFixtureDriver(name string, width, height int) (*UITestDriver, *uitestWorkspace, error) {
	build, ok := builtinFixtures[name]
	if !ok {
		return nil, nil, fmt.Errorf("unknown fixture %q (available: %s)", name, strings.Join(BuiltinFixtureNames(), ", "))
	}

	ws, err := newUITestWorkspace()
	if err != nil {
		return nil, nil, err
	}
	if build == nil {
		buildFreshFixture(ws)
	} else if err = build(ws); err != nil {
		_ = ws.Cleanup()
		return nil, nil, err
	}
	driver, err := ws.launch(width, height)
	if err != nil {
		_ = ws.Cleanup()
		return nil, nil, err
	}
	return driver, ws, nil
}

// NewCopiedWorkspaceDriver loads a real user config, copies its data dir into a
// fresh temp workspace, rewrites DataDir and project paths onto the copy, and
// launches a driver from the copy. The source config and task files are read
// only — never mutated. Returns the driver, the owning temp workspace (call
// Cleanup when done), and any error.
func NewCopiedWorkspaceDriver(sourceConfigPath string, width, height int) (*UITestDriver, *uitestWorkspace, error) {
	if strings.TrimSpace(sourceConfigPath) == "" {
		return nil, nil, fmt.Errorf("source config path is required")
	}
	// Persistence is the decomposed domain files under the workspace root, so the
	// source "config" is the whole workspace directory (the .tt/ config domain
	// files plus the project data), not a single monolithic file. Copy that durable
	// tree into a fresh temporary workspace first, then load from the copy.
	srcRoot := config.WorkspaceRootFromConfigPath(sourceConfigPath)

	root, err := os.MkdirTemp("", "tt-uitest-copy-*")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create copied workspace temp dir: %w", err)
	}
	ws := &uitestWorkspace{Root: root, ConfigPath: filepath.Join(root, filepath.Base(sourceConfigPath))}

	if err = os.CopyFS(root, os.DirFS(srcRoot)); err != nil {
		_ = ws.Cleanup()
		return nil, nil, fmt.Errorf("failed to copy source workspace: %w", err)
	}
	if err = forkCopiedWorkspaceIdentity(root); err != nil {
		_ = ws.Cleanup()
		return nil, nil, fmt.Errorf("failed to isolate copied workspace identity: %w", err)
	}

	ws.Manager = config.NewManager(ws.ConfigPath)
	if _, err = ws.Manager.LoadDomains(); err != nil {
		_ = ws.Cleanup()
		return nil, nil, fmt.Errorf("failed to load copied config: %w", err)
	}
	runtime, err := ws.Manager.LoadRuntimeState()
	if err != nil {
		_ = ws.Cleanup()
		return nil, nil, fmt.Errorf("failed to load copied runtime domains: %w", err)
	}
	ws.DataDir = root
	ws.Runtime = runtime

	driver, err := ws.launch(width, height)
	if err != nil {
		_ = ws.Cleanup()
		return nil, nil, err
	}
	return driver, ws, nil
}

// forkCopiedWorkspaceIdentity rewrites every copied synced-domain envelope with
// one fresh workspace ID, then creates canonical default machine state for that
// new identity. The source store is read before any copied artifact is removed;
// failures affect only the disposable temp copy. Removing stale .bak sidecars is
// essential because restoring one would otherwise resurrect the source ID.
func forkCopiedWorkspaceIdentity(dataDir string) error {
	source := config.NewDomainFileStore(dataDir)
	catalog, err := source.LoadCatalog()
	if err != nil {
		return err
	}
	planning, err := source.LoadPlanning()
	if err != nil {
		return err
	}

	paths := []string{
		config.WorkspaceCatalogPath(dataDir),
		config.PlanningStatePath(dataDir),
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove copied domain %s: %w", filepath.Base(path), err)
		}
		if err := os.Remove(path + ".bak"); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove copied domain backup %s: %w", filepath.Base(path), err)
		}
	}

	workspaceID := config.NewWorkspaceID()
	catalog.WorkspaceID = workspaceID
	destination := config.NewDomainFileStore(dataDir)
	if err := destination.BindWorkspaceID(workspaceID); err != nil {
		return err
	}
	if err := destination.SaveCatalog(catalog); err != nil {
		return err
	}
	if err := destination.SavePlanning(planning); err != nil {
		return err
	}
	return destination.SaveMachine(config.NewMachineState())
}

// --- Built-in fixture builders ---

// buildEmptyFixture leaves the workspace with no user projects, exercising the
// zero-task dashboard capture hint.
func buildEmptyFixture(_ *uitestWorkspace) error {
	return nil
}

// buildFreshFixture removes even the harness-seeded sandbox so NewAppModel must
// bootstrap a genuine blank install, then starts on the configured default view.
func buildFreshFixture(w *uitestWorkspace) {
	w.Runtime.SetCatalogInMemory(config.NewWorkspaceCatalog())
	w.startAtDefault = true
}

// buildRichFixture creates a project with a full task hierarchy spanning every
// retained workflow states, priorities, hierarchy, and relationships.
func buildRichFixture(w *uitestWorkspace) error {
	project, err := w.addProject("Rich Project", func(p *models.Project) {
		epic := newFixtureTask(p, "Launch platform")
		epic.Description = "Ship the first public release."
		epic.Priority = 1

		story := newFixtureTask(p, "User authentication")
		story.ParentID = &epic.ID
		story.Priority = 2

		login := newFixtureTask(p, "Build login form")
		login.ParentID = &story.ID
		login.ApplyWorkflowState(models.StateDoing)

		validate := newFixtureTask(p, "Add password validation")
		validate.ParentID = &story.ID
		validate.Tags = []string{"backend", "security"}
		validate.Context = "platform"

		mockup := newFixtureTask(p, "Design login mockup")
		mockup.ParentID = &story.ID
		markDone(mockup)

		bug := newFixtureTask(p, "Fix redirect loop after logout")
		bug.Priority = 1
		bug.Tags = []string{"urgent"}
		bug.ApplyWorkflowState(models.StateWaiting)

		idea := newFixtureTask(p, "Explore social login")
		idea.Description = "Google / GitHub OAuth."
		idea.ApplyWorkflowState(models.StateSomeday)

		inbox := newFixtureTask(p, "Triage support inbox")
		inbox.ApplyWorkflowState(models.StateInbox)
	})
	if err != nil {
		return err
	}
	globalNote := &models.Note{
		Title:      "Workspace launch brief",
		CreatedAt:  time.Date(2025, time.January, 2, 9, 0, 0, 0, time.UTC),
		ModifiedAt: time.Date(2025, time.January, 2, 10, 0, 0, 0, time.UTC),
		Filename:   "2025-01-02-090000-workspace-launch-brief.md",
		Body:       "Shared launch context for every active project.\n",
	}
	projectNote := &models.Note{
		Title:      "Rich Project delivery plan",
		CreatedAt:  time.Date(2025, time.January, 3, 9, 0, 0, 0, time.UTC),
		ModifiedAt: time.Date(2025, time.January, 3, 10, 0, 0, 0, time.UTC),
		Filename:   "2025-01-03-090000-rich-project-delivery-plan.md",
		Body:       richProjectNoteBody(),
	}
	w.noteSeeds = append(w.noteSeeds,
		fixtureNoteSeed{dir: storage.GlobalNotesDir(w.DataDir), note: globalNote},
		fixtureNoteSeed{dir: storage.NotesDir(project), note: projectNote},
	)
	w.setCurrentProject(project.ID)
	return nil
}

// richProjectNoteBody is the one note body long enough to page and to anchor
// against. Its first line is unchanged from the single-line body it grew out of,
// so every committed Notes search scenario still matches and counts exactly what
// it did before. Below it sit Markdown headings (which the pane renders rather
// than shows), forty numbered rows (so a page turn is observable), and one
// deliberately rare word — kestrel — far enough down that a body match can only
// be on screen because the preview anchored on it.
func richProjectNoteBody() string {
	lines := []string{
		"Track the authentication milestone and release readiness.",
		"",
		"## Delivery checkpoints",
		"",
	}
	for i := 1; i <= 40; i++ {
		lines = append(lines, fmt.Sprintf("- Checkpoint %02d confirms the staged rollout gate.", i))
	}
	lines = append(lines,
		"",
		"## Rollback rehearsal",
		"",
		"The rehearsal runs on the Friday before launch with the kestrel runbook as its fallback.",
		"",
	)
	return strings.Join(lines, "\n")
}

// buildBusyFixture creates a dense mix of urgent, scheduled, waiting,
// and inbox work for dashboard stress tests.
func buildBusyFixture(w *uitestWorkspace) error {
	project, err := w.addProject("Busy Project", func(p *models.Project) {
		overdue := newFixtureTask(p, "Overdue: submit quarterly report")
		overdue.Priority = 1
		overdue.ApplyWorkflowState(models.StateBacklog)
		setDue(overdue, -3*24*time.Hour)

		dueSoon := newFixtureTask(p, "Due soon: renew certificate")
		dueSoon.Priority = 2
		dueSoon.ApplyWorkflowState(models.StateBacklog)
		setDue(dueSoon, 12*time.Hour)

		urgentWaiting := newFixtureTask(p, "Waiting: production outage")
		urgentWaiting.Priority = 1
		urgentWaiting.ApplyWorkflowState(models.StateWaiting)

		waiting := newFixtureTask(p, "Waiting on vendor response")
		waiting.ApplyWorkflowState(models.StateWaiting)

		completed := newFixtureTask(p, "Completed: archive quarterly paperwork")
		markDone(completed)
		completedAt := time.Now().AddDate(0, 0, -45)
		completed.CompletedAt = &completedAt
		completed.UpdatedAt = completedAt

		canceled := newFixtureTask(p, "Canceled: retire obsolete certificate")
		canceled.ApplyWorkflowState(models.StateCanceled)
		canceledAt := time.Now().AddDate(0, 0, -120)
		canceled.CompletedAt = &canceledAt
		canceled.UpdatedAt = canceledAt
	})
	if err != nil {
		return err
	}
	_, err = w.addProject("Inbox Backlog", func(p *models.Project) {
		stale := time.Now().Add(-21 * 24 * time.Hour)
		for i := 0; i < 5; i++ {
			task := newFixtureTask(p, fmt.Sprintf("Untriaged inbox item %d", i+1))
			task.ApplyWorkflowState(models.StateInbox)
			task.CreatedAt = stale
			task.UpdatedAt = stale
		}
	})
	if err != nil {
		return err
	}
	w.setCurrentProject(project.ID)
	return nil
}

// buildWorkflowFixture creates a spread of tasks across workflow states so the
// state and planning operations have something to render.
func buildWorkflowFixture(w *uitestWorkspace) error {
	titles := []string{
		"Sketch intake flow",
		"Design settings screen",
		"Implement export pipeline",
		"Review release notes",
	}
	project, err := w.addProject("Workflow Project", func(p *models.Project) {
		for i, title := range titles {
			t := newFixtureTask(p, title)
			t.Priority = (i % 3) + 1
			t.ApplyWorkflowState(models.StateBacklog)
			if i == len(titles)-1 {
				markDone(t)
			}
		}
	})
	if err != nil {
		return err
	}
	w.setCurrentProject(project.ID)
	return nil
}

// buildNarrowFixture creates tasks with long titles and tags to stress layout
// when the driver is run at a narrow terminal width.
func buildNarrowFixture(w *uitestWorkspace) error {
	project, err := w.addProject("Narrow Project", func(p *models.Project) {
		long := newFixtureTask(p,
			"Investigate intermittent timeout when syncing very large project archives over slow connections")

		long.Priority = 1
		long.Tags = []string{"infrastructure", "reliability", "sync"}
		long.Context = "distributed-systems"
		long.ApplyWorkflowState(models.StateBacklog)

		another := newFixtureTask(p,
			"Truncated column headers overlap when the terminal is resized below the minimum comfortable width")

		another.ApplyWorkflowState(models.StateDoing)
	})
	if err != nil {
		return err
	}
	w.setCurrentProject(project.ID)
	return nil
}

// buildReleaseFixture creates a large canonical Outliner catalog with Unicode
// content. The release-hardening fixture exercises wide and minimum layouts,
// viewport bounds, and repeated resizes
// against substantially more rows than the everyday review fixtures.
func buildReleaseFixture(w *uitestWorkspace) error {
	states := []models.WorkflowState{
		models.StateInbox,
		models.StateBacklog,
		models.StateBacklog,
		models.StateDoing,
		models.StateWaiting,
		models.StatePaused,
		models.StateSomeday,
	}
	project, err := w.addProject("Release Ω Project", func(p *models.Project) {
		unicodeTask := newFixtureTask(p, "国際化 🚀 café résumé release check")
		unicodeTask.Description = "Unicode display-width and visible-search acceptance row."
		unicodeTask.Tags = []string{"i18n", "release"}
		unicodeTask.ApplyWorkflowState(models.StateDoing)

		for i := 0; i < 240; i++ {
			task := newFixtureTask(
				p,

				fmt.Sprintf("Release acceptance item %03d with bounded viewport content", i+1))

			task.Priority = (i % 3) + 1
			task.Tags = []string{"release", fmt.Sprintf("batch-%d", i%8)}
			task.ApplyWorkflowState(states[i%len(states)])
		}
	})
	if err != nil {
		return err
	}
	w.setCurrentProject(project.ID)
	return nil
}

// buildMultiProjectFixture spreads one backlog task across four distinctly named
// projects so a scenario that searches or moves between projects has more than
// one plausible destination to choose from.
func buildMultiProjectFixture(w *uitestWorkspace) error {
	projects := []struct {
		name string
		task string
	}{
		{"Client Alpha", "Alpha migration kickoff"},
		{"Client Beta", "Beta rollout checklist"},
		{"Home Errands", "Renew passport"},
		{"Reading List", "Finish the distributed systems book"},
	}
	var first *models.Project
	for _, spec := range projects {
		project, err := w.addProject(spec.name, func(p *models.Project) {
			task := newFixtureTask(p, spec.task)
			task.ApplyWorkflowState(models.StateBacklog)
		})
		if err != nil {
			return err
		}
		if first == nil {
			first = project
		}
	}
	w.setCurrentProject(first.ID)
	return nil
}

// buildInboxTriageFixture is the only fixture whose Inbox spans more than one
// project group, which is the single row shape where the Inbox cursor advance
// is observable: the last task of a non-final group has to hand the cursor to
// the first task of the next one. The `busy` fixture keeps its whole Inbox in
// one project and every other multi-project fixture carries backlog work only,
// so neither can express the crossing. Its second group also holds the nested
// Inbox parent whose surviving child is promoted into its place when the parent
// is triaged away.
//
// It is deliberately its own fixture rather than an extension of
// `multi-project`, whose four projects are read by twenty-one committed
// scenarios that would all see a changed row population.
func buildInboxTriageFixture(w *uitestWorkspace) error {
	// Alphabetical project names fix the group order, since project groups sort
	// pinned-first and then by name and nothing here is pinned.
	alpha, err := w.addProject(inboxTriageAlphaName, func(p *models.Project) {
		// The first group's two tasks are the only rows Peek can tell apart by
		// content alone, so each carries a description no other row repeats. That
		// is how the undo scenario proves which row the cursor is on.
		lead := newFixtureTask(p, inboxTriageAlphaLeadTitle)
		lead.ApplyWorkflowState(models.StateInbox)
		lead.Description = inboxTriageAlphaLeadDetail

		final := newFixtureTask(p, inboxTriageAlphaFinalTitle)
		final.ApplyWorkflowState(models.StateInbox)
		final.Description = inboxTriageAlphaFinalDetail
	})
	if err != nil {
		return err
	}
	if _, err = w.addProject(inboxTriageBetaName, func(p *models.Project) {
		parent := newFixtureTask(p, inboxTriageBetaParentTitle)
		parent.ApplyWorkflowState(models.StateInbox)

		child := newFixtureTask(p, inboxTriageBetaChildTitle)
		child.ApplyWorkflowState(models.StateInbox)
		child.ParentID = &parent.ID

		trailing := newFixtureTask(p, inboxTriageBetaTrailingTitle)
		trailing.ApplyWorkflowState(models.StateInbox)
	}); err != nil {
		return err
	}
	w.setCurrentProject(alpha.ID)
	return nil
}

// buildParkedFixture tags one project parked and leaves the other in the base
// task workspace, which is the only fixture shape where the base task workspace
// is a strict subset of the workspace's projects. Every other fixture leaves all
// of its projects untagged, so parked behavior would be indistinguishable from
// an empty catalog there.
func buildParkedFixture(w *uitestWorkspace) error {
	active, err := w.addProject(parkedFixtureWorkspaceName, func(p *models.Project) {
		checklist := newFixtureTask(p, "Ship the delivery checklist")
		checklist.Priority = 1
		checklist.ApplyWorkflowState(models.StateBacklog)

		window := newFixtureTask(p, "Confirm the release window")
		window.ApplyWorkflowState(models.StateBacklog)
	})
	if err != nil {
		return err
	}
	if _, err = w.addProject(parkedFixtureParkedName, func(p *models.Project) {
		p.Tags = []string{models.ParkedProjectTag}

		migration := newFixtureTask(p, "Revisit the paused migration")
		migration.ApplyWorkflowState(models.StateBacklog)

		vendor := newFixtureTask(p, "Reopen the vendor conversation")
		vendor.ApplyWorkflowState(models.StateInbox)
	}); err != nil {
		return err
	}
	w.setCurrentProject(active.ID)
	return nil
}

// buildProjectMixFixture holds the five project shapes the Select Project
// open-task filter distinguishes: active with an open task, active with none,
// active whose tasks are all finished, parked with an open task, and archived
// with none. It is the only fixture carrying an archived project and the only
// one where "holds open tasks" and "reaches the base task workspace" disagree in
// both directions; the finished project is what the two narrowings separate,
// since it holds tasks and no open work, so `with open tasks` hides it and
// `without open tasks` lists it beside the two known-empty projects.
func buildProjectMixFixture(w *uitestWorkspace) error {
	active, err := w.addProject(projectMixActiveName, func(p *models.Project) {
		p.Description = projectMixActiveDescription
		p.Tags = []string{"commerce"}
		// The one pinned project is also the alphabetically first one, so
		// pinned-first ordering leaves the active listing in exactly the order it
		// had before the pin — and Ctrl+P still reaches a non-empty pinned Hub.
		p.Pinned = true

		rollout := newFixtureTask(p, "Mix rollout plan")
		rollout.ApplyWorkflowState(models.StateBacklog)
	})
	if err != nil {
		return err
	}
	if _, err = w.addProject(projectMixEmptyName, func(p *models.Project) {
		p.Description = projectMixEmptyDescription
		p.Tags = []string{"charter"}
	}); err != nil {
		return err
	}
	if _, err = w.addProject(projectMixFinishedName, func(p *models.Project) {
		p.Description = projectMixFinishedDescription
		p.Tags = []string{"housekeeping"}

		markDone(newFixtureTask(p, "Mix cleanup sweep"))
		markDone(newFixtureTask(p, "Mix cleanup follow-up"))
	}); err != nil {
		return err
	}
	if _, err = w.addProject(projectMixParkedName, func(p *models.Project) {
		p.Description = projectMixParkedDescription
		p.Tags = []string{models.ParkedProjectTag, "vendor"}

		migration := newFixtureTask(p, "Mix migration audit")
		migration.ApplyWorkflowState(models.StateBacklog)
	}); err != nil {
		return err
	}
	// Archiving happens inside the mutator because catalog.UpsertProject runs
	// after it, so the flag is what the catalog records.
	if _, err = w.addProject(projectMixArchivedName, func(p *models.Project) {
		p.Description = projectMixArchivedDescription
		p.Tags = []string{"kiosk"}
		p.Archive()
	}); err != nil {
		return err
	}
	w.setCurrentProject(active.ID)
	return nil
}

// buildSignalsFixture carries the time signals no other fixture holds: a deferral
// that has already lapsed beside a whole-day and an hour-valued one still
// pending, overdue work concentrated in one project, and a durable Today
// membership. Its project name is
// deliberately long so a project header's pressure suffix has to degrade at
// narrow widths.
func buildSignalsFixture(w *uitestWorkspace) error {
	var triage *models.Task
	project, err := w.addProject(signalsFixtureProjectName, func(p *models.Project) {
		audit := newFixtureTask(p, "Alpha overdue audit")
		audit.ApplyWorkflowState(models.StateBacklog)
		setDue(audit, -3*24*time.Hour)

		rollout := newFixtureTask(p, "Alpha overdue rollout")
		rollout.ApplyWorkflowState(models.StateBacklog)
		setDue(rollout, -24*time.Hour)

		woke := newFixtureTask(p, "Alpha back from a break")
		woke.ApplyWorkflowState(models.StateBacklog)
		lapsed := time.Now().AddDate(0, 0, -1)
		woke.DeferredUntil = &lapsed
		woke.DeferCount = 3
		woke.Context = "difficult-review"
		woke.Checklist = []models.ChecklistItem{{Text: "Review the concern"}}
		woke.Comments = []*models.Comment{{Text: "Deferred before", CreatedAt: time.Now()}}
		woke.Attachments = []models.Attachment{{Filename: "brief.md", AddedAt: time.Now()}}

		pending := newFixtureTask(p, "Alpha still hidden")
		pending.ApplyWorkflowState(models.StateBacklog)
		deferred := util.StartOfDay(time.Now()).AddDate(0, 0, 5)
		pending.DeferredUntil = &deferred

		// A midnight deferral renders bare, so the clock ladder needs one row
		// whose wake time is a fixed hour rather than an offset from now.
		morning := newFixtureTask(p, "Alpha waking at nine")
		morning.ApplyWorkflowState(models.StateBacklog)
		tomorrow := time.Now().AddDate(0, 0, 1)
		wake := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 9, 0, 0, 0, time.Local)
		morning.DeferredUntil = &wake

		triage = newFixtureTask(p, "Alpha overdue intake")
		triage.ApplyWorkflowState(models.StateInbox)
		setDue(triage, -2*24*time.Hour)

		// The two extra hours keep the day bucket at 12 after the clock
		// round-trips through the schema-8 writer's second-precision timestamp.
		blocked := newFixtureTask(p, "Alpha waiting on vendor")
		blocked.ApplyWorkflowState(models.StateWaiting)
		blocked.StateChangedAt = time.Now().Add(-12*24*time.Hour - 2*time.Hour)
	})
	if err != nil {
		return err
	}
	if _, err = w.addProject(signalsFixtureOtherName, func(p *models.Project) {
		steady := newFixtureTask(p, "Beta steady work")
		steady.ApplyWorkflowState(models.StateBacklog)
	}); err != nil {
		return err
	}
	w.Runtime.Planning().AddToToday(triage.ID, project.ID)
	w.setCurrentProject(project.ID)
	return nil
}

// buildStaleFixture isolates old touch times so the Stale lane and fs facet can
// be exercised without shifting cursor-sensitive scenarios on other fixtures.
func buildStaleFixture(w *uitestWorkspace) error {
	project, err := w.addProject("Dormant Delivery", func(p *models.Project) {
		old := time.Now().AddDate(0, 0, -(models.StaleTaskThresholdDays + 10))
		for _, title := range []string{
			"Revisit dormant launch checklist",
			"Renew untouched vendor briefing",
		} {
			task := newFixtureTask(p, title)
			task.ApplyWorkflowState(models.StateBacklog)
			task.CreatedAt = old
			task.UpdatedAt = old
			task.StateChangedAt = old
		}

		fresh := newFixtureTask(p, "Draft current launch note")
		fresh.ApplyWorkflowState(models.StateBacklog)
	})
	if err != nil {
		return err
	}
	w.setCurrentProject(project.ID)
	return nil
}

// buildChecklistFixture is the one fixture whose tasks carry checklist items, so
// the inline checklist rows and their fold glyph can be exercised without
// shifting any other fixture's projection. It holds three shapes the fold ladder
// distinguishes: a checklist with no children, a checklist beneath a task that
// also has a child, and a task with neither.
func buildChecklistFixture(w *uitestWorkspace) error {
	project, err := w.addProject(checklistFixtureProjectName, func(p *models.Project) {
		runbook := newFixtureTask(p, checklistFixtureFoldTitle)
		runbook.ApplyWorkflowState(models.StateBacklog)
		runbook.Checklist = []models.ChecklistItem{
			{Text: checklistFixtureFirstItem, Done: true},
			{Text: checklistFixtureSecondItem},
			{Text: checklistFixtureThirdItem},
		}

		rollout := newFixtureTask(p, checklistFixtureParentTitle)
		rollout.ApplyWorkflowState(models.StateBacklog)
		rollout.Checklist = []models.ChecklistItem{
			{Text: "Book the maintenance window"},
			{Text: "Notify the support rota", Done: true},
		}
		soak := newFixtureTask(p, "Confirm the staging soak")
		soak.ApplyWorkflowState(models.StateBacklog)
		soak.ParentID = &rollout.ID

		plain := newFixtureTask(p, checklistFixturePlainTitle)
		plain.ApplyWorkflowState(models.StateBacklog)
	})
	if err != nil {
		return err
	}
	w.setCurrentProject(project.ID)
	return nil
}

// newFixtureTask creates a task with the next short ID, appends it to the
// project, and returns it for further customization before the tree is built.
func newFixtureTask(p *models.Project, title string) *models.Task {
	task, err := models.NewGenericTask(uuid.New().String(), p.NextShortID, title)
	if err != nil {
		// Fixture titles are authored in this file and validated by tests, so a
		// failure here is a programming error, not runtime input.
		panic(fmt.Sprintf("newFixtureTask(%q): %v", title, err))
	}
	p.Tasks = append(p.Tasks, task)
	p.NextShortID++
	return task
}

// markDone completes a task via the canonical done state and stamps CompletedAt
// like the lifecycle transition would.
func markDone(t *models.Task) {
	now := time.Now()
	t.ApplyWorkflowState(models.StateDone)
	t.CompletedAt = &now
}

// setDue sets a due date relative to now, on the whole minute the grammar
// would have produced. util.ParseDueDate builds a timed due with zero seconds
// and treats a midnight clock as the date-only signal, so a raw time.Now()
// offset is a value no user can enter: its seconds render away and the
// schema-13 codec refuses the "00:00" that is left, which broke every fixture
// build during the one minute a day the offset landed on midnight.
func setDue(t *models.Task, offset time.Duration) {
	due := fixtureDueInstant(time.Now(), offset)
	t.DueDate = &due
}

// fixtureDueInstant is setDue's clock arithmetic, split out so a test can drive
// it from a base instant the wall clock only visits for one minute a day.
func fixtureDueInstant(base time.Time, offset time.Duration) time.Time {
	return base.Add(offset).Truncate(time.Minute)
}
