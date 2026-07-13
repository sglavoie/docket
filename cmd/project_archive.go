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

	project, cfg, configManager, err := LoadConfigAndProject(args[0])
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

	if project.Archived {
		return fmt.Errorf("project '%s' is already archived", project.Name)
	}

	project.Archive()

	// Clear current project if it was the archived one. Legacy configs may
	// have stored the slug instead of the ID, so match either.
	wasCurrentProject := cfg.CurrentProject == project.ID ||
		(project.Slug != "" && cfg.CurrentProject == project.Slug)
	if wasCurrentProject {
		cfg.CurrentProject = ""
	}

	// Clean up config references (FocusQueue, DailyPlan, RecentTaskVisits)
	// for tasks in the archived project.
	if err := storage.LoadProject(project); err == nil {
		taskIDs := make(map[string]bool, len(project.Tasks))
		for _, t := range project.Tasks {
			taskIDs[t.ID] = true
		}
		cfg.RemoveTaskRefs(taskIDs)
	}

	// Archiving a project changes only catalog/config state (the archived flag, the
	// current-project pointer, the archived project's planning references), so its
	// whole durable effect is one config rewrite committed through the application
	// layer's journaled transaction boundary.
	if err := commitConfigChange(cfg, configManager); err != nil {
		return err
	}

	return out.OutputProject(project, func() {
		fmt.Println(successMsg("Archived project '%s'", project.Name))
		if wasCurrentProject {
			fmt.Printf("Note: '%s' was the current project. Use 'tt select' to choose another.\n", project.Name)
		}
	})
}

// commitConfigChange encodes the current config and commits it as one atomic
// journaled transaction through the application layer, the config-only counterpart
// of routing a task mutation through the app service. It is used by the project
// archive/unarchive commands, whose only durable effect is a config rewrite.
func commitConfigChange(cfg *config.Config, configManager *config.Manager) error {
	arts, err := encodeConfigArtifacts(configManager, cfg)
	if err != nil {
		return err
	}
	if err := translateAppError(newAppService(cfg).CommitConfig(arts...)); err != nil {
		return err
	}
	// The archive's machine-local effects — the cleared current-project pointer and
	// the archived project's recent-visit cleanup — cannot ride the workspace
	// transaction because machine.json lives outside the commit service's approved
	// root. Persist them with a separate domain save after the synced commit.
	return saveConfig(configManager, cfg)
}

func runProjectUnarchive(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return tterrors.NewRequiredFieldError("project slug")
	}

	cfg, configManager, err := LoadConfig()
	if err != nil {
		return err
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	var project *models.Project
	// Search in all projects including archived
	for _, p := range cfg.Projects {
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

	if err := commitConfigChange(cfg, configManager); err != nil {
		return err
	}

	return out.OutputProject(project, func() { fmt.Println(successMsg("Unarchived project '%s'", project.Name)) })
}
