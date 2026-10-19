package tui

import (
	"strings"
	"testing"

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
			// The add composer over a vanished owner, with only what the user
			// typed into the box: the composer capped the text on the way in, so
			// the panel is authoritative for exactly what it can show and the
			// recovery block has nothing to explain. What is proven here is the
			// other half of the contract — a refusal and an export destination
			// both stay on screen, and the file holds the draft the panel is
			// authoritative for, byte for byte. The lossy add draft is the state
			// only an editor can produce, and it has its own test below.
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

// TestProjectCommentRecoveredAddRecoveryFitsShellViewport is the same contract
// for the one state the cases above cannot reach by typing.
//
// An add composer caps what it accepts, so a draft it cannot hold never gets in
// through the keyboard. It gets in through the external editor: Ctrl+E hands the
// composer to $VISUAL, the user writes as much as they like, and what comes back
// is retained whole before anything is checked. If the project went away while
// the editor had the terminal, that retained draft is text no surface in the
// workspace holds and the box on screen cannot show — the one case where losing
// the export line loses the work itself.
//
// So this drives the real editor round trip through the real panel, takes the
// owner away in the gap the editor is up, and then asks the composed frame the
// same questions: does it fit, does it still say why the save failed and how to
// rescue the draft, and does the file it names hold the editor's result rather
// than the box's copy of it.
func TestProjectCommentRecoveredAddRecoveryFitsShellViewport(t *testing.T) {
	draft := recoveryGeometryDraft()
	testCase := recoveryGeometryCase{
		name:    "recovered_add",
		lossy:   true,
		refusal: "no longer available",
		seat:    seatRecoveryGeometryRecoveredAdd,
	}

	for _, size := range recoveryGeometrySizes {
		t.Run(size.name, func(t *testing.T) {
			runRecoveryGeometryCase(t, testCase, size, draft)
		})
	}
}
