package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

var taskDeleteForce bool

var taskDeleteCmd = &cobra.Command{
	Use:   "delete [task-id]",
	Short: "Permanently delete a task",
	Long: `Permanently delete a task and all its descendants.

This removes the task from the project entirely. This cannot be undone
(unless undo history is enabled). Dangling references in DependsOn and
RelatedTo fields of other tasks are cleaned up automatically.

If no task ID is provided, an interactive selector is shown.

Examples:
  tt task delete 42
  tt task delete 42 --force
  tt t delete abc123`,
	RunE: runTaskDelete,
}

func init() {
	taskCmd.AddCommand(taskDeleteCmd)
	taskDeleteCmd.Flags().BoolVarP(&taskDeleteForce, "force", "f", false, "Skip confirmation prompt")
	addOutputFlags(taskDeleteCmd)
}

func runTaskDelete(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	project, workspace, err := LoadRuntimeCurrentProject()
	if err != nil {
		return err
	}
	domains, err := workspace.Runtime.CloneDomains()
	if err != nil {
		return err
	}

	// Select task: from args or interactive
	task, err := SelectTask(project, args, func(t *models.Task) bool {
		return true // all tasks are deletable
	}, "Select task to delete", "No tasks in this project.")
	if err != nil {
		return err
	}

	// Count descendants for the confirmation prompt. Task UUIDs are stable, so this
	// set — computed at the CLI edge from the loaded project — keys the durable
	// planning cleanup correctly regardless of the fresh snapshot the use case loads.
	idsToDelete := collectTaskDescendantIDs(project.Tasks, map[string]bool{task.ID: true})
	count := len(idsToDelete)

	prompt := fmt.Sprintf("Permanently delete task #%d '%s'?", task.ShortID, task.Title)
	if count > 1 {
		prompt = fmt.Sprintf("Permanently delete task #%d '%s' and %s?",
			task.ShortID, task.Title, Pluralize(count-1, "descendant", "descendants"))
	}

	var confirmed bool
	if count > 1 {
		// ConfirmStrict prints its own "Type 'yes' to confirm:" prompt
		confirmed = ConfirmStrict(prompt, taskDeleteForce)
	} else {
		confirmed = ConfirmYN(prompt+" [y/N] ", taskDeleteForce)
	}
	if !confirmed {
		if !out.Quiet {
			fmt.Fprintf(os.Stderr, "Canceled\n")
		}
		return nil
	}

	// Clean the DURABLE planning refs (Focus Queue, Daily Plan, dismissals) of every
	// deleted task and fold them into the delete transaction as a config artifact, so
	// a partial delete can never leave planning refs pointing at a removed task. The
	// machine-local recent visits are intentionally excluded here and repaired
	// separately after a successful commit.
	var artifacts []storage.ArtifactChange
	if domains.Planning.RemoveDurableTaskRefs(idsToDelete) > 0 {
		arts, artErr := encodeSyncedDomainArtifacts(workspace.Runtime, domains)
		if artErr != nil {
			return artErr
		}
		artifacts = arts
	}

	// Route the delete through the application layer: subtree removal, each removed
	// task's attachment bytes, same- and cross-project reference cleanup, and the
	// durable planning artifact all commit as ONE transaction, revision-checked and
	// undoable through the workspace ledger.
	svc := newWorkspaceAppService(workspace)
	if _, err := svc.Delete(app.DeleteRequest{TaskID: task.ID, Confirm: true}, artifacts...); err != nil {
		return translateAppError(err)
	}

	// Repair the machine-local recent visits separately — non-authoritative UI state
	// updated only after the durable commit succeeded. A failure here is a warning,
	// not an error: the delete is already committed and recent visits are best-effort.
	if domains.Machine.RemoveMachineLocalTaskRefs(idsToDelete) > 0 {
		if saveErr := workspace.Runtime.CommitMachine(domains.Machine); saveErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to update recent visits: %v\n", saveErr)
		}
	}

	if out.Quiet {
		return nil
	}
	if out.JSON {
		return outputSingleTaskJSON(task)
	}
	msg := TaskMsg(project.Name, "Deleted", task)
	if count > 1 {
		msg += fmt.Sprintf(" and %s", Pluralize(count-1, "descendant", "descendants"))
	}
	fmt.Println(msg)
	return nil
}

// collectTaskDescendantIDs collects IDs of root tasks and all their descendants.
// This is a cmd-level version of the TUI's collectDescendantIDs.
func collectTaskDescendantIDs(tasks []*models.Task, rootIDs map[string]bool) map[string]bool {
	idsToDelete := make(map[string]bool)
	var collectIDs func(t *models.Task)
	collectIDs = func(t *models.Task) {
		idsToDelete[t.ID] = true
		for _, child := range t.Children {
			collectIDs(child)
		}
	}
	for _, t := range tasks {
		if rootIDs[t.ID] {
			collectIDs(t)
		}
	}
	return idsToDelete
}
