package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/grammar"
)

func TestOutlineMarkerStripLeavesCaptureText(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		marker string
		rest   string
		fails  bool
	}{
		{name: "trailing marker", body: "Ship auth rewrite !1  [[tt:4f2a9c11]]", marker: "4f2a9c11", rest: "Ship auth rewrite !1"},
		{name: "no marker", body: "Rate-limit headers $backlog", marker: "", rest: "Rate-limit headers $backlog"},
		{name: "brackets in title", body: "Fix [WIP] parser  [[tt:4f2a9c11]]", marker: "4f2a9c11", rest: "Fix [WIP] parser"},
		{name: "bracketed title without marker", body: "Fix parser", marker: "", rest: "Fix parser"},
		{name: "suffix without prefix", body: "Fix parser]]", fails: true},
		{name: "marker holding a space", body: "Fix parser  [[tt:4f2a 9c11]]", fails: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marker, rest, err := grammar.SplitOutlineMarker(tt.body)
			if tt.fails {
				if err == nil {
					t.Fatalf("split accepted %q", tt.body)
				}
				return
			}
			if err != nil {
				t.Fatalf("split: %v", err)
			}
			if marker != tt.marker {
				t.Errorf("marker = %q, want %q", marker, tt.marker)
			}
			if rest != tt.rest {
				t.Errorf("rest = %q, want %q", rest, tt.rest)
			}
		})
	}
}

// TestOutlineMarkerIndexLeavesAmbiguousShortMarkersUnresolved keeps a
// hand-authored buffer honest: the dump refuses to print colliding markers, so a
// short marker naming two tasks must be rejected rather than resolved by luck.
func TestOutlineMarkerIndexLeavesAmbiguousShortMarkersUnresolved(t *testing.T) {
	project := outlineTestProject(t)
	clash, err := models.NewGenericTask("4f2a9c11eeee", 5, "Colliding task")
	if err != nil {
		t.Fatalf("new task: %v", err)
	}
	project.Tasks = append(project.Tasks, clash)

	index := grammar.OutlineMarkerIndex(project)
	if resolved, ok := index["4f2a9c11"]; ok {
		t.Fatalf("ambiguous short marker resolved to %s", resolved)
	}
	if index["4f2a9c11aaaa"] != "4f2a9c11aaaa" || index["4f2a9c11eeee"] != "4f2a9c11eeee" {
		t.Fatal("full task IDs must stay addressable when their short markers collide")
	}
}

func TestOutlineApplyErrorReusesTheApplicationLineNumber(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		line    int
		message string
	}{
		{
			name:    "prefixed",
			err:     fmt.Errorf("line 7: title: task title cannot be empty"),
			line:    7,
			message: "title: task title cannot be empty",
		},
		{
			name:    "unprefixed",
			err:     fmt.Errorf("project not found: project-9"),
			line:    1,
			message: "project not found: project-9",
		},
		{
			name:    "unparseable number",
			err:     fmt.Errorf("line seven: broken"),
			line:    1,
			message: "line seven: broken",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			problem := outlineApplyError(tt.err)
			if problem.Line != tt.line || problem.Message != tt.message {
				t.Fatalf("got line %d %q, want line %d %q", problem.Line, problem.Message, tt.line, tt.message)
			}
		})
	}
}

// TestOutlineDumpScopeExcludesAnOrphanedTask is why scope comes from the dump
// walk instead of project.Tasks. A task whose ParentID names nothing is neither
// a root nor anyone's child, so it never reaches the buffer — and absence
// deletes now, so a scope built from project.Tasks would delete a line the user
// was never shown.
func TestOutlineDumpScopeExcludesAnOrphanedTask(t *testing.T) {
	project := outlineTestProject(t)
	project.Tasks = append(project.Tasks, nil)
	orphan, err := models.NewGenericTask("dd44ee55ffff", 9, "Orphaned by a dangling parent")
	if err != nil {
		t.Fatalf("new task: %v", err)
	}
	missing := "no-such-task"
	orphan.ParentID = &missing
	project.Tasks = append(project.Tasks, orphan)

	buffer, scope, err := dumpOutlineBuffer(project, nil, "All", time.Now())
	if err != nil {
		t.Fatalf("dump: %v", err)
	}
	if len(scope) != 4 {
		t.Fatalf("scope holds %d ids, want 4: %v", len(scope), scope)
	}
	for _, id := range scope {
		if id == orphan.ID {
			t.Fatalf("scope named the orphan the buffer never printed: %v", scope)
		}
	}
	if strings.Contains(buffer, "Orphaned by a dangling parent") {
		t.Fatalf("dump printed an unreachable task:\n%s", buffer)
	}

	lines, problems := grammar.ParseOutlineBuffer(buffer, project, nil)
	if len(problems) > 0 {
		t.Fatalf("unedited buffer was rejected: %v", problems)
	}
	if deleted := outlineDeletedTasks(project, scope, lines); len(deleted) != 0 {
		t.Fatalf("an unedited buffer asked to delete %d task(s)", len(deleted))
	}
}
