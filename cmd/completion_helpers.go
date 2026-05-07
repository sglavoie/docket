package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/config"
)

// Shell completion values for enumerated flags.
var (
	taskTypeCompletions   = []string{"epic", "story", "task", "bug", "idea"}
	statusCompletions     = []string{"todo", "in_progress", "review", "standby", "maybe", "done", "blocked", "canceled"}
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
		project, _, _, err := LoadCurrentProject()
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
		cfg, _, err := LoadConfig()
		if err != nil {
			return builtinContexts, cobra.ShellCompDirectiveNoFileComp
		}
		custom := cfg.GetCustomContextNames()
		all := make([]string, 0, len(builtinContexts)+len(custom))
		all = append(all, builtinContexts...)
		all = append(all, custom...)
		return all, cobra.ShellCompDirectiveNoFileComp
	}
}

// isKnownContext reports whether ctx matches a built-in or custom context name.
func isKnownContext(ctx string, cfg *config.Config) bool {
	for _, b := range builtinContexts {
		if strings.EqualFold(ctx, b) {
			return true
		}
	}
	for _, c := range cfg.GetCustomContextNames() {
		if strings.EqualFold(ctx, c) {
			return true
		}
	}
	return false
}

// knownContextList returns a human-readable comma-joined list of all known contexts.
func knownContextList(cfg *config.Config) string {
	all := make([]string, 0, len(builtinContexts)+len(cfg.GetCustomContextNames()))
	all = append(all, builtinContexts...)
	all = append(all, cfg.GetCustomContextNames()...)
	return strings.Join(all, ", ")
}

// completeCurrentProjectTaskIDs returns a completion function that lists task short IDs from the current project.
func completeCurrentProjectTaskIDs() func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		project, _, _, err := LoadCurrentProject()
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
		cfg, _, err := LoadConfig()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var slugs []string
		for _, p := range cfg.GetActiveProjects() {
			slugs = append(slugs, p.Slug)
		}
		return slugs, cobra.ShellCompDirectiveNoFileComp
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

// completeTemplates returns a completion function that lists task template IDs and names.
func completeTemplates() func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		cfg, _, err := LoadConfig()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		var completions []string
		for _, t := range cfg.TaskTemplates {
			completions = append(completions, t.ID+"\t"+t.Name)
		}
		return completions, cobra.ShellCompDirectiveNoFileComp
	}
}
