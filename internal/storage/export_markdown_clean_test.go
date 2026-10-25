package storage

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/util"
)

func TestWriteProjectMarkdownClean_EmptyProject(t *testing.T) {
	p := newProject("Empty")
	out := WriteProjectMarkdownClean(p)

	if !strings.HasPrefix(out, "# Project: Empty\n") {
		t.Errorf("expected project header, got: %q", out)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line for empty project, got %d: %q", len(lines), out)
	}
}

func TestWriteProjectMarkdownClean_SingleTask(t *testing.T) {
	task := mustTask("t1", 1, "Do the thing")
	task.Priority = 0
	p := newProject("My Project", task)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "## Task: (#1) Do the thing") {
		t.Errorf("expected task heading, got: %q", out)
	}
	if strings.Contains(out, "[") {
		t.Errorf("expected no badge for todo/zero-priority task, got: %q", out)
	}
}

func TestWriteProjectMarkdownClean_NestedHierarchy(t *testing.T) {
	epic := mustTask("e1", 1, "Big Epic")
	story := mustTask("s1", 2, "A Story")
	story.ParentID = &epic.ID
	task := mustTask("t1", 3, "Leaf Task")
	task.ParentID = &story.ID

	p := newProject("Nested", epic, story, task)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "## Task: (#1) Big Epic") {
		t.Errorf("missing parent task heading: %q", out)
	}
	if !strings.Contains(out, "### Task: (#2) A Story") {
		t.Errorf("missing child task heading: %q", out)
	}
	if !strings.Contains(out, "#### Task: (#3) Leaf Task") {
		t.Errorf("missing task heading: %q", out)
	}
}

func TestWriteProjectMarkdownClean_Orphan(t *testing.T) {
	parentID := "missing-parent"
	orphan := mustTask("o1", 5, "Orphan Task")
	orphan.ParentID = &parentID

	p := newProject("WithOrphan", orphan)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "## Task: (#5) Orphan Task") {
		t.Errorf("expected orphan to appear at root level: %q", out)
	}
}

func TestWriteProjectMarkdownClean_DeepNestingClamped(t *testing.T) {
	ids := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	tasks := make([]*models.Task, len(ids))
	for i, id := range ids {
		tasks[i] = mustTask(id, i+1, "Level "+id)
		if i > 0 {
			parentID := ids[i-1]
			tasks[i].ParentID = &parentID
		}
	}

	p := newProject("Deep", tasks...)
	out := WriteProjectMarkdownClean(p)

	if strings.Contains(out, "#######") {
		t.Errorf("headings were not clamped at ######: %q", out)
	}
	if !strings.Contains(out, "###### Task: (#7)") {
		t.Errorf("expected level-7+ task at ###### heading: %q", out)
	}
}

func TestWriteProjectMarkdownClean_Tags(t *testing.T) {
	task := mustTask("t1", 1, "Tagged")
	task.Tags = []string{"backend", "auth"}
	p := newProject("Tags", task)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "tags: backend, auth") {
		t.Errorf("expected tags line: %q", out)
	}
}

func TestWriteProjectMarkdownClean_ContextBesideTags(t *testing.T) {
	task := mustTask("t1", 1, "Contextual")
	task.Context = "platform-api"
	task.Tags = []string{"backend"}
	p := newProject("Context", task)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "context: platform-api\ntags: backend") {
		t.Errorf("expected context before tags: %q", out)
	}
}

func TestWriteProjectMarkdownClean_NoTagsWhenEmpty(t *testing.T) {
	task := mustTask("t1", 1, "No Tags")
	task.Tags = []string{}
	p := newProject("NoTags", task)
	out := WriteProjectMarkdownClean(p)

	if strings.Contains(out, "tags:") {
		t.Errorf("expected no tags line for empty tags: %q", out)
	}
}

func TestWriteProjectMarkdownClean_Priority(t *testing.T) {
	task := mustTask("t1", 1, "High Priority")
	task.Priority = 1
	p := newProject("Prio", task)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "!1") {
		t.Errorf("expected priority in badge: %q", out)
	}
}

func TestWriteProjectMarkdownClean_ZeroPriorityNoBadge(t *testing.T) {
	task := mustTask("t1", 1, "Zero Prio")
	task.Priority = 0
	p := newProject("ZeroPrio", task)
	out := WriteProjectMarkdownClean(p)

	if strings.Contains(out, "!0") {
		t.Errorf("zero priority should not appear in badge: %q", out)
	}
}

func TestWriteProjectMarkdownClean_DueDate(t *testing.T) {
	task := mustTask("t1", 1, "Has Due")
	due := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	task.DueDate = &due
	task.Priority = 0
	p := newProject("Due", task)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "due 2026-04-20") {
		t.Errorf("expected due date in badge: %q", out)
	}
}

// TestWriteProjectMarkdownClean_DeferredUntil pins the export decision the
// schema-10 deferral instant forced: the badge stays day-only, and the day it
// names is the reader's, not the one carried by the offset another machine wrote.
func TestWriteProjectMarkdownClean_DeferredUntil(t *testing.T) {
	task := mustTask("t1", 1, "Deferred")
	deferUntil := time.Date(2026, 5, 1, 16, 30, 0, 0, time.Local)
	task.DeferredUntil = &deferUntil
	task.Priority = 0
	p := newProject("Defer", task)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "deferred until 2026-05-01") {
		t.Errorf("expected deferral date in badge: %q", out)
	}
	if strings.Contains(out, "16:30") {
		t.Errorf("clean export badge should stay day-only: %q", out)
	}

	elsewhere := deferUntil.In(time.FixedZone("elsewhere", 12*60*60))
	task.DeferredUntil = &elsewhere
	if again := WriteProjectMarkdownClean(p); again != out {
		t.Errorf("same instant in another location changed the badge:\n%q\nwant\n%q", again, out)
	}
}

// TestWriteProjectMarkdownClean_CheckIn covers the other half of that decision
// for the value that is a calendar date rather than an instant: nothing is
// resolved into the reader's zone, because there is no time of day to resolve.
// The repeat rides the same badge segment and the review instant stays out of
// the badge, which is what keeps the export a statement of what a task still
// owes rather than of what has already been done to it.
func TestWriteProjectMarkdownClean_CheckIn(t *testing.T) {
	task := mustTask("t1", 1, "Waiting on vendor")
	task.Priority = 0
	next, err := util.StartOfCalendarDate(2026, time.September, 1, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	reviewed := time.Date(2026, 8, 20, 9, 0, 0, 0, time.UTC)
	task.NextCheckIn = &next
	task.LastReviewedAt = &reviewed
	p := newProject("CheckIn", task)

	out := WriteProjectMarkdownClean(p)
	if !strings.Contains(out, "[check-in 2026-09-01]") {
		t.Errorf("expected a one-off check-in in the badge: %q", out)
	}
	if strings.Contains(out, "2026-08-20") {
		t.Errorf("review history leaked into the badge: %q", out)
	}

	task.CheckInRepeatDays = 7
	if repeating := WriteProjectMarkdownClean(p); !strings.Contains(repeating, "[check-in 2026-09-01 every 7d]") {
		t.Errorf("expected the repeat beside its occurrence: %q", repeating)
	}

	task.NextCheckIn = nil
	task.CheckInRepeatDays = models.CheckInRepeatOff
	if none := WriteProjectMarkdownClean(p); strings.Contains(none, "check-in") {
		t.Errorf("unscheduled task named a check-in: %q", none)
	}
}

func TestWriteProjectMarkdownClean_NonTodoStatus(t *testing.T) {
	task := mustTask("t1", 1, "In Progress")
	task.ApplyWorkflowState(models.StateDoing)
	task.Priority = 0
	p := newProject("Status", task)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "doing") {
		t.Errorf("expected non-todo status in badge: %q", out)
	}
}

func TestWriteProjectMarkdownClean_TodoStatusNoBadge(t *testing.T) {
	task := mustTask("t1", 1, "Todo")
	task.ApplyWorkflowState(models.StateBacklog)
	task.Priority = 0
	p := newProject("TodoStatus", task)
	out := WriteProjectMarkdownClean(p)

	if strings.Contains(out, "[") {
		t.Errorf("todo status should not produce a badge: %q", out)
	}
}

func TestWriteProjectMarkdownClean_EmptyDescription(t *testing.T) {
	task := mustTask("t1", 1, "No Desc")
	task.Description = ""
	p := newProject("NoDesc", task)
	out := WriteProjectMarkdownClean(p)

	lines := strings.Split(out, "\n")
	taskHeadingFound := false
	for i, line := range lines {
		if strings.Contains(line, "## Task: (#1) No Desc") {
			taskHeadingFound = true
			if i+1 < len(lines) && lines[i+1] == "" {
				if i+2 < len(lines) && lines[i+2] != "" && !strings.HasPrefix(lines[i+2], "#") {
					t.Errorf("unexpected description content after empty description task: %v", lines[i+2])
				}
			}
			break
		}
	}
	if !taskHeadingFound {
		t.Errorf("task heading not found in output: %q", out)
	}
}

func TestWriteProjectMarkdownClean_NoHTMLComments(t *testing.T) {
	task := mustTask("t1", 1, "Clean")
	task.ApplyWorkflowState(models.StateDoing)
	task.Priority = 2
	task.Tags = []string{"x"}
	p := newProject("Clean", task)
	out := WriteProjectMarkdownClean(p)

	if strings.Contains(out, "<!--") || strings.Contains(out, "-->") {
		t.Errorf("clean export must not contain HTML comments: %q", out)
	}
}

func TestWriteProjectMarkdownClean_Description(t *testing.T) {
	task := mustTask("t1", 1, "With Desc")
	task.Description = "This is the description.\nWith multiple lines."
	task.Priority = 0
	p := newProject("Desc", task)
	out := WriteProjectMarkdownClean(p)

	if !strings.Contains(out, "This is the description.\nWith multiple lines.") {
		t.Errorf("expected verbatim description in output: %q", out)
	}
}

// TestWriteProjectMarkdownClean_ResumeNote covers the labeled paragraph itself:
// it is present exactly when the task holds a note, it leads the description,
// and a task holding none contributes no label at all.
func TestWriteProjectMarkdownClean_ResumeNote(t *testing.T) {
	noted := mustTask("t1", 1, "Noted")
	noted.Priority = 0
	noted.ResumeNote = "stopped at the failing merge case"
	noted.Description = "The body prose."
	bare := mustTask("t2", 2, "Bare")
	bare.Priority = 0
	bare.Description = "Body without a note."
	p := newProject("Resume", noted, bare)
	out := WriteProjectMarkdownClean(p)

	label := "**Resume:** stopped at the failing merge case"
	if !strings.Contains(out, label) {
		t.Fatalf("expected the labeled resume paragraph in output: %q", out)
	}
	if strings.Count(out, "**Resume:**") != 1 {
		t.Fatalf("a task with no resume note emitted a label: %q", out)
	}
	resumeAt := strings.Index(out, label)
	bodyAt := strings.Index(out, "The body prose.")
	if bodyAt < 0 || resumeAt > bodyAt {
		t.Fatalf("resume paragraph at %d, description at %d: want resume first in %q", resumeAt, bodyAt, out)
	}
	// The label is its own paragraph, so the description below it is never
	// swallowed into the same block.
	if !strings.Contains(out, label+"\n\nThe body prose.\n") {
		t.Fatalf("resume paragraph did not stand alone above the description: %q", out)
	}
}

// TestWriteProjectMarkdownClean_ResumeNoteEscapesInlineMarkdown holds the one
// asymmetry with the description beside it: the note is spliced after a bold
// label, so punctuation that would reformat that paragraph is escaped, while the
// description keeps being emitted verbatim.
func TestWriteProjectMarkdownClean_ResumeNoteEscapesInlineMarkdown(t *testing.T) {
	task := mustTask("t1", 1, "Escapes")
	task.Priority = 0
	task.ResumeNote = `use *ptr not &ref; see _x_ in ` + "`main`" + ` [1] <!-- keep --> a\b ~c~`
	task.Description = `Verbatim *emphasis* and <b>markup</b>.`
	p := newProject("Escaped", task)
	out := WriteProjectMarkdownClean(p)

	want := "**Resume:** use \\*ptr not \\&ref; see \\_x\\_ in \\`main\\` " +
		"\\[1\\] \\<!-- keep --\\> a\\\\b \\~c\\~"
	if !strings.Contains(out, want) {
		t.Fatalf("escaped resume paragraph missing.\n got: %q\nwant: %q", out, want)
	}
	// Stated once more as the rule rather than as one expected string: nothing on
	// the resume line carries inline Markdown meaning any more, so a note that
	// looks like an HTML comment renders as the words the author typed.
	resumeLine := ""
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "**Resume:** ") {
			resumeLine = strings.TrimPrefix(line, "**Resume:** ")
		}
	}
	escaped := false
	for _, r := range resumeLine {
		switch {
		case escaped:
			escaped = false
		case r == '\\':
			escaped = true
		case strings.ContainsRune(exportInlineMarkdownSpecials, r):
			t.Fatalf("%q reached the export unescaped: %q", r, resumeLine)
		}
	}
	if !strings.Contains(out, task.Description) {
		t.Fatalf("the description was escaped alongside the note: %q", out)
	}
}

// TestEscapeExportInlineTextLeavesOrdinaryProseAlone keeps the escape narrow: a
// note with no inline-markup punctuation exports exactly as it was written.
func TestEscapeExportInlineTextLeavesOrdinaryProseAlone(t *testing.T) {
	for _, text := range []string{
		"",
		"stopped at the failing merge case",
		"paused before step 3 (needs review), then retry",
		"重建采石场的排水系统",
	} {
		if got := escapeExportInlineText(text); got != text {
			t.Fatalf("escapeExportInlineText(%q) = %q, want it unchanged", text, got)
		}
	}
}

// TestWriteProjectMarkdownClean_Hidden keeps a hidden task in the export and
// names the flag in its badge, since the export is the project's record and
// every view leaves the task out.
func TestWriteProjectMarkdownClean_Hidden(t *testing.T) {
	task := mustTask("t1", 1, "Out of sight")
	task.Priority = 0
	task.Hidden = true
	p := newProject("Hidden", task)

	out := WriteProjectMarkdownClean(p)
	if !strings.Contains(out, "Out of sight [hidden]") {
		t.Errorf("expected the hidden task with its badge: %q", out)
	}
	task.Hidden = false
	if visible := WriteProjectMarkdownClean(p); strings.Contains(visible, "hidden") {
		t.Errorf("an unhidden task named the flag: %q", visible)
	}
}
