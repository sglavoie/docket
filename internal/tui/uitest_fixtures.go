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
//   - Built-in fixtures (empty/rich/alerts/workflow/narrow) construct a fresh
//     isolated temp data dir, write projects through storage, and launch the
//     driver from that copied config.
//   - Copied-config workspaces load a real user config, copy its data dir into
//     temp storage, rewrite DataDir + project paths to the copy, and launch the
//     driver from there — never mutating the source config or task files.
//
// Fixture builders live in internal/tui (production code) rather than
// internal/testutil so the hidden `tt tui-test` CLI (session 5+) can import them
// without pulling in test-only helpers.

// Built-in fixture names. These are the values accepted by NewFixtureDriver and
// surfaced by `tt tui-test fixtures`.
const (
	FixtureEmpty    = "empty"
	FixtureRich     = "rich"
	FixtureAlerts   = "alerts"
	FixtureWorkflow = "workflow"
	FixtureNarrow   = "narrow"
)

// builtinFixtures maps each fixture name to the builder that populates its
// workspace. The workspace is already created (empty temp data dir + config)
// when the builder runs; the builder only adds projects and sets config state.
var builtinFixtures = map[string]func(*uitestWorkspace) error{
	FixtureEmpty:    buildEmptyFixture,
	FixtureRich:     buildRichFixture,
	FixtureAlerts:   buildAlertsFixture,
	FixtureWorkflow: buildWorkflowFixture,
	FixtureNarrow:   buildNarrowFixture,
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
	Config     *config.Config
	Manager    *config.Manager
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
	cfg := config.NewConfig()
	cfg.DataDir = dataDir
	cfg.FzfEnabled = false
	cfg.GlowEnabled = false
	cfg.Projects = []*models.Project{}
	if err := cfg.EnsureDataDir(); err != nil {
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
		Config:     cfg,
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
	project.Path = w.Config.GetProjectPath(project.Slug)
	project.EnsureDefaultSections()
	if build != nil {
		build(project)
	}
	project.BuildTaskTree()

	w.Config.Projects = append(w.Config.Projects, project)
	if err := storage.SaveProject(project); err != nil {
		return nil, fmt.Errorf("failed to write project %q: %w", name, err)
	}
	return project, nil
}

// launch persists the config and builds a driver over a freshly constructed
// AppModel. NewAppModel reloads each active project's tasks from disk, so the
// driver observes exactly what storage round-tripped.
func (w *uitestWorkspace) launch(width, height int) (*UITestDriver, error) {
	if err := w.Manager.Save(w.Config); err != nil {
		return nil, fmt.Errorf("failed to write workspace config: %w", err)
	}
	app := NewAppModel(w.Config, w.Manager)
	return NewUITestDriver(app, width, height), nil
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
	// tree into a fresh temp workspace FIRST, then load from the copy: Manager.Load
	// re-saves first-load migrations, and copying before loading keeps every such
	// write on the copy so the source is never mutated. MigrateCopyTree refuses to
	// overwrite and skips runtime-only entries.
	srcRoot := config.WorkspaceRootFromConfigPath(sourceConfigPath)

	root, err := os.MkdirTemp("", "tt-uitest-copy-*")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create copied workspace temp dir: %w", err)
	}
	ws := &uitestWorkspace{Root: root, ConfigPath: filepath.Join(root, filepath.Base(sourceConfigPath))}

	if err = config.MigrateCopyTree(srcRoot, root); err != nil {
		_ = ws.Cleanup()
		return nil, nil, fmt.Errorf("failed to copy source workspace: %w", err)
	}

	ws.Manager = config.NewManager(ws.ConfigPath)
	cfg, err := ws.Manager.Load()
	if err != nil {
		_ = ws.Cleanup()
		return nil, nil, fmt.Errorf("failed to load copied config: %w", err)
	}

	// The copied workspace roots at `root`. A domain-loaded config already derives
	// DataDir and every project path there; only a legacy config with persisted
	// absolute paths still points into the source and needs rerooting onto the copy.
	srcDataDir := cfg.DataDir
	cfg.DataDir = root
	cfg.FzfEnabled = false
	cfg.GlowEnabled = false
	for _, p := range cfg.Projects {
		if p == nil {
			continue
		}
		if pathUnderRoot(root, p.Path) {
			continue
		}
		newPath, rerootErr := rerootProjectPath(p.Path, srcDataDir, root, p.ID)
		if rerootErr != nil {
			_ = ws.Cleanup()
			return nil, nil, fmt.Errorf("failed to isolate project %q: %w", p.ID, rerootErr)
		}
		p.Path = newPath
	}

	ws.DataDir = root
	ws.Config = cfg

	driver, err := ws.launch(width, height)
	if err != nil {
		_ = ws.Cleanup()
		return nil, nil, err
	}
	return driver, ws, nil
}

// pathUnderRoot reports whether path is root itself or lives inside root, on
// cleaned relative paths. It lets NewCopiedWorkspaceDriver skip rerooting project
// paths a domain reload already derived into the copy.
func pathUnderRoot(root, path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// rerootProjectPath maps a project's data directory into the copied workspace
// so writes through the copy never reach the source tree.
//
// Paths under srcDataDir are rerooted onto dstDataDir, preserving the relative
// subpath — the data dir itself was already copied wholesale by MigrateCopyTree.
// Paths outside srcDataDir (custom/external project locations) are copied into a
// deterministic per-project subdir under the copy (dstDataDir/projects/
// __external_<projectID>) and repointed there, so the original external
// directory is left byte-for-byte untouched.
//
// Containment is decided on cleaned absolute paths. Empty, unreadable, or
// non-directory external paths return an actionable error rather than silently
// leaving the project pointed at the source tree.
func rerootProjectPath(path, srcDataDir, dstDataDir, projectID string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("project path is empty")
	}
	absPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("failed to resolve project path %q: %w", path, err)
	}
	absSrc, err := filepath.Abs(filepath.Clean(srcDataDir))
	if err != nil {
		return "", fmt.Errorf("failed to resolve source data dir %q: %w", srcDataDir, err)
	}

	// Under the source data dir: reroot onto the already-copied destination.
	rel, relErr := filepath.Rel(absSrc, absPath)
	if relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.Join(dstDataDir, rel), nil
	}

	// External project directory: copy it into the workspace so writes stay
	// isolated from the original external tree.
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to read external project dir %q: %w", absPath, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("external project path %q is not a directory", absPath)
	}
	dst := filepath.Join(dstDataDir, "projects", "__external_"+projectID)
	if err := config.MigrateCopyTree(absPath, dst); err != nil {
		return "", fmt.Errorf("failed to copy external project dir %q: %w", absPath, err)
	}
	return dst, nil
}

// --- Built-in fixture builders ---

// buildEmptyFixture leaves the workspace with no user projects, exercising the
// zero-task onboarding surface (the dashboard capture hint).
func buildEmptyFixture(_ *uitestWorkspace) error {
	return nil
}

// buildRichFixture creates a project with a full task hierarchy spanning every
// task type, status, priority, and a couple of GTD sections.
func buildRichFixture(w *uitestWorkspace) error {
	project, err := w.addProject("Rich Project", func(p *models.Project) {
		epic := newFixtureTask(p, models.TaskTypeEpic, "Launch platform")
		epic.Description = "Ship the first public release."
		epic.Priority = 1

		story := newFixtureTask(p, models.TaskTypeStory, "User authentication")
		story.ParentID = &epic.ID
		story.Priority = 2

		login := newFixtureTask(p, models.TaskTypeTask, "Build login form")
		login.ParentID = &story.ID
		login.ApplyWorkflowState(models.StateInProgress)

		validate := newFixtureTask(p, models.TaskTypeTask, "Add password validation")
		validate.ParentID = &story.ID
		validate.Tags = []string{"backend", "security"}

		mockup := newFixtureTask(p, models.TaskTypeTask, "Design login mockup")
		mockup.ParentID = &story.ID
		markDone(mockup)

		bug := newFixtureTask(p, models.TaskTypeBug, "Fix redirect loop after logout")
		bug.Priority = 1
		bug.Tags = []string{"urgent"}
		bug.ApplyWorkflowState(models.StateWaiting)

		idea := newFixtureTask(p, models.TaskTypeIdea, "Explore social login")
		idea.Description = "Google / GitHub OAuth."
		idea.ApplyWorkflowState(models.StateSomeday)

		inbox := newFixtureTask(p, models.TaskTypeTask, "Triage support inbox")
		inbox.ApplyWorkflowState(models.StateInbox)
	})
	if err != nil {
		return err
	}
	w.Config.CurrentProject = project.ID
	return nil
}

// buildAlertsFixture creates tasks that trip dashboard alerts: overdue,
// due-soon, and high-priority blocked work.
func buildAlertsFixture(w *uitestWorkspace) error {
	project, err := w.addProject("Alerts Project", func(p *models.Project) {
		overdue := newFixtureTask(p, models.TaskTypeTask, "Overdue: submit quarterly report")
		overdue.Priority = 1
		overdue.ApplyWorkflowState(models.StateNext)
		setDue(overdue, -3*24*time.Hour)

		dueSoon := newFixtureTask(p, models.TaskTypeTask, "Due soon: renew certificate")
		dueSoon.Priority = 2
		dueSoon.ApplyWorkflowState(models.StateNext)
		setDue(dueSoon, 12*time.Hour)

		blocked := newFixtureTask(p, models.TaskTypeBug, "Blocked: production outage")
		blocked.Priority = 1
		blocked.ApplyWorkflowState(models.StateWaiting)

		waiting := newFixtureTask(p, models.TaskTypeTask, "Waiting on vendor response")
		waiting.ApplyWorkflowState(models.StateWaiting)
	})
	if err != nil {
		return err
	}
	_, err = w.addProject("Alert Backlog", func(p *models.Project) {
		stale := time.Now().Add(-21 * 24 * time.Hour)
		for i := 0; i < 5; i++ {
			task := newFixtureTask(p, models.TaskTypeTask, fmt.Sprintf("Untriaged alert backlog item %d", i+1))
			task.ApplyWorkflowState(models.StateInbox)
			task.CreatedAt = stale
			task.UpdatedAt = stale
		}
	})
	if err != nil {
		return err
	}
	w.Config.CurrentProject = project.ID
	return nil
}

// buildWorkflowFixture creates a spread of tasks across workflow states so the
// planning/review views have something to render.
func buildWorkflowFixture(w *uitestWorkspace) error {
	titles := []string{
		"Sketch onboarding flow",
		"Design settings screen",
		"Implement export pipeline",
		"Review release notes",
	}
	project, err := w.addProject("Workflow Project", func(p *models.Project) {
		for i, title := range titles {
			t := newFixtureTask(p, models.TaskTypeTask, title)
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
	w.Config.CurrentProject = project.ID
	return nil
}

// buildNarrowFixture creates tasks with long titles and tags to stress layout
// when the driver is run at a narrow terminal width.
func buildNarrowFixture(w *uitestWorkspace) error {
	project, err := w.addProject("Narrow Project", func(p *models.Project) {
		long := newFixtureTask(p, models.TaskTypeTask,
			"Investigate intermittent timeout when syncing very large project archives over slow connections")
		long.Priority = 1
		long.Tags = []string{"infrastructure", "reliability", "sync"}
		long.ApplyWorkflowState(models.StateNext)

		another := newFixtureTask(p, models.TaskTypeBug,
			"Truncated column headers overlap when the terminal is resized below the minimum comfortable width")
		another.ApplyWorkflowState(models.StateInProgress)
	})
	if err != nil {
		return err
	}
	w.Config.CurrentProject = project.ID
	return nil
}

// newFixtureTask creates a task with the next short ID, appends it to the
// project, and returns it for further customization before the tree is built.
func newFixtureTask(p *models.Project, taskType models.TaskType, title string) *models.Task {
	task, err := models.NewTask(uuid.New().String(), p.NextShortID, taskType, title)
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
