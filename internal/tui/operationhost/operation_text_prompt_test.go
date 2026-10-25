package operationhost

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/operation"
)

// newCommentPrompt builds the multiline prompt the Outliner's `C` mounts, with
// the Comments panel's own bound restated only here so the test does not import
// a layer sibling to read it.
func newCommentPrompt(t *testing.T) *OperationTextPromptModel {
	t.Helper()
	return NewOperationTextPromptModel(
		operation.OperationExecutionRequest{ID: operation.OperationTaskAddComment},
		"Add Comment", "Draft the launch runbook", "", "comment text",
		CommentTextPromptOptions(2000),
	)
}

// press replays one key and reports what the prompt made of it.
func press(model *OperationTextPromptModel, key string) (handled bool, msg tea.Msg) {
	_, handled, msg = model.HandleKey(keyPress(key))
	return handled, msg
}

// keyPress spells one key the way the shell delivers it. A single rune is typed
// input; anything longer is a named key.
func keyPress(key string) tea.KeyPressMsg {
	runes := []rune(key)
	if len(runes) == 1 {
		return tea.KeyPressMsg{Code: runes[0], Text: key}
	}
	switch key {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "ctrl+s":
		return tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	}
	panic("unhandled key " + key)
}

// typeText replays a string one key at a time, so the buffer is built by the
// same path a user builds it by.
func typeText(model *OperationTextPromptModel, text string) {
	for _, r := range text {
		if r == '\n' {
			model.HandleKey(keyPress("enter"))
			continue
		}
		model.HandleKey(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

// TestMultilineTextPromptTakesEnterAsANewlineAndSavesOnCtrlS is the whole reason
// Multiline exists. A comment's stored text holds newlines, so Enter has to
// write one rather than apply, and the apply has to move somewhere Enter is not.
func TestMultilineTextPromptTakesEnterAsANewlineAndSavesOnCtrlS(t *testing.T) {
	t.Parallel()

	model := newCommentPrompt(t)
	typeText(model, "first line\nsecond line")

	if handled, msg := press(model, "enter"); handled || msg != nil {
		t.Fatalf("enter = handled %v msg %T, want the buffer to keep it", handled, msg)
	}
	typeText(model, "third ##ctx @today #tag ✓ ~ref")

	handled, msg := press(model, "ctrl+s")
	if !handled {
		t.Fatal("ctrl+s did not apply")
	}
	selected, ok := msg.(operation.OperationValueSelectedMsg)
	if !ok {
		t.Fatalf("ctrl+s = %T, want OperationValueSelectedMsg", msg)
	}
	want := "first line\nsecond line\nthird ##ctx @today #tag ✓ ~ref"
	if selected.Value != want {
		t.Fatalf("value = %q, want %q", selected.Value, want)
	}
	if selected.Request.ID != operation.OperationTaskAddComment {
		t.Fatalf("request = %s, want the Add Comment request it was opened on", selected.Request.ID)
	}
}

// TestMultilineTextPromptSubmitsOnlyOnce is the double-submit guard, and it is
// the prompt's own rather than the shell's: a second Ctrl+S before the first
// commit resolves must produce nothing, so no second transaction can be opened
// against a value the user answered once.
func TestMultilineTextPromptSubmitsOnlyOnce(t *testing.T) {
	t.Parallel()

	model := newCommentPrompt(t)
	typeText(model, "only once")

	if handled, msg := press(model, "ctrl+s"); !handled || msg == nil {
		t.Fatalf("first ctrl+s = handled %v msg %T, want a selection", handled, msg)
	}
	for attempt := range 3 {
		handled, msg := press(model, "ctrl+s")
		if handled || msg != nil {
			t.Fatalf("repeat ctrl+s %d = handled %v msg %T, want nothing", attempt, handled, msg)
		}
	}

	// Restore is the retry, so it is also what re-arms the guard: the refused
	// text has to be submittable again or a failed save would be unrecoverable.
	model.Restore("only once", "storage refused it")
	if model.Notice() != "storage refused it" {
		t.Fatalf("notice = %q, want the refusal reason", model.Notice())
	}
	if handled, msg := press(model, "ctrl+s"); !handled || msg == nil {
		t.Fatalf("ctrl+s after Restore = handled %v msg %T, want a second selection", handled, msg)
	}
}

// TestMultilineTextPromptRefusesAnEmptyBufferInPlace keeps the refusal on the
// same terms the application layer uses: whitespace is not a comment, and a
// buffer holding only whitespace stays open with its notice instead of opening a
// transaction that would be rejected anyway.
func TestMultilineTextPromptRefusesAnEmptyBufferInPlace(t *testing.T) {
	t.Parallel()

	model := newCommentPrompt(t)
	typeText(model, "  \n \n ")

	handled, msg := press(model, "ctrl+s")
	if handled || msg != nil {
		t.Fatalf("ctrl+s on whitespace = handled %v msg %T, want a refusal in place", handled, msg)
	}
	if model.Notice() == "" {
		t.Fatal("a refused empty buffer said nothing")
	}
	if !strings.Contains(model.Render(80), model.Notice()) {
		t.Errorf("the notice is not on screen:\n%s", model.Render(80))
	}
}

// TestMultilineTextPromptConfirmsADiscardOnlyWhenThereIsSomethingToLose mirrors
// the Comments panel's own Esc: an untouched buffer closes, and a typed one asks
// first, because the text is the whole of what would be lost.
func TestMultilineTextPromptConfirmsADiscardOnlyWhenThereIsSomethingToLose(t *testing.T) {
	t.Parallel()

	empty := newCommentPrompt(t)
	emptyHandled, emptyMsg := press(empty, "esc")
	if !emptyHandled {
		t.Fatal("esc on an empty buffer did not close the prompt")
	}
	if _, ok := emptyMsg.(operation.OperationValueCanceledMsg); !ok {
		t.Fatalf("esc = %T, want OperationValueCanceledMsg", emptyMsg)
	}

	model := newCommentPrompt(t)
	typeText(model, "worth keeping")
	if handled, msg := press(model, "esc"); handled || msg != nil {
		t.Fatalf("esc on a typed buffer = handled %v msg %T, want the question first", handled, msg)
	}
	if !model.ConfirmingDiscard() {
		t.Fatal("esc did not arm the discard question")
	}
	screen := model.Render(80)
	if !strings.Contains(screen, "Discard comment?") {
		t.Errorf("the question is not drawn in the prompt's own box:\n%s", screen)
	}

	// n gives the buffer back untouched: the answer is "keep editing", not
	// "submit", so nothing may leak to the executor and nothing may be lost.
	if handled, msg := press(model, "n"); handled || msg != nil {
		t.Fatalf("n = handled %v msg %T, want the prompt back", handled, msg)
	}
	if model.ConfirmingDiscard() {
		t.Error("n left the question armed")
	}
	if model.Value() != "worth keeping" {
		t.Fatalf("value = %q, want the text n kept", model.Value())
	}

	press(model, "esc")
	confirmedHandled, confirmedMsg := press(model, "y")
	if !confirmedHandled {
		t.Fatal("y did not close the prompt")
	}
	if _, ok := confirmedMsg.(operation.OperationValueCanceledMsg); !ok {
		t.Fatalf("y = %T, want OperationValueCanceledMsg", confirmedMsg)
	}
}

// TestSingleLineTextPromptKeepsEnterAsItsApply pins the option boundary: the
// prompts that were here before Multiline existed must still apply on Enter and
// must still close outright on Esc, because none of them declares a discard
// question.
func TestSingleLineTextPromptKeepsEnterAsItsApply(t *testing.T) {
	t.Parallel()

	model := NewOperationTextPromptModel(
		operation.OperationExecutionRequest{ID: operation.OperationTaskSetResumeNote},
		"Set Resume Note", "Draft the launch runbook", "", "where you left off",
		ResumeNoteTextPromptOptions(),
	)
	typeText(model, "paused on the rollback step")

	handled, msg := press(model, "enter")
	if !handled {
		t.Fatal("enter did not apply on a single-line prompt")
	}
	selected, ok := msg.(operation.OperationValueSelectedMsg)
	if !ok {
		t.Fatalf("enter = %T, want OperationValueSelectedMsg", msg)
	}
	if selected.Value != "paused on the rollback step" {
		t.Fatalf("value = %q", selected.Value)
	}

	fresh := NewOperationTextPromptModel(
		operation.OperationExecutionRequest{ID: operation.OperationTaskSetResumeNote},
		"Set Resume Note", "", "typed", "", ResumeNoteTextPromptOptions(),
	)
	if handled, msg := press(fresh, "esc"); !handled {
		t.Fatalf("esc = handled %v msg %T, want an outright cancel", handled, msg)
	}
}
