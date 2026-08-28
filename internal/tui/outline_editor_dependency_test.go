package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// outlineDependencyWorkspace is the dumped project plus a second loaded project,
// so a cross-project reference has somewhere real to point.
func outlineDependencyWorkspace(t *testing.T) (*models.Project, []*models.Project) {
	t.Helper()
	scope := outlineTestProject(t)
	other := &models.Project{ID: "project-2", Name: "Radar", Slug: "radar"}
	task, err := models.NewGenericTask("aa11bb22cccc", 7, "Rotate signing keys")
	if err != nil {
		t.Fatalf("new task: %v", err)
	}
	task.ApplyWorkflowState(models.StateBacklog)
	other.Tasks = append(other.Tasks, task)
	return scope, []*models.Project{scope, other}
}

func outlineDependencyLine(t *testing.T, buffer, title string) string {
	t.Helper()
	for _, line := range strings.Split(buffer, "\n") {
		if strings.Contains(line, title) {
			return line
		}
	}
	t.Fatalf("buffer has no line for %q:\n%s", title, buffer)
	return ""
}

// TestOutlineDumpPrefersTheShortestAddressableAlias fixes the three forms one
// stored reference can print as, in the order the buffer prefers them.
func TestOutlineDumpPrefersTheShortestAddressableAlias(t *testing.T) {
	scope, projects := outlineDependencyWorkspace(t)
	scope.Tasks[0].Dependencies = []models.TaskReference{
		{ProjectID: "project-1", TaskID: "9b31de07bbbb"},
		{ProjectID: "project-2", TaskID: "aa11bb22cccc"},
		{ProjectID: "project-9", TaskID: "deadbeef0000"},
	}

	buffer, _, err := dumpOutlineBuffer(scope, projects, "All", time.Date(2026, 8, 10, 14, 2, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("dump: %v", err)
	}

	line := outlineDependencyLine(t, buffer, "Ship auth rewrite")
	for _, want := range []string{"~2", "~radar:7", "~project-9:deadbeef0000"} {
		if !strings.Contains(line, " "+want+" ") && !strings.HasSuffix(strings.TrimSpace(line), want) {
			t.Errorf("dumped line missing %q:\n%s", want, line)
		}
	}
}

// TestOutlineDumpPrintsUnavailableAndTerminalTargets keeps the dump complete: a
// reference the buffer withheld would be deleted by a user who never saw it.
func TestOutlineDumpPrintsUnavailableAndTerminalTargets(t *testing.T) {
	scope, projects := outlineDependencyWorkspace(t)
	scope.Tasks[3].ApplyWorkflowState(models.StateDone)
	scope.Tasks[0].Dependencies = []models.TaskReference{
		{ProjectID: "project-1", TaskID: "71f0b3d5dddd"},
		{ProjectID: "project-9", TaskID: "gone"},
	}

	buffer, _, err := dumpOutlineBuffer(scope, projects, "All", time.Date(2026, 8, 10, 14, 2, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	line := outlineDependencyLine(t, buffer, "Ship auth rewrite")
	if !strings.Contains(line, "~4") {
		t.Errorf("dump dropped the closed target:\n%s", line)
	}
	if !strings.Contains(line, "~project-9:gone") {
		t.Errorf("dump dropped the unavailable target:\n%s", line)
	}
}

// TestOutlineDependencyRoundTripSurvivesUnchanged is the contract that makes the
// dump safe to re-read: every alias form the dump prints parses back to the
// locator it came from.
func TestOutlineDependencyRoundTripSurvivesUnchanged(t *testing.T) {
	scope, projects := outlineDependencyWorkspace(t)
	stored := []models.TaskReference{
		{ProjectID: "project-1", TaskID: "9b31de07bbbb"},
		{ProjectID: "project-2", TaskID: "aa11bb22cccc"},
		{ProjectID: "project-9", TaskID: "deadbeef0000"},
	}
	scope.Tasks[0].Dependencies = stored

	buffer, _, err := dumpOutlineBuffer(scope, projects, "All", time.Date(2026, 8, 10, 14, 2, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	lines, problems := parseOutlineBuffer(buffer, scope, projects)
	if len(problems) > 0 {
		t.Fatalf("parse: %v", problems)
	}
	got := outlineLineFor(t, lines, "4f2a9c11aaaa").Dependencies
	if len(got) != len(stored) {
		t.Fatalf("dependencies = %v, want %d", got, len(stored))
	}
	for i, reference := range stored {
		if got[i].Reference() != reference {
			t.Errorf("dependency %d = %v, want %v", i, got[i], reference)
		}
	}
}

func outlineLineFor(t *testing.T, lines []app.OutlinePlanLine, taskID string) app.OutlinePlanLine {
	t.Helper()
	for _, line := range lines {
		if line.TaskID == taskID {
			return line
		}
	}
	t.Fatalf("no parsed line for %q", taskID)
	return app.OutlinePlanLine{}
}

func TestOutlineDependencyParseFormsTheFinalSet(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string
	}{
		{
			// Deleting a dumped token is how a dependency is removed: the surviving
			// positives are the whole set.
			name: "deleted token is dropped",
			body: "Ship auth rewrite ~2",
			want: []string{"project-1:9b31de07bbbb"},
		},
		{
			name: "no token is the empty set",
			body: "Ship auth rewrite",
			want: nil,
		},
		{
			name: "clear states the empty set",
			body: "Ship auth rewrite ~0",
			want: nil,
		},
		{
			name: "negative subtracts from the line's positives",
			body: "Ship auth rewrite ~2 ~4 -~4",
			want: []string{"project-1:9b31de07bbbb"},
		},
		{
			name: "repeated positives collapse",
			body: "Ship auth rewrite ~2 ~2",
			want: []string{"project-1:9b31de07bbbb"},
		},
		{
			name: "qualified and bare aliases mix",
			body: "Ship auth rewrite ~2 ~radar:7",
			want: []string{"project-1:9b31de07bbbb", "project-2:aa11bb22cccc"},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			scope, projects := outlineDependencyWorkspace(t)
			buffer := tt.body + "  [[tt:4f2a9c11]]"
			lines, problems := parseOutlineBuffer(buffer, scope, projects)
			if len(problems) > 0 {
				t.Fatalf("parse: %v", problems)
			}
			got := make([]string, 0, len(lines[0].Dependencies))
			for _, locator := range lines[0].Dependencies {
				got = append(got, locator.Reference().Locator())
			}
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("dependencies = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOutlineDependencyParseRefusals(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			// ~0 is the empty set, so it cannot also be a member of a set.
			name: "clear cannot be combined with an addition",
			body: "Ship auth rewrite ~0 ~2",
			want: "~0 is the empty set and cannot be combined",
		},
		{
			name: "clear cannot be combined with a removal",
			body: "Ship auth rewrite ~0 -~2",
			want: "~0 is the empty set and cannot be combined",
		},
		{
			name: "negative clear is not a clear form",
			body: "Ship auth rewrite -~0",
			want: "-~0 is not a clear form",
		},
		{
			name: "unknown alias is named",
			body: "Ship auth rewrite ~99",
			want: `dependency not found: "99"`,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			scope, projects := outlineDependencyWorkspace(t)
			buffer := tt.body + "  [[tt:4f2a9c11]]"
			lines, problems := parseOutlineBuffer(buffer, scope, projects)
			if len(problems) == 0 {
				t.Fatalf("parse accepted %q: %+v", tt.body, lines)
			}
			if !strings.Contains(problems[0].message, tt.want) {
				t.Errorf("message = %q, want it to contain %q", problems[0].message, tt.want)
			}
		})
	}
}
