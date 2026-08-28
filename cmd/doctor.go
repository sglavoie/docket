package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check workspace data for integrity issues",
	Long: `Validate the retained workspace domains, protected Unfiled project,
migration snapshots, project files, task identities, and hierarchy
references. Diagnostics are read-only unless --fix is explicitly supplied;
immutable migration snapshots are never changed.`,
	Args: cobra.NoArgs,
	RunE: runDoctor,
}

func init() {
	doctorCmd.Flags().Bool("fix", false, "Automatically fix issues where possible")
	rootCmd.AddCommand(doctorCmd)
}

type doctorIssue struct {
	project string
	detail  string
}

type doctorResults struct {
	workspaceIssues             []doctorIssue
	workspaceNotes              []doctorIssue
	loadFailures                []doctorIssue
	duplicateIDs                []doctorIssue
	duplicateShortIDs           []doctorIssue
	missingTaskFiles            []doctorIssue
	brokenParentRefs            []doctorIssue
	invalidCompletionTimestamps []doctorIssue
	brokenTaskRefs              []doctorIssue
	unavailableTaskRefs         []doctorIssue
	emptyProjects               []doctorIssue
}

func (r *doctorResults) totalIssues() int {
	return len(r.workspaceIssues) + len(r.loadFailures) +
		len(r.duplicateIDs) + len(r.duplicateShortIDs) +
		len(r.missingTaskFiles) + len(r.brokenParentRefs) +
		len(r.invalidCompletionTimestamps) + len(r.brokenTaskRefs)
}

func (r *doctorResults) fixableIssues() int {
	return len(r.duplicateShortIDs) + len(r.brokenParentRefs)
}

func runDoctor(cmd *cobra.Command, _ []string) error {
	fix, _ := cmd.Flags().GetBool("fix")

	workspace, err := LoadCLIWorkspace()
	if err != nil {
		return err
	}

	everyProject := workspace.Runtime.AllProjects()
	var loadedProjects []*models.Project
	results := &doctorResults{}
	checkWorkspaceHealth(workspace, results)

	// Load every retained project, including archived projects, so terminal work
	// and hierarchy integrity remain visible.
	for _, project := range everyProject {
		if loadErr := storage.LoadProject(project); loadErr != nil {
			results.loadFailures = append(results.loadFailures, doctorIssue{
				project: project.Name,
				detail:  loadErr.Error(),
			})
			continue
		}
		project.BuildTaskTree()
		loadedProjects = append(loadedProjects, project)
	}

	checkMissingTaskFiles(everyProject, results)
	checkEmptyProjects(loadedProjects, results)
	checkDuplicateIDs(loadedProjects, results)
	checkDuplicateShortIDs(loadedProjects, results)
	checkBrokenParentRefs(loadedProjects, results)
	checkCompletionTimestamps(loadedProjects, results)
	checkTaskReferences(everyProject, loadedProjects, results)

	if fix {
		fixIssues(loadedProjects)
	}

	printResults(results)

	if results.totalIssues() > 0 {
		switch {
		case fix && results.fixableIssues() > 0:
			fmt.Println("\nSome issues were fixed. Re-run 'tt doctor' to verify.")
		case !fix && results.fixableIssues() > 0:
			fmt.Println("\nRun 'tt doctor --fix' to automatically fix fixable issues.")
		default:
			fmt.Println("\nResolve the reported integrity issues, then re-run 'tt doctor'.")
		}
		return fmt.Errorf("found %d issue(s)", results.totalIssues())
	}

	if len(results.emptyProjects) > 0 || len(results.workspaceNotes) > 0 || len(results.unavailableTaskRefs) > 0 {
		fmt.Println("\nAll checks passed (with informational notes above).")
	} else {
		fmt.Println("\nAll checks passed.")
	}
	return nil
}

func checkWorkspaceHealth(workspace *CLIWorkspace, results *doctorResults) {
	if workspace == nil || workspace.Runtime == nil {
		results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
			project: "Workspace",
			detail:  "runtime state is unavailable",
		})
		return
	}

	runtime := workspace.Runtime
	catalog := runtime.Catalog()
	if catalog == nil {
		results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
			project: "Workspace",
			detail:  "workspace catalog is unavailable",
		})
		return
	}

	schema := catalog.SchemaVersion
	switch {
	case schema == config.CurrentConfigVersion:
		results.workspaceNotes = append(results.workspaceNotes, doctorIssue{
			project: "Migration",
			detail:  fmt.Sprintf("workspace schema %d is current", schema),
		})
	case schema < config.CurrentConfigVersion:
		results.workspaceNotes = append(results.workspaceNotes, doctorIssue{
			project: "Migration",
			detail:  fmt.Sprintf("workspace schema %d is pre-v%d; startup migration remains pending", schema, config.CurrentConfigVersion),
		})
	default:
		results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
			project: "Migration",
			detail:  fmt.Sprintf("workspace schema %d is newer than supported schema %d", schema, config.CurrentConfigVersion),
		})
	}

	checkRetainedDomainArtifacts(runtime.DataDir(), schema, results)
	checkProtectedUnfiled(catalog, results)
	checkWorkspaceUndoLedger(runtime.DataDir(), results)
	checkMigrationSnapshots(runtime.DataDir(), schema, results)
}

func checkRetainedDomainArtifacts(dataDir string, schema int, results *doctorResults) {
	required := []struct {
		name string
		path string
	}{
		{name: "workspace", path: config.WorkspaceCatalogPath(dataDir)},
		{name: "planning", path: config.PlanningStatePath(dataDir)},
	}
	if schema < config.V6ConfigSchemaVersion {
		// A legacy v4 workspace owns both retired domains. During the staged
		// runtime cutover, newly bootstrapped workspaces own neither even though
		// activation has not yet raised their catalog schema. Treat one surviving
		// retired domain as a partial legacy bundle and require its peer.
		settingsPath := config.SettingsPath(dataDir)
		automationPath := config.AutomationPath(dataDir)
		_, settingsErr := os.Lstat(settingsPath)
		_, automationErr := os.Lstat(automationPath)
		if !os.IsNotExist(settingsErr) || !os.IsNotExist(automationErr) {
			required = append(required,
				struct {
					name string
					path string
				}{name: "settings", path: settingsPath},
				struct {
					name string
					path string
				}{name: "automation", path: automationPath},
			)
		}
	}
	for _, domain := range required {
		info, err := os.Lstat(domain.path)
		switch {
		case os.IsNotExist(err):
			results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
				project: "Domains",
				detail:  fmt.Sprintf("retained %s domain is missing: %s", domain.name, domain.path),
			})
		case err != nil:
			results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
				project: "Domains",
				detail:  fmt.Sprintf("cannot inspect retained %s domain %s: %v", domain.name, domain.path, err),
			})
		case !info.Mode().IsRegular():
			results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
				project: "Domains",
				detail:  fmt.Sprintf("retained %s domain is not a regular file: %s", domain.name, domain.path),
			})
		}
	}

	if schema >= config.V6ConfigSchemaVersion {
		for _, retired := range []struct {
			name string
			path string
		}{
			{name: "settings", path: config.SettingsPath(dataDir)},
			{name: "automation", path: config.AutomationPath(dataDir)},
		} {
			if _, err := os.Lstat(retired.path); err == nil {
				results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
					project: "Domains",
					detail:  fmt.Sprintf("retired %s domain still exists: %s", retired.name, retired.path),
				})
			} else if !os.IsNotExist(err) {
				results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
					project: "Domains",
					detail:  fmt.Sprintf("cannot inspect retired %s domain %s: %v", retired.name, retired.path, err),
				})
			}
		}
	}
}

func checkProtectedUnfiled(catalog *config.WorkspaceCatalog, results *doctorResults) {
	unfiledID := catalog.SystemUnfiledProjectID
	if unfiledID == "" {
		results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
			project: "Unfiled",
			detail:  "protected system Unfiled ID is missing",
		})
		return
	}
	for _, project := range catalog.Projects {
		if project.ID != unfiledID {
			continue
		}
		if project.Name != config.SystemUnfiledName || project.Slug != config.SystemUnfiledSlug {
			results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
				project: "Unfiled",
				detail: fmt.Sprintf(
					"protected project %q has identity %q/%q; want %q/%q",
					unfiledID,
					project.Name,
					project.Slug,
					config.SystemUnfiledName,
					config.SystemUnfiledSlug,
				),
			})
		}
		if project.Archived || project.ArchivedAt != nil {
			results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
				project: "Unfiled",
				detail:  fmt.Sprintf("protected project %q is archived", unfiledID),
			})
		}
		return
	}
	results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
		project: "Unfiled",
		detail:  fmt.Sprintf("protected system Unfiled references unknown project %q", unfiledID),
	})
}

func checkWorkspaceUndoLedger(dataDir string, results *doctorResults) {
	ledger := storage.NewWorkspaceLedger(config.UndoLedgerDir(dataDir), []string{dataDir})
	status, err := ledger.Validate()
	if err != nil {
		results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
			project: "Undo",
			detail:  err.Error(),
		})
		return
	}
	results.workspaceNotes = append(results.workspaceNotes, doctorIssue{
		project: "Undo",
		detail: fmt.Sprintf(
			"workspace ledger is valid (%s, %s)",
			Pluralize(status.UndoEntries, "undo entry", "undo entries"),
			Pluralize(status.RedoEntries, "redo entry", "redo entries"),
		),
	})
}

func checkMigrationSnapshots(dataDir string, schema int, results *doctorResults) {
	v5Inspection, err := config.InspectPreSimplificationV5Snapshots(dataDir)
	if err != nil {
		results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
			project: "Migration",
			detail:  err.Error(),
		})
		return
	}
	for _, issue := range v5Inspection.Issues {
		results.workspaceIssues = append(results.workspaceIssues, doctorIssue{
			project: "Migration",
			detail:  issue,
		})
	}
	if v5Inspection.HasArtifacts {
		results.workspaceNotes = append(results.workspaceNotes, doctorIssue{
			project: "Migration",
			detail:  fmt.Sprintf("%s recorded", Pluralize(len(v5Inspection.Pairs), "pre-v5 snapshot pair", "pre-v5 snapshot pairs")),
		})
	}
	v6Inspection, err := config.InspectPreConsolidationV6Snapshots(dataDir)
	if err != nil {
		results.workspaceIssues = append(results.workspaceIssues, doctorIssue{project: "Migration", detail: err.Error()})
		return
	}
	for _, issue := range v6Inspection.Issues {
		results.workspaceIssues = append(results.workspaceIssues, doctorIssue{project: "Migration", detail: issue})
	}
	if v6Inspection.HasArtifacts {
		results.workspaceNotes = append(results.workspaceNotes, doctorIssue{
			project: "Migration",
			detail:  fmt.Sprintf("%s recorded", Pluralize(len(v6Inspection.Pairs), "pre-v6 snapshot pair", "pre-v6 snapshot pairs")),
		})
	} else if schema >= config.V6ConfigSchemaVersion {
		results.workspaceNotes = append(results.workspaceNotes, doctorIssue{project: "Migration", detail: "clean schema-6 workspace; no pre-v6 snapshot recorded"})
	}
	v7Inspection, err := config.InspectPreScopeV7Snapshots(dataDir)
	if err != nil {
		results.workspaceIssues = append(results.workspaceIssues, doctorIssue{project: "Migration", detail: err.Error()})
		return
	}
	for _, issue := range v7Inspection.Issues {
		results.workspaceIssues = append(results.workspaceIssues, doctorIssue{project: "Migration", detail: issue})
	}
	if v7Inspection.HasArtifacts {
		results.workspaceNotes = append(results.workspaceNotes, doctorIssue{
			project: "Migration",
			detail:  fmt.Sprintf("%s recorded", Pluralize(len(v7Inspection.Pairs), "pre-v7 snapshot pair", "pre-v7 snapshot pairs")),
		})
	} else if schema >= config.V7ConfigSchemaVersion {
		results.workspaceNotes = append(results.workspaceNotes, doctorIssue{project: "Migration", detail: "clean schema-7 workspace; no pre-v7 snapshot recorded"})
	}
}

func checkMissingTaskFiles(projects []*models.Project, results *doctorResults) {
	for _, project := range projects {
		taskFile := filepath.Join(project.Path, "tasks.md")
		if _, err := os.Stat(taskFile); os.IsNotExist(err) {
			results.missingTaskFiles = append(results.missingTaskFiles, doctorIssue{
				project: project.Name,
				detail:  taskFile,
			})
		}
	}
}

func checkEmptyProjects(projects []*models.Project, results *doctorResults) {
	for _, project := range projects {
		if len(project.Tasks) == 0 {
			results.emptyProjects = append(results.emptyProjects, doctorIssue{
				project: project.Name,
			})
		}
	}
}

func checkDuplicateIDs(projects []*models.Project, results *doctorResults) {
	globalIDs := make(map[string]string) // taskID -> project name

	for _, project := range projects {
		localIDs := make(map[string]bool)
		for _, task := range project.Tasks {
			// Check within project
			if localIDs[task.ID] {
				results.duplicateIDs = append(results.duplicateIDs, doctorIssue{
					project: project.Name,
					detail:  fmt.Sprintf("duplicate within project: %s", task.Title),
				})
			}
			localIDs[task.ID] = true

			// Check across projects
			if otherProject, exists := globalIDs[task.ID]; exists {
				results.duplicateIDs = append(results.duplicateIDs, doctorIssue{
					project: project.Name,
					detail:  fmt.Sprintf("also exists in project %s", otherProject),
				})
			} else {
				globalIDs[task.ID] = project.Name
			}
		}
	}
}

func checkDuplicateShortIDs(projects []*models.Project, results *doctorResults) {
	for _, project := range projects {
		seen := make(map[int]string) // shortID -> first task title
		for _, task := range project.Tasks {
			if first, exists := seen[task.ShortID]; exists {
				results.duplicateShortIDs = append(results.duplicateShortIDs, doctorIssue{
					project: project.Name,
					detail:  fmt.Sprintf("short ID #%d used by %q and %q", task.ShortID, first, task.Title),
				})
			} else {
				seen[task.ShortID] = task.Title
			}
		}
	}
}

func checkBrokenParentRefs(projects []*models.Project, results *doctorResults) {
	for _, project := range projects {
		taskMap := buildTaskMap(project)
		for _, task := range project.Tasks {
			if task.ParentID != nil && *task.ParentID != "" {
				if _, exists := taskMap[*task.ParentID]; !exists {
					results.brokenParentRefs = append(results.brokenParentRefs, doctorIssue{
						project: project.Name,
						detail:  fmt.Sprintf("parent %s not found (task: %s)", *task.ParentID, task.Title),
					})
				}
			}
		}
	}
}

// checkTaskReferences audits the stored soft task references of every loaded
// project against the whole catalog. It reports malformed locators, repeated
// locators on one task, a task naming itself, a locator whose project the
// catalog does not know, and a locator whose project is loaded but does not own
// the task.
//
// A catalog-known project this run could not load is reported as an
// informational note, never a broken reference: the target may well exist and be
// perfectly addressable, and calling it dead would invite exactly the silent
// pruning this check exists to prevent. Nothing here is fixable — an externally
// corrupted reference is repaired by the user, not deleted by a scan.
func checkTaskReferences(everyProject, loadedProjects []*models.Project, results *doctorResults) {
	cataloged := make(map[string]bool, len(everyProject))
	for _, project := range everyProject {
		if project != nil {
			cataloged[project.ID] = true
		}
	}
	loadedTasks := make(map[string]map[string]bool, len(loadedProjects))
	for _, project := range loadedProjects {
		if project == nil {
			continue
		}
		loadedTasks[project.ID] = buildTaskMapPresence(project)
	}

	for _, project := range loadedProjects {
		if project == nil {
			continue
		}
		for _, task := range project.Tasks {
			if task == nil || len(task.Dependencies) == 0 {
				continue
			}
			seen := make(map[models.TaskReference]bool, len(task.Dependencies))
			for _, reference := range task.Dependencies {
				detail := taskReferenceDefect(project, task, reference, seen, cataloged, loadedTasks)
				switch {
				case detail == "":
				case cataloged[reference.ProjectID] && loadedTasks[reference.ProjectID] == nil:
					results.unavailableTaskRefs = append(results.unavailableTaskRefs, doctorIssue{
						project: project.Name,
						detail:  detail,
					})
				default:
					results.brokenTaskRefs = append(results.brokenTaskRefs, doctorIssue{
						project: project.Name,
						detail:  detail,
					})
				}
				seen[reference] = true
			}
		}
	}
}

// taskReferenceDefect describes what is wrong with one stored reference, or ""
// when it resolves cleanly. The order of the tests is the order of severity: a
// locator that cannot be parsed is never also measured against the workspace.
func taskReferenceDefect(
	project *models.Project,
	task *models.Task,
	reference models.TaskReference,
	seen map[models.TaskReference]bool,
	cataloged map[string]bool,
	loadedTasks map[string]map[string]bool,
) string {
	if err := reference.Validate(); err != nil {
		return fmt.Sprintf("malformed reference on task %q: %v", task.Title, err)
	}
	if seen[reference] {
		return fmt.Sprintf("duplicate reference %s on task %q", reference.Locator(), task.Title)
	}
	if reference.ProjectID == project.ID && reference.TaskID == task.ID {
		return fmt.Sprintf("task %q references itself", task.Title)
	}
	if !cataloged[reference.ProjectID] {
		return fmt.Sprintf("reference %s on task %q names an unknown project", reference.Locator(), task.Title)
	}
	tasks, loaded := loadedTasks[reference.ProjectID]
	if !loaded {
		return fmt.Sprintf("reference %s on task %q is unavailable: its project did not load", reference.Locator(), task.Title)
	}
	if !tasks[reference.TaskID] {
		return fmt.Sprintf("reference %s on task %q names a task its project does not own", reference.Locator(), task.Title)
	}
	return ""
}

func buildTaskMapPresence(project *models.Project) map[string]bool {
	present := make(map[string]bool, len(project.Tasks))
	for _, task := range project.Tasks {
		if task != nil && task.ID != "" {
			present[task.ID] = true
		}
	}
	return present
}

// checkCompletionTimestamps surfaces records that completion-age cleanup must
// exclude. Terminal tasks require a non-zero CompletedAt, while nonterminal
// tasks must not retain one; doctor reports both cases rather than inventing an
// age or silently repairing lifecycle history.
func checkCompletionTimestamps(projects []*models.Project, results *doctorResults) {
	for _, project := range projects {
		for _, task := range project.Tasks {
			terminal := task.IsTerminalState()
			switch {
			case terminal && (task.CompletedAt == nil || task.CompletedAt.IsZero()):
				results.invalidCompletionTimestamps = append(results.invalidCompletionTimestamps, doctorIssue{
					project: project.Name,
					detail:  fmt.Sprintf("terminal task #%d %q has no valid completed_at", task.ShortID, task.Title),
				})
			case !terminal && task.CompletedAt != nil:
				results.invalidCompletionTimestamps = append(results.invalidCompletionTimestamps, doctorIssue{
					project: project.Name,
					detail:  fmt.Sprintf("nonterminal task #%d %q retains completed_at", task.ShortID, task.Title),
				})
			}
		}
	}
}

func fixIssues(projects []*models.Project) {
	dirtyProjects := make(map[string]bool)

	for _, project := range projects {
		// Fix duplicate short IDs by reassigning duplicates
		seenShortIDs := make(map[int]bool)
		for _, task := range project.Tasks {
			if seenShortIDs[task.ShortID] {
				newID := project.GetNextShortID()
				fmt.Printf("  Fixed: reassigned duplicate short ID #%d to #%d for task %s in %s\n", task.ShortID, newID, task.Title, project.Name)
				task.ShortID = newID
				dirtyProjects[project.ID] = true
			}
			seenShortIDs[task.ShortID] = true
		}

		taskMap := buildTaskMap(project)
		for _, task := range project.Tasks {
			// Fix broken parent refs
			if task.ParentID != nil && *task.ParentID != "" {
				if _, exists := taskMap[*task.ParentID]; !exists {
					fmt.Printf("  Fixed: cleared broken parent ref %s from task %s in %s\n", *task.ParentID, task.Title, project.Name)
					task.ParentID = nil
					dirtyProjects[project.ID] = true
				}
			}
		}

		if dirtyProjects[project.ID] {
			if err := storage.SaveProjectLocked(project); err != nil {
				fmt.Fprintf(os.Stderr, "error: failed to save project %s: %v\n", project.Name, err)
			}
		}
	}
}

func buildTaskMap(project *models.Project) map[string]*models.Task {
	m := make(map[string]*models.Task, len(project.Tasks))
	for _, task := range project.Tasks {
		m[task.ID] = task
	}
	return m
}

func printResults(results *doctorResults) {
	printInfoSection("Workspace status", results.workspaceNotes)
	printSection("Workspace integrity issues", results.workspaceIssues)
	printSection("Project load failures", results.loadFailures)
	printSection("Missing task files", results.missingTaskFiles)
	printSection("Duplicate task IDs", results.duplicateIDs)
	printSection("Duplicate short IDs", results.duplicateShortIDs)
	printSection("Broken parent refs", results.brokenParentRefs)
	printSection("Invalid completion timestamps", results.invalidCompletionTimestamps)
	printSection("Broken task references", results.brokenTaskRefs)
	printInfoSection("Unavailable task references", results.unavailableTaskRefs)
	printInfoSection("Empty projects", results.emptyProjects)
}

func printSection(name string, issues []doctorIssue) {
	fmt.Printf("\n%s: %d\n", name, len(issues))
	for _, issue := range issues {
		fmt.Printf("  [%s] %s\n", issue.project, issue.detail)
	}
}

func printInfoSection(name string, issues []doctorIssue) {
	fmt.Printf("\n%s (info): %d\n", name, len(issues))
	for _, issue := range issues {
		if issue.detail == "" {
			fmt.Printf("  %s\n", issue.project)
			continue
		}
		fmt.Printf("  [%s] %s\n", issue.project, issue.detail)
	}
}
