package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

var archiveForce bool
var archiveAll bool

var taskArchiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Archive completed tasks",
	Long: `Move completed tasks from tasks.md to completed.md.

Completed tasks are appended to the archive file and removed from the main tasks file.
When the archive file exceeds 5MB, it is rotated (completed.md -> completed.1.md, etc.).

Use --force to skip the confirmation prompt.

Examples:
  tt task archive          # Archive with confirmation
  tt task archive --force  # Archive without confirmation
  tt task archive --all    # Archive completed tasks across all projects
  tt task archive list     # List archived task summaries
  tt task archive show ID  # Show an archived task`,
	RunE: runTaskArchive,
}

func init() {
	taskCmd.AddCommand(taskArchiveCmd)
	taskArchiveCmd.Flags().BoolVarP(&archiveForce, "force", "f", false, "Skip confirmation prompt")
	taskArchiveCmd.Flags().BoolVar(&archiveAll, "all", false, "Archive completed tasks across all projects")
	addOutputFlags(taskArchiveCmd)
}

func runTaskArchive(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	if archiveAll {
		return runTaskArchiveAll(out)
	}

	project, workspace, err := LoadRuntimeCurrentProject()
	if err != nil {
		return err
	}
	domains, err := workspace.Runtime.CloneDomains()
	if err != nil {
		return err
	}

	completedTasks := project.GetCompletedTasks()
	if len(completedTasks) == 0 {
		if out.JSON {
			entry := struct {
				ArchivedCount int    `json:"archived_count"`
				Project       string `json:"project"`
			}{ArchivedCount: 0, Project: project.Name}
			return printJSON(entry)
		}
		if !out.Quiet {
			fmt.Println("No completed tasks to archive.")
		}
		return nil
	}

	// Confirmation unless --force
	if !ConfirmYN(fmt.Sprintf("Archive %s to completed.md? [y/N]: ", Pluralize(len(completedTasks), "completed task", "completed tasks")), archiveForce) {
		if !out.Quiet {
			fmt.Fprintf(os.Stderr, "Canceled\n")
		}
		return nil
	}

	// Capture the archived task IDs from the loaded project (stable UUIDs) so the
	// durable planning cleanup keys correctly regardless of the fresh snapshot the
	// use case loads.
	archivedIDs := make(map[string]bool, len(completedTasks))
	for _, t := range completedTasks {
		archivedIDs[t.ID] = true
	}
	archivedLocators := configTaskLocatorsForProject(project.ID, archivedIDs)

	// Clean the DURABLE planning refs (Focus Queue and Daily Plan) of every
	// archived task and fold them into the archive transaction as a config artifact,
	// so a partial archive can never leave planning refs pointing at an archived
	// task. Machine-local recent visits are repaired separately after the commit.
	var artifacts []storage.ArtifactChange
	if domains.Planning.RemoveDurableTaskLocators(archivedLocators) > 0 {
		arts, artErr := encodeSyncedDomainArtifacts(workspace.Runtime, domains)
		if artErr != nil {
			return artErr
		}
		artifacts = arts
	}

	// Route the archive through the application layer: active-task removal, archive
	// file rewrites/rotation, refreshed index, archived-task attachment retention,
	// and the durable planning artifact all commit as ONE transaction, revision-
	// checked and undoable through the workspace ledger.
	svc := newWorkspaceAppService(workspace)
	res, err := svc.ArchiveTasks(project.ID, artifacts...)
	if err != nil {
		return translateAppError(err)
	}

	// Repair the machine-local recent visits separately — non-authoritative UI state
	// updated only after the durable commit succeeded. A failure here is a warning,
	// not an error: the archive is already committed and recent visits are best-effort.
	if domains.Machine.RemoveMachineLocalTaskLocators(archivedLocators) > 0 {
		if saveErr := workspace.Runtime.CommitMachine(domains.Machine); saveErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to update recent visits: %v\n", saveErr)
		}
	}

	if out.JSON {
		entry := struct {
			ArchivedCount int    `json:"archived_count"`
			Project       string `json:"project"`
		}{ArchivedCount: res.ArchivedCount, Project: project.Name}
		return printJSON(entry)
	}
	if !out.Quiet {
		fmt.Printf("Archived %s to completed.md\n", Pluralize(res.ArchivedCount, "task", "tasks"))
	}
	return nil
}

func runTaskArchiveAll(out MutateOutput) error {
	workspace, err := LoadCLIWorkspaceStrict()
	if err != nil {
		return err
	}
	domains, err := workspace.Runtime.CloneDomains()
	if err != nil {
		return err
	}

	type archiveCandidate struct {
		Project        *models.Project
		CompletedCount int
		ArchivedIDs    map[string]bool
	}

	projects := workspace.Runtime.ActiveProjects()
	totalArchived := 0
	var failures []string
	var results []projectArchiveResult
	var candidates []archiveCandidate

	for _, project := range projects {
		if loadErr := storage.LoadProject(project); loadErr != nil {
			failures = append(failures, fmt.Sprintf("[%s] load failed: %v", project.Name, loadErr))
			continue
		}

		completedTasks := project.GetCompletedTasks()
		if len(completedTasks) == 0 {
			continue
		}

		if !ConfirmYN(fmt.Sprintf("[%s] Archive %s? [y/N]: ", project.Name, Pluralize(len(completedTasks), "completed task", "completed tasks")), archiveForce) {
			if !out.Quiet && !out.JSON {
				fmt.Printf("[%s] Skipped.\n", project.Name)
			}
			continue
		}

		archivedIDs := make(map[string]bool, len(completedTasks))
		for _, t := range completedTasks {
			archivedIDs[t.ID] = true
		}
		candidates = append(candidates, archiveCandidate{
			Project:        project,
			CompletedCount: len(completedTasks),
			ArchivedIDs:    archivedIDs,
		})
	}

	if len(candidates) == 0 {
		return emitArchiveAllOutput(out, totalArchived, results, failures)
	}

	// Aggregate the archived task locators across every confirmed candidate and clean
	// their DURABLE planning refs (Focus Queue and Daily Plan), folding them into
	// the archive-all transaction as a single config artifact so the cleanup commits
	// atomically with the whole batch. Machine-local recent visits are repaired
	// separately after the commit.
	var allArchivedLocators []config.ProjectTaskLocator
	projectIDs := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		projectIDs = append(projectIDs, candidate.Project.ID)
		allArchivedLocators = append(allArchivedLocators,
			configTaskLocatorsForProject(candidate.Project.ID, candidate.ArchivedIDs)...)
	}

	var artifacts []storage.ArtifactChange
	if domains.Planning.RemoveDurableTaskLocators(allArchivedLocators) > 0 {
		arts, artErr := encodeSyncedDomainArtifacts(workspace.Runtime, domains)
		if artErr != nil {
			return artErr
		}
		artifacts = arts
	}

	// Archive every candidate as ONE atomic, ledger-backed transaction: a single undo
	// reverses the whole batch and a failure leaves every project un-archived.
	svc := newWorkspaceAppService(workspace)
	res, err := svc.ArchiveAllTasks(projectIDs, artifacts...)
	if err != nil {
		return translateAppError(err)
	}

	for _, pr := range res.Projects {
		results = append(results, projectArchiveResult{Name: pr.ProjectName, Archived: pr.ArchivedCount})
		if !out.Quiet && !out.JSON {
			fmt.Printf("[%s] Archived %s\n", pr.ProjectName, Pluralize(pr.ArchivedCount, "task", "tasks"))
		}
	}
	totalArchived = res.TotalArchived

	// Repair machine-local recent visits once after the archive commit — best-effort,
	// warning-only, non-authoritative UI state.
	if domains.Machine.RemoveMachineLocalTaskLocators(allArchivedLocators) > 0 {
		if saveErr := workspace.Runtime.CommitMachine(domains.Machine); saveErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to update recent visits: %v\n", saveErr)
		}
	}

	return emitArchiveAllOutput(out, totalArchived, results, failures)
}

// projectArchiveResult is the per-project archive-all summary rendered under --json.
type projectArchiveResult struct {
	Name     string `json:"name"`
	Archived int    `json:"archived"`
}

// emitArchiveAllOutput renders the archive-all summary: a JSON envelope under
// --json, a per-project total otherwise, and a non-nil error when any project
// failed so the caller's exit status reflects the partial failure.
func emitArchiveAllOutput(out MutateOutput, totalArchived int, results []projectArchiveResult, failures []string) error {
	if out.JSON {
		if results == nil {
			results = []projectArchiveResult{}
		}
		summary := struct {
			TotalArchived int                    `json:"total_archived"`
			Projects      []projectArchiveResult `json:"projects"`
		}{TotalArchived: totalArchived, Projects: results}
		return printJSON(summary)
	}

	if !out.Quiet {
		fmt.Printf("\nTotal: archived %s across all projects\n", Pluralize(totalArchived, "task", "tasks"))
	}
	if len(failures) > 0 {
		fmt.Printf("\n%d project(s) had errors:\n", len(failures))
		for _, f := range failures {
			fmt.Printf("  %s\n", f)
		}
		return fmt.Errorf("%s had errors during archiving", Pluralize(len(failures), "project", "projects"))
	}
	return nil
}
