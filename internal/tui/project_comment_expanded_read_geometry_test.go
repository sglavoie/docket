package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// This file asks of the expanded comment reader the question the recovery
// geometry file asks of the composer: does what the panel draws survive the
// shell that crops it?
//
// The reader is the one place a comment is promised whole. Everything else
// abbreviates — the list shows an excerpt, Peek shows a window of a document
// built for a row — so a reader that renders the entire body and then lets
// AppModel.View cut it off at the bottom of the terminal is worse than an
// excerpt: it looks complete. The frames read here are therefore the composed
// AppModel.View at real terminal sizes, reached by ordinary key presses, and
// the body is longer than either terminal by construction.

// The comment below is what the reader has to hand back whole. Each paragraph
// names itself, so a frame can be asked which part of the body it is showing
// rather than merely whether it is showing something; the tail marker sits past
// the last of them and is what a reader that cannot scroll never reaches.
const (
	expandedReadParagraphs = 24
	expandedReadTailMarker = "QUEUE-FINALE-7f2a"
	expandedReadTail       = expandedReadTailMarker + " fin de la lecture ✓"
)

// expandedReadCommentID is the stored comment the reader opens. It is fixed
// rather than minted so the seeding stays a plain document write.
const expandedReadCommentID = "7f2a1b6d-3c45-4e08-9a72-5d0e1c83b4af"

// expandedReadCreatedAt is the instant the reader's header prints. Pinning it
// keeps the header a constant the test can look for.
var expandedReadCreatedAt = time.Date(2026, 5, 18, 14, 5, 0, 0, time.UTC)

// expandedReadMarker is one paragraph's name. Zero-padding keeps `PARA-02` from
// matching inside `PARA-020`, which nothing generates today but which a later
// widening of the count would introduce silently.
func expandedReadMarker(index int) string {
	return fmt.Sprintf("PARA-%02d", index)
}

// expandedReadComment builds the body: one self-naming paragraph per marker,
// blank-line separated so Markdown keeps them apart, then the tail. Every line
// carries non-ASCII runes, because the reader measures a rendered width and a
// body of plain ASCII would never exercise that.
//
// The whole thing stays under CommentCharLimit: this is a comment a user could
// have typed into the panel, not one only a document write can produce.
func expandedReadComment() string {
	var b strings.Builder
	for index := 1; index <= expandedReadParagraphs; index++ {
		fmt.Fprintf(&b, "« %s » ligne de lecture ünicode → repère\n\n", expandedReadMarker(index))
	}
	b.WriteString(expandedReadTail)
	return b.String()
}

// expandedReadSize is one supported terminal. Both are short enough that the
// body cannot fit, which is the whole premise; 60x20 is also narrow enough that
// the guidance under the window wraps and competes for the rows the body wants.
type expandedReadSize struct {
	name          string
	width, height int
}

var expandedReadSizes = []expandedReadSize{
	{name: "120x24", width: 120, height: 24},
	{name: "60x20", width: 60, height: 20},
}

// expandedReadWalkLimit bounds every traversal. A reader that stops advancing
// before the end would otherwise spin: the loops here end when a press stops
// changing the frame, and this is what turns a reader that never changes it into
// a failure instead of a hang.
const expandedReadWalkLimit = 400

// TestProjectCommentExpandedReadReachesWholeComment is the geometry contract for
// the expanded comment reader.
//
// A reader is only a reader if every line can be reached, and the rows it has to
// reach them in are not its own: the shell crops the panel to what is left under
// the status strip, so a body drawn past that line is drawn nowhere. Each size
// below opens a body longer than the terminal through `;` then Enter and then
// asks the composed frame — not the panel's View — to give up the whole comment
// one press at a time, in both directions, with the ends behaving like ends and
// the frame fitting the terminal at every step.
func TestProjectCommentExpandedReadReachesWholeComment(t *testing.T) {
	for _, size := range expandedReadSizes {
		t.Run(size.name, func(t *testing.T) {
			runExpandedReadCase(t, size)
		})
	}
}

// runExpandedReadCase is one terminal size, start to finish.
func runExpandedReadCase(t *testing.T, size expandedReadSize) {
	t.Helper()
	driver, workspace, _ := addCommentFixtureApp(t)
	driver.Resize(size.width, size.height)
	seedExpandedReadComment(t, driver)
	before := projectCommentsOnDisk(t, workspace)

	openExpandedReadComment(t, driver)
	identity := expandedReadIdentity(t, driver)

	// The premise. If the body fitted, every assertion below would pass on a
	// reader that does nothing at all.
	opening := expandedReadFrame(t, driver, size)
	if !strings.Contains(opening, expandedReadMarker(1)) {
		t.Fatalf("the reader does not open on the first paragraph at %s:\n%s", size.name, driver.Render())
	}
	if strings.Contains(opening, expandedReadTailMarker) {
		t.Fatalf("the body fits %s whole, so this case proves nothing about a reader that has to scroll", size.name)
	}

	// A redraw is not an event: nothing about looking again may move the window.
	if again := expandedReadFrame(t, driver, size); again != opening {
		t.Fatalf("a redraw moved the reader's window at %s", size.name)
	}

	// Down one row at a time, which is the finest the reader moves and therefore
	// the traversal that can skip nothing. Reaching the end is the clamp: the
	// walk stops when a press stops changing the frame.
	seen := expandedReadWalk(t, driver, size, "j")
	assertExpandedReadMarkersSeen(t, seen, size, "j")
	assertExpandedReadAtEnd(t, driver, size, "j")

	// Back up the same way, which has to give the body back a second time — a
	// reader that only traveled forward would have proven half a window.
	seen = expandedReadWalk(t, driver, size, "k")
	assertExpandedReadMarkersSeen(t, seen, size, "k")
	assertExpandedReadAtTop(t, driver, size, "k")

	// The paging keys cover the same ground in bigger steps, so the step may
	// never be bigger than the window: a page that skipped a row would lose it
	// with no way back to it except the single-row keys.
	for _, key := range []string{"ctrl+d", "pgdown", "ctrl+f"} {
		expandedReadToTop(t, driver)
		seen = expandedReadWalk(t, driver, size, key)
		assertExpandedReadMarkersSeen(t, seen, size, key)
		assertExpandedReadAtEnd(t, driver, size, key)
	}
	for _, key := range []string{"ctrl+u", "pgup", "ctrl+b"} {
		expandedReadToEnd(t, driver)
		seen = expandedReadWalk(t, driver, size, key)
		assertExpandedReadMarkersSeen(t, seen, size, key)
		assertExpandedReadAtTop(t, driver, size, key)
	}

	// The ends by name. Both spellings of each are the panel's own grammar, so
	// both are pressed rather than one waved through as an alias.
	for _, key := range []string{"G", "end"} {
		expandedReadToTop(t, driver)
		runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: key}})
		if frame := expandedReadFrame(t, driver, size); !strings.Contains(frame, expandedReadTailMarker) {
			t.Fatalf("%s does not reach the end of the comment at %s:\n%s", key, size.name, driver.Render())
		}
	}
	for _, keys := range [][]string{{"g", "g"}, {"home"}} {
		expandedReadToEnd(t, driver)
		for _, key := range keys {
			runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: key}})
		}
		frame := expandedReadFrame(t, driver, size)
		if !strings.Contains(frame, expandedReadMarker(1)) || strings.Contains(frame, expandedReadTailMarker) {
			t.Fatalf("%v does not return to the top of the comment at %s:\n%s", keys, size.name, driver.Render())
		}
	}

	// A resize is the one event that re-lays out every row the body competes
	// for. The other supported size has to fit too, and coming back has to leave
	// a reader who can still reach both ends.
	assertExpandedReadSurvivesResize(t, driver, size)

	// Closing and reopening is the identity half: the reader is open on one
	// comment, and neither control may leave the panel on another one.
	for _, key := range []string{"enter", "esc"} {
		expandedReadToEnd(t, driver)
		runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: key}})
		panel := mountedProjectCommentsPanel(t, driver.App())
		if panel.ExpandedView {
			t.Fatalf("%s did not close the reader at %s:\n%s", key, size.name, driver.Render())
		}
		if got := expandedReadIdentity(t, driver); got != identity {
			t.Fatalf("%s left the panel on %q rather than %q", key, got, identity)
		}
		runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: "enter"}})
		if got := expandedReadIdentity(t, driver); got != identity {
			t.Fatalf("reopening after %s landed on %q rather than %q", key, got, identity)
		}
		// Reopening starts at the top, because an offset kept across a close
		// would be a reader that opens mid-sentence on a comment nobody was
		// reading a moment ago.
		frame := expandedReadFrame(t, driver, size)
		if !strings.Contains(frame, expandedReadMarker(1)) {
			t.Fatalf("reopening after %s did not start at the top of the comment at %s:\n%s", key, size.name, driver.Render())
		}
	}

	// Reading is reading. Every press above resolves an owner and submits
	// nothing, so the stored collection is exactly what it was.
	assertExpandedReadCommentsUnchanged(t, workspace, before)
}

// seedExpandedReadComment stores the long body as the fixture project's one
// comment, which is what makes `;` then Enter open a reader on a body no
// terminal can hold.
func seedExpandedReadComment(t *testing.T, driver *UITestDriver) {
	t.Helper()
	project := driver.App().projectByID(projectCommentFixtureSlug)
	if project == nil {
		t.Fatalf("the shell does not hold project %q", projectCommentFixtureSlug)
	}
	body := expandedReadComment()
	if len([]rune(body)) > 2000 {
		t.Fatalf("the seeded comment is %d rune(s) long, past what the panel would accept", len([]rune(body)))
	}
	project.Comments = []*models.ProjectComment{{
		ID:        expandedReadCommentID,
		Text:      body,
		CreatedAt: expandedReadCreatedAt,
	}}
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject(%q): %v", project.ID, err)
	}
}

// openExpandedReadComment mounts the panel on the project header and expands the
// stored comment, by the two keys the guide names and nothing else.
func openExpandedReadComment(t *testing.T, driver *UITestDriver) {
	t.Helper()
	seatCursorOnProjectHeader(t, driver.App().crossOutliner)
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: ";"},
		{Type: UITestActionKey, Key: "enter"},
	})
	panel := mountedProjectCommentsPanel(t, driver.App())
	if !panel.ExpandedView {
		t.Fatalf("Enter did not expand the comment:\n%s", driver.Render())
	}
}
