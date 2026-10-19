package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

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
