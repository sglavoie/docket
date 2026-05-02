package tui

import (
	"testing"
)

func TestNotesCommands_ShowNotes(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.notesCommands()
	cmd := findCommand(cmds, "show notes")
	if cmd == nil {
		t.Fatal("expected to find 'show notes' command")
	}

	result := cmd.Handler()
	if _, ok := result.(ShowNotesMsg); !ok {
		t.Errorf("expected ShowNotesMsg, got %T", result)
	}
}

func TestNotesCommands_ShowNotes_NilProject(t *testing.T) {
	app := newTestAppModel()
	app.currentProject = nil

	cmds := app.notesCommands()
	cmd := findCommand(cmds, "show notes")
	if cmd == nil {
		t.Fatal("expected to find 'show notes' command")
	}

	result := cmd.Handler()
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestNotesCommands_CreateNote(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.notesCommands()
	cmd := findCommand(cmds, "create note")
	if cmd == nil {
		t.Fatal("expected to find 'create note' command")
	}

	result := cmd.Handler()
	if _, ok := result.(ShowNoteCreatorMsg); !ok {
		t.Errorf("expected ShowNoteCreatorMsg, got %T", result)
	}
}
