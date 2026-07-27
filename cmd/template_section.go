package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

// parseCanonicalWorkflowStateInput validates one public canonical-state value
// and returns the normalized model enum. Staged CLI commands share this helper
// so creation, editing, templates, and archive filters reject the same legacy or
// misspelled vocabulary consistently.
func parseCanonicalWorkflowStateInput(field, value string) (models.WorkflowState, error) {
	value = strings.TrimSpace(value)
	state, ok := models.ParseWorkflowState(value)
	if !ok {
		if replacement, retired := retiredStateReplacement(value); retired {
			return "", fmt.Errorf("%w: %s %q was removed; use %s", tterrors.ErrInvalidValue, field, value, replacement)
		}
		return "", fmt.Errorf("%w: invalid %s %q (valid: %s)", tterrors.ErrInvalidValue, field, value, strings.Join(stateCompletions, ", "))
	}
	return state, nil
}

// stagedLifecycleFlagProvided reports whether a canonical lifecycle flag was
// explicitly supplied. The non-empty value fallback keeps direct handler tests
// and adapter callers honest even when they do not execute Cobra's flag parser.
func stagedLifecycleFlagProvided(cmd *cobra.Command, name, value string) bool {
	return (cmd != nil && cmd.Flags().Changed(name)) || strings.TrimSpace(value) != ""
}
