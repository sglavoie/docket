package help

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/operation"
)

func actionHelpFixture() []operation.HelpItem {
	return []operation.HelpItem{
		{Key: "ww", Description: "Wait on Task", Category: "Tasks", Aliases: []string{"workflow"},
			Target: "[work:1] Fix the scheduler", Action: &operation.HelpAction{ID: operation.OperationTaskSetState, Value: models.StateWaiting, HasValue: true}},
		{Key: "wd", Description: "Complete Task", Category: "Tasks", Aliases: []string{"workflow"},
			Target: "[work:1] Fix the scheduler", Action: &operation.HelpAction{ID: operation.OperationTaskSetState, Value: models.StateDone, HasValue: true}},
		{Key: "e", Description: "Edit Task", Category: "Tasks", DisabledReason: "Choose a task to use this action.",
			Action: &operation.HelpAction{ID: operation.OperationCoreEdit}},
		{Key: "@@defer", Description: "Defer syntax", Category: helpGrammarCategory},
	}
}

func newActionHelp(t *testing.T, query string) *HelpViewModel {
	t.Helper()
	items := actionHelpFixture()
	m := NewHelpViewModel(items, "Outliner")
	m.SetSize(60, 18)
	m.EnableActions(operation.CommandContext{}, items)
	m.Update(helpKeyPress("/"))
	m.Update(helpKeyPress(query))
	return m
}

func TestActionHelpRunsSelectedPresetAndFreezesContext(t *testing.T) {
	m := newActionHelp(t, "workflow")
	ctx := operation.CommandContext{Entity: operation.EntityContext{ProjectID: "project", TaskID: "original"}}
	ctx.Selection.Visual = true
	ctx.Selection.Tasks = append(ctx.Selection.Tasks, operation.CurrentTaskIdentity(ctx))
	m.EnableActions(ctx, actionHelpFixture())
	ctx.Selection.Tasks[0].TaskID = "changed"
	m.Update(helpKeyPress("down"))
	_, cmd := m.Update(helpKeyPress("enter"))
	if cmd == nil {
		t.Fatal("Enter did not run the selected result while typing")
	}
	msg, ok := cmd().(RunActionMsg)
	if !ok || msg.Source != m || msg.Request.ID != operation.OperationTaskSetState || msg.Request.Value != models.StateDone || !msg.Request.HasValue {
		t.Fatalf("wrong selected preset: %+v", msg)
	}
	if msg.Request.Context.Selection.Tasks[0].TaskID != "original" || !msg.Request.Context.Selection.Visual {
		t.Fatal("action context followed a mutated selection")
	}
	if !strings.Contains(m.View().Content, "Target: [work:1] Fix the scheduler") {
		t.Fatal("the chosen action's target is absent")
	}
}

func TestActionHelpNeverRunsDisabledReferenceOrEmptyResults(t *testing.T) {
	for _, query := range []string{"Edit Task", "@@defer", "no such action"} {
		t.Run(query, func(t *testing.T) {
			m := newActionHelp(t, query)
			_, cmd := m.Update(helpKeyPress("enter"))
			if cmd != nil {
				t.Fatal("a non-runnable result emitted an action")
			}
			spec, _ := m.ContextualGuidance()
			if strings.Contains(guidanceText(spec), "Enter run") {
				t.Fatal("guidance advertised an unavailable execution")
			}
		})
	}
}

func TestActionHelpLiteralLabelOutranksLooseFuzzyMatch(t *testing.T) {
	items := []operation.HelpItem{
		{Key: "y", Description: "Copy title and description", Category: "Tasks", Action: &operation.HelpAction{ID: operation.OperationTaskCopyDetails}},
		{Key: `\`, Description: "Named Lenses", Category: "Discovery", Action: &operation.HelpAction{ID: operation.OperationCoreNamedLenses}},
	}
	m := NewHelpViewModel(items, "Outliner")
	m.SetSize(60, 18)
	m.EnableActions(operation.CommandContext{}, items)
	m.Update(helpKeyPress("/"))
	m.Update(helpKeyPress("lens"))
	selected, ok := m.selectedActionItem()
	if !ok || selected.Action.ID != operation.OperationCoreNamedLenses {
		t.Fatalf("literal action name did not lead the results: %+v", selected)
	}
}

func TestActionHelpTypingAndCategorySearchRemainLiteral(t *testing.T) {
	m := newActionHelp(t, "work")
	m.Update(helpKeyPress("j"))
	if m.searchQuery != "workj" {
		t.Fatal("a navigation alias swallowed typed text")
	}
	m.Update(helpKeyPress("backspace"))
	m.Update(helpKeyPress("esc"))
	m.Update(helpKeyPress("right")) // Tasks.
	m.Update(helpKeyPress("right")) // Grammar.
	if m.otherCategoryMatches != 2 {
		t.Fatalf("missing cross-category matches: %d", m.otherCategoryMatches)
	}
	m.Update(helpKeyPress("tab"))
	if m.activeTab != 0 || len(m.filteredItems) != 2 {
		t.Fatal("Tab did not show all action matches")
	}
}

func TestActionHelpResizesAndRestoresSelectedResultWithFreshTargets(t *testing.T) {
	items := make([]operation.HelpItem, 45)
	for i := range items {
		items[i] = operation.HelpItem{Key: fmt.Sprintf("k%d", i), Description: fmt.Sprintf("Action %d — 修复界面 %s", i, strings.Repeat("wrapped text ", 4)),
			Category: "Tasks", Target: "[work:1] " + strings.Repeat("long task title ", 10), Action: &operation.HelpAction{ID: operation.OperationCoreCapture}}
	}
	var session Session
	m := session.Open(items, "Outliner", 100, 33)
	m.EnableActions(operation.CommandContext{}, items)
	m.Update(helpKeyPress("/"))
	m.Update(helpKeyPress("Action"))
	m.Update(helpKeyPress("esc"))
	m.Update(helpKeyPress("G"))
	for _, size := range [][2]int{{60, 18}, {80, 22}, {120, 38}, {60, 18}} {
		m.SetSize(size[0], size[1])
		screen := m.View().Content
		if lipgloss.Height(screen) > size[1] || lipgloss.Width(screen) > size[0] || !strings.Contains(screen, "Action 44") {
			t.Fatalf("selected result lost or overflow at %v:\n%s", size, screen)
		}
	}
	fresh := append([]operation.HelpItem(nil), items...)
	fresh[44].Target = "[work:2] Fresh context"
	m = session.Open(fresh, "Outliner", 60, 18)
	m.EnableActions(operation.CommandContext{}, fresh)
	selected, _ := m.selectedActionItem()
	if selected.Key != "k44" || m.HasTextInput() || !strings.Contains(m.View().Content, "Fresh context") {
		t.Fatalf("reopening lost selection or kept stale targets:\n%s", m.View().Content)
	}
	// Clearing a search returns to ordinary reference browsing.
	m.Update(helpKeyPress("backspace"))
	if m.actionSearch() || m.activeTab != 0 {
		t.Fatal("clearing a search did not restore reference help")
	}
}
