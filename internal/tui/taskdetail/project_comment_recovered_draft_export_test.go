package taskdetail

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// The three markers below are the parts of one draft that a lossy recovery loses
// separately. The head survives anything, so finding it proves nothing; the tail
// is what a truncating composer drops first; and the middle is the one the panel
// can neither store, save, nor draw — it sits past CommentCharLimit and inside
// the rows the recovery excerpt elides, so the only thing that can hand it back
// is the recovery action itself.
const (
	exportDraftHead   = "HEAD-MARKER-9c31 héllo→ first line"
	exportDraftMiddle = "MIDDLE-MARKER-7b04\tunreachable\tinterior ✓"
	exportDraftTail   = "TAIL-MARKER-2e58 →last line"
)

// exportDraftFiller carries a tab and a non-ASCII rune on every line, so the
// sanitizer the composer applies has something to destroy the whole way down
// rather than only at the markers.
const exportDraftFiller = "padding ünicode\twith an interior tab\n"

// exportDraftText builds the external editor's result: a head, enough filler to
// push the middle marker past the composer's rune limit, the middle, more filler
// than the recovery excerpt's tail rows, and the tail. Its shape is the argument
// — every one of those pieces is a place a recovery can silently stop.
func exportDraftText() string {
	var b strings.Builder
	b.WriteString(exportDraftHead + "\n")
	for len([]rune(b.String())) <= CommentCharLimit {
		b.WriteString(exportDraftFiller)
	}
	b.WriteString(exportDraftMiddle + "\n")
	for range 8 {
		b.WriteString(exportDraftFiller)
	}
	b.WriteString(exportDraftTail)
	return b.String()
}

// ctrlX is the recovery press, delivered as a key the way the shell delivers one.
var ctrlX = tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl}

// TestProjectCommentRecoveredDraftCanBeExportedAfterTargetLoss is the
// user-reachable half of the no-lost-drafts contract.
//
// Retaining a draft the user cannot get to is not recovery. The panel keeps the
// editor's exact bytes, but everything that reads them is either refused or
// lossy once the target is gone: a save and an editor launch both stop at the
// missing owner or comment, the composer holds only CommentCharLimit runes with
// tabs rewritten, and the recovery block is a fixed excerpt with the middle
// elided. So each case below writes a draft whose middle exists nowhere on
// screen, presses the advertised key through Update, and reads the file the
// panel names — the whole of it, byte for byte — then does it again after the
// two events most likely to quietly end the retention. Nothing may be written to
// the workspace by any of it: the reason the comment could not be saved is
// exactly as true after the recovery as before it.
func TestProjectCommentRecoveredDraftCanBeExportedAfterTargetLoss(t *testing.T) {
	// The recovery writes into the system temporary directory, which is the point:
	// the file has to outlive the panel. Pointing that directory at the test's own
	// keeps this run from leaving copies of the draft behind.
	t.Setenv("TMPDIR", t.TempDir())
	text := exportDraftText()

	t.Run("deleted_comment", func(t *testing.T) {
		model, project := newTestProjectCommentsPanel(t, projectCommentsFixture(t))
		model.SetSize(160, 40)
		model.Cursor = 0
		ref, ok := model.commentRefAt(0)
		if !ok {
			t.Fatal("the panel drew no comment row")
		}

		// The editor was launched from the list, so the comment it was opened on is
		// removed by another surface while nothing is on screen to hold the result.
		project.Comments = project.Comments[:1]
		saveExternalChange(t, project)

		refusal := refusalText(t, deliverToPanel(t, model, editorResult(model, commentMutationEdit, ref, text)))
		if !strings.Contains(refusal, "deleted externally") {
			t.Fatalf("refusal = %q, want it to name the vanished target", refusal)
		}
		assertDraftExportSurvivesTargetLoss(t, model, project, commentMutationEdit, text, "deleted externally", "Older")
	})

	t.Run("missing_owner", func(t *testing.T) {
		// Both composers, because the owner is gone for both and the recovery is
		// advertised in both: an add has no comment to name and an edit names one
		// the panel can still see but no longer write.
		addModel, addProject := removedOwnerPanel(t)
		addModel.SetSize(160, 40)
		refusal := refusalText(t, deliverToPanel(t, addModel, editorResult(addModel, commentMutationAdd, CommentRef{}, text)))
		if !strings.Contains(refusal, "no longer available") {
			t.Fatalf("refusal = %q, want it to name the missing owner", refusal)
		}
		assertDraftExportSurvivesTargetLoss(t, addModel, addProject, commentMutationAdd, text, "no longer available", "Older", "Newer")

		editModel, editProject := removedOwnerPanel(t)
		editModel.SetSize(160, 40)
		ref, ok := editModel.commentRefAt(0)
		if !ok {
			t.Fatal("the panel drew no comment row")
		}
		refusal = refusalText(t, deliverToPanel(t, editModel, editorResult(editModel, commentMutationEdit, ref, text)))
		if !strings.Contains(refusal, "no longer available") {
			t.Fatalf("refusal = %q, want it to name the missing owner", refusal)
		}
		assertDraftExportSurvivesTargetLoss(t, editModel, editProject, commentMutationEdit, text, "no longer available", "Older", "Newer")
	})
}

// assertDraftExportSurvivesTargetLoss is the shared body of every case: prove
// the composer really is short of the draft, prove the key that recovers it is
// named on screen, and then spend that key three times — once on the refusal, once
// after a reload, and once after a second refusal — requiring the whole draft each
// time and requiring the workspace to be untouched by all of it.
func assertDraftExportSurvivesTargetLoss(
	t *testing.T,
	model *CommentsModel,
	project *models.Project,
	kind commentMutationKind,
	text string,
	retryRefusal string,
	stored ...string,
) {
	t.Helper()

	composer := model.TextInput.Value()
	if kind == commentMutationEdit {
		composer = model.EditTextInput.Value()
	}
	if strings.Contains(composer, exportDraftMiddle) {
		t.Fatal("the composer is holding the middle of the draft, so this case proves nothing about recovering what it drops")
	}
	if strings.Contains(composer, "\t") {
		t.Fatal("the composer kept a tab, so this case proves nothing about recovering what it rewrites")
	}
	if view := panelText(t, model); strings.Contains(view, exportDraftMiddle) {
		t.Fatalf("the middle of the draft is already on screen, so this case proves nothing about recovering it:\n%s", view)
	}
	assertVisible(t, model, recoveredDraftReadOnlyHint)

	// Every press names a file of its own, so a later one that quietly did nothing
	// cannot pass by handing back the file an earlier one wrote.
	written := map[string]bool{}
	assertExportedDraft(t, model, written, text)
	assertStoredComments(t, project, stored...)

	// A reload is not an answer to anything the user asked, and the draft is no
	// less irreplaceable after one.
	if !model.RefreshAfterReload() {
		t.Fatal("the refresh asked the shell to dismiss a panel holding a recovered draft")
	}
	assertExportedDraft(t, model, written, text)
	assertStoredComments(t, project, stored...)

	// The save is refused for the same reason it was refused the first time. The
	// refusal is what the recovery exists beside, so it has to survive the recovery
	// as intact as the recovery survives it.
	retry := refusalText(t, deliverToPanel(t, model, ctrlS))
	if !strings.Contains(retry, retryRefusal) {
		t.Fatalf("retry refusal = %q, want %q", retry, retryRefusal)
	}
	assertExportedDraft(t, model, written, text)
	assertVisible(t, model, retryRefusal)
	assertStoredComments(t, project, stored...)
	assertRecoverable(t, model, kind, text)
}

// assertExportedDraft presses the recovery key, reads the file the panel names
// on screen, and requires its bytes to be the editor's. The path is taken from
// the rendered view rather than from the model because a recovery the user cannot
// find is the defect this test exists for: if the panel stops saying where the
// draft went, this fails.
func assertExportedDraft(t *testing.T, model *CommentsModel, written map[string]bool, want string) {
	t.Helper()

	deliverToPanel(t, model, ctrlX)
	view := panelText(t, model)
	start := strings.Index(view, draftExportStatusPrefix)
	if start < 0 {
		t.Fatalf("the panel never named a file for the draft:\n%s", view)
	}
	line := view[start+len(draftExportStatusPrefix):]
	if end := strings.IndexByte(line, '\n'); end >= 0 {
		line = line[:end]
	}
	path := strings.TrimSpace(line)
	if path == "" {
		t.Fatalf("the panel named an empty path for the draft:\n%s", view)
	}
	if written[path] {
		t.Fatalf("the recovery reported %q again, so this press wrote nothing", path)
	}
	written[path] = true

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the file the panel named cannot be read: %v", err)
	}
	if string(contents) != want {
		t.Fatalf("the recovered draft is not the editor's result (%d rune(s) short):\ngot  %q\nwant %q",
			len([]rune(want))-len([]rune(string(contents))), tailOf(string(contents)), tailOf(want))
	}
	for _, marker := range []string{exportDraftHead, exportDraftMiddle, exportDraftTail} {
		if !strings.Contains(string(contents), marker) {
			t.Fatalf("the recovered draft is missing %q", marker)
		}
	}
}
