package help

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/operation"
)

func paletteFixture() []operation.HelpItem {
	return append(actionHelpFixture(),
		operation.HelpItem{Key: "a", Description: "Capture", Category: "Tasks", Rank: 1,
			Action: &operation.HelpAction{ID: operation.OperationCoreCapture}},
		operation.HelpItem{Key: ":", Description: "Command Palette", Category: "System",
			Action: &operation.HelpAction{ID: operation.OperationCoreCommandPalette}},
	)
}

func typePalette(p *CommandPaletteModel, text string) {
	for _, r := range text {
		p.HandleKey(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func TestCommandPaletteListsRunnableRowsRankedFirst(t *testing.T) {
	p := NewCommandPalette(operation.CommandContext{}, paletteFixture())
	labels := make([]string, 0, len(p.results))
	for _, item := range p.results {
		labels = append(labels, item.Description)
	}
	// Ranked available rows lead, the unavailable one trails, and neither the
	// grammar prose nor the palette itself is a row.
	want := []string{"Capture", "Wait on Task", "Complete Task", "Edit Task"}
	if strings.Join(labels, "|") != strings.Join(want, "|") {
		t.Fatalf("empty palette = %v, want %v", labels, want)
	}
}

func TestCommandPaletteFuzzyQueryRunsThePresetWithFrozenContext(t *testing.T) {
	ctx := operation.CommandContext{Entity: operation.EntityContext{ProjectID: "project", TaskID: "original"}}
	p := NewCommandPalette(ctx, paletteFixture())
	ctx.Entity.TaskID = "changed"
	typePalette(p, "cmplt")
	if item, _ := p.Selected(); item.Description != "Complete Task" {
		t.Fatalf("fuzzy query selected %q", item.Description)
	}
	_, dismiss, msg := p.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	run, ok := msg.(RunPaletteActionMsg)
	if dismiss || !ok || run.Source != p || run.Request.Value != models.StateDone || !run.Request.HasValue {
		t.Fatalf("Enter = dismiss %v, %+v", dismiss, msg)
	}
	if run.Request.Context.Entity.TaskID != "original" {
		t.Fatal("the request followed a context changed after opening")
	}
}

func TestCommandPaletteUnavailableRowRunsNothingAndExplains(t *testing.T) {
	p := NewCommandPalette(operation.CommandContext{}, paletteFixture())
	typePalette(p, "Edit Task")
	if _, _, msg := p.HandleKey(tea.KeyPressMsg{Code: tea.KeyEnter}); msg != nil {
		t.Fatalf("an unavailable row ran: %+v", msg)
	}
	if !strings.Contains(p.Render(60), "Choose a task to use this action.") {
		t.Fatal("the reason is not under the list")
	}
	if !p.guidanceSpec().Hints[0].Dimmed {
		t.Fatal("Enter run is not dimmed over a row it would refuse")
	}
	if _, dismiss, _ := p.HandleKey(tea.KeyPressMsg{Code: tea.KeyEscape}); !dismiss {
		t.Fatal("Esc did not close the palette")
	}
}

func TestCommandPaletteHoldsItsHeightAndWidth(t *testing.T) {
	p := NewCommandPalette(operation.CommandContext{}, paletteFixture())
	full := p.RenderWithin(60, 20)
	typePalette(p, "zzzz")
	empty := p.RenderWithin(60, 20)
	if lipgloss.Height(full) != lipgloss.Height(empty)+1 && lipgloss.Height(full) != lipgloss.Height(empty) {
		t.Fatalf("the box resized from %d to %d rows while narrowing", lipgloss.Height(full), lipgloss.Height(empty))
	}
	if !strings.Contains(empty, "No matching actions.") || lipgloss.Width(empty) > 60 || lipgloss.Height(full) > 19 {
		t.Fatalf("palette does not fit 60x20:\n%s", full)
	}
	p.HandlePaste(tea.PasteMsg{Content: "\nab\n"})
	if p.query != "zzzzab" {
		t.Fatalf("paste = %q", p.query)
	}
}
