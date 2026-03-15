package cmd

import "github.com/spf13/cobra"

// Shell completion values for enumerated flags.
var (
	taskTypeCompletions   = []string{"epic", "story", "task", "bug", "idea"}
	statusCompletions     = []string{"todo", "in_progress", "review", "standby", "done", "blocked", "canceled"}
	sortCompletions       = []string{"priority", "created", "updated", "status", "title"}
	categoryCompletions   = []string{"bug", "feature", "process"}
	contextCompletions    = []string{"personal", "work"}
	statusIconCompletions = []string{"planning", "wip", "active", "review", "testing", "stabilizing", "maintenance", "bugfix", "paused", "archived"}
)

// completeEnum returns a completion function for a fixed set of valid values.
func completeEnum(values []string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return values, cobra.ShellCompDirectiveNoFileComp
	}
}

// completeProjectSections returns a completion function that loads the current project's sections.
func completeProjectSections() func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		project, _, _, err := LoadCurrentProject()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return project.GetSections(), cobra.ShellCompDirectiveNoFileComp
	}
}

// completeConfigStages returns a completion function that loads workflow stages from config.
func completeConfigStages() func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		cfg, _, err := LoadConfig()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return cfg.Stages, cobra.ShellCompDirectiveNoFileComp
	}
}
