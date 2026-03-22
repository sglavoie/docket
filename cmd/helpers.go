package cmd

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/storage/format"
	"github.com/sglavoie/tt/internal/tterrors"
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

// TaskMsg formats a standard success message for task operations.
// Format: "[project] Verb Type #id: title"
func TaskMsg(projectName, verb string, task *models.Task) string {
	return fmt.Sprintf("[%s] %s %s #%d: %s", projectName, verb, task.Type.HeadingPrefix(), task.ShortID, task.Title)
}

// successMsg formats a success message with a ✓ prefix.
// Format: "✓ <message>"
func successMsg(format string, args ...any) string {
	return "✓ " + fmt.Sprintf(format, args...)
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

// LoadConfig loads the configuration file, creating it if it does not exist.
// Returns the config, manager, and any error encountered.
func LoadConfig() (*config.Config, *config.Manager, error) {
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.LoadOrCreate()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}
	// Apply config-driven storage settings
	if cfg.MaxActivityEntries != nil {
		storage.MaxActivityEntries = *cfg.MaxActivityEntries
	}
	external.SetFzfTimeout(cfg.GetFzfTimeout())
	return cfg, configManager, nil
}

// LoadConfigStrict loads the configuration file, returning an error if it does
// not exist. Use this instead of LoadConfig when the config must already exist
// (e.g., workflow and template commands that operate on existing config data).
func LoadConfigStrict() (*config.Config, *config.Manager, error) {
	configManager := config.NewManager(GetConfigPath())
	cfg, err := configManager.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config: %w", err)
	}
	if cfg.MaxActivityEntries != nil {
		storage.MaxActivityEntries = *cfg.MaxActivityEntries
	}
	external.SetFzfTimeout(cfg.GetFzfTimeout())
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

// FindActionTaskByID searches all loadable projects for a task by short ID or UUID prefix.
// Returns the task and its project, or nil, nil if not found.
func FindActionTaskByID(cfg *config.Config, query string) (*models.Task, *models.Project) {
	var shortID int
	isShortID := false
	if _, err := fmt.Sscanf(query, "%d", &shortID); err == nil {
		isShortID = true
	}

	for _, project := range cfg.GetAllLoadableProjects() {
		if err := storage.LoadProject(project); err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping project %s: %v\n", project.Name, err)
			continue
		}
		for _, task := range project.Tasks {
			if isShortID && task.ShortID == shortID {
				return task, project
			}
			if !isShortID && strings.HasPrefix(task.ID, query) {
				return task, project
			}
		}
	}
	return nil, nil
}

// LoadConfigAndProject loads the config and looks up a project by slug.
// Returns an error if the project is not found.
func LoadConfigAndProject(slug string) (*models.Project, *config.Config, *config.Manager, error) {
	cfg, mgr, err := LoadConfig()
	if err != nil {
		return nil, nil, nil, err
	}
	project := cfg.GetProject(slug)
	if project == nil {
		return nil, nil, nil, tterrors.NewProjectNotFoundError(slug)
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

// outputSingleTaskJSON marshals a task to indented JSON and prints it.
func outputSingleTaskJSON(task *models.Task) error {
	tags := task.Tags
	if tags == nil {
		tags = []string{}
	}
	entry := struct {
		ID        string   `json:"id"`
		ShortID   int      `json:"short_id"`
		Type      string   `json:"type"`
		Title     string   `json:"title"`
		Status    string   `json:"status"`
		Priority  int      `json:"priority"`
		Tags      []string `json:"tags"`
		Section   string   `json:"section,omitempty"`
		CreatedAt string   `json:"created_at"`
		UpdatedAt string   `json:"updated_at"`
		DueDate   *string  `json:"due_date,omitempty"`
	}{
		ID:        task.ID,
		ShortID:   task.ShortID,
		Type:      task.Type.String(),
		Title:     task.Title,
		Status:    task.Status.String(),
		Priority:  task.Priority,
		Tags:      tags,
		Section:   task.Section,
		CreatedAt: task.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: task.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if task.DueDate != nil {
		s := task.DueDate.Format("2006-01-02T15:04:05Z07:00")
		entry.DueDate = &s
	}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal task to JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// outputTasksJSON marshals a slice of tasks to indented JSON and prints it.
func outputTasksJSON(tasks []*models.Task) error {
	type taskEntry struct {
		ID        string   `json:"id"`
		ShortID   int      `json:"short_id"`
		Type      string   `json:"type"`
		Title     string   `json:"title"`
		Status    string   `json:"status"`
		Priority  int      `json:"priority"`
		Tags      []string `json:"tags"`
		Section   string   `json:"section,omitempty"`
		CreatedAt string   `json:"created_at"`
		UpdatedAt string   `json:"updated_at"`
	}
	entries := make([]taskEntry, 0, len(tasks))
	for _, task := range tasks {
		tags := task.Tags
		if tags == nil {
			tags = []string{}
		}
		entries = append(entries, taskEntry{
			ID:        task.ID,
			ShortID:   task.ShortID,
			Type:      task.Type.String(),
			Title:     task.Title,
			Status:    task.Status.String(),
			Priority:  task.Priority,
			Tags:      tags,
			Section:   task.Section,
			CreatedAt: task.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt: task.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal tasks to JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
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
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal blocker to JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// outputNoteJSON marshals a note (with its scope) to indented JSON and prints it.
func outputNoteJSON(note *models.Note, scope string) error {
	tags := note.Tags
	if tags == nil {
		tags = []string{}
	}
	entry := struct {
		Filename string   `json:"filename"`
		Title    string   `json:"title"`
		Tags     []string `json:"tags"`
		Scope    string   `json:"scope"`
	}{
		Filename: note.Filename,
		Title:    note.Title,
		Tags:     tags,
		Scope:    scope,
	}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal note to JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// outputSingleProjectJSON marshals a project to indented JSON and prints it.
func outputSingleProjectJSON(project *models.Project) error {
	entry := struct {
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
	}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal project to JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
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
