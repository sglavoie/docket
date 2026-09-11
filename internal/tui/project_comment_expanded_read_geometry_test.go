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

// expandedReadIdentity is the comment the panel is open on, read as the stored
// id rather than as a cursor index: an index is the same number whichever
// comment moved under it.
func expandedReadIdentity(t *testing.T, driver *UITestDriver) string {
	t.Helper()
	panel := mountedProjectCommentsPanel(t, driver.App())
	entries := panel.Entries()
	if panel.Cursor < 0 || panel.Cursor >= len(entries) {
		t.Fatalf("the panel's cursor is %d over %d entries", panel.Cursor, len(entries))
	}
	return entries[panel.Cursor].ID
}

// expandedReadFrame reads the composed shell frame and requires it to fit the
// terminal. This is the reading the defect was about: the panel's own View is a
// surface the terminal never shows.
func expandedReadFrame(t *testing.T, driver *UITestDriver, size expandedReadSize) string {
	t.Helper()
	observation := driver.Observe(ObserveOptions{})
	frame := observation.PlainScreen
	if len(observation.LayoutIssues) > 0 {
		t.Fatalf("the composed frame does not fit %s: %v\n%s", size.name, observation.LayoutIssues, frame)
	}
	if rows := len(strings.Split(strings.TrimRight(frame, "\n"), "\n")); rows > size.height {
		t.Fatalf("the composed frame is %d row(s) tall at %s:\n%s", rows, size.name, frame)
	}
	// The close controls are what a reader spends to get out of the reader, and
	// they are drawn under the body — which is exactly where a body given too
	// many rows would push them off the screen.
	flat := strings.Join(strings.Fields(strings.ReplaceAll(frame, "\n", " ")), " ")
	for _, want := range []string{"Enter", "Esc"} {
		if !strings.Contains(flat, want) {
			t.Fatalf("the composed frame at %s never names %q:\n%s", size.name, want, frame)
		}
	}
	return frame
}

// expandedReadWalk presses one key until it stops changing the frame, which is
// the reader admitting it has hit a boundary, and collects every paragraph the
// frames named along the way.
func expandedReadWalk(t *testing.T, driver *UITestDriver, size expandedReadSize, key string) map[string]bool {
	t.Helper()
	seen := map[string]bool{}
	previous := ""
	for press := 0; press < expandedReadWalkLimit; press++ {
		frame := expandedReadFrame(t, driver, size)
		collectExpandedReadMarkers(frame, seen)
		if frame == previous {
			return seen
		}
		previous = frame
		runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: key}})
	}
	t.Fatalf("%s never stopped moving the reader at %s after %d presses", key, size.name, expandedReadWalkLimit)
	return seen
}

func collectExpandedReadMarkers(frame string, seen map[string]bool) {
	for index := 1; index <= expandedReadParagraphs; index++ {
		if marker := expandedReadMarker(index); strings.Contains(frame, marker) {
			seen[marker] = true
		}
	}
	if strings.Contains(frame, expandedReadTailMarker) {
		seen[expandedReadTailMarker] = true
	}
}

// assertExpandedReadMarkersSeen is the contract in one line: every part of the
// body became visible, the tail included.
func assertExpandedReadMarkersSeen(t *testing.T, seen map[string]bool, size expandedReadSize, key string) {
	t.Helper()
	var missing []string
	for index := 1; index <= expandedReadParagraphs; index++ {
		if marker := expandedReadMarker(index); !seen[marker] {
			missing = append(missing, marker)
		}
	}
	if !seen[expandedReadTailMarker] {
		missing = append(missing, expandedReadTailMarker)
	}
	if len(missing) > 0 {
		t.Fatalf("traveling the comment with %s at %s never showed %s", key, size.name, strings.Join(missing, ", "))
	}
}

// assertExpandedReadAtEnd requires the far end to behave like an end: the tail is
// on screen and further presses of the key that got there change nothing.
func assertExpandedReadAtEnd(t *testing.T, driver *UITestDriver, size expandedReadSize, key string) {
	t.Helper()
	frame := expandedReadFrame(t, driver, size)
	if !strings.Contains(frame, expandedReadTailMarker) {
		t.Fatalf("%s stopped before the end of the comment at %s:\n%s", key, size.name, frame)
	}
	assertExpandedReadBoundaryHolds(t, driver, size, key, frame)
}

// assertExpandedReadAtTop is its mirror, and requires the same of the near end.
func assertExpandedReadAtTop(t *testing.T, driver *UITestDriver, size expandedReadSize, key string) {
	t.Helper()
	frame := expandedReadFrame(t, driver, size)
	if !strings.Contains(frame, expandedReadMarker(1)) {
		t.Fatalf("%s stopped before the start of the comment at %s:\n%s", key, size.name, frame)
	}
	assertExpandedReadBoundaryHolds(t, driver, size, key, frame)
}

// assertExpandedReadBoundaryHolds spends the key past the boundary. A reader
// whose offset kept counting there would come back a screenful at a time when
// the user turned around, which reads as a key that stopped working.
func assertExpandedReadBoundaryHolds(t *testing.T, driver *UITestDriver, size expandedReadSize, key, frame string) {
	t.Helper()
	for press := 0; press < 3; press++ {
		runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: key}})
		if got := expandedReadFrame(t, driver, size); got != frame {
			t.Fatalf("%s moved the reader past the boundary at %s:\n%s", key, size.name, got)
		}
	}
}

func expandedReadToTop(t *testing.T, driver *UITestDriver) {
	t.Helper()
	runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: "home"}})
}

func expandedReadToEnd(t *testing.T, driver *UITestDriver) {
	t.Helper()
	runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: "end"}})
}

// assertExpandedReadSurvivesResize takes the terminal to every other supported
// size and back, from the far end of the body — the position a resize is most
// likely to invalidate, since a shorter terminal shows fewer rows and therefore
// moves the end further down than the offset the reader is holding.
//
// What a resize owes is not that offset. A window measured once at mount would
// keep drawing rows the new terminal does not have, and an offset kept past the
// new end would leave the reader looking at blank rows it could not scroll off,
// so what is required here is the thing a reader actually needs: a frame that
// fits, a body still on it, and both ends still reachable.
func assertExpandedReadSurvivesResize(t *testing.T, driver *UITestDriver, size expandedReadSize) {
	t.Helper()
	for _, other := range expandedReadSizes {
		if other.name == size.name {
			continue
		}
		expandedReadToEnd(t, driver)
		driver.Resize(other.width, other.height)
		assertExpandedReadStillReads(t, driver, other)
	}
	expandedReadToEnd(t, driver)
	driver.Resize(size.width, size.height)
	assertExpandedReadStillReads(t, driver, size)
	expandedReadToTop(t, driver)
}

// assertExpandedReadStillReads is what "still reading" means at a size the
// reader was not opened at: the panel is the same one, the frame fits and is
// showing body rather than empty rows, and neither end has gone out of reach.
func assertExpandedReadStillReads(t *testing.T, driver *UITestDriver, size expandedReadSize) {
	t.Helper()
	if !mountedProjectCommentsPanel(t, driver.App()).ExpandedView {
		t.Fatalf("resizing to %s closed the reader:\n%s", size.name, driver.Render())
	}
	frame := expandedReadFrame(t, driver, size)
	showing := map[string]bool{}
	collectExpandedReadMarkers(frame, showing)
	if len(showing) == 0 {
		t.Fatalf("resizing to %s left the reader on no part of the comment:\n%s", size.name, frame)
	}
	expandedReadToEnd(t, driver)
	if got := expandedReadFrame(t, driver, size); !strings.Contains(got, expandedReadTailMarker) {
		t.Fatalf("the reader cannot reach the end of the comment at %s:\n%s", size.name, got)
	}
	expandedReadToTop(t, driver)
	if got := expandedReadFrame(t, driver, size); !strings.Contains(got, expandedReadMarker(1)) {
		t.Fatalf("the reader cannot reach the top of the comment at %s:\n%s", size.name, got)
	}
}

// assertExpandedReadCommentsUnchanged is reading's price: nothing.
func assertExpandedReadCommentsUnchanged(t *testing.T, workspace *uitestWorkspace, before []*models.ProjectComment) {
	t.Helper()
	after := projectCommentsOnDisk(t, workspace)
	if len(after) != len(before) {
		t.Fatalf("reading changed the stored comment count from %d to %d", len(before), len(after))
	}
	for index := range after {
		if after[index].ID != before[index].ID ||
			after[index].Text != before[index].Text ||
			!after[index].CreatedAt.Equal(before[index].CreatedAt) {
			t.Fatalf("reading rewrote stored comment %d", index)
		}
	}
}

// TestExpandedReadKeepsShortAndTaskOwnedCommentsWhole is the other half of the
// window: a body the terminal can hold must still be drawn whole, with no
// counter row claiming there is more, and the reader a task owns must behave
// exactly like the one a project owns. The bounded reader is one code path over
// both owners, and this is what keeps the bound from becoming a floor.
func TestExpandedReadKeepsShortAndTaskOwnedCommentsWhole(t *testing.T) {
	const shortComment = "Une seule ligne, lisible d'un coup — MARQUE-COURTE ✓"

	for _, size := range expandedReadSizes {
		t.Run(size.name+"/project_owner", func(t *testing.T) {
			driver, _, _ := addCommentFixtureApp(t)
			driver.Resize(size.width, size.height)
			project := driver.App().projectByID(projectCommentFixtureSlug)
			if project == nil {
				t.Fatalf("the shell does not hold project %q", projectCommentFixtureSlug)
			}
			project.Comments = []*models.ProjectComment{{
				ID:        expandedReadCommentID,
				Text:      shortComment,
				CreatedAt: expandedReadCreatedAt,
			}}
			if err := storage.SaveProject(project); err != nil {
				t.Fatalf("SaveProject(%q): %v", project.ID, err)
			}
			openExpandedReadComment(t, driver)
			assertExpandedReadShowsWhole(t, driver, size, shortComment)
		})

		t.Run(size.name+"/task_owner", func(t *testing.T) {
			driver, _, view := addCommentFixtureApp(t)
			driver.Resize(size.width, size.height)
			seatCursorOnTaskTitled(t, view, checklistFixturePlainTitle)
			runChecklistActions(t, driver, []UITestAction{
				{Type: UITestActionKey, Key: "C"},
				{Type: UITestActionText, Text: shortComment},
				{Type: UITestActionKey, Key: "ctrl+s"},
			})
			seatCursorOnTaskTitled(t, view, checklistFixturePlainTitle)
			runChecklistActions(t, driver, []UITestAction{
				{Type: UITestActionKey, Key: ";"},
				{Type: UITestActionKey, Key: "enter"},
			})
			panel := mountedProjectCommentsPanel(t, driver.App())
			if !panel.ExpandedView {
				t.Fatalf("Enter did not expand the task's comment:\n%s", driver.Render())
			}
			assertExpandedReadShowsWhole(t, driver, size, shortComment)
		})
	}
}

// assertExpandedReadShowsWhole requires a body the window can hold to be drawn
// whole and to say nothing about scrolling, and the reader to still close.
func assertExpandedReadShowsWhole(t *testing.T, driver *UITestDriver, size expandedReadSize, body string) {
	t.Helper()
	frame := expandedReadFrame(t, driver, size)
	flat := strings.Join(strings.Fields(strings.ReplaceAll(frame, "\n", " ")), " ")
	want := strings.Join(strings.Fields(body), " ")
	if !strings.Contains(flat, want) {
		t.Fatalf("the composed frame at %s does not show the whole comment:\n%s", size.name, frame)
	}
	if strings.Contains(flat, "of 1 ·") || strings.Contains(flat, "lines 1-1 of") {
		t.Fatalf("the reader counts rows for a comment that needed no scrolling at %s:\n%s", size.name, frame)
	}
	runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: "esc"}})
	if mountedProjectCommentsPanel(t, driver.App()).ExpandedView {
		t.Fatalf("Esc did not close the reader at %s:\n%s", size.name, driver.Render())
	}
}
