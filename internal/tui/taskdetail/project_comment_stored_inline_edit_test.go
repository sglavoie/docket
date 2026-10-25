package taskdetail

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// storedCommentTail ends every comment below. It is the part a quick-entry
// textarea drops first and the part no case can retype from the screen, so a
// stored comment that still carries it is the shortest true statement that the
// bytes nobody saw came back.
const storedCommentTail = "STORED-TAIL-6a19 →end"

// storedInlineEditCreated is the fixture's older comment's instant; the comment
// under test is minted an hour later, so the panel draws it first and the
// timestamp is something a rewrite could visibly move.
var storedInlineEditCreated = time.Date(2026, 3, 4, 9, 0, 0, 0, time.UTC)

// storedInlineEditCase is one comment already in the workspace, and whether the
// quick-entry box can hold it whole.
//
// That second field is the axis the whole test turns on. A comment the box takes
// unchanged must stay an ordinary editable comment — the fix may not buy safety
// by making every edit read-only — while a comment it cannot take is a rendering
// of something bigger, and every route out of the composer has to write the
// bigger thing.
type storedInlineEditCase struct {
	name          string
	text          string
	representable bool
}

// storedInlineEditCases are the four ways a stored comment outruns the box, plus
// the control.
//
// They are separate cases because they are lost by separate mechanisms: the
// CharLimit truncation is counted in runes, so an ASCII comment and a multi-byte
// one reach it at different byte lengths; a tab is rewritten rather than dropped;
// and a control rune is dropped from a comment short enough that no limit is
// anywhere near it. A single long draft would exercise the first of those and
// pass while the other three still lost text.
func storedInlineEditCases() []storedInlineEditCase {
	return []storedInlineEditCase{
		{name: "long_ascii", text: strings.Repeat("x", CommentCharLimit+64) + "\n" + storedCommentTail},
		{name: "long_unicode", text: strings.Repeat("héllo→", CommentCharLimit/3) + "\n" + storedCommentTail},
		{name: "tabs", text: "column\tsecond\tthird\nindented\tvalue\n" + storedCommentTail},
		{name: "short_control", text: "alert\x07bell " + storedCommentTail},
		{name: "short_representable", text: "plain and short " + storedCommentTail, representable: true},
	}
}

// storedInlineEditFixture persists a project whose newest comment is the one
// under test and mounts a panel that has never seen anything else.
//
// The order matters and is the arrangement the finding describes: the comment is
// written to the workspace first, by something that is not this panel, and the
// panel is then opened over it. Nothing seats a draft, nothing refuses a commit,
// and no retention helper is called — the only thing that will put text into a
// composer here is the user pressing e over a row.
func storedInlineEditFixture(t *testing.T, text string) (*CommentsModel, *models.Project) {
	t.Helper()
	project, err := models.NewProject("project-1", "Comments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Comments = []*models.ProjectComment{
		{ID: olderCommentID, Text: "Older", CreatedAt: storedInlineEditCreated},
		{ID: newerCommentID, Text: text, CreatedAt: storedInlineEditCreated.Add(time.Hour)},
	}
	saveExternalChange(t, project)

	coordinators := newTestSecondaryCoordinators(project)
	model := newProjectCommentsPanel(project, coordinators.ProjectComment, func(id string) *models.Project {
		if project.ID == id {
			return project
		}
		return nil
	})
	model.SetSize(160, 40)

	// The premise, read back through storage rather than assumed: a comment the
	// workspace could not hold in the first place would make every assertion below
	// vacuous.
	assertStoredCommentText(t, project, text)
	return model, project
}

// assertStoredCommentText reads the project back through an object the panel has
// never held and names the whole collection: the edited comment's exact bytes,
// its minted identity, its untouched instant, and the neighbor beside it.
//
// All four are the contract together. A rewrite that preserved the text but
// re-minted the id, moved the instant, or reordered the collection would be a
// different comment wearing the right words.
func assertStoredCommentText(t *testing.T, project *models.Project, want string) {
	t.Helper()
	fresh, err := models.NewProject(project.ID, project.Name, project.Path)
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if len(fresh.Comments) != 2 {
		t.Fatalf("stored comments = %d, want the fixture's two", len(fresh.Comments))
	}

	older, newer := fresh.Comments[0], fresh.Comments[1]
	if older.ID != olderCommentID || older.Text != "Older" || !older.CreatedAt.Equal(storedInlineEditCreated) {
		t.Fatalf("the neighboring comment changed: %q %q %v", older.ID, older.Text, older.CreatedAt)
	}
	if newer.ID != newerCommentID {
		t.Fatalf("the edited comment was minted a new identity: %q", newer.ID)
	}
	if !newer.CreatedAt.Equal(storedInlineEditCreated.Add(time.Hour)) {
		t.Fatalf("the edited comment's instant moved to %v", newer.CreatedAt)
	}
	if newer.Text != want {
		t.Fatalf("the stored comment lost %d rune(s):\ngot  %q\nwant %q",
			len([]rune(want))-len([]rune(newer.Text)), tailOf(newer.Text), tailOf(want))
	}
}

// assertStoredInlineEditOpened enters the composer the way the key does and says
// what state that left the panel in, which is the fix itself.
//
// A comment the box cannot represent must be held verbatim beside it, previewed
// rather than buffered, and explained on screen — a preview the user is not told
// about is a truncation waiting to be blamed on them. A comment the box can hold
// must be none of those things: it is seated whole, so there is nothing to
// preview and nothing to warn about.
func assertStoredInlineEditOpened(t *testing.T, model *CommentsModel, testCase storedInlineEditCase) {
	t.Helper()
	model.Cursor = 0
	if cmd := model.StartEditSelectedComment(); cmd == nil {
		t.Fatal("e over the stored comment opened no composer")
	}
	if !model.EditMode {
		t.Fatal("the panel is not in edit mode after opening the stored comment")
	}
	assertRecoverable(t, model, commentMutationEdit, testCase.text)

	composer := model.composerValue(commentMutationEdit)
	if testCase.representable {
		if composer != testCase.text {
			t.Fatalf("the box did not take the whole comment, so it is the wrong control:\ngot  %q\nwant %q",
				tailOf(composer), tailOf(testCase.text))
		}
		if model.showsDraftPreview() {
			t.Fatal("a comment the box holds whole was made a read-only preview")
		}
		return
	}

	if composer == testCase.text {
		t.Fatalf("the box took %q whole, so this case proves nothing about unseen bytes", tailOf(testCase.text))
	}
	if !model.showsDraftPreview() {
		t.Fatal("the box is short of the stored comment and is not previewing it")
	}

	view := panelText(t, model)
	notice := strings.Index(view, recoveredDraftNotice)
	if notice < 0 {
		t.Fatalf("the panel never says the box is not showing the whole comment:\n%s", view)
	}
	block := view[notice:]
	if !strings.Contains(block, storedCommentTail) {
		t.Fatalf("the kept-text block does not reach the end of the stored comment:\n%s", block)
	}
	if !strings.Contains(block, recoveredDraftReadOnlyHint) {
		t.Fatalf("the kept-text block does not say the box above it is a preview:\n%s", block)
	}
}

// assertStoredCommentExports presses the advertised recovery key and reads the
// file the panel names, then asks the inline external-editor route the same
// question through the expression that builds its buffer.
//
// Both are ways out of a preview, and a way out that hands over the box's
// rendering instead of the comment is worse than no way out at all: the user
// would carry the truncation into the editor and save it back believing they had
// the whole thing.
func assertStoredCommentExports(t *testing.T, model *CommentsModel, want string) {
	t.Helper()
	deliverToPanel(t, model, ctrlX)
	view := panelText(t, model)
	if !strings.Contains(view, strings.TrimSpace(draftExportStatusPrefix)) {
		t.Fatalf("Ctrl+X named no file for the stored comment:\n%s", view)
	}
	path := exportedDraftPathFromView(view)
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the file Ctrl+X named cannot be read: %v", err)
	}
	if string(contents) != want {
		t.Fatalf("Ctrl+X wrote %d rune(s) short of the stored comment:\ngot  %q\nwant %q",
			len([]rune(want))-len([]rune(string(contents))), tailOf(string(contents)), tailOf(want))
	}

	// The exact expression the inline Ctrl+E route appends to its buffer.
	if got := model.submissionText(commentMutationEdit, model.composerValue(commentMutationEdit)); got != want {
		t.Fatalf("Ctrl+E would carry %q into the editor, want the whole stored comment %q", tailOf(got), tailOf(want))
	}
}

// TestProjectCommentStoredInlineEditPreservesFullText holds the opening half of
// the no-lost-comments contract.
//
// Every other retention in this panel exists because the text was somewhere no
// copy of it survived — an external editor's deleted temporary file, a
// submission a commit refused. A stored comment is the opposite: it is safe in
// the workspace, and pressing e is the one act that puts it somewhere it can be
// destroyed. The composer is a quick-entry textarea that stops at
// CommentCharLimit, rewrites tabs as four spaces, and drops control runes, and
// the save it feeds is measured against a precondition still naming the whole
// original — so the workspace accepts the shortened value as an ordinary edit
// and the missing bytes are gone with no refusal, no warning, and no undo of the
// user's own making. Ctrl+S with nothing typed is enough to do it.
//
// So each case below starts from a comment already on disk, opens the composer
// through the key's own entry point rather than by seating a draft, and requires
// the workspace to be holding the same bytes afterwards — through a save that
// changed nothing, through Ctrl+X and the editor buffer, and through two
// ordinary presses that must not be allowed to spend the invisible part of the
// comment on the visible one. The last case is the control: a comment the box
// does hold whole stays an ordinary editable comment, and the edit lands.
func TestProjectCommentStoredInlineEditPreservesFullText(t *testing.T) {
	// Ctrl+X writes into the system temporary directory. Pointing it at the test's
	// own keeps this run from leaving copies of the comments behind.
	t.Setenv("TMPDIR", t.TempDir())

	for _, testCase := range storedInlineEditCases() {
		t.Run(testCase.name, func(t *testing.T) {
			model, project := storedInlineEditFixture(t, testCase.text)
			assertStoredInlineEditOpened(t, model, testCase)
			assertStoredCommentExports(t, model, testCase.text)

			// A save with nothing typed. This is the finding's own reproduction, and
			// what it must now write is the comment that was already there.
			if _, ok := settle(t, model, deliverToPanel(t, model, ctrlS)).(CommentEditedMsg); !ok {
				t.Fatal("a no-edit save over the stored comment did not commit")
			}
			if model.EditMode {
				t.Fatal("the composer stayed open after the transaction confirmed")
			}
			if _, held := model.retainedDraftText(commentMutationEdit); held {
				t.Fatal("a confirmed commit kept the stored comment beside a closed composer")
			}
			assertStoredCommentText(t, project, testCase.text)

			// Leaving a comment nobody has touched is not a discard question. The box
			// being a preview is the panel's own doing, so asking here would be asking
			// about a truncation the user never made and has no way to decline.
			assertStoredInlineEditOpened(t, model, testCase)
			deliverToPanel(t, model, tea.KeyPressMsg{Code: tea.KeyEscape})
			if model.IsConfirmActive() {
				t.Fatal("Esc over an untouched stored comment asked whether to discard it")
			}
			if model.EditMode {
				t.Fatal("Esc left the composer open over an untouched stored comment")
			}
			assertStoredCommentText(t, project, testCase.text)

			// An ordinary edit: a backspace over the character the cursor sits on and a
			// rune typed in its place. Neither is an instruction to discard the rest.
			assertStoredInlineEditOpened(t, model, testCase)
			before := model.composerValue(commentMutationEdit)
			for _, press := range recoveredDraftEditPresses() {
				deliverToPanel(t, model, press)
			}

			want := testCase.text
			after := model.composerValue(commentMutationEdit)
			if testCase.representable {
				runes := []rune(testCase.text)
				want = string(runes[:len(runes)-1]) + "Z"
				if after != want {
					t.Fatalf("a comment the box holds whole stopped being editable:\ngot  %q\nwant %q",
						tailOf(after), tailOf(want))
				}
			} else {
				if after != before {
					t.Fatalf("the presses edited a read-only preview:\ngot  %q\nwant %q", tailOf(after), tailOf(before))
				}
				assertRecoverable(t, model, commentMutationEdit, testCase.text)
				assertVisible(t, model, draftPreviewReadOnlyStatus)
			}

			if _, ok := settle(t, model, deliverToPanel(t, model, ctrlS)).(CommentEditedMsg); !ok {
				t.Fatal("the save after an ordinary edit did not commit")
			}
			assertStoredCommentText(t, project, want)
		})
	}
}
