package cmd

import "github.com/spf13/cobra"

// Shell completion values for enumerated flags.
var (
	taskTypeCompletions = []string{"epic", "story", "task", "bug", "idea"}
	statusCompletions   = []string{"todo", "in_progress", "review", "standby", "done", "blocked", "canceled"}
	sortCompletions     = []string{"priority", "created", "updated", "status", "title"}
	categoryCompletions = []string{"bug", "feature", "process"}
)

// completeEnum returns a completion function for a fixed set of valid values.
func completeEnum(values []string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return values, cobra.ShellCompDirectiveNoFileComp
	}
}
