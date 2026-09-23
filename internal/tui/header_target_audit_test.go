package tui

import (
	"sort"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/status"
)

// A project header row is cursorable, so every key in the Outliner grammar is
// pressable there. What each operation resolves from that cursor is classified
// below by the only three answers a header may produce:
//
//   - "reject" — the operation needs a task the header does not name, so target
//     resolution refuses it with the shared rejection sentence.
//   - "project" — the operation genuinely acts on the project the header names
//     or on the surrounding scope.
//   - "target-free" — the operation carries no target at all (navigation, mode
//     cycling, search, system).
//
// A new or retargeted operation must be classified here deliberately, so a task
// mutation can never silently acquire a project target from a header cursor.
var headerTargetAudit = map[operation.OperationID]string{
	"tui.checklist.actions":           "reject",
	"tui.checklist.add-item":          "reject",
	"tui.checklist.demote-child":      "reject",
	"tui.checklist.edit-item":         "reject",
	"tui.checklist.move-bottom":       "reject",
	"tui.checklist.move-down":         "reject",
	"tui.checklist.move-top":          "reject",
	"tui.checklist.move-up":           "reject",
	"tui.checklist.promote-item":      "reject",
	"tui.checklist.toggle-item":       "reject",
	"tui.core.actions":                "reject",
	"tui.core.activate-group":         "target-free",
	"tui.core.back":                   "target-free",
	"tui.core.backlog":                "reject",
	"tui.core.capture":                "target-free",
	"tui.core.collapse-parent":        "target-free",
	"tui.core.conflicts":              "target-free",
	"tui.core.cursor-bottom":          "target-free",
	"tui.core.cursor-down":            "target-free",
	"tui.core.cursor-page-down":       "target-free",
	"tui.core.cursor-page-up":         "target-free",
	"tui.core.cursor-top":             "target-free",
	"tui.core.cursor-up":              "target-free",
	"tui.core.edit":                   "reject",
	"tui.core.edit-description":       "reject",
	"tui.core.expand":                 "target-free",
	"tui.core.external-editor":        "reject",
	"tui.core.focus-latest":           "target-free",
	"tui.core.focus-parent":           "reject",
	"tui.core.focus-subtree":          "reject",
	"tui.core.goto-all":               "target-free",
	"tui.core.goto-backlog":           "target-free",
	"tui.core.goto-due":               "target-free",
	"tui.core.goto-inbox":             "target-free",
	"tui.core.goto-next":              "target-free",
	"tui.core.goto-parent-task":       "reject",
	"tui.core.goto-recent":            "target-free",
	"tui.core.goto-relation":          "reject",
	"tui.core.goto-task-project":      "reject",
	"tui.core.goto-today":             "target-free",
	"tui.core.help":                   "target-free",
	"tui.core.jump-backward":          "target-free",
	"tui.core.jump-forward":           "target-free",
	"tui.core.multi-select":           "target-free",
	"tui.core.named-lenses":           "target-free",
	"tui.core.next":                   "reject",
	"tui.core.next-header":            "target-free",
	"tui.core.next-work-mode":         "target-free",
	"tui.core.notes":                  "target-free",
	"tui.core.open-note":              "target-free",
	"tui.core.outline-editor":         "target-free",
	"tui.core.peek":                   "project",
	"tui.core.peek-half-page-down":    "project",
	"tui.core.peek-half-page-up":      "project",
	"tui.core.peek-page-down":         "project",
	"tui.core.peek-page-up":           "project",
	"tui.core.previous-header":        "target-free",
	"tui.core.previous-work-mode":     "target-free",
	"tui.core.query-readout":          "target-free",
	"tui.core.quit":                   "target-free",
	"tui.core.range-select":           "target-free",
	"tui.core.redo":                   "target-free",
	"tui.core.reorder-bottom":         "project",
	"tui.core.reorder-down":           "project",
	"tui.core.reorder-top":            "project",
	"tui.core.reorder-up":             "project",
	"tui.core.reset-to-launch-view":   "target-free",
	"tui.core.scope-search":           "target-free",
	"tui.core.scroll-cursor-bottom":   "target-free",
	"tui.core.scroll-cursor-middle":   "target-free",
	"tui.core.scroll-cursor-top":      "target-free",
	"tui.core.select-all":             "target-free",
	"tui.core.select-pinned-project":  "target-free",
	"tui.core.select-project":         "target-free",
	"tui.core.today":                  "reject",
	"tui.core.toggle-project-groups":  "target-free",
	"tui.core.toggle-sections":        "target-free",
	"tui.core.toggle-subtrees":        "target-free",
	"tui.core.toggle-menu-bar":        "target-free",
	"tui.core.open-menu-bar":          "target-free",
	"tui.core.toggle-theme":           "target-free",
	"tui.core.undo":                   "target-free",
	"tui.core.zoom-section":           "target-free",
	"tui.filter.clear":                "target-free",
	"tui.filter.completion-age":       "target-free",
	"tui.filter.contexts":             "target-free",
	"tui.filter.projects":             "target-free",
	"tui.filter.dependency":           "target-free",
	"tui.filter.due":                  "target-free",
	"tui.filter.hierarchy":            "target-free",
	"tui.filter.include-archived":     "target-free",
	"tui.filter.only-archived":        "target-free",
	"tui.filter.only-parked":          "target-free",
	"tui.filter.pinned":               "target-free",
	"tui.filter.priority":             "target-free",
	"tui.filter.review":               "target-free",
	"tui.filter.stale":                "target-free",
	"tui.filter.project-clear":        "target-free",
	"tui.filter.project-tags":         "target-free",
	"tui.filter.show-parked":          "target-free",
	"tui.filter.states":               "target-free",
	"tui.filter.tags":                 "target-free",
	"tui.project.actions":             "project",
	"tui.project.add-comment":         "project",
	"tui.project.comments":            "project",
	"tui.project.set-tags":            "project",
	"tui.scope.pin-project":           "project",
	"tui.scope.rename-project":        "project",
	"tui.sort.clear":                  "target-free",
	"tui.sort.created":                "target-free",
	"tui.sort.due":                    "target-free",
	"tui.sort.priority":               "target-free",
	"tui.sort.ranked":                 "target-free",
	"tui.sort.title":                  "target-free",
	"tui.sort.updated":                "target-free",
	"tui.task.add-comment":            "reject",
	"tui.task.adopt-children":         "reject",
	"tui.task.attachments":            "reject",
	"tui.task.comments":               "reject",
	"tui.task.copy-details":           "reject",
	"tui.task.copy-title":             "reject",
	"tui.task.copy-title-description": "reject",
	"tui.task.indent":                 "reject",
	"tui.task.mark-reviewed":          "reject",
	"tui.task.move-to-project":        "reject",
	"tui.task.new-child":              "reject",
	"tui.task.outdent":                "reject",
	"tui.task.references":             "reject",
	"tui.task.repeat":                 "reject",
	"tui.task.set-due-date":           "reject",
	"tui.task.step-due-earlier":       "reject",
	"tui.task.step-due-later":         "reject",
	"tui.task.set-priority":           "reject",
	"tui.task.set-parent":             "reject",
	"tui.task.set-resume-note":        "reject",
	"tui.task.set-state":              "reject",
	"tui.task.set-tags":               "reject",
	"tui.task.toggle-pin":             "reject",
	"tui.task.toggle-review":          "reject",
}

func TestHeaderTargetResolutionIsClassified(t *testing.T) {
	ctx := headerTargetCursorContext(t)
	app := headerTargetAuditApp(t)
	registry := app.GetOperationRegistry()
	strategy, ok := registry.InputStrategy(operation.InputStrategySimplified)
	if !ok {
		t.Fatal("simplified input strategy is not registered")
	}

	resolver := operation.OperationTargetResolver{}
	observed := make(map[operation.OperationID]string)
	for _, binding := range strategy.Bindings {
		if _, seen := observed[binding.OperationID]; seen {
			continue
		}
		descriptor, exists := registry.Descriptor(binding.OperationID)
		if !exists {
			t.Errorf("input strategy binds unregistered operation %s", binding.OperationID)
			continue
		}
		observed[binding.OperationID] = headerTargetClassification(t, resolver, descriptor, ctx)
	}

	var unclassified, stale, mismatched []string
	for id, classification := range observed {
		expected, ok := headerTargetAudit[id]
		switch {
		case !ok:
			unclassified = append(unclassified, string(id)+" resolves "+classification)
		case expected != classification:
			mismatched = append(mismatched, string(id)+" is classified "+expected+" but resolves "+classification)
		}
	}
	for id := range headerTargetAudit {
		if _, ok := observed[id]; !ok {
			stale = append(stale, string(id))
		}
	}
	for label, failures := range map[string][]string{
		"operation reaches a project header cursor without a classification": unclassified,
		"classified operation is no longer in the Outliner grammar":          stale,
		"operation no longer resolves what it is classified as":              mismatched,
	} {
		if len(failures) > 0 {
			sort.Strings(failures)
			t.Errorf("%s:\n%s", label, joinLines(failures))
		}
	}
}

// TestHeaderTargetRejectionIsOneSentence keeps the refusal a single human
// sentence rather than the internal cardinality diagnostic it replaced.
func TestHeaderTargetRejectionIsOneSentence(t *testing.T) {
	ctx := headerTargetCursorContext(t)
	app := headerTargetAuditApp(t)
	registry := app.GetOperationRegistry()
	resolver := operation.OperationTargetResolver{}

	checked := 0
	for id, classification := range headerTargetAudit {
		if classification != "reject" {
			continue
		}
		descriptor, ok := registry.Descriptor(id)
		if !ok {
			t.Errorf("operation %s is not registered", id)
			continue
		}
		_, err := resolver.Resolve(descriptor, ctx)
		if err == nil {
			t.Errorf("operation %s resolved a target from a project header cursor", id)
			continue
		}
		if err.Error() != operation.OperationTargetRejectionMessage(operation.OperationCategoryTask) {
			t.Errorf("operation %s rejects with %q, not the shared sentence", id, err.Error())
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no rejecting operation found; the invariant is not auditing anything")
	}
}

// TestHeaderTargetCoreOperationsRefuseInsteadOfNoOp covers the delegated core
// operations, whose bindings bypass target resolution and would otherwise do
// nothing at all from a header cursor.
func TestHeaderTargetCoreOperationsRefuseInsteadOfNoOp(t *testing.T) {
	ctx := headerTargetCursorContext(t)
	app := headerTargetAuditApp(t)
	registry := app.GetOperationRegistry()

	for id, classification := range headerTargetAudit {
		if classification != "reject" || !operation.CoreInputOperationRequiresTask(id) {
			continue
		}
		descriptor, ok := registry.Descriptor(id)
		if !ok {
			t.Errorf("operation %s is not registered", id)
			continue
		}
		result := app.executeCoreInputOperation(operation.OperationInvocation{Descriptor: descriptor, Context: ctx})
		if result.Err == nil {
			t.Errorf("core operation %s silently did nothing from a project header cursor", id)
			continue
		}
		if result.Err.Error() != operation.OperationTargetRejectionMessage(operation.OperationCategoryTask) {
			t.Errorf("core operation %s rejects with %q, not the shared sentence", id, result.Err.Error())
		}
	}
}

// TestHeaderTargetKeyPressesReachTheRejection presses each refused key path on a
// project header cursor. The screen keeps the last error until something
// replaces it, so a scenario cannot tell one refusal from the next; this drives
// the keys one at a time from a clean shell instead.
func TestHeaderTargetKeyPressesReachTheRejection(t *testing.T) {
	for _, sequence := range [][]string{
		// Every operation under w and under ! is a task mutation, so both chords
		// refuse at the prefix rather than waiting for a second key that can never
		// resolve.
		{"w"},
		{"!"},
		{"@"},
		{"y"},
		// e is absent: it is one of the three keys a project header answers, beside
		// * and #, so from here it edits the project rather than refusing. The
		// task-editing meaning is what the header refuses, and E and Ctrl+E below
		// still cover that the rest of the edit family stays task-only.
		{"E"},
		{"ctrl+e"},
		{"shift+enter"},
		// b is absent for the same reason e is: a header answers it. Peek reads a
		// project header's detail as readily as a task's, so from here the key opens
		// the pane rather than refusing.
		{">"},
		{"<"},
		{"%"},
		{"]"},
		{"["},
	} {
		t.Run(strings.Join(sequence, ""), func(t *testing.T) {
			app := headerTargetAuditApp(t)
			outlinerView := app.crossOutliner
			if outlinerView == nil {
				t.Fatal("fixture app has no Outliner")
			}
			outlinerView.Cursor = headerTargetProjectHeaderRow(t, outlinerView)

			var last tea.Cmd
			for _, name := range sequence {
				key, err := parseUITestKey(name)
				if err != nil {
					t.Fatalf("parse %q: %v", name, err)
				}
				handled, cmd := app.handleSimplifiedInputKey(key)
				if !handled {
					t.Fatalf("key %q was not consumed by the Outliner grammar", name)
				}
				last = cmd
			}
			if last == nil {
				t.Fatal("the key path did nothing instead of refusing the header cursor")
			}
			failure, ok := last().(status.ErrorMsg)
			if !ok {
				t.Fatalf("the key path produced %T, not a refusal", last())
			}
			if failure.Err.Error() != operation.OperationTargetRejectionMessage(operation.OperationCategoryTask) {
				t.Fatalf("refusal reads %q, not the shared sentence", failure.Err.Error())
			}
		})
	}
}

// headerTargetCursorContext parks the Outliner cursor on a project header row
// and returns the context dispatch would build from it.
func headerTargetCursorContext(t *testing.T) operation.CommandContext {
	t.Helper()
	app := headerTargetAuditApp(t)
	outlinerView := app.crossOutliner
	if outlinerView == nil {
		t.Fatal("fixture app has no Outliner")
	}
	outlinerView.Cursor = headerTargetProjectHeaderRow(t, outlinerView)
	ctx := app.operationCommandContext()
	ctx.View = presentation.ViewCrossProjectOutliner
	return ctx
}

func headerTargetClassification(
	t *testing.T,
	resolver operation.OperationTargetResolver,
	descriptor operation.OperationDescriptor,
	ctx operation.CommandContext,
) string {
	t.Helper()
	resolved, err := resolver.Resolve(descriptor, ctx)
	if err != nil {
		return "reject"
	}
	if resolved.Count() == 0 {
		return "target-free"
	}
	for _, target := range resolved.Targets {
		if target.Entity.Kind != operation.EntityKindProject {
			t.Errorf(
				"operation %s resolved %s from a project header cursor",
				descriptor.ID,
				target.Entity,
			)
			return "unclassifiable"
		}
	}
	return "project"
}

func joinLines(values []string) string {
	joined := ""
	for _, value := range values {
		joined += value + "\n"
	}
	return joined
}
