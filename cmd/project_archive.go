package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tterrors"
)

var projectArchiveCmd = &cobra.Command{
	Use:   "archive <slug-or-id>",
	Short: "Archive a project",
	Long: `Archive a project to hide it from the active list.

Archived projects are not deleted and can be unarchived later.

Examples:
  tt project archive old-project
  tt p archive my-project`,
	RunE: runProjectArchive,
}

var projectUnarchiveCmd = &cobra.Command{
	Use:   "unarchive <slug-or-id>",
	Short: "Unarchive a project",
	Long: `Restore an archived project to the active list.

Examples:
  tt project unarchive old-project
  tt p unarchive my-project`,
	RunE: runProjectUnarchive,
}

func init() {
	projectCmd.AddCommand(projectArchiveCmd)
	projectCmd.AddCommand(projectUnarchiveCmd)
	addOutputFlags(projectArchiveCmd)
	addOutputFlags(projectUnarchiveCmd)
}

func runProjectArchive(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("project slug")
	}

	project, workspace, err := LoadRuntimeProject(args[0])
	if err != nil {
		return err
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	if project.Sandbox {
		return fmt.Errorf("cannot archive the sandbox project")
	}
	if workspace.Runtime.IsSystemInboxProject(project.ID) {
		return fmt.Errorf("cannot archive the system Inbox project")
	}

	if project.Archived {
		return fmt.Errorf("project '%s' is already archived", project.Name)
	}

	project.Archive()
	domains, err := workspace.Runtime.CloneDomains()
	if err != nil {
		return err
	}
	domains.Catalog.UpsertProject(project)

	// Clear current project if it was the archived one. Legacy configs may
	// have stored the slug instead of the ID, so match either.
	wasCurrentProject := domains.Machine.CurrentProject == project.ID ||
		(project.Slug != "" && domains.Machine.CurrentProject == project.Slug)
	if wasCurrentProject {
		domains.Machine.CurrentProject = ""
	}

	// Clean up config references (FocusQueue, DailyPlan, RecentTaskVisits)
	// for tasks in the archived project.
	if err := storage.LoadProject(project); err == nil {
		taskIDs := make(map[string]bool, len(project.Tasks))
		for _, t := range project.Tasks {
			taskIDs[t.ID] = true
		}
		domains.Planning.RemoveDurableTaskRefs(taskIDs)
		domains.Machine.RemoveMachineLocalTaskRefs(taskIDs)
	}

	// Archiving a project changes only catalog/config state (the archived flag, the
	// current-project pointer, the archived project's planning references), so its
	// whole durable effect is one config rewrite committed through the application
	// layer's journaled transaction boundary.
	if err := commitProjectDomainChange(workspace, domains); err != nil {
		return err
	}

	return out.OutputProject(project, func() {
		fmt.Println(successMsg("Archived project '%s'", project.Name))
		if wasCurrentProject {
			fmt.Printf("Note: '%s' was the current project. Use 'tt select' to choose another.\n", project.Name)
		}
	})
}

// commitProjectDomainChange commits the four synced domains atomically, then
// persists machine-local navigation/history state separately because
// machine.json lives outside the workspace transaction root.
func commitProjectDomainChange(workspace *CLIWorkspace, domains *config.Domains) error {
	if err := commitSyncedDomains(workspace, domains); err != nil {
		return err
	}
	return workspace.Runtime.CommitMachine(domains.Machine)
}

func runProjectUnarchive(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("project slug")
	}

	workspace, err := LoadCLIWorkspace()
	if err != nil {
		return err
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	var project *models.Project
	// Search in all projects including archived
	for _, p := range workspace.Runtime.AllProjects() {
		if p.ID == args[0] || p.Slug == args[0] || p.Name == args[0] {
			project = p
			break
		}
	}

	if project == nil {
		return tterrors.NewProjectNotFoundError(args[0])
	}

	if !project.Archived {
		return fmt.Errorf("project '%s' is not archived", project.Name)
	}

	project.Unarchive()

	if err := commitProjectCatalog(workspace, project); err != nil {
		return err
	}

	return out.OutputProject(project, func() { fmt.Println(successMsg("Unarchived project '%s'", project.Name)) })
}
