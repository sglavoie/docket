package taskform

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func navigateToExtrasTab(m *TaskFormModel) *TaskFormModel {
	m.activeTab = formTabExtras
	m.focusIndex = 0
	return m
}

func TestTaskForm_Extras_IsTheFourthTab(t *testing.T) {
	if formTabCount != 4 {
		t.Fatalf("formTabCount = %d, want 4", formTabCount)
	}
	if len(formTabNames) != formTabCount {
		t.Fatalf("formTabNames has %d entries, want %d", len(formTabNames), formTabCount)
	}
	if formTabNames[formTabExtras] != "Extras" {
		t.Errorf("formTabNames[formTabExtras] = %q, want Extras", formTabNames[formTabExtras])
	}
	if got := formStaticFieldCount(formTabExtras); got != 0 {
		t.Errorf("formStaticFieldCount(formTabExtras) = %d, want 0", got)
	}
}

func TestTaskForm_Extras_ToggleAndFieldCount(t *testing.T) {
	m := navigateToExtrasTab(newTestTaskForm())
	m.draft.attachments = []formAttachmentDraft{{value: "https://example.com/a"}}

	if got := m.fieldCountForActiveTab(); got != 4 {
		t.Fatalf("collapsed field count = %d, want 4 (two headers and two relation fields)", got)
	}
	m = sendFormKey(m, syntheticKey('a'))
	if !m.attachmentsExpanded {
		t.Fatal("a should expand the Attachments section")
	}
	if got := m.fieldCountForActiveTab(); got != 5 {
		t.Errorf("expanded field count = %d, want 5 (two headers, two relation fields and one draft)", got)
	}
	m = sendFormKey(m, syntheticKey('a'))
	if m.attachmentsExpanded {
		t.Error("a should collapse the Attachments section again")
	}
}

func TestTaskForm_Extras_CtrlLOpensEditOverlayForANewDraft(t *testing.T) {
	m := navigateToExtrasTab(newTestTaskForm())
	m = sendFormKey(m, syntheticCtrlKey('l'))

	if m.activeOverlay != formOverlayAttachmentEdit {
		t.Fatalf("activeOverlay = %d, want formOverlayAttachmentEdit", m.activeOverlay)
	}
	if len(m.draft.attachments) != 1 {
		t.Fatalf("expected one draft appended, got %d", len(m.draft.attachments))
	}
	if !m.attachmentsExpanded {
		t.Error("adding a draft should expand the section so it is visible")
	}

	m.attachmentItemInput.SetValue("https://example.com/spec.pdf")
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))

	if m.activeOverlay != formOverlayNone {
		t.Error("confirming should close the overlay")
	}
	if len(m.draft.attachments) != 1 || m.draft.attachments[0].value != "https://example.com/spec.pdf" {
		t.Errorf("draft = %+v, want the confirmed value", m.draft.attachments)
	}
}

func TestTaskForm_Extras_BlankConfirmDropsTheDraft(t *testing.T) {
	m := navigateToExtrasTab(newTestTaskForm())
	m = sendFormKey(m, syntheticCtrlKey('l'))
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))

	if len(m.draft.attachments) != 0 {
		t.Errorf("a blank confirm should drop the draft; got %d", len(m.draft.attachments))
	}
}

func TestTaskForm_Extras_CanceledNewDraftIsRemoved(t *testing.T) {
	m := navigateToExtrasTab(newTestTaskForm())
	m = sendFormKey(m, syntheticCtrlKey('l'))
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEscape))

	if len(m.draft.attachments) != 0 {
		t.Errorf("canceling a brand-new draft should remove it; got %d", len(m.draft.attachments))
	}
}

func TestTaskForm_Extras_CtrlDDeletesFocusedDraft(t *testing.T) {
	m := navigateToExtrasTab(newTestTaskForm())
	m.draft.attachments = []formAttachmentDraft{{value: "first"}, {value: "second"}}
	m.attachmentsExpanded = true
	m.focusIndex = 1

	m = sendFormKey(m, syntheticCtrlKey('d'))
	if len(m.draft.attachments) != 1 || m.draft.attachments[0].value != "second" {
		t.Errorf("after Ctrl+D: drafts = %+v, want only 'second'", m.draft.attachments)
	}
}

func TestTaskForm_Extras_CtrlJKReorderDrafts(t *testing.T) {
	m := navigateToExtrasTab(newTestTaskForm())
	m.draft.attachments = []formAttachmentDraft{{value: "first"}, {value: "second"}}
	m.attachmentsExpanded = true
	m.focusIndex = 1

	m = sendFormKey(m, syntheticCtrlKey('j'))
	if m.draft.attachments[0].value != "second" {
		t.Fatalf("after Ctrl+J: drafts = %+v, want 'second' first", m.draft.attachments)
	}
	if m.focusIndex != 2 {
		t.Errorf("focus should follow the moved draft; got %d", m.focusIndex)
	}
	m = sendFormKey(m, syntheticCtrlKey('k'))
	if m.draft.attachments[0].value != "first" {
		t.Errorf("after Ctrl+K: drafts = %+v, want 'first' first", m.draft.attachments)
	}
}

func TestTaskForm_Extras_EnterOnHeaderToggles(t *testing.T) {
	m := navigateToExtrasTab(newTestTaskForm())
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if !m.attachmentsExpanded {
		t.Error("Enter on the section header should expand it")
	}
}

// The Extras tab is the reason buildKeyMap is tab-aware: Ctrl+D and Ctrl+J/K
// mean something different here from what they mean on Schedule and Children.
func TestTaskForm_Extras_KeyMapIsTabScoped(t *testing.T) {
	extras := navigateToExtrasTab(newTestTaskForm())
	entries := extras.buildKeyMap().Entries

	descFor := func(key string) string {
		for _, e := range entries {
			if e.Binding != nil && e.Binding.Key == key {
				return e.Binding.Description
			}
		}
		return ""
	}
	if got := descFor("ctrl+d"); got != "Delete focused draft" {
		t.Errorf("Extras Ctrl+D = %q, want the delete row rather than Schedule's due-date picker", got)
	}
	if got := descFor("ctrl+j"); got != "Move attachment down" {
		t.Errorf("Extras Ctrl+J = %q, want the attachment reorder row rather than Children's", got)
	}
	for _, key := range extras.HandledKeys() {
		if key == "ctrl+t" || key == "S" {
			t.Errorf("Extras still advertises the Children-only key %q", key)
		}
	}

	children := navigateToChildrenTab(newTestTaskForm())
	for _, key := range children.HandledKeys() {
		if key == "ctrl+l" {
			t.Error("Children still advertises the Extras-only key ctrl+l")
		}
	}
}

func TestTaskForm_Extras_SaveCommitsAttachmentWithTheTask(t *testing.T) {
	env := newScenarioEnv(t, "create")
	m := scenarioCreateForm(t, env)

	m.draft.title = "Wire the Extras tab"
	m.titleInput.SetValue("Wire the Extras tab")
	m.draft.attachments = []formAttachmentDraft{
		{value: "https://example.com/spec.pdf"},
		{value: "   "}, // blank drafts are skipped, exactly as blank subtasks are
	}

	msg := runCreate(t, m)

	task := env.reload(t).GetTaskByID(msg.Result.TaskID)
	if task == nil {
		t.Fatal("created task absent from reloaded project")
	}
	if len(task.Attachments) != 1 {
		t.Fatalf("task carries %d attachment(s), want 1", len(task.Attachments))
	}
	if got := task.Attachments[0]; !got.IsURL || got.Filename != "https://example.com/spec.pdf" {
		t.Errorf("attachment = %+v, want the drafted URL classified as a URL", got)
	}
}

func TestTaskForm_Extras_CommentsIsTheSecondSection(t *testing.T) {
	m := navigateToExtrasTab(newTestTaskForm())
	m.draft.comments = []formCommentDraft{{text: "ask about the SSO deadline"}}

	if got := m.fieldCountForActiveTab(); got != 4 {
		t.Fatalf("collapsed field count = %d, want 4 (two headers and two relation fields)", got)
	}
	if got := m.extrasCommentsHeaderIdx(); got != 1 {
		t.Fatalf("comments header idx = %d, want 1 while Attachments is collapsed", got)
	}

	m = sendFormKey(m, syntheticKey('c'))
	if !m.commentsExpanded {
		t.Fatal("c should expand the Comments section")
	}
	if got := m.fieldCountForActiveTab(); got != 5 {
		t.Errorf("expanded field count = %d, want 5 (two headers, two relation fields and one draft)", got)
	}

	// Expanding Attachments pushes the Comments header down by its drafts.
	m.draft.attachments = []formAttachmentDraft{{value: "https://example.com/a"}}
	m.attachmentsExpanded = true
	if got := m.extrasCommentsHeaderIdx(); got != 2 {
		t.Errorf("comments header idx = %d, want 2 once Attachments shows one draft", got)
	}
	m.focusIndex = 3
	kind, idx := m.focusedExtrasItem()
	if kind != extrasFocusCommentItem || idx != 0 {
		t.Errorf("focusedExtrasItem() = (%d, %d), want the first comment draft", kind, idx)
	}
}

func TestTaskForm_Extras_CtrlYOpensCommentOverlayForANewDraft(t *testing.T) {
	m := navigateToExtrasTab(newTestTaskForm())
	m = sendFormKey(m, syntheticCtrlKey('y'))

	if m.activeOverlay != formOverlayCommentEdit {
		t.Fatalf("activeOverlay = %d, want formOverlayCommentEdit", m.activeOverlay)
	}
	if len(m.draft.comments) != 1 {
		t.Fatalf("expected one comment draft appended, got %d", len(m.draft.comments))
	}
	if !m.commentsExpanded {
		t.Error("adding a draft should expand the section so it is visible")
	}

	m.commentItemInput.SetValue("ask about the SSO deadline")
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))

	if m.activeOverlay != formOverlayNone {
		t.Error("confirming should close the overlay")
	}
	if len(m.draft.comments) != 1 || m.draft.comments[0].text != "ask about the SSO deadline" {
		t.Errorf("draft = %+v, want the confirmed text", m.draft.comments)
	}
}

func TestTaskForm_Extras_BlankCommentConfirmDropsTheDraft(t *testing.T) {
	m := navigateToExtrasTab(newTestTaskForm())
	m = sendFormKey(m, syntheticCtrlKey('y'))
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))

	if len(m.draft.comments) != 0 {
		t.Errorf("a blank confirm should drop the draft; got %d", len(m.draft.comments))
	}
}

func TestTaskForm_Extras_CanceledNewCommentDraftIsRemoved(t *testing.T) {
	m := navigateToExtrasTab(newTestTaskForm())
	m = sendFormKey(m, syntheticCtrlKey('y'))
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEscape))

	if len(m.draft.comments) != 0 {
		t.Errorf("canceling a brand-new draft should remove it; got %d", len(m.draft.comments))
	}
}

func TestTaskForm_Extras_CtrlDDeletesFocusedCommentDraft(t *testing.T) {
	m := navigateToExtrasTab(newTestTaskForm())
	m.draft.comments = []formCommentDraft{{text: "first"}, {text: "second"}}
	m.commentsExpanded = true
	m.focusIndex = m.extrasCommentsHeaderIdx() + 1

	m = sendFormKey(m, syntheticCtrlKey('d'))
	if len(m.draft.comments) != 1 || m.draft.comments[0].text != "second" {
		t.Errorf("after Ctrl+D: drafts = %+v, want only 'second'", m.draft.comments)
	}
}

func TestTaskForm_Extras_EnterOnCommentsHeaderToggles(t *testing.T) {
	m := navigateToExtrasTab(newTestTaskForm())
	m.focusIndex = m.extrasCommentsHeaderIdx()
	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if !m.commentsExpanded {
		t.Error("Enter on the Comments header should expand it")
	}
	if m.attachmentsExpanded {
		t.Error("Enter on the Comments header must not touch the Attachments section")
	}
}

// c is the Children tab's checklist toggle and the Extras tab's comments
// toggle; the tab-scoped keymap is what keeps both labels honest.
func TestTaskForm_Extras_CommentKeysAreTabScoped(t *testing.T) {
	descFor := func(m *TaskFormModel, key string) string {
		for _, e := range m.buildKeyMap().Entries {
			if e.Binding != nil && e.Binding.Key == key {
				return e.Binding.Description
			}
		}
		return ""
	}

	extras := navigateToExtrasTab(newTestTaskForm())
	if got := descFor(extras, "c"); got != "Toggle comments section" {
		t.Errorf("Extras c = %q, want the comments toggle", got)
	}
	if got := descFor(extras, "ctrl+y"); got != "Add comment" {
		t.Errorf("Extras Ctrl+Y = %q, want the add-comment row", got)
	}

	children := navigateToChildrenTab(newTestTaskForm())
	if got := descFor(children, "c"); got != "Toggle checklist section" {
		t.Errorf("Children c = %q, want the checklist toggle", got)
	}
	for _, key := range children.HandledKeys() {
		if key == "ctrl+y" {
			t.Error("Children still advertises the Extras-only key ctrl+y")
		}
	}
}

func TestTaskForm_Extras_SaveCommitsTaskAttachmentAndCommentTogether(t *testing.T) {
	env := newScenarioEnv(t, "create")
	m := scenarioCreateForm(t, env)

	m.draft.title = "Wire the Comments section"
	m.titleInput.SetValue("Wire the Comments section")
	m.draft.attachments = []formAttachmentDraft{{value: "https://example.com/spec.pdf"}}
	m.draft.comments = []formCommentDraft{
		{text: "ask about the SSO deadline"},
		{text: "   "}, // blank drafts are skipped, exactly as blank attachments are
	}

	msg := runCreate(t, m)

	task := env.reload(t).GetTaskByID(msg.Result.TaskID)
	if task == nil {
		t.Fatal("created task absent from reloaded project")
	}
	if len(task.Attachments) != 1 {
		t.Fatalf("task carries %d attachment(s), want 1", len(task.Attachments))
	}
	if len(task.Comments) != 1 {
		t.Fatalf("task carries %d comment(s), want 1", len(task.Comments))
	}
	if got := task.Comments[0].Text; got != "ask about the SSO deadline" {
		t.Errorf("comment = %q, want the drafted text", got)
	}
}

// A compact line carrying &link expands into the form rather than refusing,
// because the Extras tab is now the channel that refusal was waiting on.
func TestTaskComposer_ExpandSeedsAttachmentDraft(t *testing.T) {
	runtime, project := newTestRuntimeWithProject()
	m := NewTaskComposerModel(TaskComposerPreset{
		Kind:        ComposerProject,
		Project:     project,
		AllProjects: []*models.Project{project},
	}, runtime)
	m.Input.SetValue("Ship the release &https://example.com/spec.pdf")

	m.ExpandSingle()
	if len(m.Errors) > 0 {
		t.Fatalf("expanding a line with &link should not refuse; errors = %+v", m.Errors)
	}
	if m.Expanded == nil {
		t.Fatal("expandSingle should have produced an expanded form")
	}
	if len(m.Expanded.draft.attachments) != 1 {
		t.Fatalf("expanded form carries %d attachment draft(s), want 1", len(m.Expanded.draft.attachments))
	}
	if got := m.Expanded.draft.attachments[0].value; got != "https://example.com/spec.pdf" {
		t.Errorf("seeded draft = %q, want the captured value", got)
	}
	if !m.Expanded.attachmentsExpanded {
		t.Error("a seeded Attachments section should open so the value is visible")
	}
}

// A compact line carrying ;comment; expands into the form for the same reason
// &link does: the Extras tab's Comments section is now that value's channel.
func TestTaskComposer_ExpandSeedsCommentDraft(t *testing.T) {
	runtime, project := newTestRuntimeWithProject()
	m := NewTaskComposerModel(TaskComposerPreset{
		Kind:        ComposerProject,
		Project:     project,
		AllProjects: []*models.Project{project},
	}, runtime)
	m.Input.SetValue("Call Dana ;ask about the SSO deadline;")

	m.ExpandSingle()
	if len(m.Errors) > 0 {
		t.Fatalf("expanding a line with ;comment; should not refuse; errors = %+v", m.Errors)
	}
	if m.Expanded == nil {
		t.Fatal("expandSingle should have produced an expanded form")
	}
	if len(m.Expanded.draft.comments) != 1 {
		t.Fatalf("expanded form carries %d comment draft(s), want 1", len(m.Expanded.draft.comments))
	}
	if got := m.Expanded.draft.comments[0].text; got != "ask about the SSO deadline" {
		t.Errorf("seeded draft = %q, want the captured text", got)
	}
	if !m.Expanded.commentsExpanded {
		t.Error("a seeded Comments section should open so the value is visible")
	}
	if got := m.Expanded.draft.title; got != "Call Dana" {
		t.Errorf("title = %q, want the comment span peeled off it", got)
	}
}

// A terminal paste arrives as tea.PasteMsg, not tea.KeyPressMsg, so it never
// reaches handleFormOverlayKey; these two tests drive the real Update dispatch
// to prove the Extras item overlays still receive it.
func TestTaskForm_Extras_AttachmentOverlayAcceptsPaste(t *testing.T) {
	m := navigateToExtrasTab(newTestTaskForm())
	m = sendFormKey(m, syntheticCtrlKey('l'))
	if m.activeOverlay != formOverlayAttachmentEdit {
		t.Fatalf("activeOverlay = %d, want formOverlayAttachmentEdit", m.activeOverlay)
	}
	m.hasChanges = false

	const pasted = "https://github.com/acme/app/issues/412"
	m = sendFormMsg(m, tea.PasteMsg{Content: pasted})

	if m.activeOverlay != formOverlayAttachmentEdit {
		t.Fatalf("a paste must leave the overlay open; activeOverlay = %d", m.activeOverlay)
	}
	if got := m.attachmentItemInput.Value(); got != pasted {
		t.Fatalf("attachmentItemInput = %q, want the pasted value %q", got, pasted)
	}
	if !m.hasChanges {
		t.Error("a paste that changed the input should mark the form dirty")
	}

	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if len(m.draft.attachments) != 1 || m.draft.attachments[0].value != pasted {
		t.Errorf("confirmed drafts = %+v, want the pasted value", m.draft.attachments)
	}
}

func TestTaskForm_Extras_CommentOverlayAcceptsPaste(t *testing.T) {
	m := navigateToExtrasTab(newTestTaskForm())
	m = sendFormKey(m, syntheticCtrlKey('y'))
	if m.activeOverlay != formOverlayCommentEdit {
		t.Fatalf("activeOverlay = %d, want formOverlayCommentEdit", m.activeOverlay)
	}
	m.hasChanges = false

	const pasted = "ask about the SSO deadline"
	m = sendFormMsg(m, tea.PasteMsg{Content: pasted})

	if m.activeOverlay != formOverlayCommentEdit {
		t.Fatalf("a paste must leave the overlay open; activeOverlay = %d", m.activeOverlay)
	}
	if got := m.commentItemInput.Value(); got != pasted {
		t.Fatalf("commentItemInput = %q, want the pasted text %q", got, pasted)
	}
	if !m.hasChanges {
		t.Error("a paste that changed the input should mark the form dirty")
	}

	m = sendFormKey(m, syntheticSpecialKey(tea.KeyEnter))
	if len(m.draft.comments) != 1 || m.draft.comments[0].text != pasted {
		t.Errorf("confirmed drafts = %+v, want the pasted text", m.draft.comments)
	}
}

func TestTaskForm_Extras_EmptyPasteLeavesTheFormClean(t *testing.T) {
	m := navigateToExtrasTab(newTestTaskForm())
	m = sendFormKey(m, syntheticCtrlKey('l'))
	m.hasChanges = false

	m = sendFormMsg(m, tea.PasteMsg{Content: ""})

	if m.hasChanges {
		t.Error("a paste that changed nothing should not mark the form dirty")
	}
}
