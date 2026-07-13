package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
)

type testNotesConfig struct {
	editor         string
	externalEditor string
}

func (c testNotesConfig) GetEditor() string { return c.editor }

func (c testNotesConfig) GetExternalEditor() string { return c.externalEditor }

func newTestNotesView(notes []*models.Note) *NotesViewModel {
	project := &models.Project{Name: "test"}
	m := &NotesViewModel{
		project:       project,
		notes:         notes,
		filterTags:    make(map[string]bool),
		selectedNotes: make(map[string]bool),
	}
	m.SetSize(80, 40)
	return m
}

func sampleNotes() []*models.Note {
	return []*models.Note{
		{Title: "Note A", Filename: "a.md"},
		{Title: "Note B", Filename: "b.md"},
		{Title: "Note C", Filename: "c.md"},
	}
}

func TestCursorMovement(t *testing.T) {
	m := newTestNotesView(sampleNotes())

	// j moves down
	m.Update(syntheticKey('j'))
	if m.cursor != 1 {
		t.Errorf("expected cursor=1 after j, got %d", m.cursor)
	}

	// k moves up
	m.Update(syntheticKey('k'))
	if m.cursor != 0 {
		t.Errorf("expected cursor=0 after k, got %d", m.cursor)
	}

	// k at top stays at 0
	m.Update(syntheticKey('k'))
	if m.cursor != 0 {
		t.Errorf("expected cursor=0 at top, got %d", m.cursor)
	}

	// G goes to bottom
	m.Update(syntheticKey('G'))
	if m.cursor != 2 {
		t.Errorf("expected cursor=2 after G, got %d", m.cursor)
	}

	// j at bottom stays
	m.Update(syntheticKey('j'))
	if m.cursor != 2 {
		t.Errorf("expected cursor=2 at bottom, got %d", m.cursor)
	}
}

func TestMultiSelectToggle(t *testing.T) {
	m := newTestNotesView(sampleNotes())

	// V enters multi-select
	m.Update(syntheticKey('V'))
	if !m.multiSelectMode {
		t.Error("expected multi-select mode after V")
	}

	// space toggles selection and moves cursor down
	m.Update(syntheticSpecialKey(tea.KeySpace))
	if !m.selectedNotes["a.md"] {
		t.Error("expected a.md selected after space")
	}
	if m.cursor != 1 {
		t.Errorf("expected cursor=1 after space, got %d", m.cursor)
	}

	// 'a' selects all
	m.Update(syntheticKey('a'))
	if len(m.selectedNotes) != 3 {
		t.Errorf("expected 3 selected after 'a', got %d", len(m.selectedNotes))
	}

	// V exits multi-select
	m.Update(syntheticKey('V'))
	if m.multiSelectMode {
		t.Error("expected multi-select off after second V")
	}
}

func TestSortCycling(t *testing.T) {
	m := newTestNotesView(sampleNotes())

	if m.sortMode != noteSortModified {
		t.Errorf("expected initial sort=modified, got %v", m.sortMode)
	}

	m.cycleSortMode()
	if m.sortMode != noteSortCreated {
		t.Errorf("expected sort=created after first cycle, got %v", m.sortMode)
	}

	m.cycleSortMode()
	if m.sortMode != noteSortTitleAZ {
		t.Errorf("expected sort=titleAZ after second cycle, got %v", m.sortMode)
	}

	m.cycleSortMode()
	if m.sortMode != noteSortTitleZA {
		t.Errorf("expected sort=titleZA after third cycle, got %v", m.sortMode)
	}

	m.cycleSortMode()
	if m.sortMode != noteSortModified {
		t.Errorf("expected sort=modified after fourth cycle, got %v", m.sortMode)
	}
}

func TestRenameFlowCancel(t *testing.T) {
	m := newTestNotesView(sampleNotes())

	m.startRename()
	if !m.renaming {
		t.Error("expected renaming=true after startRename")
	}
	if m.renameInput.Value() != "Note A" {
		t.Errorf("expected rename input value='Note A', got %q", m.renameInput.Value())
	}

	// Esc cancels
	m.handleRenameKey(syntheticSpecialKey(tea.KeyEscape))
	if m.renaming {
		t.Error("expected renaming=false after Esc")
	}
	if m.statusMessage != "Rename canceled" {
		t.Errorf("expected 'Rename canceled' status, got %q", m.statusMessage)
	}
}

func TestPendingGKeyConsumed(t *testing.T) {
	m := newTestNotesView(sampleNotes())

	// Move cursor down first
	m.cursor = 2

	// Press g then d — should consume the d, NOT trigger delete
	m.Update(syntheticKey('g'))
	if m.pendingPrefix != "g" {
		t.Error("expected pendingPrefix=\"g\" after g")
	}

	m.Update(syntheticKey('d'))
	if m.pendingPrefix != "" {
		t.Error("expected pendingPrefix=\"\" after second key")
	}
	if m.cursor != 2 {
		t.Errorf("expected cursor=2 after gd (only gg jumps to top), got %d", m.cursor)
	}
	if m.confirm.IsActive() {
		t.Error("expected no delete confirmation after gd — second key should be consumed")
	}
}

func TestPendingGKeyDeselectAll(t *testing.T) {
	m := newTestNotesView(sampleNotes())
	m.multiSelectMode = true
	m.selectedNotes = map[string]bool{"a.md": true, "b.md": true}

	m.Update(syntheticKey('g'))
	m.Update(syntheticKey('X'))

	if len(m.selectedNotes) != 0 {
		t.Error("expected all deselected after gX")
	}
}

func TestOpenCurrentNoteReferenceCreatesSnapshotAndUsesExternalEditor(t *testing.T) {
	note := &models.Note{
		Title:       "Reference Note",
		Filename:    "reference.md",
		Body:        "Reference body",
		LinkedTasks: []string{"task-1"},
	}
	m := newTestNotesView([]*models.Note{note})
	m.config = testNotesConfig{editor: "vi", externalEditor: "code"}
	m.cachedTasks = []*models.Task{{
		ID:            "task-1",
		ShortID:       7,
		Type:          models.TaskTypeTask,
		Title:         "Linked task",
		WorkflowState: models.StateReady,
	}}

	oldCreate := createReferenceSnapshotFile
	oldOpen := openReferenceFile
	t.Cleanup(func() {
		createReferenceSnapshotFile = oldCreate
		openReferenceFile = oldOpen
	})

	var snapshotContent string
	var snapshotPrefix string
	var snapshotSuffix string
	createReferenceSnapshotFile = func(content string, prefix string, suffix string) (string, error) {
		snapshotContent = content
		snapshotPrefix = prefix
		snapshotSuffix = suffix
		return "/tmp/tt-note-reference.md", nil
	}

	var openedEditor string
	var openedPath string
	openReferenceFile = func(editor string, path string) (external.ReferenceOpenResult, error) {
		openedEditor = editor
		openedPath = path
		return external.ReferenceOpenResult{
			Status: external.ReferenceOpenStatusOpened,
			Editor: editor,
			Path:   path,
		}, nil
	}

	cmd := m.openCurrentNoteReference()
	if cmd == nil {
		t.Fatal("expected status timeout command")
	}
	if openedEditor != "code" {
		t.Fatalf("opened editor = %q, want code", openedEditor)
	}
	if openedPath != "/tmp/tt-note-reference.md" {
		t.Fatalf("opened path = %q", openedPath)
	}
	if snapshotPrefix != "tt-note-reference-" || snapshotSuffix != ".md" {
		t.Fatalf("snapshot naming = %q %q", snapshotPrefix, snapshotSuffix)
	}
	for _, want := range []string{
		"# Note: Reference Note",
		external.ReferenceSnapshotWarning,
		"Reference body",
		"Linked task",
	} {
		if !strings.Contains(snapshotContent, want) {
			t.Fatalf("snapshot content missing %q:\n%s", want, snapshotContent)
		}
	}
	if m.statusMessage != "Opened note reference copy" {
		t.Fatalf("status = %q, want opened reference status", m.statusMessage)
	}
}

func TestOpenCurrentNoteReferenceWithParseErrorUsesRawFileContent(t *testing.T) {
	dir := t.TempDir()
	rawContent := "---\ntitle: [broken\n---\n# Raw heading\n"
	if err := os.WriteFile(filepath.Join(dir, "broken.md"), []byte(rawContent), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	note := &models.Note{
		Title:      "Broken",
		Filename:   "broken.md",
		ParseError: "invalid YAML: missing bracket",
	}
	m := &NotesViewModel{
		notesDir:      dir,
		isGlobal:      true,
		notes:         []*models.Note{note},
		config:        testNotesConfig{editor: "vi", externalEditor: "code"},
		filterTags:    make(map[string]bool),
		selectedNotes: make(map[string]bool),
	}

	oldCreate := createReferenceSnapshotFile
	oldOpen := openReferenceFile
	t.Cleanup(func() {
		createReferenceSnapshotFile = oldCreate
		openReferenceFile = oldOpen
	})

	var snapshotContent string
	createReferenceSnapshotFile = func(content string, prefix string, suffix string) (string, error) {
		snapshotContent = content
		return "/tmp/tt-note-reference.md", nil
	}
	openReferenceFile = func(editor string, path string) (external.ReferenceOpenResult, error) {
		return external.ReferenceOpenResult{Status: external.ReferenceOpenStatusOpened}, nil
	}

	cmd := m.openCurrentNoteReference()
	if cmd == nil {
		t.Fatal("expected status timeout command")
	}
	for _, want := range []string{
		external.ReferenceSnapshotWarning,
		"This note could not be parsed",
		"invalid YAML: missing bracket",
		"## Raw File Content",
		rawContent,
	} {
		if !strings.Contains(snapshotContent, want) {
			t.Fatalf("raw reference content missing %q:\n%s", want, snapshotContent)
		}
	}
}

func TestEditCurrentNoteStillUsesBlockingEditFlow(t *testing.T) {
	m := newTestNotesView([]*models.Note{{Title: "Editable", Filename: "editable.md"}})
	m.config = testNotesConfig{editor: "true", externalEditor: "code"}

	oldOpen := openReferenceFile
	t.Cleanup(func() {
		openReferenceFile = oldOpen
	})
	openReferenceFile = func(string, string) (external.ReferenceOpenResult, error) {
		t.Fatal("reference opener should not run for e/edit")
		return external.ReferenceOpenResult{}, nil
	}

	cmd := m.editCurrentNote()
	if cmd == nil {
		t.Fatal("expected blocking editor command")
	}
}

func TestNotesHelpDescribesReferenceOpenSeparatelyFromEdit(t *testing.T) {
	m := newTestNotesView(sampleNotes())
	items := m.GetHelpItems()

	var editDescription string
	var referenceDescription string
	for _, item := range items {
		switch item.Key {
		case "e":
			editDescription = item.Description
		case "Ctrl+O":
			referenceDescription = item.Description
		}
	}

	if editDescription != "Edit note" {
		t.Fatalf("e help = %q, want Edit note", editDescription)
	}
	if referenceDescription != "Open note reference copy" {
		t.Fatalf("Ctrl+O help = %q, want reference copy help", referenceDescription)
	}
}
