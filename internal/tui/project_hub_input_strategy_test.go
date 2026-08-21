package tui

import "testing"

// TestProjectHubOperationsCannotAppearInBothDirectGrammarAndMenu holds the Hub
// to the one-path rule the startup gate enforces: the six operations the browse
// grammar reaches on press one are absent from the Space menu.
func TestProjectHubOperationsCannotAppearInBothDirectGrammarAndMenu(t *testing.T) {
	if err := validateOperationOnePath(projectOperationManifest(), ProjectHubInputStrategy()); err != nil {
		t.Fatalf("validateOperationOnePath() = %v, want nil", err)
	}
}
