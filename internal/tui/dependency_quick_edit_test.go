package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/grammar"
)

func TestQuickEditDependencyPatchHalves(t *testing.T) {
	home, routed := dependencyWorkspace()
	projects := []*models.Project{home, routed}
	task := home.Tasks[0]
	task.Dependencies = []models.TaskReference{
		{ProjectID: "home-project", TaskID: "home-3"},
		{ProjectID: "gone-project", TaskID: "gone-9"},
	}

	tests := []struct {
		name       string
		input      string
		wantAdd    []string
		wantRemove []string
		wantClear  bool
	}{
		{name: "no token preserves the stored set", input: "Edited task"},
		{name: "addition", input: "Edited task ~2", wantAdd: []string{"home-project:home-2"}},
		{
			name:       "removal of a closed target",
			input:      "Edited task -~3",
			wantRemove: []string{"home-project:home-3"},
		},
		{
			name:       "removal of an unavailable target by stable locator",
			input:      "Edited task -~gone-project:gone-9",
			wantRemove: []string{"gone-project:gone-9"},
		},
		{
			name:       "one line adds and removes",
			input:      "Edited task -~3 ~routed:1",
			wantAdd:    []string{"routed-project:routed-1"},
			wantRemove: []string{"home-project:home-3"},
		},
		{name: "clear", input: "Edited task ~0", wantClear: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := grammar.ParseQuickEdit(tt.input, task, home, projects)
			if err != nil {
				t.Fatalf("parseQuickEdit(%q): %v", tt.input, err)
			}
			patch := result.DependencyPatch()
			if got := dependencyLocatorStrings(patch.Add); !slices.Equal(got, tt.wantAdd) {
				t.Errorf("add = %v, want %v", got, tt.wantAdd)
			}
			if got := dependencyLocatorStrings(patch.Remove); !slices.Equal(got, tt.wantRemove) {
				t.Errorf("remove = %v, want %v", got, tt.wantRemove)
			}
			if patch.Clear != tt.wantClear {
				t.Errorf("clear = %v, want %v", patch.Clear, tt.wantClear)
			}
			if tt.wantAdd == nil && tt.wantRemove == nil && !tt.wantClear && !patch.IsZero() {
				t.Errorf("a line naming no dependency produced %+v, want a zero patch", patch)
			}
		})
	}
}

func TestQuickEditDependencyDiagnostics(t *testing.T) {
	home, routed := dependencyWorkspace()
	projects := []*models.Project{home, routed}
	task := home.Tasks[0]

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"clear with an addition", "Edited task ~0 ~2", "clearing dependencies cannot be combined"},
		{"clear with a removal", "Edited task ~0 -~2", "clearing dependencies cannot be combined"},
		{"adding and removing one target", "Edited task ~2 -~2", "conflicting dependency patch"},
		{"self reference", "Edited task ~1", "a task cannot depend on itself"},
		{"terminal addition", "Edited task ~3", "cannot depend on"},
		{"unresolved addition", "Edited task ~999", `dependency not found: "999"`},
		{"ambiguous addition", "Edited task ~Ship", "ambiguous"},
		{"removal is not a clear", "Edited task -~0", "is not a clear form"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := grammar.ParseQuickEdit(tt.input, task, home, projects)
			if err == nil {
				t.Fatalf("parseQuickEdit(%q) accepted the line", tt.input)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want it to contain %q", err, tt.want)
			}
		})
	}
}

// TestQuickEditDependencyPreviewShowsTheResultingSet checks the preview a
// composer paints: the stored set with the patch already applied.
func TestQuickEditDependencyPreviewShowsTheResultingSet(t *testing.T) {
	home, routed := dependencyWorkspace()
	projects := []*models.Project{home, routed}
	task := home.Tasks[0]
	task.Dependencies = []models.TaskReference{{ProjectID: "home-project", TaskID: "home-3"}}

	previewLocators := func(input string) []string {
		t.Helper()
		result, err := grammar.ParseQuickEdit(input, task, home, projects)
		if err != nil {
			t.Fatalf("parseQuickEdit(%q): %v", input, err)
		}
		values := make([]string, len(result.Preview.Dependencies))
		for i, reference := range result.Preview.Dependencies {
			values[i] = reference.Locator()
		}
		return values
	}

	if got := previewLocators("Edited task"); !slices.Equal(got, []string{"home-project:home-3"}) {
		t.Errorf("untouched preview = %v, want the stored set", got)
	}
	if got := previewLocators("Edited task ~2"); !slices.Equal(got, []string{"home-project:home-3", "home-project:home-2"}) {
		t.Errorf("addition preview = %v, want the addition appended", got)
	}
	if got := previewLocators("Edited task -~3"); len(got) != 0 {
		t.Errorf("removal preview = %v, want an empty set", got)
	}
	if got := previewLocators("Edited task ~0"); len(got) != 0 {
		t.Errorf("clear preview = %v, want an empty set", got)
	}
	if len(task.Dependencies) != 1 {
		t.Fatalf("preview mutated the live task: %v", task.Dependencies)
	}
}

// TestQuickEditAddExistingTerminalOrUnavailableIsNoOp holds the inline editor to
// the same existing-versus-new distinction the application layer applies. A
// stored reference survives its target closing or its project leaving the loaded
// set, so naming it again with ~ must parse, preview as the unchanged set, and
// reach commit as a request the resolver also treats as a no-op. Only a target
// the task does not already store is measured against the addition rules.
func TestQuickEditAddExistingTerminalOrUnavailableIsNoOp(t *testing.T) {
	home, routed := dependencyWorkspace()
	projects := []*models.Project{home, routed}
	task := home.Tasks[0]
	stored := []models.TaskReference{
		{ProjectID: "home-project", TaskID: "home-3"},
		{ProjectID: "gone-project", TaskID: "gone-9"},
	}
	task.Dependencies = models.CloneTaskReferences(stored)

	accepted := []struct {
		name  string
		input string
		want  string
	}{
		{"stored target that has since closed", "Edited task ~3", "home-project:home-3"},
		{"stored target whose project is no longer loaded", "Edited task ~gone-project:gone-9", "gone-project:gone-9"},
	}
	for _, tt := range accepted {
		t.Run(tt.name, func(t *testing.T) {
			result, err := grammar.ParseQuickEdit(tt.input, task, home, projects)
			if err != nil {
				t.Fatalf("parseQuickEdit(%q): %v", tt.input, err)
			}
			patch := result.DependencyPatch()
			if got := dependencyLocatorStrings(patch.Add); !slices.Equal(got, []string{tt.want}) {
				t.Errorf("add = %v, want %v", got, []string{tt.want})
			}
			if len(patch.Remove) != 0 || patch.Clear {
				t.Errorf("patch = %+v, want additions alone", patch)
			}
			preview := make([]string, len(result.Preview.Dependencies))
			for i, reference := range result.Preview.Dependencies {
				preview[i] = reference.Locator()
			}
			if !slices.Equal(preview, []string{"home-project:home-3", "gone-project:gone-9"}) {
				t.Errorf("preview = %v, want the stored set unchanged", preview)
			}
		})
	}

	// A target the task does not already store is still judged as a new addition.
	bare := home.Tasks[3]
	bare.Dependencies = nil
	refused := []struct {
		name  string
		input string
		want  string
	}{
		{"new terminal target", "Shipping receipts ~3", "cannot depend on"},
		{"new unavailable target", "Shipping receipts ~gone-project:gone-8", `dependency not found: "gone-project:gone-8"`},
	}
	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := grammar.ParseQuickEdit(tt.input, bare, home, projects); err == nil {
				t.Fatalf("parseQuickEdit(%q) accepted the line", tt.input)
			} else if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want it to contain %q", err, tt.want)
			}
		})
	}
}
