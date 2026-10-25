package outliner

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/chrome"
)

// peekProjectComment mints one stored comment. The instant is derived from the
// index rather than taken from the clock, so the ordering assertions below name
// a fixed sequence instead of whichever order two same-millisecond writes landed
// in.
func peekProjectComment(index int, text string) *models.ProjectComment {
	return &models.ProjectComment{
		ID:        fmt.Sprintf("00000000-0000-4000-8000-%012d", index),
		Text:      text,
		CreatedAt: time.Date(2026, 3, 1+index, 9, 30, 0, 0, time.UTC),
	}
}

// peekCommentedProject is a project whose detail cannot fit one window: a long
// description, tags, and several comments of which one wraps. Storage order is
// oldest first, which is what the presentation reverses.
func peekCommentedProject(id string) *models.Project {
	project := focusedHeaderProject(id)
	project.Name = strings.ToUpper(id[:1]) + id[1:] + " Platform"
	project.Description = strings.Repeat("Wrapped project description content ", 6)
	project.Tags = []string{"internal", "platform"}
	project.Comments = []*models.ProjectComment{
		peekProjectComment(0, "Oldest decision about the vendor contract"),
		peekProjectComment(1, "Middle note"),
		peekProjectComment(2, strings.Repeat("Newest and deliberately long enough to wrap the pane. ", 4)),
	}
	return project
}

func peekProjectDocument(project *models.Project, width int) inlinePeekDocument {
	return inlinePeekProjectDocumentFor(inlinePeekProjectOptions{Project: project, Indent: "  ", Width: width, Window: testPeekWindow()})
}

func peekDocumentText(doc inlinePeekDocument) string {
	return ansi.Strip(strings.Join(doc.Body, "\n"))
}

// firstIndexOf is the ordering assertion's one read. It fails the test rather
// than returning a sentinel, because every caller below is naming content the
// fixture put in the document on purpose.
func firstIndexOf(t *testing.T, haystack, needle, label string) int {
	t.Helper()
	index := strings.Index(haystack, needle)
	if index < 0 {
		t.Fatalf("the document omits %s (%q):\n%s", label, needle, haystack)
	}
	return index
}

// TestProjectPeekNamesEveryStoredDetailInOrder is the document's content
// contract: the project's own name first, then description, tags, and the
// comment bodies newest first. Body order is asserted by position rather than by
// presence, because the sections are what a reader scans and a reordering is
// invisible to a membership check.
func TestProjectPeekNamesEveryStoredDetailInOrder(t *testing.T) {
	project := peekCommentedProject("alpha")
	doc := peekProjectDocument(project, 80)

	body := ansi.Strip(strings.Join(doc.Body, "\n"))
	name := firstIndexOf(t, body, project.Name, "the project name")
	description := firstIndexOf(t, body, "Description: ", "the description label")
	if name >= description {
		t.Fatalf("the name does not lead the body: name %d, description %d:\n%s", name, description, body)
	}
	tags := firstIndexOf(t, body, "#platform", "the tag chips")
	comments := firstIndexOf(t, body, "Comments: ", "the comment label")
	if description >= tags || tags >= comments {
		t.Fatalf("body order = description %d, tags %d, comments %d:\n%s", description, tags, comments, body)
	}

	// Newest first, and every stamp drawn, which is what tells two comments of
	// the same length apart.
	newest := firstIndexOf(t, body, "Newest and deliberately long", "the newest comment")
	middle := firstIndexOf(t, body, "Middle note", "the middle comment")
	oldest := firstIndexOf(t, body, "Oldest decision", "the oldest comment")
	if newest >= middle || middle >= oldest {
		t.Fatalf("comment order = newest %d, middle %d, oldest %d:\n%s", newest, middle, oldest, body)
	}
	for _, comment := range project.Comments {
		stamp := ansi.Strip(chrome.FormatTimestampBracket(comment.CreatedAt))
		if !strings.Contains(body, stamp) {
			t.Fatalf("comment %s drew no stamp %q:\n%s", comment.ID, stamp, body)
		}
	}

	// The long comment is whole rather than elided: every word of it is reachable
	// somewhere in the document, because the page is what bounds the section.
	unwrapped := strings.Join(strings.Fields(body), " ")
	if !strings.Contains(unwrapped, strings.TrimSpace(project.Comments[2].Text)) {
		t.Fatalf("the newest comment was elided:\n%s", body)
	}
	if !doc.Paged() {
		t.Fatalf("a project with three comments fit one window: %d rows", len(doc.Body))
	}
}

func TestProjectPeekSummarizesTasksBySection(t *testing.T) {
	parent := primaryRowsTask("parent", models.StateBacklog)
	child := primaryRowsTask("child", models.StateDoing)
	child.ParentID = &parent.ID
	project := primaryRowsProject("alpha",
		parent,
		child,
		primaryRowsTask("inbox", models.StateInbox),
		primaryRowsTask("done", models.StateDone),
		primaryRowsTask("canceled", models.StateCanceled),
	)
	project.Name = "Alpha Platform"
	project.Description = "Planning the next release"
	doc := peekProjectDocument(project, 80)
	body := strings.Join(strings.Fields(peekDocumentText(doc)), " ")
	for _, want := range []string{"Tasks: 5", "Doing 1", "Inbox 1", "Backlog 1", "Done 1", "Canceled 1"} {
		if !strings.Contains(body, want) {
			t.Fatalf("project summary omits %q:\n%s", want, body)
		}
	}
	if firstIndexOf(t, body, "Tasks: 5", "task count") >= firstIndexOf(t, body, "Description:", "description") {
		t.Fatal("task summary should precede project details")
	}

	base := doc.Fingerprint
	child.WorkflowState = models.StateWaiting
	if got := peekProjectDocument(project, 80).Fingerprint; got == base {
		t.Fatal("changing a task's section left the project Peek fingerprint unchanged")
	}
	project.Tasks = append(project.Tasks, primaryRowsTask("another", models.StateBacklog))
	if got := peekProjectDocument(project, 80).Fingerprint; got == base {
		t.Fatal("adding a task left the project Peek fingerprint unchanged")
	}
}

func TestProjectPeekDistinguishesEmptyAndUnloadedTaskLists(t *testing.T) {
	project := primaryRowsProject("empty")
	project.Tasks = []*models.Task{}
	empty := strings.Join(strings.Fields(peekDocumentText(peekProjectDocument(project, 80))), " ")
	if !strings.Contains(empty, "Tasks: 0 · no sections") {
		t.Fatalf("empty project summary = %q", empty)
	}
	project.Tasks = nil
	unloaded := strings.Join(strings.Fields(peekDocumentText(peekProjectDocument(project, 80))), " ")
	if !strings.Contains(unloaded, "Tasks: Not loaded") || strings.Contains(unloaded, "Tasks: 0") {
		t.Fatalf("unloaded project summary = %q", unloaded)
	}
}

// TestProjectPeekSpendsOneRowPerVisualLine holds the document to its own row
// accounting. The comment popup is the multiline TextEntry provider, so a stored
// body can carry newlines of its own; a row hiding one would count as a single
// line everywhere the height is measured and paint two on screen, which is a
// layout issue rather than a rendering choice.
func TestProjectPeekSpendsOneRowPerVisualLine(t *testing.T) {
	project := primaryRowsProject("wrapped")
	project.Name = "Wrapped"
	project.Description = "First paragraph.\n\nSecond paragraph."
	project.Comments = []*models.ProjectComment{
		peekProjectComment(0, "Opening line of the note.\n\nA blank line above this one."),
	}

	doc := peekProjectDocument(project, 80)
	for i, line := range doc.Body {
		if strings.Contains(line, "\n") {
			t.Fatalf("row %d carries a newline of its own: %q", i, ansi.Strip(line))
		}
	}
	// The blank row between the two halves is dropped rather than spent, for the
	// same reason every other Peek body drops one: five rows cannot afford it.
	body := ansi.Strip(strings.Join(doc.Body, "\n"))
	for _, want := range []string{"First paragraph.", "Second paragraph.", "Opening line of the note.", "A blank line above this one."} {
		if !strings.Contains(strings.Join(strings.Fields(body), " "), want) {
			t.Fatalf("the document omits %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "\n\n") {
		t.Fatalf("a blank row survived into the document:\n%q", body)
	}
}

// TestProjectPeekAlwaysHasSomethingToSay covers the empty project, which is the
// state a header is most often parked on: a project created a minute ago has no
// description, no tags, and no comments, and a Peek that drew nothing there
// would read as a broken key rather than as an empty project.
func TestProjectPeekAlwaysHasSomethingToSay(t *testing.T) {
	empty := primaryRowsProject("empty")
	empty.Name = "Empty"
	doc := peekProjectDocument(empty, 80)

	if doc.Empty() {
		t.Fatal("an empty project produced no document")
	}
	if doc.Paged() {
		t.Fatalf("an empty project paged: %d rows", len(doc.Body))
	}
	body := ansi.Strip(strings.Join(doc.Body, "\n"))
	if want := "Comments: No comments yet. Press C to add one."; !strings.Contains(strings.Join(strings.Fields(body), " "), want) {
		t.Fatalf("empty body = %q, want %q", body, want)
	}

	// A project the population no longer holds is the other end of the same
	// question, and it is the one case that draws nothing at all.
	if got := peekProjectDocument(nil, 80); !got.Empty() {
		t.Fatalf("an unresolved header produced %+v", got)
	}
	if got := peekProjectDocument(empty, 0); !got.Empty() {
		t.Fatalf("a zero-width pane produced %+v", got)
	}
}

// TestProjectPeekFingerprintReadsStoredValuesAlone is the reset rule's input
// half. The fingerprint says what the document is, so it must move when the
// project's stored detail moves and stay still for everything else — a re-wrap at
// another width, and the relative age beside every stamp, which drifts on the
// minute tick and would otherwise throw a reader back to the top mid-page.
func TestProjectPeekFingerprintReadsStoredValuesAlone(t *testing.T) {
	project := peekCommentedProject("alpha")
	base := peekProjectDocument(project, 80).Fingerprint

	if got := peekProjectDocument(project, 40).Fingerprint; got != base {
		t.Fatal("a narrower pane produced a different document")
	}
	if got := peekProjectDocument(project, 120).Fingerprint; got != base {
		t.Fatal("a wider pane produced a different document")
	}
	if got := peekProjectDocument(peekCommentedProject("alpha"), 80).Fingerprint; got != base {
		t.Fatal("rebuilding the same project produced a different document")
	}
	for _, drift := range []string{"ago", "from now"} {
		if strings.Contains(base, drift) {
			t.Fatalf("the fingerprint carries the drifting age %q: %q", drift, base)
		}
	}

	for _, test := range []struct {
		name   string
		mutate func(*models.Project)
	}{
		{name: "rename", mutate: func(p *models.Project) { p.Name = "Renamed" }},
		{name: "description", mutate: func(p *models.Project) { p.Description += " and one more sentence" }},
		{name: "tags", mutate: func(p *models.Project) { p.Tags = append(p.Tags, "parked") }},
		{name: "comment text", mutate: func(p *models.Project) { p.Comments[1].Text = "Edited" }},
		{name: "comment instant", mutate: func(p *models.Project) {
			p.Comments[1].CreatedAt = p.Comments[1].CreatedAt.Add(time.Second)
		}},
		{name: "comment added", mutate: func(p *models.Project) {
			p.Comments = append(p.Comments, peekProjectComment(3, "Newer still"))
		}},
		{name: "comment deleted", mutate: func(p *models.Project) { p.Comments = p.Comments[:2] }},
	} {
		t.Run(test.name, func(t *testing.T) {
			changed := peekCommentedProject("alpha")
			test.mutate(changed)
			if got := peekProjectDocument(changed, 80).Fingerprint; got == base {
				t.Fatalf("%s left the fingerprint unchanged: %q", test.name, got)
			}
		})
	}
}

// TestTaskPeekKeepsItsCommentCount is the boundary this session did not cross.
// A project header shows comment bodies because nothing else on the Outliner
// does; a task's comments have the `;` panel standing behind their count, so the
// task document still counts them and names none.
func TestTaskPeekKeepsItsCommentCount(t *testing.T) {
	task := pagedPeekTask()
	task.Comments = []*models.Comment{{Text: "First decision"}, {Text: "Second decision"}}
	doc := inlinePeekDocumentFor(inlinePeekOptions{Task: task, Indent: "  ", Width: 80, Window: testPeekWindow()})

	body := strings.Join(strings.Fields(ansi.Strip(strings.Join(doc.Body, "\n"))), " ")
	if !strings.Contains(body, "Comments: 2") {
		t.Fatalf("task Peek dropped its comment count:\n%s", body)
	}
	for _, comment := range task.Comments {
		if strings.Contains(body, comment.Text) {
			t.Fatalf("task Peek named comment body %q:\n%s", comment.Text, body)
		}
	}
}
