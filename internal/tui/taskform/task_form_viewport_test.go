package taskform

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

// formViewportItemCount is deliberately larger than any window the form can open
// at the minimum supported height, so every "reach the last one" assertion below
// is a genuine scroll rather than a body that happened to fit.
const formViewportItemCount = 12

// newLoadedTaskForm builds a form whose four tabs are all taller than the
// minimum-height window: every dynamic section is expanded and holds
// formViewportItemCount uniquely named drafts.
func newLoadedTaskForm() *TaskFormModel {
	m := newTestTaskForm()
	for i := range formViewportItemCount {
		m.draft.checklist = append(m.draft.checklist, models.ChecklistItem{Text: fmt.Sprintf("c%02d", i)})
		m.draft.subtasks = append(m.draft.subtasks, formSubtaskDraft{title: fmt.Sprintf("s%02d", i)})
		m.draft.attachments = append(m.draft.attachments, formAttachmentDraft{value: fmt.Sprintf("a%02d", i)})
		m.draft.comments = append(m.draft.comments, formCommentDraft{text: fmt.Sprintf("m%02d", i)})
	}
	m.checklistExpanded = true
	m.subtasksExpanded = true
	m.attachmentsExpanded = true
	m.commentsExpanded = true
	return m
}

// formFieldWitness is the text only the named field puts on screen, so an
// assertion that finds it has found that field and not its neighbor.
func formFieldWitness(tab, focus int) string {
	switch tab {
	case formTabBasics:
		return []string{"Title:", "Description:", "Priority:", "Parent:", "State:", "Flags:", "Tags:", "Context:"}[focus]
	case formTabSchedule:
		return []string{"Due:", "Next check-in:", "Repeat:"}[focus]
	case formTabChildren:
		switch {
		case focus == 0:
			return fmt.Sprintf("Checklist (%d)", formViewportItemCount)
		case focus <= formViewportItemCount:
			return fmt.Sprintf("c%02d", focus-1)
		case focus == formViewportItemCount+1:
			return fmt.Sprintf("Subtasks (%d)", formViewportItemCount)
		default:
			return fmt.Sprintf("s%02d", focus-formViewportItemCount-2)
		}
	case formTabExtras:
		switch {
		case focus == 0:
			return fmt.Sprintf("Attachments (%d)", formViewportItemCount)
		case focus <= formViewportItemCount:
			return fmt.Sprintf("a%02d", focus-1)
		case focus == formViewportItemCount+1:
			return fmt.Sprintf("Comments (%d)", formViewportItemCount)
		default:
			return fmt.Sprintf("m%02d", focus-formViewportItemCount-2)
		}
	}
	return ""
}

// TestTaskFormEveryFieldIsReachableAtMinimumHeight is the session's done-when in
// one assertion: at the form's floor, tabbing through a tab's fields must bring
// each one on screen, including the last draft of a twelve-item section and the
// expanded scheduling rows below Due.
func TestTaskFormEveryFieldIsReachableAtMinimumHeight(t *testing.T) {
	for _, tab := range []int{formTabBasics, formTabSchedule, formTabChildren, formTabExtras} {
		m := newLoadedTaskForm()
		m.SetSize(formMinWidth, formMinHeight)
		m.activeTab = tab
		m.focusIndex = 0
		m.syncInputFocus()

		count := m.fieldCountForActiveTab()
		for focus := range count {
			plain := ansi.Strip(m.View().Content)
			witness := formFieldWitness(tab, focus)
			if !strings.Contains(plain, witness) {
				t.Fatalf("tab %d focus %d: %q is off screen at %dx%d:\n%s",
					tab, focus, witness, formMinWidth, formMinHeight, plain)
			}
			m = sendFormKey(m, syntheticSpecialKey(tea.KeyTab))
		}
		if m.focusIndex != 0 {
			t.Fatalf("tab %d: traversing %d fields did not wrap back to the first: focus=%d",
				tab, count, m.focusIndex)
		}
	}
}

// TestTaskFormViewportTravelsBackwardToo covers Shift+Tab: the window has to
// follow the focus up as readily as down, or the backward half of the traversal
// walks off the top of a body it just scrolled.
func TestTaskFormViewportTravelsBackwardToo(t *testing.T) {
	m := newLoadedTaskForm()
	m.SetSize(formMinWidth, formMinHeight)
	m.activeTab = formTabExtras
	m.focusIndex = 0

	count := m.fieldCountForActiveTab()
	for step := range count {
		m = sendFormKey(m, syntheticShiftKey(tea.KeyTab))
		focus := (count - 1 - step) % count
		witness := formFieldWitness(formTabExtras, focus)
		if plain := ansi.Strip(m.View().Content); !strings.Contains(plain, witness) {
			t.Fatalf("shift+tab to focus %d: %q is off screen:\n%s", focus, witness, plain)
		}
	}
}

// TestTaskFormViewportLeavesAFittingBodyAlone keeps the window honest about
// costing nothing when it is not needed: a short tab must render without the
// position row, so the common case does not lose a row to a scroll it never does.
func TestTaskFormViewportLeavesAFittingBodyAlone(t *testing.T) {
	m := newTestTaskForm()
	m.SetSize(formMinWidth, formMinHeight)
	m.activeTab = formTabSchedule

	plain := ansi.Strip(m.View().Content)
	if strings.Contains(plain, "above ·") {
		t.Fatalf("a body that fits must not draw the position row:\n%s", plain)
	}
	for _, want := range []string{"Due:", "Next check-in:", "Repeat:"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("Schedule row %q missing:\n%s", want, plain)
		}
	}
}

// TestTaskFormViewportNamesWhatIsOffScreen asserts the replacement for the old
// "resize terminal to view all" notice: a windowed body says exactly how many
// rows sit in each direction, which is advice a reader can act on with Tab.
func TestTaskFormViewportNamesWhatIsOffScreen(t *testing.T) {
	m := newLoadedTaskForm()
	m.SetSize(formMinWidth, formMinHeight)
	m.activeTab = formTabChildren
	m.focusIndex = 0

	plain := ansi.Strip(m.View().Content)
	if strings.Contains(plain, "resize terminal to view all") {
		t.Fatalf("the form still tells a reader to resize instead of scrolling:\n%s", plain)
	}
	if !strings.Contains(plain, "0 above ·") {
		t.Fatalf("a windowed body must name what is off screen:\n%s", plain)
	}

	m.focusIndex = m.fieldCountForActiveTab() - 1
	tail := ansi.Strip(m.View().Content)
	if !strings.Contains(tail, "· 0 below") {
		t.Fatalf("the window at the last field must have nothing below it:\n%s", tail)
	}
}

// TestTaskFormValidationRefusalShowsBannerAndOffendingField covers the failure
// path the viewport exists for: a refusal seats the focus on the offending row
// and costs the body a row for the banner, so both have to be readable at once.
func TestTaskFormValidationRefusalShowsBannerAndOffendingField(t *testing.T) {
	env := newScenarioEnv(t, "viewport-refusal")
	m := scenarioCreateForm(t, env)
	m.SetSize(formMinWidth, formMinHeight)
	m.titleInput.SetValue("Ship release")
	m.contextInput.SetValue("!!!")

	if cmd := m.handleSave(); cmd != nil {
		t.Fatal("an invalid context must not reach the coordinator")
	}
	if m.err == nil {
		t.Fatal("an invalid context must report an error")
	}

	plain := ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "Context:") {
		t.Fatalf("the refused field is off screen:\n%s", plain)
	}
	if !strings.Contains(plain, "context") {
		t.Fatalf("the validation banner is off screen:\n%s", plain)
	}
	assertFitsWithin(t, m.View().Content, formMinWidth, formMinHeight)
}

// TestTaskFormViewportFollowsAResize checks the window is a function of the
// current height rather than a remembered offset: shrinking must not strand the
// focus, and growing must give the rows back.
func TestTaskFormViewportFollowsAResize(t *testing.T) {
	m := newLoadedTaskForm()
	m.activeTab = formTabChildren
	m.focusIndex = m.fieldCountForActiveTab() - 1

	m.SetSize(formMinWidth, formMinHeight)
	small := ansi.Strip(m.View().Content)
	if !strings.Contains(small, formFieldWitness(formTabChildren, m.focusIndex)) {
		t.Fatalf("the focused field is off screen after shrinking:\n%s", small)
	}

	m.SetSize(100, 40)
	large := ansi.Strip(m.View().Content)
	if strings.Contains(large, "above ·") {
		t.Fatalf("a body that fits after growing must not keep the position row:\n%s", large)
	}
	if !strings.Contains(large, formFieldWitness(formTabChildren, 0)) {
		t.Fatalf("growing the terminal lost the first field:\n%s", large)
	}
}

// TestTaskFormViewportReturnsFromANestedPicker checks that the overlay round
// trip leaves the window where the focus is: the overlay is chrome of its own
// and declares no fields, so the body underneath must be windowed again from the
// focus it is returned to, not from wherever it was before the picker opened.
func TestTaskFormViewportReturnsFromANestedPicker(t *testing.T) {
	m := newLoadedTaskForm()
	m.SetSize(formMinWidth, formMinHeight)
	m.activeTab = formTabExtras
	m.focusIndex = m.fieldCountForActiveTab() - 1
	last := formFieldWitness(formTabExtras, m.focusIndex)

	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter)) // edit the focused comment draft
	if m.activeOverlay != formOverlayCommentEdit {
		t.Fatalf("Enter on a comment draft opened overlay %d", m.activeOverlay)
	}
	if plain := ansi.Strip(m.View().Content); strings.Contains(plain, "above ·") {
		t.Fatalf("an overlay body must not carry the form's position row:\n%s", plain)
	}

	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEsc))
	if m.activeOverlay != formOverlayNone {
		t.Fatalf("Esc left overlay %d open", m.activeOverlay)
	}
	if plain := ansi.Strip(m.View().Content); !strings.Contains(plain, last) {
		t.Fatalf("returning from the picker stranded the focused draft %q:\n%s", last, plain)
	}
}

// TestTaskFormHelpPaneNeverStealsTheWindowsLastRow covers the one block the
// viewport cannot shorten: the help column is reference text beside the field
// being edited, so at a height where the two disagree the pane gives up its
// tail rather than the body giving up its position row.
func TestTaskFormHelpPaneNeverStealsTheWindowsLastRow(t *testing.T) {
	m := newLoadedTaskForm()
	// Wide enough for the two-column layout, short enough that the Extras pane
	// is taller than the body window it sits beside.
	m.SetSize(100, formMinHeight)
	m.activeTab = formTabExtras
	m.focusIndex = m.fieldCountForActiveTab() - 1

	plain := ansi.Strip(m.View().Content)
	if !strings.Contains(plain, "Ctrl+L") {
		t.Fatalf("the two-column layout did not open at 100 columns:\n%s", plain)
	}
	if strings.Contains(plain, "resize terminal to view all") {
		t.Fatalf("an over-tall help pane clipped the body instead of itself:\n%s", plain)
	}
	if !strings.Contains(plain, "· 0 below") {
		t.Fatalf("the position row is the body's last row and must survive the join:\n%s", plain)
	}
	assertFitsWithin(t, m.View().Content, 100, formMinHeight)
}

// TestClampFormHelpPaneKeepsItsOwnColumn states the cut's two properties: a pane
// that fits is untouched, and one that does not loses its tail to a marker
// narrow enough to stay inside the column it was rendered for.
func TestClampFormHelpPaneKeepsItsOwnColumn(t *testing.T) {
	pane := "Extras\n──────\n a  one\n b  two\n c  three"
	if got := clampFormHelpPane(pane, 5); got != pane {
		t.Fatalf("a pane that fits must be returned untouched, got:\n%s", got)
	}
	cut := ansi.Strip(clampFormHelpPane(pane, 3))
	lines := strings.Split(cut, "\n")
	if len(lines) != 3 {
		t.Fatalf("clamped pane has %d rows, want 3:\n%s", len(lines), cut)
	}
	if lines[2] != "…" {
		t.Fatalf("the cut pane must name itself with one narrow marker, got %q", lines[2])
	}
}

// TestFormViewportOffsetPinsATallFieldToItsStart states the one rule a span
// wider than the window needs: a control whose start is off screen cannot be
// read at all, while its tail is one field away from being the next field's.
func TestFormViewportOffsetPinsATallFieldToItsStart(t *testing.T) {
	marks := []formFieldMark{{field: 0, line: 0}, {field: 1, line: 2}, {field: 2, line: 12}, {field: 3, line: 18}}
	if got := formViewportOffset(20, marks, 1, 4); got != 2 {
		t.Fatalf("offset for the ten-line field 1 in a four-row window = %d, want 2", got)
	}
	if got := formViewportOffset(20, marks, 0, 4); got != 0 {
		t.Fatalf("offset for the first field = %d, want 0", got)
	}
	if got := formViewportOffset(20, marks, 3, 4); got != 16 {
		t.Fatalf("offset for the two-line last field = %d, want 16", got)
	}
	if got := formViewportOffset(20, marks, 9, 4); got != 0 {
		t.Fatalf("offset for an undeclared field = %d, want 0", got)
	}
	if got := formViewportOffset(3, marks, 3, 4); got != 0 {
		t.Fatalf("offset for a body that fits = %d, want 0", got)
	}
}
