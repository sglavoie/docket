package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

func TestOtherCommands_ProjectFocus(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.otherCommands()
	cmd := findCommand(cmds, "project focus queue")
	if cmd == nil {
		t.Fatal("expected to find 'project focus queue' command")
	}
	for _, alias := range cmd.Aliases {
		if alias == "project active tasks" {
			t.Fatal("project Focus Queue command should not describe queued work as active")
		}
	}

	result := cmd.Handler()
	msg, ok := result.(ShowOutlinerMsg)
	if !ok || !msg.Options.SetFocus || msg.Options.Focus != OutlinerNext {
		t.Errorf("expected Next-focused ShowOutlinerMsg, got %#v", result)
	}
}

func TestOtherCommands_ProjectFocus_NilProject(t *testing.T) {
	app := newTestAppModel()
	app.currentProject = nil

	cmds := app.otherCommands()
	cmd := findCommand(cmds, "project focus queue")
	if cmd == nil {
		t.Fatal("expected to find 'project focus queue' command")
	}

	result := cmd.Handler()
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestFocusQueuePaletteHasNoLegacyRemovalCommand(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	registry := NewCommandRegistry()
	for i, group := range app.commandGroups() {
		if err := registry.RegisterBatch(group); err != nil {
			t.Fatalf("RegisterBatch[%d] failed: %v", i, err)
		}
	}

	for _, view := range []PresentationID{ViewOutliner} {
		if cmd := findCommand(registry.GetForView(view), "remove from focus"); cmd != nil {
			t.Fatalf("%s palette should not expose legacy queue removal: %+v", view, cmd)
		}
	}
}

func TestOtherCommands_BlockersView(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.otherCommands()
	cmd := findCommand(cmds, "blockers view")
	if cmd == nil {
		t.Fatal("expected to find 'blockers view' command")
	}

	result := cmd.Handler()
	msg, ok := result.(ShowBlockersMsg)
	if !ok {
		t.Fatalf("expected ShowBlockersMsg, got %T", result)
	}
	if msg.Project == nil {
		t.Error("expected Project to be set")
	}
}

func TestProjectBlockersShortcutMatchesLiveOutlinerKeymap(t *testing.T) {
	app, project := newTestAppModelWithProject()
	outliner := NewOutlinerModel(project, app.configManager, app.runtime)

	var projectBlockers *keymap.Binding
	var blockerPrefix *keymap.PrefixNode
	for _, entry := range outliner.buildKeyMap().Entries {
		if entry.Binding != nil && entry.Binding.Description == "Project blockers" {
			projectBlockers = entry.Binding
		}
		if entry.PrefixNode != nil && entry.PrefixNode.Description == "Blockers" {
			blockerPrefix = entry.PrefixNode
		}
	}
	if projectBlockers == nil {
		t.Fatal("built Outliner keymap is missing the Project blockers binding")
	}

	command := findCommand(app.otherCommands(), "blockers view")
	if command == nil {
		t.Fatal("expected to find 'blockers view' command")
	}
	if command.Shortcut != projectBlockers.Key {
		t.Errorf("blockers view shortcut = %q, live Outliner binding = %q", command.Shortcut, projectBlockers.Key)
	}

	if blockerPrefix == nil {
		t.Fatal("built Outliner keymap is missing the blocker prefix")
	}
	if blockerPrefix.Key != "b" {
		t.Errorf("blocker prefix = %q, want lowercase b", blockerPrefix.Key)
	}
}
