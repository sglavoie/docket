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

	configPath := filepath.Join(root, "tt.json")
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
	if err := build(ws); err != nil {
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

	// Pure read of the source config file — the source is never written to.
	// Note: config.Manager.Load re-saves the config when it applies first-load
	// migrations (default rules, templates), so we must not Load the source in
	// place. Instead we stage the raw bytes in the temp workspace and load
	// there, keeping every migration write on the copy.
	srcConfigBytes, err := os.ReadFile(sourceConfigPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read source config %q: %w", sourceConfigPath, err)
	}

	root, err := os.MkdirTemp("", "tt-uitest-copy-*")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create copied workspace temp dir: %w", err)
	}
	ws := &uitestWorkspace{Root: root, ConfigPath: filepath.Join(root, "tt.json")}

	if err := os.WriteFile(ws.ConfigPath, srcConfigBytes, config.FilePermissions); err != nil {
		_ = ws.Cleanup()
		return nil, nil, fmt.Errorf("failed to stage source config: %w", err)
	}
	ws.Manager = config.NewManager(ws.ConfigPath)
	cfg, err := ws.Manager.Load()
	if err != nil {
		_ = ws.Cleanup()
		return nil, nil, fmt.Errorf("failed to load staged config: %w", err)
	}
	srcDataDir := cfg.DataDir
	if strings.TrimSpace(srcDataDir) == "" {
		_ = ws.Cleanup()
		return nil, nil, fmt.Errorf("source config %q has no data_dir", sourceConfigPath)
	}

	// Copy the durable contents of the source data dir into the temp copy.
	// MigrateCopyTree refuses to overwrite and skips runtime-only entries.
	dstDataDir := filepath.Join(root, "data")
	if err := config.MigrateCopyTree(srcDataDir, dstDataDir); err != nil {
		_ = ws.Cleanup()
		return nil, nil, fmt.Errorf("failed to copy data dir: %w", err)
	}

	// Rewrite the config onto the copy. Because the staged config was loaded
	// from (and is saved back to) the temp path, the source config file stays
	// byte-for-byte untouched.
	cfg.DataDir = dstDataDir
	cfg.FzfEnabled = false
	cfg.GlowEnabled = false
	for _, p := range cfg.Projects {
		if p == nil {
			continue
		}
		newPath, err := rerootProjectPath(p.Path, srcDataDir, dstDataDir, p.ID)
		if err != nil {
			_ = ws.Cleanup()
			return nil, nil, fmt.Errorf("failed to isolate project %q: %w", p.ID, err)
		}
		p.Path = newPath
	}

	ws.DataDir = dstDataDir
	ws.Config = cfg

	driver, err := ws.launch(width, height)
	if err != nil {
		_ = ws.Cleanup()
		return nil, nil, err
	}
	return driver, ws, nil
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
		login.Status = models.StatusInProgress
		login.SetSection(models.SectionNext)

		validate := newFixtureTask(p, models.TaskTypeTask, "Add password validation")
		validate.ParentID = &story.ID
		validate.Status = models.StatusTodo
		validate.Tags = []string{"backend", "security"}

		mockup := newFixtureTask(p, models.TaskTypeTask, "Design login mockup")
		mockup.ParentID = &story.ID
		markDone(mockup)

		bug := newFixtureTask(p, models.TaskTypeBug, "Fix redirect loop after logout")
		bug.Priority = 1
		bug.Tags = []string{"urgent"}
		bug.Status = models.StatusBlocked

		idea := newFixtureTask(p, models.TaskTypeIdea, "Explore social login")
		idea.Description = "Google / GitHub OAuth."
		idea.SetSection(models.SectionSomeday)

		inbox := newFixtureTask(p, models.TaskTypeTask, "Triage support inbox")
		inbox.SetSection(models.SectionInbox)
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
		overdue.SetSection(models.SectionNext)
		setDue(overdue, -3*24*time.Hour)

		dueSoon := newFixtureTask(p, models.TaskTypeTask, "Due soon: renew certificate")
		dueSoon.Priority = 2
		dueSoon.SetSection(models.SectionNext)
		setDue(dueSoon, 12*time.Hour)

		blocked := newFixtureTask(p, models.TaskTypeBug, "Blocked: production outage")
		blocked.Priority = 1
		blocked.Status = models.StatusBlocked

		waiting := newFixtureTask(p, models.TaskTypeTask, "Waiting on vendor response")
		waiting.SetSection(models.SectionWaiting)
	})
	if err != nil {
		return err
	}
	_, err = w.addProject("Alert Backlog", func(p *models.Project) {
		stale := time.Now().Add(-21 * 24 * time.Hour)
		for i := 0; i < 5; i++ {
			task := newFixtureTask(p, models.TaskTypeTask, fmt.Sprintf("Untriaged alert backlog item %d", i+1))
			task.SetSection(models.SectionInbox)
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

// buildWorkflowFixture creates tasks spread across workflow stages so the
// stage-oriented views have something to render.
func buildWorkflowFixture(w *uitestWorkspace) error {
	stages := []struct {
		title string
		stage string
	}{
		{"Sketch onboarding flow", "Brainstorm"},
		{"Design settings screen", "Design"},
		{"Implement export pipeline", "Implement"},
		{"Review release notes", "Review"},
	}
	project, err := w.addProject("Workflow Project", func(p *models.Project) {
		for i, s := range stages {
			t := newFixtureTask(p, models.TaskTypeTask, s.title)
			t.Stage = s.stage
			t.Priority = (i % 3) + 1
			t.SetSection(models.SectionNext)
			if i == len(stages)-1 {
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
		long.SetSection(models.SectionNext)

		another := newFixtureTask(p, models.TaskTypeBug,
			"Truncated column headers overlap when the terminal is resized below the minimum comfortable width")
		another.Status = models.StatusInProgress
		another.SetSection(models.SectionNext)
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

// markDone completes a task and stamps CompletedAt/section like the lifecycle
// transition would.
func markDone(t *models.Task) {
	now := time.Now()
	t.Status = models.StatusDone
	t.CompletedAt = &now
	t.SetSection(models.SectionCompleted)
}

// setDue sets a due date relative to now.
func setDue(t *models.Task, offset time.Duration) {
	due := time.Now().Add(offset)
	t.DueDate = &due
}
