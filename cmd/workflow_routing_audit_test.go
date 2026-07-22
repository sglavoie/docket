package cmd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type cliWorkflowRoute struct {
	file      string
	handler   string
	required  []string
	forbidden []string
}

// legacyWorkflowEnabledCLIRoutes is the explicit regression baseline for every
// command family that historically evaluated workflow rules. Adding a new bare
// service call to one of these handlers, or dropping its workflow-aware call,
// fails the AST audit below.
var legacyWorkflowEnabledCLIRoutes = []cliWorkflowRoute{
	{file: "add.go", handler: "runQuickAdd", required: []string{"AddTaskWithWorkflow"}, forbidden: []string{"AddTask"}},
	{file: "task_add.go", handler: "runTaskAdd", required: []string{"AddTaskWithWorkflow"}, forbidden: []string{"AddTask"}},
	{file: "template_use.go", handler: "runTemplateUse", required: []string{"ApplyTemplateWithWorkflow"}, forbidden: []string{"ApplyTemplate"}},
	{file: "task_edit.go", handler: "runTaskEdit", required: []string{"EditWithWorkflow"}, forbidden: []string{"Edit"}},
	{file: "task_promote.go", handler: "changeTaskType", required: []string{"ChangeTaskTypeWithWorkflowByLocator"}, forbidden: []string{"ChangeTaskType", "ChangeTaskTypeWithWorkflow"}},
	{file: "task_state.go", handler: "runAppTransition", required: []string{"StateWithWorkflowByLocator"}, forbidden: []string{"Transition", "StateWithWorkflow"}},
	{file: "task_complete.go", handler: "runStateCommand", required: []string{"StateWithWorkflowByLocator"}, forbidden: []string{"Transition", "StateWithWorkflow"}},
	{file: "task_priority.go", handler: "runTaskPriority", required: []string{"SetPriorityWithWorkflowByLocator"}, forbidden: []string{"SetPriority", "SetPriorityWithWorkflow"}},
	{file: "task_dim.go", handler: "runTaskDim", required: []string{"ToggleDimmedWithWorkflowByLocator"}, forbidden: []string{"ToggleDimmed", "ToggleDimmedWithWorkflow"}},
	{file: "task_dim.go", handler: "runTaskUndim", required: []string{"UndimWithWorkflowByLocator"}, forbidden: []string{"Undim", "UndimWithWorkflow"}},
	{file: "task_snooze.go", handler: "runTaskSnooze", required: []string{"GuardedSnoozeWithWorkflowByLocator"}, forbidden: []string{"Snooze", "GuardedSnoozeWithWorkflow"}},
	{file: "task_park.go", handler: "runTaskPark", required: []string{"ParkWithWorkflowByLocator"}, forbidden: []string{"Park", "ParkWithWorkflow"}},
	{file: "task_park.go", handler: "runTaskUnpark", required: []string{"UnparkWithWorkflowByLocator"}, forbidden: []string{"Unpark", "UnparkWithWorkflow"}},
	{file: "epic_baseline.go", handler: "runEpicBaseline", required: []string{"BaselineEpicWithWorkflowByLocator"}, forbidden: []string{"BaselineEpic", "BaselineEpicWithWorkflow"}},
	{file: "task_attr.go", handler: "runTaskAttr", required: []string{"ApplyTypeAttrsWithWorkflowByLocator", "ClearTypeAttrsWithWorkflowByLocator"}, forbidden: []string{"ApplyTypeAttrs", "ClearTypeAttrs", "ApplyTypeAttrsWithWorkflow", "ClearTypeAttrsWithWorkflow"}},
	{file: "task_comment_add.go", handler: "runTaskCommentAdd", required: []string{"AddCommentWithWorkflowByLocator"}, forbidden: []string{"AddComment", "AddCommentWithWorkflow"}},
	{file: "task_comment_remove.go", handler: "runTaskCommentRemove", required: []string{"RemoveCommentWithWorkflowByLocator", "ClearCommentsWithWorkflowByLocator"}, forbidden: []string{"RemoveComment", "ClearComments", "RemoveCommentWithWorkflow", "ClearCommentsWithWorkflow"}},
	{file: "task_comment_resolve.go", handler: "runTaskCommentResolve", required: []string{"ToggleCommentResolvedWithWorkflowByLocator"}, forbidden: []string{"ToggleCommentResolved", "ToggleCommentResolvedWithWorkflow"}},
	{file: "task_depends.go", handler: "runTaskDepends", required: []string{"AddDependencyWithWorkflowByLocator", "RemoveDependencyWithWorkflowByLocator", "ClearDependenciesWithWorkflowByLocator"}, forbidden: []string{"AddDependency", "RemoveDependency", "ClearDependencies", "AddDependencyWithWorkflow", "RemoveDependencyWithWorkflow", "ClearDependenciesWithWorkflow"}},
}

var knownOwnerCLILocatorRoutes = []cliWorkflowRoute{
	{file: "task_comment_edit.go", handler: "runTaskCommentEdit", required: []string{"EditCommentByLocator"}, forbidden: []string{"EditComment"}},
	{file: "task_recur.go", handler: "runTaskRecur", required: []string{"SkipRecurrenceByLocator", "DeferRecurrenceByLocator", "ClearRecurrenceByLocator", "SetRecurrenceModeByLocator", "SetRecurrenceByLocator"}, forbidden: []string{"SkipRecurrence", "DeferRecurrence", "ClearRecurrence", "SetRecurrenceMode", "SetRecurrence"}},
	{file: "score_set.go", handler: "runScoreSet", required: []string{"SetScoreDimensionsByLocator"}, forbidden: []string{"SetScoreDimensions"}},
	{file: "attach.go", handler: "runAttach", required: []string{"AttachURLByLocator", "AttachCodeReferenceByLocator", "AttachFileByLocator"}, forbidden: []string{"AttachURL", "AttachCodeReference", "AttachFile"}},
	{file: "attachment_remove.go", handler: "runAttachmentRemove", required: []string{"RemoveAttachmentByLocator"}, forbidden: []string{"RemoveAttachment"}},
	{file: "task_move.go", handler: "runTaskHierarchyMove", required: []string{"MoveTaskUpByLocator", "MoveTaskDownByLocator", "IndentTaskByLocator", "OutdentTaskByLocator"}, forbidden: []string{"MoveTaskUp", "MoveTaskDown", "IndentTask", "OutdentTask"}},
}

// historicallyBareCLIHandlers records representative mutation families that did
// not evaluate workflow rules before the application-layer migration. They must
// remain free of workflow-aware service calls: comment edit, recurrence, scoring,
// attachments, notes, and ordinary workflow/template configuration mutations.
var historicallyBareCLIHandlers = []struct {
	file    string
	handler string
}{
	{file: "task_comment_edit.go", handler: "runTaskCommentEdit"},
	{file: "task_recur.go", handler: "runTaskRecur"},
	{file: "score_set.go", handler: "runScoreSet"},
	{file: "score_assign.go", handler: "runScoreAssign"},
	{file: "score_assign.go", handler: "runScoreUnassign"},
	{file: "attach.go", handler: "runAttach"},
	{file: "attachment_remove.go", handler: "runAttachmentRemove"},
	{file: "note_add.go", handler: "runNoteAdd"},
	{file: "note_edit.go", handler: "runNoteEdit"},
	{file: "note_delete.go", handler: "runNoteDelete"},
	{file: "workflow_add.go", handler: "runWorkflowAdd"},
	{file: "workflow_edit.go", handler: "runWorkflowEdit"},
	{file: "workflow_delete.go", handler: "runWorkflowDelete"},
	{file: "template_add.go", handler: "runTemplateAdd"},
	{file: "template_edit.go", handler: "runTemplateEdit"},
	{file: "template_delete.go", handler: "runTemplateDelete"},
}

func TestLegacyWorkflowEnabledCLIRoutingAudit(t *testing.T) {
	for _, route := range legacyWorkflowEnabledCLIRoutes {
		route := route
		t.Run(route.handler, func(t *testing.T) {
			calls := selectorCallsInFunction(t, route.file, route.handler)
			for _, required := range route.required {
				if !calls[required] {
					t.Errorf("%s no longer calls %s", route.handler, required)
				}
			}
			for _, forbidden := range route.forbidden {
				if calls[forbidden] {
					t.Errorf("%s regressed to bare method %s", route.handler, forbidden)
				}
			}
		})
	}
}

func TestHistoricallyBareCLIRoutingAudit(t *testing.T) {
	for _, route := range historicallyBareCLIHandlers {
		route := route
		t.Run(route.handler, func(t *testing.T) {
			calls := selectorCallsInFunction(t, route.file, route.handler)
			var workflowCalls []string
			for call := range calls {
				if strings.Contains(call, "WithWorkflow") {
					workflowCalls = append(workflowCalls, call)
				}
			}
			sort.Strings(workflowCalls)
			if len(workflowCalls) > 0 {
				t.Fatalf("historically bare handler gained workflow routing: %s", strings.Join(workflowCalls, ", "))
			}
		})
	}
}

func TestKnownOwnerCLIMutationsRetainTaskLocators(t *testing.T) {
	for _, route := range knownOwnerCLILocatorRoutes {
		route := route
		t.Run(route.handler, func(t *testing.T) {
			calls := selectorCallsInFunction(t, route.file, route.handler)
			for _, required := range route.required {
				if !calls[required] {
					t.Errorf("%s no longer calls %s", route.handler, required)
				}
			}
			for _, forbidden := range route.forbidden {
				if calls[forbidden] {
					t.Errorf("%s discarded project ownership through %s", route.handler, forbidden)
				}
			}
		})
	}
}

func selectorCallsInFunction(t *testing.T, filename, function string) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	path := filepath.Join(".", filename)
	parsed, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", filename, err)
	}

	var body *ast.BlockStmt
	for _, declaration := range parsed.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if ok && fn.Name.Name == function {
			body = fn.Body
			break
		}
	}
	if body == nil {
		t.Fatalf("function %s not found in %s", function, filename)
	}

	calls := make(map[string]bool)
	ast.Inspect(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if ok {
			calls[selector.Sel.Name] = true
		}
		return true
	})
	return calls
}
