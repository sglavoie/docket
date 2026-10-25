package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/textgrammar"
	"github.com/sglavoie/docket/internal/tui/grammar"
)

func TestOutlineDumpRendersIndentedParentageAndMarkers(t *testing.T) {
	project := outlineTestProject(t)
	project.Tasks[0].Priority = 1
	due := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	project.Tasks[0].DueDate = &due
	project.Tasks[0].Tags = []string{"epic"}
	project.Tasks[0].Context = "platform"
	project.Tasks[1].Tags = []string{"backend"}

	now := time.Date(2026, 8, 10, 14, 2, 0, 0, time.UTC)
	buffer, _, err := dumpOutlineBuffer(project, nil, "Backlog", now)
	if err != nil {
		t.Fatalf("dump: %v", err)
	}

	want := []string{
		"# Docket outline · project: Sonar · mode: Backlog · 2026-08-10 14:02",
		"Ship auth rewrite !1 @2026-08-20 #epic ##platform $doing  [[tt:4f2a9c11]]",
		"  Token refresh path #backend $backlog  [[tt:9b31de07]]",
		"    Retry budget $backlog  [[tt:c0184aa2]]",
		"Sweep dead flags $inbox  [[tt:71f0b3d5]]",
	}
	for _, line := range want {
		if !strings.Contains(buffer, line+"\n") {
			t.Fatalf("dump missing %q:\n%s", line, buffer)
		}
	}
}

// TestOutlineDumpNamesEachHiddenSubtreeInAComment keeps hidden work in the
// buffer, so removing a parent never orphans it, and names each hidden subtree
// once in a comment the parser skips: the round trip still yields every line.
func TestOutlineDumpNamesEachHiddenSubtreeInAComment(t *testing.T) {
	project := outlineTestProject(t)
	project.Tasks[1].Hidden = true // Token refresh path, parent of Retry budget
	project.Tasks[2].Hidden = true // stored again inside the hidden subtree
	buffer, dumped, err := dumpOutlineBuffer(project, nil, "All", time.Now())
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	if got := strings.Count(buffer, outlineHiddenSubtreeComment+"\n"); got != 1 {
		t.Fatalf("hidden comments = %d, want one per hidden subtree:\n%s", got, buffer)
	}
	if !strings.Contains(buffer, outlineHiddenSubtreeComment+"\n  Token refresh path") {
		t.Fatalf("the comment does not stand above the hidden subtree:\n%s", buffer)
	}
	lines, problems := grammar.ParseOutlineBuffer(buffer, project, nil)
	if len(problems) != 0 || len(lines) != len(dumped) {
		t.Fatalf("round trip = %d lines, %v; want %d lines", len(lines), problems, len(dumped))
	}
}

// TestOutlineDumpIsDeterministic keeps the buffer a stable function of the
// project, so an unedited round trip can never report a change nobody made.
func TestOutlineDumpIsDeterministic(t *testing.T) {
	project := outlineTestProject(t)
	now := time.Date(2026, 8, 10, 14, 2, 0, 0, time.UTC)
	first, _, err := dumpOutlineBuffer(project, nil, "All", now)
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	for range 5 {
		again, _, err := dumpOutlineBuffer(project, nil, "All", now)
		if err != nil {
			t.Fatalf("dump: %v", err)
		}
		if again != first {
			t.Fatalf("dump is not deterministic:\n%s\n---\n%s", first, again)
		}
	}
}

func TestOutlineDumpRefusesCollidingMarkers(t *testing.T) {
	project := outlineTestProject(t)
	clash, err := models.NewGenericTask("4f2a9c11eeee", 5, "Colliding task")
	if err != nil {
		t.Fatalf("new task: %v", err)
	}
	project.Tasks = append(project.Tasks, clash)

	if _, _, err := dumpOutlineBuffer(project, nil, "All", time.Now()); err == nil {
		t.Fatal("dump accepted a project whose short markers collide")
	} else if !strings.Contains(err.Error(), "share the marker") {
		t.Fatalf("collision refusal reads %q", err.Error())
	}
}

// TestOutlineRoundTripOfAnUneditedBufferIsFaithful drives the dump straight back
// through the parser: the buffer is declarative, so every value it prints must
// parse back into the same plan line.
func TestOutlineRoundTripOfAnUneditedBufferIsFaithful(t *testing.T) {
	project := outlineTestProject(t)
	project.Tasks[0].Priority = 2
	project.Tasks[0].Tags = []string{"epic", "auth"}
	project.Tasks[0].Context = "platform"
	// The due date is a calendar day, so the round trip has to be faithful at the
	// instant the parser mints for that day; anything else would report an edit
	// nobody made.
	due := time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local)
	project.Tasks[0].DueDate = &due

	buffer, _, err := dumpOutlineBuffer(project, nil, "All", time.Now())
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	lines, problems := grammar.ParseOutlineBuffer(buffer, project, nil)
	if len(problems) > 0 {
		t.Fatalf("unedited buffer was rejected: %v", problems)
	}
	if len(lines) != len(project.Tasks) {
		t.Fatalf("plan holds %d lines, want %d", len(lines), len(project.Tasks))
	}
	for i, line := range lines {
		task := project.Tasks[i]
		if line.TaskID != task.ID {
			t.Errorf("line %d names %s, want %s", i, line.TaskID, task.ID)
		}
		if line.Title != task.Title {
			t.Errorf("line %d title = %q, want %q", i, line.Title, task.Title)
		}
		if line.Priority != task.Priority {
			t.Errorf("line %d priority = %d, want %d", i, line.Priority, task.Priority)
		}
		if string(line.Destination) == "" {
			t.Errorf("line %d dropped its destination", i)
		}
	}
	if lines[0].Due == nil || !lines[0].Due.Equal(due) {
		t.Errorf("round trip lost the due date: %v", lines[0].Due)
	}
	if strings.Join(lines[0].Tags, ",") != "epic,auth" {
		t.Errorf("round trip tags = %v, want [epic auth]", lines[0].Tags)
	}
	if lines[0].Context != "platform" {
		t.Errorf("round trip context = %q, want platform", lines[0].Context)
	}
	if lines[1].Depth != 1 || lines[2].Depth != 2 {
		t.Errorf("round trip depths = %d/%d, want 1/2", lines[1].Depth, lines[2].Depth)
	}
}

func TestOutlineParseBuildsPlanLinesForNewAndEditedTasks(t *testing.T) {
	project := outlineTestProject(t)
	buffer := strings.Join([]string{
		"# comment",
		"",
		"Ship auth rewrite !1 $doing  [[tt:4f2a9c11]]",
		"  Rate-limit headers $backlog",
		"Sweep dead flags  [[tt:71f0b3d5]]",
	}, "\n")

	lines, problems := grammar.ParseOutlineBuffer(buffer, project, nil)
	if len(problems) > 0 {
		t.Fatalf("buffer was rejected: %v", problems)
	}
	if len(lines) != 3 {
		t.Fatalf("plan holds %d lines, want 3", len(lines))
	}
	if lines[0].SourceLine != 3 || lines[1].SourceLine != 4 || lines[2].SourceLine != 5 {
		t.Errorf("source lines = %d/%d/%d, want 3/4/5", lines[0].SourceLine, lines[1].SourceLine, lines[2].SourceLine)
	}
	if lines[0].TaskID != "4f2a9c11aaaa" {
		t.Errorf("marker resolved to %q", lines[0].TaskID)
	}
	if lines[1].TaskID != "" {
		t.Errorf("markerless line claimed task %q instead of creating one", lines[1].TaskID)
	}
	if lines[1].Depth != 1 {
		t.Errorf("new child depth = %d, want 1", lines[1].Depth)
	}
	if lines[0].Priority != 1 {
		t.Errorf("priority = %d, want 1", lines[0].Priority)
	}
	// An omitted destination stays empty so the application layer preserves the
	// state the task already holds instead of laundering a transition.
	if lines[2].Destination != "" {
		t.Errorf("omitted destination became %q", lines[2].Destination)
	}
	if lines[0].Destination != textgrammar.DestinationDoing {
		t.Errorf("destination = %q, want doing", lines[0].Destination)
	}
}

func TestOutlineParseRejectsEveryOutlinePhaseError(t *testing.T) {
	project := outlineTestProject(t)
	tests := []struct {
		name    string
		buffer  string
		line    int
		message string
	}{
		{
			name:    "odd indent",
			buffer:  "Ship auth rewrite  [[tt:4f2a9c11]]\n   Token refresh path  [[tt:9b31de07]]",
			line:    2,
			message: "indent of 3 spaces is not a multiple of 2",
		},
		{
			name:    "tab indent",
			buffer:  "Ship auth rewrite  [[tt:4f2a9c11]]\n\tToken refresh path",
			line:    2,
			message: "indent with spaces, not tabs",
		},
		{
			name:    "indent jump",
			buffer:  "Ship auth rewrite  [[tt:4f2a9c11]]\n    Retry budget",
			line:    2,
			message: "indent jumps from level 0 to level 2; indent one level at a time",
		},
		{
			name:    "unknown marker",
			buffer:  "Ship auth rewrite  [[tt:deadbeef]]",
			line:    1,
			message: "marker [[tt:deadbeef]] does not name a task in this project",
		},
		{
			name:    "duplicate marker",
			buffer:  "Ship auth rewrite  [[tt:4f2a9c11]]\nShip auth rewrite again  [[tt:4f2a9c11]]",
			line:    2,
			message: "marker [[tt:4f2a9c11]] already claimed on line 1",
		},
		{
			name:    "malformed marker",
			buffer:  "Ship auth rewrite  [[tt:]]",
			line:    1,
			message: `marker "[[tt:]]" is malformed`,
		},
		{
			name:    "marker suffix without prefix",
			buffer:  "Ship auth rewrite]]",
			line:    1,
			message: "a line ending in ]] must carry a [[tt:…]] marker",
		},
		{
			name:    "orphaned by a rejected parent",
			buffer:  "Ship auth rewrite  [[tt:deadbeef]]\n  Token refresh path  [[tt:9b31de07]]",
			line:    2,
			message: "its parent on line 1 was rejected, so it has nowhere to attach",
		},
		{
			name:    "over the nesting cap",
			buffer:  outlineDeepBuffer(models.MaxNestingDepth + 1),
			line:    models.MaxNestingDepth + 2,
			message: fmt.Sprintf("nests deeper than the maximum of %d levels", models.MaxNestingDepth),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines, problems := grammar.ParseOutlineBuffer(tt.buffer, project, nil)
			if lines != nil {
				t.Fatalf("a rejected buffer produced %d plan lines", len(lines))
			}
			if !outlineProblemsContain(problems, tt.line, tt.message) {
				t.Fatalf("problems %v do not name line %d: %s", problems, tt.line, tt.message)
			}
		})
	}
}

func TestOutlineParseRejectsEveryLinePhaseError(t *testing.T) {
	project := outlineTestProject(t)
	tests := []struct {
		name    string
		buffer  string
		line    int
		message string
	}{
		{
			name:    "empty title",
			buffer:  "!1 $doing  [[tt:4f2a9c11]]",
			line:    1,
			message: "task title cannot be empty",
		},
		{
			name:    "parent sigil",
			buffer:  "Ship auth rewrite >2  [[tt:4f2a9c11]]",
			line:    1,
			message: "indentation sets parentage here; remove the >parent sigil",
		},
		{
			name:    "child sigil",
			buffer:  "Ship auth rewrite <2  [[tt:4f2a9c11]]",
			line:    1,
			message: "indentation sets parentage here; remove the <child sigil",
		},
		{
			name:    "project sigil",
			buffer:  "Ship auth rewrite %other  [[tt:4f2a9c11]]",
			line:    1,
			message: "the outline buffer owns the project; remove the %project sigil",
		},
		{
			name:    "defer sigil",
			buffer:  "Ship auth rewrite @@+2h  [[tt:4f2a9c11]]",
			line:    1,
			message: "the outline buffer cannot defer; remove the @@defer sigil",
		},
		{
			name:   "unknown destination",
			buffer: "Ship auth rewrite $ready  [[tt:4f2a9c11]]",
			line:   1,
		},
		{
			name:   "retired priority",
			buffer: "Ship auth rewrite !5  [[tt:4f2a9c11]]",
			line:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines, problems := grammar.ParseOutlineBuffer(tt.buffer, project, nil)
			if lines != nil {
				t.Fatalf("a rejected buffer produced %d plan lines", len(lines))
			}
			if len(problems) == 0 {
				t.Fatal("the buffer was accepted")
			}
			if tt.message != "" && !outlineProblemsContain(problems, tt.line, tt.message) {
				t.Fatalf("problems %v do not name line %d: %s", problems, tt.line, tt.message)
			}
			if problems[0].Line != tt.line {
				t.Fatalf("first problem names line %d, want %d", problems[0].Line, tt.line)
			}
		})
	}
}

// TestOutlineParseCollectsEveryProblemInOnePass is the contract that makes the
// error surface worth having: a buffer wrong in several places reports all of
// them, in line order, instead of one reopen per mistake.
// TestOutlineAcceptsEmbeddedComparisonText keeps the child rejection anchored to
// the whitespace-delimited sigil: a comparison inside a title is not a <child.
func TestOutlineAcceptsEmbeddedComparisonText(t *testing.T) {
	project := outlineTestProject(t)
	lines, problems := grammar.ParseOutlineBuffer("Keep a<b under 5 ms  [[tt:4f2a9c11]]", project, nil)
	if len(problems) > 0 {
		t.Fatalf("buffer was rejected: %v", problems)
	}
	if len(lines) != 1 || lines[0].Title != "Keep a<b under 5 ms" {
		t.Fatalf("plan = %+v, want the comparison preserved in the title", lines)
	}
}

func TestOutlineParseCollectsEveryProblemInOnePass(t *testing.T) {
	project := outlineTestProject(t)
	buffer := strings.Join([]string{
		"Ship auth rewrite  [[tt:4f2a9c11]]",
		"   Token refresh path  [[tt:9b31de07]]",
		"!1  [[tt:71f0b3d5]]",
		"Sweep dead flags again  [[tt:71f0b3d5]]",
	}, "\n")

	lines, problems := grammar.ParseOutlineBuffer(buffer, project, nil)
	if lines != nil {
		t.Fatalf("a rejected buffer produced %d plan lines", len(lines))
	}
	if len(problems) != 3 {
		t.Fatalf("collected %d problems, want 3: %v", len(problems), problems)
	}
	for i := 1; i < len(problems); i++ {
		if problems[i-1].Line > problems[i].Line {
			t.Fatalf("problems are not in line order: %v", problems)
		}
	}
}
