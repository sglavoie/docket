package tui

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/taskdetail"
)

// This file asks the one question the panel's own tests cannot: does the
// recovery the Comments panel renders survive the shell that crops it?
//
// Every other draft-recovery test mounts the panel directly and reads its View,
// which is a surface the terminal never shows. AppModel clips a route view to
// the rows left under the status strip, so a panel that draws a refusal, a
// recovery block, the whole of a long draft's excerpt, a footer, and then the
// line naming the file it just wrote is a panel whose last rows are cut off the
// screen — and the line that gets cut is the only thing telling the user where
// their unsaved text went. So the frames read here are the composed
// AppModel.View at real terminal sizes, and the export path is taken from those
// frames rather than from the panel's status field.

// The draft below is what a recovery has to hand back whole. Its head survives
// any truncation, so finding it proves nothing; the tail marker sits past
// CommentCharLimit and is what the quick-entry composer drops first, and every
// line carries an interior tab and a non-ASCII rune so the composer's sanitizer
// has something to destroy the whole way down.
const (
	recoveryGeometryHead       = "HEAD-MARKER-5d18 héllo→ première ligne"
	recoveryGeometryTailMarker = "TAIL-MARKER-5d18"
	recoveryGeometryTail       = recoveryGeometryTailMarker + "\tréservé\tà la récupération ✓"
	recoveryGeometryFiller     = "remplissage ünicode\twith an interior tab\n"
)

// recoveryGeometryCommentID is the stored comment the three edit cases open. It
// is fixed rather than minted so the seeding stays a plain document write.
const recoveryGeometryCommentID = "5d18a3c7-6b2f-4e91-8a04-72c1de5f9b30"

// recoveryGeometryNotice and recoveryGeometryHint are the two sentences the
// recovery block owes a reader, spelled here as literals rather than borrowed
// from taskdetail: what the shell shows is a user-facing contract, and a test
// that imported the constants would keep passing if both sides changed together
// and the screen stopped saying anything useful.
const (
	recoveryGeometryNotice = "The box above cannot show all of this draft. The complete text is kept and is what a save writes"
	recoveryGeometryHint   = "The box is a read-only preview; Ctrl+X writes the whole draft to a file."
)

// recoveryGeometryDraft builds a draft long enough that the composer cannot hold
// it: a head, filler past the rune limit, and a tail marker that therefore
// exists nowhere the box can show.
func recoveryGeometryDraft() string {
	var b strings.Builder
	b.WriteString(recoveryGeometryHead + "\n")
	for len([]rune(b.String())) <= taskdetail.CommentCharLimit {
		b.WriteString(recoveryGeometryFiller)
	}
	b.WriteString(recoveryGeometryTail)
	return b.String()
}

// recoveryGeometrySize is one supported terminal. Both are small enough that the
// panel has to choose what to draw, which is the whole point: 120x24 is an
// ordinary window and 60x20 is the narrow one where the prose wraps and the
// excerpt has to yield its rows entirely.
type recoveryGeometrySize struct {
	name          string
	width, height int
	// roomy marks the size at which nothing may have been given up: the drop
	// ladder exists for the narrow terminal, and a rung spent at 120x24 would be
	// the panel hiding something it had room for.
	roomy bool
}

var recoveryGeometrySizes = []recoveryGeometrySize{
	{name: "120x24", width: 120, height: 24, roomy: true},
	{name: "60x20", width: 60, height: 20},
}

// recoveryGeometryCase is one way a draft becomes unsavable, plus what the
// screen owes the user in that state.
type recoveryGeometryCase struct {
	name string
	// lossy reports whether the composer holds less than the draft, which is
	// what makes the recovery block appear at all.
	lossy bool
	// refusal is the substring of the error the panel must be showing beside the
	// draft, or empty where the draft is merely too long to display.
	refusal string
	// seat drives the app into the state and returns the bytes a recovery must
	// write, which is the draft the panel is authoritative for.
	seat func(t *testing.T, driver *UITestDriver, draft string) string
}

// TestProjectCommentRecoveryFitsShellViewport is the geometry contract for
// comment draft recovery.
//
// A recovery is only a recovery if the user can act on it, and every part of it
// competes for the same rows: the refusal explaining why the save failed, the
// block explaining that the box is not the whole draft, the excerpt proving the
// draft survived, the footer naming the keys, and the line naming the file the
// export wrote. The panel cannot spend more rows than the shell will show, so
// each case below presses the advertised key through ordinary input dispatch and
// then requires the composed frame — not the panel's View, not its status field
// — to fit the terminal and still name a file that really holds the draft.
func TestProjectCommentRecoveryFitsShellViewport(t *testing.T) {
	draft := recoveryGeometryDraft()

	cases := []recoveryGeometryCase{
		{
			// The plainest lossy state and the only one with no refusal: the
			// stored comment is simply longer than the composer, so the recovery
			// block is competing with nothing but the panel's own chrome.
			name:  "stored_edit",
			lossy: true,
			seat: func(t *testing.T, driver *UITestDriver, draft string) string {
				t.Helper()
				seedRecoveryGeometryComment(t, driver, draft)
				openRecoveryGeometryEdit(t, driver)
				return draft
			},
		},
		{
			// The add composer over a vanished owner. Nothing retains a lossy add
			// draft — the retention points all seat text the box already holds —
			// so what is proven here is the other half of the contract: a refusal
			// and an export destination both stay on screen, and the file holds
			// the draft the panel is authoritative for, byte for byte.
			name:    "missing_owner_add",
			refusal: "no longer available",
			seat: func(t *testing.T, driver *UITestDriver, draft string) string {
				t.Helper()
				seatCursorOnProjectHeader(t, driver.App().crossOutliner)
				runChecklistActions(t, driver, []UITestAction{
					{Type: UITestActionKey, Key: ";"},
					{Type: UITestActionKey, Key: "a"},
				})
				panel := mountedProjectCommentsPanel(t, driver.App())
				panel.TextInput.SetValue(draft)
				want := strings.TrimSpace(panel.TextInput.Value())
				if want == draft {
					t.Fatal("the composer took the whole draft, so this case proves nothing about a draft it cannot hold")
				}
				removeRecoveryGeometryOwner(t, driver)
				refuseRecoveryGeometrySave(t, driver)
				return want
			},
		},
		{
			// The same loss under the edit composer, where the recovery block and
			// the refusal are both on screen and both have to survive the crop.
			name:    "missing_owner_edit",
			lossy:   true,
			refusal: "no longer available",
			seat: func(t *testing.T, driver *UITestDriver, draft string) string {
				t.Helper()
				seedRecoveryGeometryComment(t, driver, draft)
				openRecoveryGeometryEdit(t, driver)
				removeRecoveryGeometryOwner(t, driver)
				refuseRecoveryGeometrySave(t, driver)
				return draft
			},
		},
		{
			// The owner is still there and the comment is not, so the panel adds
			// the orphaned-edit notice above everything else — the tallest of the
			// four states and the one a fixed layout clips first.
			name:    "deleted_comment_edit",
			lossy:   true,
			refusal: "deleted externally",
			seat: func(t *testing.T, driver *UITestDriver, draft string) string {
				t.Helper()
				seedRecoveryGeometryComment(t, driver, draft)
				openRecoveryGeometryEdit(t, driver)
				deleteRecoveryGeometryComment(t, driver)
				refuseRecoveryGeometrySave(t, driver)
				return draft
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			for _, size := range recoveryGeometrySizes {
				t.Run(size.name, func(t *testing.T) {
					runRecoveryGeometryCase(t, testCase, size, draft)
				})
			}
		})
	}
}

// runRecoveryGeometryCase is one case at one terminal size, start to finish: a
// fresh workspace, the state, the recovery press, and then the three readings
// that have to agree — the first frame, the frame after a redraw, and the frame
// after the terminal has changed size and changed back.
func runRecoveryGeometryCase(t *testing.T, testCase recoveryGeometryCase, size recoveryGeometrySize, draft string) {
	t.Helper()
	driver, workspace, _ := addCommentFixtureApp(t)
	tempDir := recoveryGeometryTempDir(t)

	driver.Resize(size.width, size.height)
	want := testCase.seat(t, driver, draft)
	if want == "" {
		t.Fatal("the case seated no draft")
	}

	before := projectCommentsOnDisk(t, workspace)
	runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: "ctrl+x"}})

	// The first reading is the one the defect was about: the press has just
	// happened and the answer to it is the last thing the panel drew.
	frame := assertRecoveryGeometryFrame(t, driver, size, testCase)
	path := recoveryGeometryExportPath(t, frame, tempDir)
	assertRecoveryGeometryFile(t, path, want)

	// A redraw is not an event. The panel has to keep saying where the draft went
	// for as long as the user has not asked it something else.
	if got := recoveryGeometryExportPath(t, assertRecoveryGeometryFrame(t, driver, size, testCase), tempDir); got != path {
		t.Fatalf("a redraw changed the export destination from %q to %q", path, got)
	}

	// A resize is an event, and it is the one that re-lays out every row the
	// recovery competes for. The other supported size has to fit too, and coming
	// back has to leave the answer where it was.
	for _, other := range recoveryGeometrySizes {
		if other.name == size.name {
			continue
		}
		driver.Resize(other.width, other.height)
		if got := recoveryGeometryExportPath(t, assertRecoveryGeometryFrame(t, driver, other, testCase), tempDir); got != path {
			t.Fatalf("resizing to %s changed the export destination from %q to %q", other.name, path, got)
		}
	}
	driver.Resize(size.width, size.height)
	if got := recoveryGeometryExportPath(t, assertRecoveryGeometryFrame(t, driver, size, testCase), tempDir); got != path {
		t.Fatalf("resizing back to %s changed the export destination from %q to %q", size.name, path, got)
	}
	assertRecoveryGeometryFile(t, path, want)

	// The reason the comment could not be saved is exactly as true after the
	// recovery as before it: the export asks the owner for nothing and writes
	// nothing to the workspace.
	assertRecoveryGeometryCommentsUnchanged(t, workspace, before)
	assertRecoveryGeometryDraftHeld(t, driver, tempDir, want)
}

// recoveryGeometryTempDir is where the export writes, which is what makes the
// file outlive the panel. Pointing the system temporary directory at one of this
// subtest's own keeps the run from leaving copies of the draft behind and gives
// the frame reader a prefix it can anchor the wrapped path on.
//
// It is deliberately not t.TempDir(): that name carries the test's own, and a
// path made long by this file's naming would be measuring the test rather than
// the panel. This one is the length a real generated path has.
func recoveryGeometryTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "tt-draft-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("TMPDIR", dir)
	return dir
}

// seedRecoveryGeometryComment stores the long draft as the fixture project's one
// comment, which is what makes `e` seat a draft the composer cannot hold: the
// edit composer is opened on the stored text, so a stored comment longer than
// CommentCharLimit is a recovered draft the panel reached by ordinary keys.
func seedRecoveryGeometryComment(t *testing.T, driver *UITestDriver, draft string) {
	t.Helper()
	project := recoveryGeometryProject(t, driver)
	project.Comments = []*models.ProjectComment{{
		ID:        recoveryGeometryCommentID,
		Text:      draft,
		CreatedAt: time.Date(2026, 3, 4, 9, 30, 0, 0, time.UTC),
	}}
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject(%q): %v", project.ID, err)
	}
}

// deleteRecoveryGeometryComment is another surface removing the comment while the
// edit composer is open on it, which is the state the panel refuses by name.
func deleteRecoveryGeometryComment(t *testing.T, driver *UITestDriver) {
	t.Helper()
	project := recoveryGeometryProject(t, driver)
	project.Comments = nil
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject(%q): %v", project.ID, err)
	}
}

// removeRecoveryGeometryOwner takes the project out from under the open composer.
// It has to happen after the composer is open, because the panel resolves its
// owner by identity on every reload and would never have mounted otherwise.
func removeRecoveryGeometryOwner(t *testing.T, driver *UITestDriver) {
	t.Helper()
	app := driver.App()
	if app.projectByID(projectCommentFixtureSlug) == nil {
		t.Fatalf("the shell never held project %q", projectCommentFixtureSlug)
	}
	app.domain.projects = nil
}

func recoveryGeometryProject(t *testing.T, driver *UITestDriver) *models.Project {
	t.Helper()
	project := driver.App().projectByID(projectCommentFixtureSlug)
	if project == nil {
		t.Fatalf("the shell does not hold project %q", projectCommentFixtureSlug)
	}
	return project
}

// openRecoveryGeometryEdit mounts the panel on the project header and opens the
// stored comment in the edit composer.
func openRecoveryGeometryEdit(t *testing.T, driver *UITestDriver) {
	t.Helper()
	seatCursorOnProjectHeader(t, driver.App().crossOutliner)
	runChecklistActions(t, driver, []UITestAction{
		{Type: UITestActionKey, Key: ";"},
		{Type: UITestActionKey, Key: "e"},
	})
	panel := mountedProjectCommentsPanel(t, driver.App())
	if !panel.EditMode {
		t.Fatalf("e did not open the edit composer:\n%s", driver.Render())
	}
	if panel.EditTextInput.Value() == recoveryGeometryDraft() {
		t.Fatal("the composer took the whole stored comment, so this case proves nothing about a draft it cannot hold")
	}
}

// refuseRecoveryGeometrySave spends the save the state cannot honor, which is
// what puts the refusal on screen beside the draft it protected.
func refuseRecoveryGeometrySave(t *testing.T, driver *UITestDriver) {
	t.Helper()
	runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: "ctrl+s"}})
}

// assertRecoveryGeometryFrame reads the composed shell frame and requires it to
// fit the terminal and to still be saying everything the state owes: the
// recovery instructions where the draft is lossy, the active refusal where there
// is one, the controls that end the composer, and the destination the export
// wrote to.
func assertRecoveryGeometryFrame(t *testing.T, driver *UITestDriver, size recoveryGeometrySize, testCase recoveryGeometryCase) string {
	t.Helper()
	observation := driver.Observe(ObserveOptions{})
	frame := observation.PlainScreen
	if len(observation.LayoutIssues) > 0 {
		t.Fatalf("the composed frame does not fit %s: %v\n%s", size.name, observation.LayoutIssues, frame)
	}
	if rows := len(strings.Split(strings.TrimRight(frame, "\n"), "\n")); rows > size.height {
		t.Fatalf("the composed frame is %d row(s) tall at %s:\n%s", rows, size.name, frame)
	}

	flat := flattenRecoveryGeometryFrame(frame)
	required := []string{
		// The line the whole defect was about.
		"Draft written to",
		// The two controls a composer may never be left without: one that keeps
		// the text and one that admits to discarding it.
		"Ctrl+S",
		"Esc",
	}
	if testCase.lossy {
		// Where the box is a preview, the sentence that says so also names the
		// key that hands the draft back. That one is owed at every width; the
		// explanation above it and the excerpt under it are what the ladder is
		// allowed to spend, so they are owed only where there were rows to spend.
		required = append(required, recoveryGeometryHint)
		if size.roomy {
			required = append(required, recoveryGeometryNotice, recoveryGeometryTailMarker)
		}
	} else {
		required = append(required, "Ctrl+X")
	}
	if testCase.refusal != "" {
		required = append(required, testCase.refusal)
	}
	for _, want := range required {
		if !strings.Contains(flat, want) {
			t.Fatalf("the composed frame at %s never says %q:\n%s", size.name, want, frame)
		}
	}
	return frame
}

// flattenRecoveryGeometryFrame turns the frame back into one line so a wrapped
// sentence can be looked for as the sentence it is. Wrapping replaces a space
// with a row break, so rejoining on a space restores the prose; only a token too
// long for the terminal is broken elsewhere, and none of the sentences asserted
// here carry one.
func flattenRecoveryGeometryFrame(frame string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(frame, "\n", " ")), " ")
}

// recoveryGeometryExportPathPattern is anchored on the subtest's own temporary
// directory because the path is the one piece of the frame that has no words:
// at a narrow width it is hard-broken across rows, so it is reassembled from the
// rows rather than read off one.
func recoveryGeometryExportPath(t *testing.T, frame, tempDir string) string {
	t.Helper()
	if !strings.Contains(flattenRecoveryGeometryFrame(frame), "Draft written to") {
		t.Fatalf("the composed frame names no file for the draft:\n%s", frame)
	}
	var joined strings.Builder
	for _, line := range strings.Split(frame, "\n") {
		joined.WriteString(strings.TrimSpace(line))
	}
	pattern := regexp.MustCompile(regexp.QuoteMeta(tempDir) + `/tt-comment-draft-\S*?\.md`)
	path := pattern.FindString(joined.String())
	if path == "" {
		t.Fatalf("the composed frame does not carry a usable export path under %s:\n%s", tempDir, frame)
	}
	return path
}

// assertRecoveryGeometryFile reads the file the frame named and requires the
// whole draft, byte for byte. Nothing else in this test can prove the recovery
// worked: the composer is short of the draft by construction and the excerpt is
// an excerpt, so the file is the only complete copy the user can reach.
func assertRecoveryGeometryFile(t *testing.T, path, want string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the file the frame named cannot be read: %v", err)
	}
	if string(contents) != want {
		t.Fatalf("the exported draft is not the draft the panel holds (%d rune(s) short):\ngot  %q\nwant %q",
			len([]rune(want))-len([]rune(string(contents))),
			recoveryGeometryTailOf(string(contents)), recoveryGeometryTailOf(want))
	}
	if !strings.Contains(string(contents), recoveryGeometryHead) {
		t.Fatalf("the exported draft is missing %q", recoveryGeometryHead)
	}
}

func recoveryGeometryTailOf(text string) string {
	runes := []rune(text)
	if len(runes) <= 80 {
		return text
	}
	return "…" + string(runes[len(runes)-80:])
}

// assertRecoveryGeometryCommentsUnchanged is the export's price: nothing. The
// recovery resolves no owner and submits no mutation, so the stored collection
// has to be exactly what it was before the press.
func assertRecoveryGeometryCommentsUnchanged(t *testing.T, workspace *uitestWorkspace, before []*models.ProjectComment) {
	t.Helper()
	after := projectCommentsOnDisk(t, workspace)
	if len(after) != len(before) {
		t.Fatalf("the recovery changed the stored comment count from %d to %d", len(before), len(after))
	}
	for i := range after {
		if after[i].ID != before[i].ID || after[i].Text != before[i].Text || !after[i].CreatedAt.Equal(before[i].CreatedAt) {
			t.Fatalf("the recovery rewrote stored comment %d", i)
		}
	}
}

// assertRecoveryGeometryDraftHeld is the other half: the export is a copy, not a
// handover. The composer is still open and the panel is still authoritative for
// the same bytes, so a user who exported by accident has lost nothing.
func assertRecoveryGeometryDraftHeld(t *testing.T, driver *UITestDriver, tempDir, want string) {
	t.Helper()
	panel := mountedProjectCommentsPanel(t, driver.App())
	if _, open := panelDraft(panel); !open {
		t.Fatalf("the recovery closed the composer:\n%s", driver.Render())
	}
	runChecklistActions(t, driver, []UITestAction{{Type: UITestActionKey, Key: "ctrl+x"}})
	frame := driver.Observe(ObserveOptions{}).PlainScreen
	again := recoveryGeometryExportPath(t, frame, tempDir)
	assertRecoveryGeometryFile(t, again, want)
}
