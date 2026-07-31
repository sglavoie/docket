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
	FixtureEmpty    = "empty"
	FixtureFresh    = "fresh"
	FixtureRich     = "rich"
	FixtureBusy     = "busy"
	FixtureWorkflow = "workflow"
	FixtureNarrow   = "narrow"
	FixtureContexts = "contexts"
	FixtureRelease  = "release"
)

// builtinFixtures maps each fixture name to the builder that populates its
// workspace. The workspace is already created (empty temp data dir + config)
// when the builder runs; the builder only adds projects and sets config state.
var builtinFixtures = map[string]func(*uitestWorkspace) error{
	FixtureEmpty:    buildEmptyFixture,
	FixtureFresh:    buildFreshFixture,
	FixtureRich:     buildRichFixture,
	FixtureBusy:     buildBusyFixture,
	FixtureWorkflow: buildWorkflowFixture,
	FixtureNarrow:   buildNarrowFixture,
	FixtureContexts: buildContextsFixture,
	FixtureRelease:  buildReleaseFixture,
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
	return os.RemoveAll(w.Root)
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
	// Reload the synced files so RuntimeState holds the revisions just written;
	// bootstrap commits must compare against those baselines. Preserve the
	// intentionally in-memory machine fixture. A newly minted fixture identity
	// gets a canonical default machine file only to prevent legacy-path import;
	// fixture-specific machine values remain in memory until the app commits one.
	machine := cloneMachineState(w.Runtime.Machine())
	if err := w.Runtime.Reload(); err != nil {
		return nil, fmt.Errorf("failed to reload workspace domains: %w", err)
	}
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
	if app.startupMessage != nil {
		driver.dispatch(app.startupMessage)
		driver.drain()
	}
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
	if err = build(ws); err != nil {
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
func buildFreshFixture(w *uitestWorkspace) error {
	w.Runtime.SetCatalogInMemory(config.NewWorkspaceCatalog())
	w.startAtDefault = true
	return nil
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
		login.ApplyWorkflowState(models.StateInProgress)

		validate := newFixtureTask(p, "Add password validation")
		validate.ParentID = &story.ID
		validate.Tags = []string{"backend", "security"}

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
		Body:       "Track the authentication milestone and release readiness.\n",
	}
	w.noteSeeds = append(w.noteSeeds,
		fixtureNoteSeed{dir: storage.GlobalNotesDir(w.DataDir), note: globalNote},
		fixtureNoteSeed{dir: storage.NotesDir(project), note: projectNote},
	)
	w.setCurrentProject(project.ID)
	return nil
}

// buildBusyFixture creates a dense mix of urgent, scheduled, blocked, waiting,
// and inbox work for dashboard stress tests.
func buildBusyFixture(w *uitestWorkspace) error {
	project, err := w.addProject("Busy Project", func(p *models.Project) {
		overdue := newFixtureTask(p, "Overdue: submit quarterly report")
		overdue.Priority = 1
		overdue.ApplyWorkflowState(models.StateNext)
		setDue(overdue, -3*24*time.Hour)

		dueSoon := newFixtureTask(p, "Due soon: renew certificate")
		dueSoon.Priority = 2
		dueSoon.ApplyWorkflowState(models.StateNext)
		setDue(dueSoon, 12*time.Hour)

		blocked := newFixtureTask(p, "Blocked: production outage")
		blocked.Priority = 1
		blocked.ApplyWorkflowState(models.StateWaiting)

		waiting := newFixtureTask(p, "Waiting on vendor response")
		waiting.ApplyWorkflowState(models.StateWaiting)
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
			t.ApplyWorkflowState(models.StateNext)
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
		long.ApplyWorkflowState(models.StateNext)

		another := newFixtureTask(p,
			"Truncated column headers overlap when the terminal is resized below the minimum comfortable width")

		another.ApplyWorkflowState(models.StateInProgress)
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
		models.StateReady,
		models.StateNext,
		models.StateInProgress,
		models.StateWaiting,
		models.StatePaused,
		models.StateSomeday,
	}
	project, err := w.addProject("Release Ω Project", func(p *models.Project) {
		unicodeTask := newFixtureTask(p, "国際化 🚀 café résumé release check")
		unicodeTask.Description = "Unicode display-width and visible-search acceptance row."
		unicodeTask.Tags = []string{"i18n", "release"}
		unicodeTask.ApplyWorkflowState(models.StateInProgress)

		for i := 0; i < 240; i++ {
			task := newFixtureTask(
				p,

				fmt.Sprintf("Release acceptance item %03d with bounded viewport content", i+1))

			task.Priority = (i % 5) + 1
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

// buildContextsFixture spreads projects across several context labels so Context
// Hub scenarios can prove a scope actually narrows. Every other fixture takes
// the default "personal" label for all of its projects, which leaves a single
// selectable regular context and nothing to narrow away.
func buildContextsFixture(w *uitestWorkspace) error {
	projects := []struct {
		name    string
		context string
		task    string
	}{
		{"Client Alpha", "work", "Alpha migration kickoff"},
		{"Client Beta", "work", "Beta rollout checklist"},
		{"Home Errands", "personal", "Renew passport"},
		{"Reading List", "research", "Finish the distributed systems book"},
	}
	var first *models.Project
	for _, spec := range projects {
		project, err := w.addProject(spec.name, func(p *models.Project) {
			p.Context = spec.context
			task := newFixtureTask(p, spec.task)
			task.ApplyWorkflowState(models.StateNext)
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

// setDue sets a due date relative to now.
func setDue(t *models.Task, offset time.Duration) {
	due := time.Now().Add(offset)
	t.DueDate = &due
}
