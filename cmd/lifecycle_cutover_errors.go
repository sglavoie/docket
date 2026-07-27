package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func validateRootArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 0 && (args[0] == "section" || args[0] == "sec") {
		return fmt.Errorf("tt %s was removed; use tt task list --state <state>", args[0])
	}
	return cobra.NoArgs(cmd, args)
}

func validateTaskArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		replacement := map[string]string{
			"block":   "wait",
			"standby": "pause",
			"maybe":   "someday",
		}[args[0]]
		if replacement != "" {
			return fmt.Errorf("tt task %s was removed; use tt task %s <ref...>", args[0], replacement)
		}
	}
	return cobra.NoArgs(cmd, args)
}

func lifecycleFlagError(_ *cobra.Command, err error) error {
	switch {
	case strings.Contains(err.Error(), "unknown flag: --status"):
		return fmt.Errorf("--status was removed; use --state <state>")
	case strings.Contains(err.Error(), "unknown flag: --section"):
		return fmt.Errorf("--section was removed; use --state <state>")
	default:
		return err
	}
}

func retiredStateReplacement(value string) (string, bool) {
	replacements := map[string]string{
		"todo":      "inbox, ready, or next",
		"active":    "ready",
		"start":     "in_progress",
		"started":   "in_progress",
		"progress":  "in_progress",
		"blocked":   "waiting",
		"standby":   "paused",
		"hold":      "paused",
		"maybe":     "someday",
		"complete":  "done",
		"completed": "done",
		"cancel":    "canceled",
		"cancelled": "canceled", //nolint:misspell // Intentional rejected-input alias.
	}
	replacement, ok := replacements[strings.ToLower(strings.TrimSpace(value))]
	return replacement, ok
}
