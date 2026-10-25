package taskdetail

import (
	"fmt"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// pendingSubmittedText is the draft every case below hands to a commit. It is
// short enough for the quick-entry box to hold whole and carries no tab, which is
// what keeps these cases about the commit window alone: the composer is an
// ordinary editable buffer right up to the moment its text is submitted, so a
// press refused after that is refused for that reason and no other.
const pendingSubmittedText = "the text the commit was handed"

// pendingWindowInput is what a user does while a save is in flight, spelled the
// way a terminal spells it. Four messages, each changing text a different way: a
// rune typed, a character deleted, the newline the composer binds itself rather
// than delegating, and a bracketed paste — which arrives as one message rather
// than a run of key presses, and is the one route that can add a whole paragraph
// in a single step.
func pendingWindowInput() []tea.Msg {
	return []tea.Msg{
		tea.KeyPressMsg{Code: 'Z', Text: "Z"},
		tea.KeyPressMsg{Code: tea.KeyBackspace},
		tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift},
		tea.PasteMsg{Content: "and the paragraph pasted after it"},
	}
}

// describePendingInput names one of those messages in a failure line, because
// which of the four got through is the whole diagnosis.
func describePendingInput(msg tea.Msg) string {
	switch typed := msg.(type) {
	case tea.KeyPressMsg:
		return typed.String()
	case tea.PasteMsg:
		return "a terminal paste"
	default:
		return fmt.Sprintf("%T", msg)
	}
}

// pendingInputCase is one owner, one act, and one outcome for the interval
// between a submission and its result.
type pendingInputCase struct {
	name string
	kind commentMutationKind
	// mount builds the panel with its composer already open over
	// pendingSubmittedText, and returns the reader that names what the workspace
	// actually holds. The reader reads storage rather than the fixture pointer: what
	// the panel wrote is the contract, and the fixture is refreshed by the snapshot
	// on every commit.
	mount func(t *testing.T) (*CommentsModel, func(*testing.T) []string)
	// refused says the commit comes back as a refusal rather than a confirmation.
	refused bool
	// wantPersisted is the stored collection once that outcome has landed, in
	// canonical storage order. It names every comment rather than a count, so a
	// submission that committed twice and one that committed something else are
	// both failures here.
	wantPersisted []string
}

// TestCommentsPendingInputDoesNotLoseDraftEdits closes the input window a
// commit-aware composer opens.
//
// A submission does not close the composer: the text stays on screen, focused,
// until the transaction answers. That is deliberate — a refusal has to have
// something to give back — but it leaves a box that looks like a buffer while its
// contents are already in flight. Nothing about either outcome can carry a
// keystroke made inside that interval: a confirmed commit closes the composer and
// resets it, and a refused one seats the string that was submitted. So text
// accepted there belongs to no outcome at all, and accepting it is the loss.
//
// The panel therefore refuses to accept it, and says so, for as long as the
// interval lasts. Each case below submits, holds the result, spends four
// text-changing messages on the composer through Update, and then delivers the
// result it held. What has to be true is the same eight times over: the composer
// never showed the input as accepted, the reason was on screen, the submitted
// string is what a save would still write, and exactly one comment reached
// storage saying exactly what was submitted.
func TestCommentsPendingInputDoesNotLoseDraftEdits(t *testing.T) {
	for _, tc := range pendingInputCases() {
		t.Run(tc.name, func(t *testing.T) {
			model, persisted := tc.mount(t)

			// The control for all of it: a composer previewing a kept draft is already
			// read-only for its own reason, so a case that opened one would prove
			// nothing about the pending window.
			if model.showsDraftPreview() {
				t.Fatal("the composer is previewing a kept draft, so its read-only refusal would not be the pending one")
			}
			composer := model.composerValue(tc.kind)
			if composer != pendingSubmittedText {
				t.Fatalf("the composer opened holding %q, want %q", composer, pendingSubmittedText)
			}

			cmd := deliverToPanel(t, model, ctrlS)
			if cmd == nil {
				t.Fatal("Ctrl+S raised no commit")
			}
			if model.pending == nil {
				t.Fatal("the submission left nothing pending, so there is no window to test")
			}

			// The transaction runs here and its result is held rather than delivered.
			// Everything below happens in the interval the panel is waiting through.
			result := cmd()

			for _, input := range pendingWindowInput() {
				deliverToPanel(t, model, input)
				press := describePendingInput(input)
				if got := model.composerValue(tc.kind); got != composer {
					t.Fatalf("%s was accepted into a composer whose text is in flight:\ngot  %q\nwant %q", press, got, composer)
				}
				if model.StatusMessage != pendingSubmissionReadOnlyStatus {
					t.Fatalf("%s was turned away without saying why: status = %q", press, model.StatusMessage)
				}
				assertVisible(t, model, pendingSubmissionReadOnlyStatus)
				if !model.HasTextInput() {
					t.Fatalf("%s closed the composer over the submitted draft", press)
				}
				if model.IsConfirmActive() {
					t.Fatalf("%s raised a question nobody asked for", press)
				}
				if model.pending == nil {
					t.Fatalf("%s cleared the submission the panel is waiting on", press)
				}
				if got := model.submissionText(tc.kind, model.composerValue(tc.kind)); got != pendingSubmittedText {
					t.Fatalf("after %s a save would write %q, want the submitted %q", press, got, pendingSubmittedText)
				}
			}

			// The same window refuses a second submission, so nothing that reaches
			// storage below can be a duplicate raised from inside it.
			if second := deliverToPanel(t, model, ctrlS); second != nil {
				t.Fatal("a second Ctrl+S inside the pending window raised a second commit")
			}

			model.Update(result)

			if tc.refused {
				msg, ok := result.(CommentMutationFailedMsg)
				if !ok {
					t.Fatalf("the commit resolved to %T, want a refusal", result)
				}
				if msg.Err == nil {
					t.Fatal("the refusal named no error")
				}
				if !model.HasTextInput() {
					t.Fatal("a refused save closed the composer over the draft")
				}
				if got := model.composerValue(tc.kind); got != pendingSubmittedText {
					t.Fatalf("a refused save left the composer holding %q, want %q", got, pendingSubmittedText)
				}
				assertRecoverable(t, model, tc.kind, pendingSubmittedText)
				if model.Err == nil {
					t.Fatal("a refused save left no reason on the panel")
				}
			} else {
				switch result.(type) {
				case CommentAddedMsg, CommentEditedMsg:
				default:
					t.Fatalf("the commit resolved to %T, want a confirmation", result)
				}
				if model.HasTextInput() {
					t.Fatal("the composer stayed open after the transaction confirmed")
				}
				if model.Err != nil {
					t.Fatalf("a confirmed commit left the error %v", model.Err)
				}
			}
			if model.pending != nil {
				t.Fatal("the outcome left the submission outstanding")
			}

			if got := persisted(t); !slices.Equal(got, tc.wantPersisted) {
				t.Fatalf("stored comments = %q, want %q", got, tc.wantPersisted)
			}
		})
	}
}

// pendingInputCases spans the two owners, the two composers, and the two
// outcomes, because the loss the refusal prevents is written twice — once where a
// confirmation closes the composer and once where a refusal reseats the submitted
// string — and both halves exist for a task comment and a project comment alike.
func pendingInputCases() []pendingInputCase {
	return []pendingInputCase{
		{
			name: "project_add_success",
			kind: commentMutationAdd,
			mount: func(t *testing.T) (*CommentsModel, func(*testing.T) []string) {
				return projectPendingPanel(t, commentMutationAdd, false)
			},
			wantPersisted: []string{"Older", "Newer", pendingSubmittedText},
		},
		{
			name: "project_add_failure",
			kind: commentMutationAdd,
			mount: func(t *testing.T) (*CommentsModel, func(*testing.T) []string) {
				return projectPendingPanel(t, commentMutationAdd, true)
			},
			refused:       true,
			wantPersisted: []string{"Older", "Newer"},
		},
		{
			name: "project_edit_success",
			kind: commentMutationEdit,
			mount: func(t *testing.T) (*CommentsModel, func(*testing.T) []string) {
				return projectPendingPanel(t, commentMutationEdit, false)
			},
			// The panel presents a project's comments newest first, so the row the
			// cursor opens is the one stored last.
			wantPersisted: []string{"Older", pendingSubmittedText},
		},
		{
			name: "project_edit_failure",
			kind: commentMutationEdit,
			mount: func(t *testing.T) (*CommentsModel, func(*testing.T) []string) {
				return projectPendingPanel(t, commentMutationEdit, true)
			},
			refused:       true,
			wantPersisted: []string{"Older", "Newer"},
		},
		{
			name: "task_add_success",
			kind: commentMutationAdd,
			mount: func(t *testing.T) (*CommentsModel, func(*testing.T) []string) {
				return taskPendingPanel(t, commentMutationAdd, false)
			},
			wantPersisted: []string{"stored", pendingSubmittedText},
		},
		{
			name: "task_add_failure",
			kind: commentMutationAdd,
			mount: func(t *testing.T) (*CommentsModel, func(*testing.T) []string) {
				return taskPendingPanel(t, commentMutationAdd, true)
			},
			refused:       true,
			wantPersisted: []string{"stored"},
		},
		{
			name: "task_edit_success",
			kind: commentMutationEdit,
			mount: func(t *testing.T) (*CommentsModel, func(*testing.T) []string) {
				return taskPendingPanel(t, commentMutationEdit, false)
			},
			wantPersisted: []string{pendingSubmittedText},
		},
		{
			name: "task_edit_failure",
			kind: commentMutationEdit,
			mount: func(t *testing.T) (*CommentsModel, func(*testing.T) []string) {
				return taskPendingPanel(t, commentMutationEdit, true)
			},
			refused:       true,
			wantPersisted: []string{"stored"},
		},
	}
}

// projectPendingPanel mounts the project panel over a real service with its
// composer open on the draft about to be submitted.
//
// The refusal is arranged by archiving the project rather than by withholding the
// owner, because the panel's own preconditions have to pass: a mutation refused
// before it is submitted never becomes pending, and the interval this test is
// about would never open.
func projectPendingPanel(t *testing.T, kind commentMutationKind, archived bool) (*CommentsModel, func(*testing.T) []string) {
	t.Helper()
	model, project := newTestProjectCommentsPanel(t, projectCommentsFixture(t))
	model.SetSize(120, 40)
	openPendingComposer(t, model, kind)
	project.Archived = archived
	return model, func(t *testing.T) []string {
		t.Helper()
		texts := make([]string, 0, len(persistedProjectComments(t, project)))
		for _, comment := range persistedProjectComments(t, project) {
			texts = append(texts, comment.Text)
		}
		return texts
	}
}

// taskPendingPanel does the same over a task owner. Its refusal comes from
// coordinators that know nothing about the project, which is this package's
// standing way to make a task commit fail at the application layer; the fixture is
// written to storage first so the reader below has a document to read whether or
// not the commit reaches one.
func taskPendingPanel(t *testing.T, kind commentMutationKind, refused bool) (*CommentsModel, func(*testing.T) []string) {
	t.Helper()
	project, task := commentsFixture(t, "stored")
	saveExternalChange(t, project)
	model := newTestCommentsPanel(task, project)
	if refused {
		model = newRefusingCommentsPanel(task, project)
	}
	model.SetSize(120, 40)
	openPendingComposer(t, model, kind)
	return model, func(t *testing.T) []string {
		t.Helper()
		return persistedTaskCommentTexts(t, project)
	}
}

// openPendingComposer opens one composer the way a key press opens it and seats
// the text the submission will carry.
func openPendingComposer(t *testing.T, model *CommentsModel, kind commentMutationKind) {
	t.Helper()
	switch kind {
	case commentMutationAdd:
		if model.BeginAddMode() == nil {
			t.Fatal("the add composer did not open")
		}
		model.TextInput.SetValue(pendingSubmittedText)
	case commentMutationEdit:
		model.Cursor = 0
		if model.StartEditSelectedComment() == nil {
			t.Fatal("the edit composer did not open")
		}
		model.EditTextInput.SetValue(pendingSubmittedText)
	case commentMutationRemove:
		t.Fatal("a removal opens no composer, so it has no input window")
	}
}

// persistedTaskCommentTexts reads the fixture's task back from storage through an
// object nothing in the test has ever held, for the same reason
// persistedProjectComments does.
func persistedTaskCommentTexts(t *testing.T, project *models.Project) []string {
	t.Helper()
	fresh, err := models.NewProject(project.ID, project.Name, project.Path)
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if len(fresh.Tasks) != 1 {
		t.Fatalf("the fixture project stored %d task(s), want exactly one", len(fresh.Tasks))
	}
	texts := make([]string, 0, len(fresh.Tasks[0].Comments))
	for _, comment := range fresh.Tasks[0].Comments {
		texts = append(texts, comment.Text)
	}
	return texts
}
