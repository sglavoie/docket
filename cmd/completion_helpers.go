package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// Shell completion values for enumerated flags.
var (
	taskTypeCompletions   = []string{"epic", "story", "task", "bug", "idea"}
	statusCompletions     = []string{"todo", "in_progress", "review", "standby", "maybe", "done", "blocked", "canceled"}
	stateCompletions      = []string{"inbox", "ready", "next", "in_progress", "waiting", "paused", "review", "someday", "done", "canceled"}
	sortCompletions       = []string{"priority", "created", "updated", "status", "title"}
	categoryCompletions   = []string{"bug", "feature", "process"}
	statusIconCompletions = []string{"planning", "wip", "active", "review", "testing", "stabilizing", "maintenance", "bugfix", "paused", "archived"}
)

// builtinContexts are the default context values always available.
var builtinContexts = []string{"personal", "work"}

// completeEnum returns a completion function for a fixed set of valid values.
func completeEnum(values []string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return values, cobra.ShellCompDirectiveNoFileComp
	}
}

// completeProjectSections returns a completion function that loads the current project's sections.
func completeProjectSections() func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		project, _, err := LoadRuntimeCurrentProject()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return project.GetSections(), cobra.ShellCompDirectiveNoFileComp
	}
}

// completeContexts returns a completion function that combines built-in contexts
// with any custom contexts defined in the user's config.
func completeContexts() func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		workspace, err := LoadCLIWorkspace()
		if err != nil {
			return builtinContexts, cobra.ShellCompDirectiveNoFileComp
		}
		custom := workspace.Runtime.Machine().GetCustomContextNames()
		all := make([]string, 0, len(builtinContexts)+len(custom))
		all = append(all, builtinContexts...)
		all = append(all, custom...)
		return all, cobra.ShellCompDirectiveNoFileComp
	}
}

// isKnownContext reports whether ctx matches a built-in or custom context name.
// customNames is the sorted custom-context list loaded from the machine domain.
func isKnownContext(ctx string, customNames []string) bool {
	for _, b := range builtinContexts {
		if strings.EqualFold(ctx, b) {
			return true
		}
	}
	for _, c := range customNames {
		if strings.EqualFold(ctx, c) {
			return true
		}
	}
	return false
}

// knownContextList returns a human-readable comma-joined list of all known
// contexts, given the custom-context names loaded from the machine domain.
func knownContextList(customNames []string) string {
	all := make([]string, 0, len(builtinContexts)+len(customNames))
	all = append(all, builtinContexts...)
	all = append(all, customNames...)
	return strings.Join(all, ", ")
}

// completeCurrentProjectTaskIDs returns a completion function that lists task short IDs from the current project.
func completeCurrentProjectTaskIDs() func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		project, _, err := LoadRuntimeCurrentProject()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		completions := make([]string, 0, len(project.Tasks))
		for _, t := range project.Tasks {
			completions = append(completions, fmt.Sprintf("%d\t%s", t.ShortID, t.Title))
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}
}

// completeProjectSlugs returns a completion function that lists active project slugs.
func completeProjectSlugs() func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		workspace, err := LoadCLIWorkspace()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		projects := workspace.Runtime.ActiveProjects()
		var slugs []string
		for _, p := range projects {
			slugs = append(slugs, p.Slug)
		}
		return slugs, cobra.ShellCompDirectiveNoFileComp
	}
}

// completeTemplates returns a completion function that lists task template IDs and names.
func completeTemplates() func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		workspace, err := LoadCLIWorkspace()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		templates := workspace.Runtime.Automation().GetTaskTemplates()
		var completions []string
		for _, t := range templates {
			completions = append(completions, t.ID+"\t"+t.Name)
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}
}
