package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/coordinator"
)

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
	driver.bootLaunchCommands()
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

	catalog := coordinator.CloneWorkspaceCatalog(w.Runtime.Catalog())
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
	driver.bootLaunchCommands()
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
	catalog := coordinator.CloneWorkspaceCatalog(w.Runtime.Catalog())
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
