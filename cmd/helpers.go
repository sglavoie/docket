package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/storage/format"
	"github.com/sglavoie/tt/internal/template"
	"github.com/sglavoie/tt/internal/tterrors"
	"github.com/sglavoie/tt/internal/util"
)

// Dependencies holds injectable dependencies for commands.
// This allows swapping implementations for testing.
type Dependencies struct {
	Selector   external.FuzzySelector
	Renderer   external.MarkdownRenderer
	Editor     external.Editor
	Repository storage.ProjectRepository
}

// Deps is the global dependencies instance used by commands.
// Tests can replace this with mock implementations.
var Deps = Dependencies{
	Selector:   external.DefaultSelector,
	Renderer:   external.DefaultRenderer,
	Editor:     external.DefaultEditor,
	Repository: storage.DefaultRepository,
}

const defaultDataDirDisplay = "~/.tt (an existing iCloud Drive tt workspace is used if present)"

// TaskMsg formats a standard success message for task operations.
// Format: "[project] Verb Type #id: title"
func TaskMsg(projectName, verb string, task *models.Task) string {
	return fmt.Sprintf("[%s] %s %s #%d: %s", projectName, verb, task.Type.HeadingPrefix(), task.ShortID, task.Title)
}

// successMsg formats a success message with a ✓ prefix.
// Format: "✓ <message>"
func successMsg(format string, args ...any) string {
	return icons.Get("success") + " " + fmt.Sprintf(format, args...)
}

// NoItemsMsg returns a standardized "no <thing> found" message with an actionable hint.
// Use this for empty-state messages across all commands.
func NoItemsMsg(thing, hint string) string {
	if hint != "" {
		return fmt.Sprintf("No %s found. Create one with: %s", thing, hint)
	}
	return fmt.Sprintf("No %s found.", thing)
}

// NoCurrentProjectError returns a standardized error for when no project is selected.
func NoCurrentProjectError() error {
	return fmt.Errorf("%w. Select one with: tt select, or specify with --project", tterrors.ErrNoCurrentProject)
}

// Debugf prints a formatted message to stderr if verbose mode is enabled.
func Debugf(format string, args ...any) {
	if IsVerbose() {
		logging.Warn("[debug] "+format, args...)
	}
}

// Pluralize returns "1 task" or "3 tasks" based on count.
func Pluralize(count int, singular, plural string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}

func shortDisplayID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// CommandContext bundles the commonly-needed config and project data for commands.
// This reduces duplicate loading code and makes it easier to pass context around.
type CommandContext struct {
	Project       *models.Project
	Config        *config.Config
	ConfigManager *config.Manager
}

// ConfirmYN prompts the user with a [y/N] question. Returns true if the user
// answers y or yes (case-insensitive). When force is true, skips the prompt and
// returns true immediately.
func ConfirmYN(prompt string, force bool) bool {
	if force {
		return true
	}
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false // safe default: abort on I/O error
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

// ConfirmStrict prompts the user to type "yes" to confirm a destructive action.
// Returns true only if the user types exactly "yes". When force is true, skips
// the prompt and returns true immediately.
func ConfirmStrict(prompt string, force bool) bool {
	if force {
		return true
	}
	fmt.Println(prompt)
	fmt.Print("Type 'yes' to confirm: ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false // safe default: abort on I/O error
	}
	return strings.TrimSpace(strings.ToLower(response)) == "yes"
}

// applyConfigSettings applies config-driven settings to storage and external packages.
func applyConfigSettings(cfg *config.Config) {
	if cfg.MaxActivityEntries != nil {
		storage.MaxActivityEntries = *cfg.MaxActivityEntries
	}
	external.SetFzfTimeout(cfg.GetFzfTimeout())
	icons.SetUseEmoji(cfg.GetUseEmoji())
}

// LoadConfig loads the configuration file, creating it if it does not exist.
// Returns the config, manager, and any error encountered.
func LoadConfig() (*config.Config, *config.Manager, error) {
	configManager := config.NewManager(resolveConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}
	applyConfigSettings(cfg)
	return cfg, configManager, nil
}

// LoadConfigStrict loads the configuration file, returning an error if it does
// not exist. Use this instead of LoadConfig when the config must already exist
// (e.g., workflow and template commands that operate on existing config data).
func LoadConfigStrict() (*config.Config, *config.Manager, error) {
	configManager := config.NewManager(resolveConfigPath())
	cfg, err := configManager.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}
	applyConfigSettings(cfg)
	return cfg, configManager, nil
}

// LoadCurrentProject loads the config and current project with its tasks.
// Returns an error if no project is selected or if loading fails.
// Parser warnings are printed to stderr (always for data corrections, verbose mode for others).
func LoadCurrentProject() (*models.Project, *config.Config, *config.Manager, error) {
	cfg, configManager, err := LoadConfig()
	if err != nil {
		return nil, nil, nil, err
	}

	project := cfg.GetCurrentProject()
	if project == nil {
		return nil, nil, nil, NoCurrentProjectError()
	}

	Debugf("loading project: %s (%s)", project.Name, project.Path)

	result, err := storage.LoadProjectWithWarnings(project)
	if err != nil {
		if errors.Is(err, tterrors.ErrParseError) {
			return nil, nil, nil, fmt.Errorf("corrupted tasks file in %s: %w", project.Name, err)
		}
		return nil, nil, nil, fmt.Errorf("failed to load project: %w", err)
	}

	Debugf("loaded %d tasks", len(project.Tasks))

	// Print warnings to stderr (data corrections always, deprecations always, validation in verbose mode)
	for _, w := range result.Warnings {
		switch w.Type {
		case format.WarningDataCorrection, format.WarningDeprecation:
			fmt.Fprintf(os.Stderr, "warning: %s\n", w.Message)
		default:
			Debugf("parser warning: %s", w.Message)
		}
	}

	return project, cfg, configManager, nil
}

// FindTaskGloballyByID searches loadable projects for a task by reference.
// Returns the task and its project, or nil, nil if not found. This is the
// shared cross-project lookup for destructive commands, so it fails closed on
// ambiguity rather than silently acting on the first match.
//
// Reference forms:
//   - "project-slug:#short-id" / "project-slug:uuid": a qualified reference
//     resolved within the named project only. A leading '#' on the task part
//     is tolerated.
//   - A bare numeric short ID: short IDs are unique only within a project, so
//     the query is collected across all projects and an AmbiguousIdentifierError
//     is returned when more than one project owns that short ID. The caller must
//     then qualify the reference (project-slug:#short-id).
//   - A bare UUID prefix: all loadable projects are scanned and ambiguity is an
//     explicit error. Matching is case-insensitive — UUIDs are stored lowercase
//     but users may paste uppercase.
//
// An empty query never matches; HasPrefix("", "") would otherwise match the
// first task in the first loadable project.
func FindTaskGloballyByID(cfg *config.Config, query string) (*models.Task, *models.Project, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil, nil
	}

	ref := models.ParseTaskRef(query)

	// Qualified reference: resolve strictly within the named project. This is
	// the unambiguous form a caller must use to target a short ID that is not
	// globally unique.
	if ref.IsCrossProject() {
		project := cfg.GetProject(ref.ProjectID)
		if project == nil {
			return nil, nil, tterrors.NewProjectNotFoundError(ref.ProjectID)
		}
		if err := storage.LoadProject(project); err != nil {
			return nil, nil, fmt.Errorf("failed to load project %s: %w", project.Name, err)
		}
		task, err := FindTaskByIdentifier(project, strings.TrimPrefix(ref.TaskID, "#"))
		if err != nil {
			return nil, nil, err
		}
		return task, project, nil
	}

	type match struct {
		task    *models.Task
		project *models.Project
	}

	// Bare numeric short ID: unique only within a project. Collect matches
	// across every project and reject a cross-project collision instead of
	// silently returning the first hit and acting on the wrong task.
	if ref.IsNumericShortID() {
		shortID, _ := strconv.Atoi(query)
		var shortMatches []match
		for _, project := range cfg.GetAllLoadableProjects() {
			if err := storage.LoadProject(project); err != nil {
				fmt.Fprintf(os.Stderr, "warning: skipping project %s: %v\n", project.Name, err)
				continue
			}
			if task := project.GetTaskByShortID(shortID); task != nil {
				shortMatches = append(shortMatches, match{task: task, project: project})
			}
		}
		switch len(shortMatches) {
		case 0:
			return nil, nil, nil
		case 1:
			return shortMatches[0].task, shortMatches[0].project, nil
		default:
			limit := min(len(shortMatches), 5)
			parts := make([]string, limit)
			for i := range limit {
				parts[i] = fmt.Sprintf("%s:#%d %s", shortMatches[i].project.Slug, shortMatches[i].task.ShortID, shortMatches[i].task.Title)
			}
			candidates := strings.Join(parts, ", ")
			if len(shortMatches) > 5 {
				candidates += fmt.Sprintf(", ... (%d more)", len(shortMatches)-5)
			}
			return nil, nil, tterrors.NewAmbiguousIdentifierError(query, candidates)
		}
	}

	// Bare UUID or UUID prefix: scoped globally with an explicit ambiguity error.
	lowerQuery := strings.ToLower(query)
	var prefixMatches []match
	for _, project := range cfg.GetAllLoadableProjects() {
		if err := storage.LoadProject(project); err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping project %s: %v\n", project.Name, err)
			continue
		}
		for _, task := range project.Tasks {
			if strings.HasPrefix(strings.ToLower(task.ID), lowerQuery) {
				prefixMatches = append(prefixMatches, match{task: task, project: project})
			}
		}
	}

	switch len(prefixMatches) {
	case 0:
		return nil, nil, nil
	case 1:
		return prefixMatches[0].task, prefixMatches[0].project, nil
	default:
		limit := min(len(prefixMatches), 5)
		parts := make([]string, limit)
		for i := range limit {
			parts[i] = fmt.Sprintf("#%d %s (%s)", prefixMatches[i].task.ShortID, prefixMatches[i].task.Title, shortDisplayID(prefixMatches[i].task.ID))
		}
		candidates := strings.Join(parts, ", ")
		if len(prefixMatches) > 5 {
			candidates += fmt.Sprintf(", ... (%d more)", len(prefixMatches)-5)
		}
		return nil, nil, tterrors.NewAmbiguousIdentifierError(query, candidates)
	}
}

// LoadConfigAndProject loads the config and looks up a project by slug or ID.
// Returns an error if the project is not found.
func LoadConfigAndProject(projectRef string) (*models.Project, *config.Config, *config.Manager, error) {
	cfg, mgr, err := LoadConfig()
	if err != nil {
		return nil, nil, nil, err
	}
	project := cfg.GetProject(projectRef)
	if project == nil {
		return nil, nil, nil, tterrors.NewProjectNotFoundError(projectRef)
	}
	return project, cfg, mgr, nil
}

// ReportBatchErrors prints a summary and returns nil on partial success, error on total failure.
func ReportBatchErrors(succeeded int, errs []string) error {
	if len(errs) == 0 {
		return nil
	}
	if succeeded > 0 {
		fmt.Printf("\n%d succeeded, %d failed:\n", succeeded, len(errs))
		for _, e := range errs {
			fmt.Printf("  %s\n", e)
		}
		return nil // partial success = exit 0
	}
	return fmt.Errorf("%s", strings.Join(errs, "\n"))
}

// saveConfig persists the config and wraps any error in a standard SaveConfigError.
func saveConfig(mgr *config.Manager, cfg *config.Config) error {
	if err := mgr.Save(cfg); err != nil {
		return tterrors.NewSaveConfigError(err)
	}
	return nil
}

// planningStore returns the manager's narrow planning-domain seam (focus queue,
// daily plan). Commands that only touch planning state read and write it through
// this store instead of the config.Config facade; load and save must reuse the
// single returned instance so the workspace ID adopted on load is re-stamped on
// save.
func planningStore(mgr *config.Manager) config.PlanningStore {
	return mgr.RuntimeDependencies().Planning
}

// automationStore returns the manager's narrow automation-domain seam (workflow
// rules, eval state). Workflow commands read and mutate rules through this store
// rather than the config.Config facade. As with planningStore, reuse the single
// returned instance for both load and save.
func automationStore(mgr *config.Manager) config.AutomationStore {
	return mgr.RuntimeDependencies().Automation
}

// catalogWorkspaceDataDir returns the workspace data directory that owns the
// manager's config, derived from the config path's parent. It equals the
// DataDir a domain-loaded config carries, so catalog-domain project helpers can
// derive project paths without dereferencing the config.Config facade.
func catalogWorkspaceDataDir(mgr *config.Manager) string {
	return config.WorkspaceRootFromConfigPath(mgr.GetConfigPath())
}

// activeProjects returns the workspace's active projects through the catalog
// domain seam instead of the config.Config facade. It is the drop-in for
// cfg.GetActiveProjects at command consumers that already hold the manager.
func activeProjects(mgr *config.Manager) ([]*models.Project, error) {
	return mgr.RuntimeDependencies().ActiveProjects(catalogWorkspaceDataDir(mgr))
}

// allLoadableProjects returns the workspace's non-archived projects (internal
// included) through the catalog domain seam, mirroring
// cfg.GetAllLoadableProjects.
func allLoadableProjects(mgr *config.Manager) ([]*models.Project, error) {
	return mgr.RuntimeDependencies().AllLoadableProjects(catalogWorkspaceDataDir(mgr))
}

// lookupProject resolves a project by ID or slug through the catalog domain
// seam, returning nil when none matches (mirroring cfg.GetProject).
func lookupProject(mgr *config.Manager, idOrSlug string) (*models.Project, error) {
	return mgr.RuntimeDependencies().Project(catalogWorkspaceDataDir(mgr), idOrSlug)
}

// sandboxProject resolves the sandbox project through the catalog domain seam,
// returning nil when the workspace has none (mirroring cfg.GetSandboxProject).
func sandboxProject(mgr *config.Manager) (*models.Project, error) {
	return mgr.RuntimeDependencies().SandboxProject(catalogWorkspaceDataDir(mgr))
}

// allProjects returns every project in the workspace (archived, sandbox, and
// internal included) through the catalog domain seam, mirroring the raw
// cfg.Projects slice for consumers that need the unfiltered set.
func allProjects(mgr *config.Manager) ([]*models.Project, error) {
	return mgr.RuntimeDependencies().AllProjects(catalogWorkspaceDataDir(mgr))
}

// loadSettings returns the workspace's portable settings through the settings
// domain seam instead of the config.Config facade. It is the drop-in for reads
// of cfg's settings fields (themes, undo depth, etc.) at command consumers that
// already hold the manager.
func loadSettings(mgr *config.Manager) (*config.Settings, error) {
	return mgr.RuntimeDependencies().Settings.LoadSettings()
}

// customContextNames returns the sorted custom-context names through the machine
// domain seam, mirroring cfg.GetCustomContextNames.
func customContextNames(mgr *config.Manager) ([]string, error) {
	return mgr.RuntimeDependencies().CustomContextNames()
}

// taskTemplates returns the workspace's task templates through the automation
// domain seam, mirroring reads of cfg.TaskTemplates.
func taskTemplates(mgr *config.Manager) ([]template.TaskTemplate, error) {
	return mgr.RuntimeDependencies().TaskTemplates()
}

// savePlanning persists a single planning-domain artifact, wrapping any error in
// a standard SaveConfigError to match saveConfig's behavior.
func savePlanning(store config.PlanningStore, planning *config.PlanningState) error {
	if err := store.SavePlanning(planning); err != nil {
		return tterrors.NewSaveConfigError(err)
	}
	return nil
}

// saveAutomation persists a single automation-domain artifact, wrapping any
// error in a standard SaveConfigError to match saveConfig's behavior.
func saveAutomation(store config.AutomationStore, automation *config.AutomationConfig) error {
	if err := store.SaveAutomation(automation); err != nil {
		return tterrors.NewSaveConfigError(err)
	}
	return nil
}

// printJSON marshals v as indented JSON and prints it to stdout.
func printJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// addOutputFlags registers --json and --quiet flags on a command.
// Use getOutputOpts to read them back in RunE.
func addOutputFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("json", false, "output result as JSON")
	cmd.Flags().Bool("quiet", false, "suppress output on success")
}

// getOutputOpts reads --json and --quiet from a command and returns a MutateOutput.
// Returns an error if both flags are set.
func getOutputOpts(cmd *cobra.Command) (MutateOutput, error) {
	if cmd == nil {
		return MutateOutput{}, nil
	}
	jsonOut, _ := cmd.Flags().GetBool("json")
	quiet, _ := cmd.Flags().GetBool("quiet")
	out := MutateOutput{JSON: jsonOut, Quiet: quiet}
	return out, out.CheckMutualExclusion()
}

// MutateOutput controls how mutation commands print their results.
// JSON and Quiet are mutually exclusive; validate with CheckMutualExclusion.
type MutateOutput struct {
	JSON  bool
	Quiet bool
}

// CheckMutualExclusion returns an error if both JSON and Quiet are set.
func (o MutateOutput) CheckMutualExclusion() error {
	if o.JSON && o.Quiet {
		return fmt.Errorf("--json and --quiet are mutually exclusive")
	}
	return nil
}

// OutputTask prints a single task as JSON (--json), suppresses output (--quiet),
// or prints the standard success message.
func (o MutateOutput) OutputTask(projectName, verb string, task *models.Task) error {
	if o.Quiet {
		return nil
	}
	if o.JSON {
		return outputSingleTaskJSON(task)
	}
	fmt.Println(TaskMsg(projectName, verb, task))
	return nil
}

// OutputProject prints a project as JSON (--json), suppresses output (--quiet),
// or runs the provided fallback print function.
func (o MutateOutput) OutputProject(project *models.Project, fallback func()) error {
	if o.Quiet {
		return nil
	}
	if o.JSON {
		return outputSingleProjectJSON(project)
	}
	fallback()
	return nil
}

type mutateTaskJSONEntry struct {
	ID        string   `json:"id"`
	ShortID   int      `json:"short_id"`
	Type      string   `json:"type"`
	Title     string   `json:"title"`
	Status    string   `json:"status"`
	State     string   `json:"state"`
	Priority  int      `json:"priority"`
	Tags      []string `json:"tags"`
	Section   string   `json:"section,omitempty"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
	DueDate   *string  `json:"due_date,omitempty"`
	HasTime   bool     `json:"has_time"`
}

func taskToMutateJSONEntry(task *models.Task) mutateTaskJSONEntry {
	tags := task.Tags
	if tags == nil {
		tags = []string{}
	}
	entry := mutateTaskJSONEntry{
		ID:        task.ID,
		ShortID:   task.ShortID,
		Type:      task.Type.String(),
		Title:     task.Title,
		Status:    models.LegacyStatusForState(task.State()).String(),
		State:     task.State().String(),
		Priority:  task.Priority,
		Tags:      tags,
		Section:   models.SectionForState(task.State()),
		CreatedAt: task.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: task.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if task.DueDate != nil {
		s := task.DueDate.Format("2006-01-02T15:04:05Z07:00")
		entry.DueDate = &s
		entry.HasTime = task.HasTime
	}
	return entry
}

func outputSingleTaskJSON(task *models.Task) error {
	return printJSON(taskToMutateJSONEntry(task))
}

func outputTasksJSON(tasks []*models.Task) error {
	entries := make([]mutateTaskJSONEntry, 0, len(tasks))
	for _, task := range tasks {
		entries = append(entries, taskToMutateJSONEntry(task))
	}
	return printJSON(entries)
}

// taskInFocusQueue reports Focus Queue membership. It is the config-aware
// replacement for the retired Task.Active flag in CLI output: "active" is now
// Focus Queue membership under the workflow-state model (session 33.7.6.3).
func taskInFocusQueue(cfg *config.Config, taskID string) bool {
	if cfg == nil {
		return false
	}
	for _, item := range cfg.GetFocusQueue() {
		if item.TaskID == taskID {
			return true
		}
	}
	return false
}

// outputBlockerJSON marshals a blocker to indented JSON and prints it.
func outputBlockerJSON(blocker *models.Blocker) error {
	entry := struct {
		ID          string  `json:"id"`
		Title       string  `json:"title"`
		Description string  `json:"description,omitempty"`
		CreatedAt   string  `json:"created_at"`
		ResolvedAt  *string `json:"resolved_at,omitempty"`
	}{
		ID:          blocker.ID,
		Title:       blocker.Title,
		Description: blocker.Description,
		CreatedAt:   blocker.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if blocker.ResolvedAt != nil {
		s := blocker.ResolvedAt.Format("2006-01-02T15:04:05Z07:00")
		entry.ResolvedAt = &s
	}
	return printJSON(entry)
}

// outputNoteJSON marshals a note (with its scope) to indented JSON and prints it.
func outputNoteJSON(note *models.Note, scope string) error {
	tags := note.Tags
	if tags == nil {
		tags = []string{}
	}
	return printJSON(struct {
		Filename string   `json:"filename"`
		Title    string   `json:"title"`
		Tags     []string `json:"tags"`
		Scope    string   `json:"scope"`
	}{
		Filename: note.Filename,
		Title:    note.Title,
		Tags:     tags,
		Scope:    scope,
	})
}

// outputSingleProjectJSON marshals a project to indented JSON and prints it.
func outputSingleProjectJSON(project *models.Project) error {
	return printJSON(struct {
		ID          string `json:"id"`
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Context     string `json:"context"`
		Path        string `json:"path"`
		Description string `json:"description,omitempty"`
		StatusIcon  string `json:"status_icon,omitempty"`
	}{
		ID:          project.ID,
		Slug:        project.Slug,
		Name:        project.Name,
		Context:     project.Context,
		Path:        project.Path,
		Description: project.Description,
		StatusIcon:  project.StatusIcon,
	})
}

// LoadConfigAndProjectOrCurrent loads the config and looks up a project by slug
// if args is non-empty, otherwise falls back to the current project.
func LoadConfigAndProjectOrCurrent(args []string) (*models.Project, *config.Config, *config.Manager, error) {
	cfg, mgr, err := LoadConfig()
	if err != nil {
		return nil, nil, nil, err
	}
	var project *models.Project
	if len(args) > 0 {
		project = cfg.GetProject(args[0])
		if project == nil {
			return nil, nil, nil, tterrors.NewProjectNotFoundError(args[0])
		}
	} else {
		project = cfg.GetCurrentProject()
		if project == nil {
			return nil, nil, nil, NoCurrentProjectError()
		}
	}
	return project, cfg, mgr, nil
}

// formatTaskDueDate returns a human-readable due date string for CLI output.
// When the task carries a time component (HasTime), the time is included.
func formatTaskDueDate(task *models.Task) string {
	if task.DueDate == nil {
		return "none"
	}
	if task.HasTime {
		return task.DueDate.Format(util.DateTimeFormat)
	}
	return task.DueDate.Format(util.DateFormat)
}
