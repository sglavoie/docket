package cmd

import (
	"bufio"
	"encoding/json"
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
func FindTaskGloballyByID(projects []*models.Project, query string) (*models.Task, *models.Project, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil, nil
	}

	ref := models.ParseTaskRef(query)

	// Qualified reference: resolve strictly within the named project. This is
	// the unambiguous form a caller must use to target a short ID that is not
	// globally unique.
	if ref.IsCrossProject() {
		var project *models.Project
		for _, candidate := range projects {
			if candidate != nil && (candidate.ID == ref.ProjectID || candidate.Slug == ref.ProjectID) {
				project = candidate
				break
			}
		}
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
		for _, project := range projects {
			if project == nil {
				continue
			}
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
	for _, project := range projects {
		if project == nil {
			continue
		}
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

// catalogWorkspaceDataDir returns the workspace data directory that owns the
// manager's config, derived from the config path's parent. It equals the
// DataDir a domain-loaded config carries, so catalog-domain project helpers can
// derive project paths without dereferencing the config.Config facade.
func catalogWorkspaceDataDir(mgr *config.Manager) string {
	return config.WorkspaceRootFromConfigPath(mgr.GetConfigPath())
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

// taskInFocusQueue reports Focus Queue membership for CLI JSON output.
func taskInFocusQueue(items []config.FocusQueueItem, taskID string) bool {
	for _, item := range items {
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
