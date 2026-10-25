package projecthub

import (
	"testing"

	"github.com/sglavoie/docket/internal/tui/operation"
)

// TestProjectHubOperationsCannotAppearInBothDirectGrammarAndMenu holds the Hub
// to the one-path rule the startup gate enforces: the six operations the browse
// grammar reaches on press one are absent from the Space menu.
func TestProjectHubOperationsCannotAppearInBothDirectGrammarAndMenu(t *testing.T) {
	if err := operation.ValidateOperationOnePath(ProjectOperationManifest(), ProjectHubInputStrategy()); err != nil {
		t.Fatalf("ValidateOperationOnePath() = %v, want nil", err)
	}
}
