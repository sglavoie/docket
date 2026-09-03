package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func foundationTestContext() operation.CommandContext {
	ctx := PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	ctx.Entity = operation.EntityContext{ProjectID: "project-b", TaskID: "task-current"}
	ctx.CurrentEntity = operation.TaskEntity("project-b", "task-current")
	ctx.Selection.Tasks = []taskref.TaskIdentity{{ProjectID: "project-a", TaskID: "task-1"}}
	ctx.Selection.EntityIdentities = []operation.EntityIdentity{operation.TaskEntity("project-a", "task-1")}
	ctx.ScopeProjectIDs = []string{"project-a", "project-b"}
	return ctx
}

func TestFoundationInvariants(t *testing.T) {
	t.Run("every planned identity form is typed and pointer free", func(t *testing.T) {
		forms := []struct {
			name       string
			identity   operation.EntityIdentity
			want       operation.EntityIdentity
			wantString string
		}{
			{
				"task", operation.TaskEntity(" project-a ", " task-1 "),
				operation.EntityIdentity{Kind: operation.EntityKindTask, ProjectID: "project-a", TaskID: "task-1"},
				"task:project-a/task-1",
			},
			{
				"project", operation.ProjectEntity(" project-a "),
				operation.EntityIdentity{Kind: operation.EntityKindProject, ProjectID: "project-a"},
				"project:project-a",
			},
			{
				"global note", operation.GlobalNoteEntity(" scratch.md "),
				operation.EntityIdentity{Kind: operation.EntityKindNote, Scope: operation.EntityScopeGlobal, Name: "scratch.md"},
				"note:scratch.md",
			},
			{
				"project note", operation.ProjectNoteEntity("project-a", "design.md"),
				operation.EntityIdentity{
					Kind: operation.EntityKindNote, Scope: operation.EntityScopeProject,
					ProjectID: "project-a", Name: "design.md",
				},
				"note:project-a/design.md",
			},
			{
				"workspace tag", WorkspaceTagEntity(" #backend "),
				operation.EntityIdentity{Kind: operation.EntityKindTag, Scope: operation.EntityScopeGlobal, Name: "backend"},
				"tag:backend",
			},
			{
				"project tag", ProjectTagEntity("project-a", "#api"),
				operation.EntityIdentity{
					Kind: operation.EntityKindTag, Scope: operation.EntityScopeProject,
					ProjectID: "project-a", Name: "api",
				},
				"tag:project-a/api",
			},
			{
				"scope", ScopeEntity("project-a"),
				operation.EntityIdentity{Kind: operation.EntityKindScope, Name: "project-a"},
				"scope:project-a",
			},
			{
				"mode", ModeEntity(chrome.ModeInbox),
				operation.EntityIdentity{Kind: operation.EntityKindMode, Name: "inbox"},
				"mode:inbox",
			},
			{
				"tool", ToolEntity("weekly-review"),
				operation.EntityIdentity{Kind: operation.EntityKindTool, Name: "weekly-review"},
				"tool:weekly-review",
			},
			{
				"operation", OperationEntity("tui.task.edit"),
				operation.EntityIdentity{Kind: operation.EntityKindOperation, Name: "tui.task.edit"},
				"operation:tui.task.edit",
			},
		}
		rendered := make(map[string]string, len(forms))
		for _, form := range forms {
			if form.identity != form.want {
				t.Fatalf("%s identity = %#v, want %#v", form.name, form.identity, form.want)
			}
			if !form.identity.Valid() {
				t.Fatalf("%s identity should be valid: %#v", form.name, form.identity)
			}
			if got := form.identity.String(); got != form.wantString {
				t.Fatalf("%s String = %q, want %q", form.name, got, form.wantString)
			}
			if owner, exists := rendered[form.wantString]; exists {
				t.Fatalf("%s and %s share identity string %q", owner, form.name, form.wantString)
			}
			rendered[form.wantString] = form.name
		}

		task := taskref.TaskIdentity{ProjectID: "project-a", TaskID: "task-1"}
		if got := operation.EntityForTaskIdentity(task).Task(); got != task {
			t.Fatalf("task round trip = %#v, want %#v", got, task)
		}
		if got := operation.ProjectEntity("project-a").Task(); got != (taskref.TaskIdentity{}) {
			t.Fatalf("non-task identity yielded task %#v", got)
		}
	})

	t.Run("malformed identities never resolve", func(t *testing.T) {
		invalid := []struct {
			name     string
			identity operation.EntityIdentity
		}{
			{"unset kind", operation.EntityIdentity{}},
			{"unknown kind", operation.EntityIdentity{Kind: "widget", Name: "x"}},
			{"task without task ID", operation.EntityIdentity{Kind: operation.EntityKindTask, ProjectID: "project-a"}},
			{"task without project", operation.EntityIdentity{Kind: operation.EntityKindTask, TaskID: "task-1"}},
			{"task with foreign field", operation.EntityIdentity{
				Kind: operation.EntityKindTask, ProjectID: "project-a", TaskID: "task-1", Name: "extra",
			}},
			{"project without ID", operation.EntityIdentity{Kind: operation.EntityKindProject}},
			{"project with task", operation.EntityIdentity{
				Kind: operation.EntityKindProject, ProjectID: "project-a", TaskID: "task-1",
			}},
			{"note without scope", operation.EntityIdentity{Kind: operation.EntityKindNote, Name: "scratch.md"}},
			{"project note without project", operation.EntityIdentity{
				Kind: operation.EntityKindNote, Scope: operation.EntityScopeProject, Name: "scratch.md",
			}},
			{"global note with project", operation.EntityIdentity{
				Kind: operation.EntityKindNote, Scope: operation.EntityScopeGlobal,
				ProjectID: "project-a", Name: "scratch.md",
			}},
			{"tag without name", operation.EntityIdentity{Kind: operation.EntityKindTag, Scope: operation.EntityScopeGlobal}},
			{"tag that normalizes away", WorkspaceTagEntity(" # ")},
			{"mode with project", operation.EntityIdentity{
				Kind: operation.EntityKindMode, ProjectID: "project-a", Name: "inbox",
			}},
			{"tool without name", operation.EntityIdentity{Kind: operation.EntityKindTool}},
		}
		for _, test := range invalid {
			if test.identity.Valid() {
				t.Fatalf("%s should be invalid: %#v", test.name, test.identity)
			}
		}

		identities := make([]operation.EntityIdentity, 0, len(invalid))
		for _, test := range invalid {
			identities = append(identities, test.identity)
		}
		identities = append(identities, operation.TaskEntity("project-b", "task-2"), operation.TaskEntity("project-a", "task-1"))
		identities = append(identities, operation.TaskEntity("project-a", "task-1"))
		want := []operation.EntityIdentity{
			operation.TaskEntity("project-a", "task-1"),
			operation.TaskEntity("project-b", "task-2"),
		}
		if got := operation.NormalizedEntityIdentities(identities); !slices.Equal(got, want) {
			t.Fatalf("normalized identities = %v, want %v", got, want)
		}
	})

	t.Run("every target source resolves", func(t *testing.T) {
		// A task-category descriptor may only name a task source; the project
		// sources are exercised under the category that owns them.
		sources := []struct {
			source      operation.OperationTargetSource
			category    operation.OperationCategory
			cardinality operation.TargetCardinality
			wantCount   int
		}{
			{operation.OperationTargetGlobal, operation.OperationCategorySystem, operation.TargetCardinalityNone, 0},
			{operation.OperationTargetCurrentTask, operation.OperationCategoryTask, operation.TargetCardinalityOne, 1},
			{operation.OperationTargetSelectedTasks, operation.OperationCategoryTask, operation.TargetCardinalityOneOrMore, 1},
			{operation.OperationTargetCurrentProject, operation.OperationCategoryProject, operation.TargetCardinalityOne, 1},
			{operation.OperationTargetScopedProjects, operation.OperationCategoryProject, operation.TargetCardinalityOneOrMore, 2},
			{operation.OperationTargetCurrentEntity, operation.OperationCategoryTask, operation.TargetCardinalityOne, 1},
			{operation.OperationTargetSelectedEntities, operation.OperationCategoryTask, operation.TargetCardinalityOneOrMore, 1},
		}
		ctx := foundationTestContext()
		for _, test := range sources {
			if !operation.ValidOperationTargetSource(test.source) {
				t.Fatalf("%s should be a valid target source", test.source)
			}
			descriptor := operation.OperationDescriptor{
				ID:                operation.OperationID("test." + string(test.source)),
				Label:             string(test.source),
				Category:          test.category,
				TargetSource:      test.source,
				TargetCardinality: test.cardinality,
				Risk:              operation.OperationRiskReadOnly,
			}
			targets, err := (operation.OperationTargetResolver{}).Resolve(descriptor, ctx)
			if err != nil {
				t.Fatalf("Resolve(%s): %v", test.source, err)
			}
			if got := targets.Count(); got != test.wantCount {
				t.Fatalf("Resolve(%s) count = %d, want %d", test.source, got, test.wantCount)
			}
			if test.source == operation.OperationTargetGlobal {
				continue
			}
			for _, target := range targets.Targets {
				if !target.Entity.Valid() {
					t.Fatalf("Resolve(%s) produced untyped target %#v", test.source, target)
				}
			}
		}
	})

}

func TestOperationTargetResolverResolvesTypedEntityIdentities(t *testing.T) {
	ctx := PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	ctx.CurrentEntity = operation.ProjectNoteEntity("project-a", "design.md")
	ctx.Selection.EntityIdentities = []operation.EntityIdentity{
		WorkspaceTagEntity("#review"),
		ProjectTagEntity("project-a", "api"),
		WorkspaceTagEntity("review"),
	}

	current := operation.OperationDescriptor{
		ID: "note.rename", Label: "Rename note", Category: operation.OperationCategoryNote,
		TargetSource: operation.OperationTargetCurrentEntity, TargetCardinality: operation.TargetCardinalityOne,
		Risk: operation.OperationRiskMutation,
	}
	targets, err := (operation.OperationTargetResolver{}).Resolve(current, ctx)
	if err != nil {
		t.Fatalf("Resolve current entity: %v", err)
	}
	wantCurrent := []operation.EntityIdentity{operation.ProjectNoteEntity("project-a", "design.md")}
	if got := targets.Entities(); !slices.Equal(got, wantCurrent) {
		t.Fatalf("current entities = %v, want %v", got, wantCurrent)
	}
	if got := targets.ProjectIDs(); !slices.Equal(got, []string{"project-a"}) {
		t.Fatalf("project-scoped note should expose its project, got %v", got)
	}
	if got := targets.Tasks(); len(got) != 0 {
		t.Fatalf("note target exposed tasks %v", got)
	}

	selected := operation.OperationDescriptor{
		ID: "tag.rename", Label: "Rename tag", Category: operation.OperationCategoryTag,
		TargetSource: operation.OperationTargetSelectedEntities, TargetCardinality: operation.TargetCardinalityOneOrMore,
		Risk: operation.OperationRiskMutation,
	}
	targets, err = (operation.OperationTargetResolver{}).Resolve(selected, ctx)
	if err != nil {
		t.Fatalf("Resolve selected entities: %v", err)
	}
	wantSelected := []operation.EntityIdentity{
		WorkspaceTagEntity("review"),
		ProjectTagEntity("project-a", "api"),
	}
	if got := targets.Entities(); !slices.Equal(got, wantSelected) {
		t.Fatalf("selected entities = %v, want %v", got, wantSelected)
	}
}

func TestOperationTargetResolverProjectsTaskSelectionOntoBothSources(t *testing.T) {
	ctx := PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	ctx.Entity = operation.EntityContext{ProjectID: "project-b", TaskID: "task-current"}
	ctx.Selection.Tasks = []taskref.TaskIdentity{
		{ProjectID: "project-b", TaskID: "task-2"},
		{ProjectID: "project-a", TaskID: "task-1"},
	}

	descriptor := operation.OperationDescriptor{
		ID: "task.bulk", Label: "Bulk", Category: operation.OperationCategoryTask,
		TargetSource: operation.OperationTargetSelectedTasks, TargetCardinality: operation.TargetCardinalityOneOrMore,
		Risk: operation.OperationRiskMutation,
	}
	targets, err := (operation.OperationTargetResolver{}).Resolve(descriptor, ctx)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	wantTasks := []taskref.TaskIdentity{
		{ProjectID: "project-a", TaskID: "task-1"},
		{ProjectID: "project-b", TaskID: "task-2"},
	}
	if got := targets.Tasks(); !slices.Equal(got, wantTasks) {
		t.Fatalf("Tasks = %v, want %v", got, wantTasks)
	}
	if got := targets.Entities(); !slices.Equal(got, []operation.EntityIdentity{
		operation.TaskEntity("project-a", "task-1"),
		operation.TaskEntity("project-b", "task-2"),
	}) {
		t.Fatalf("task targets should also carry typed identities, got %v", got)
	}

	// A context that carries only a task selection still resolves the
	// selected-entity source, so both sources name the same rows.
	descriptor.TargetSource = operation.OperationTargetSelectedEntities
	targets, err = (operation.OperationTargetResolver{}).Resolve(descriptor, ctx)
	if err != nil {
		t.Fatalf("Resolve selected entities: %v", err)
	}
	if got := targets.Tasks(); !slices.Equal(got, wantTasks) {
		t.Fatalf("selected-entity fallback tasks = %v, want %v", got, wantTasks)
	}
}

func TestOperationTargetResolverRejectsStaleAndEmptyTargets(t *testing.T) {
	stale := PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	stale.CurrentEntity = operation.EntityIdentity{Kind: operation.EntityKindNote, Name: "orphan.md"}
	stale.Selection.EntityIdentities = []operation.EntityIdentity{{Kind: operation.EntityKindTask, ProjectID: "gone"}}

	for _, source := range []operation.OperationTargetSource{
		operation.OperationTargetCurrentEntity,
		operation.OperationTargetSelectedEntities,
		operation.OperationTargetCurrentTask,
		operation.OperationTargetCurrentProject,
	} {
		descriptor := operation.OperationDescriptor{
			ID: operation.OperationID("stale." + string(source)), Label: "Stale",
			Category: operation.OperationCategoryTask, TargetSource: source,
			TargetCardinality: operation.TargetCardinalityOne, Risk: operation.OperationRiskMutation,
		}
		if _, err := (operation.OperationTargetResolver{}).Resolve(descriptor, stale); err == nil ||
			err.Error() != operation.OperationTargetRejectionMessage(operation.OperationCategoryTask) {
			t.Fatalf("Resolve(%s) error = %v, want the target rejection sentence", source, err)
		}
	}

	descriptor := operation.OperationDescriptor{
		ID: "invalid.source", Label: "Invalid", Category: operation.OperationCategoryTask,
		TargetSource: "cursor-row", TargetCardinality: operation.TargetCardinalityOne,
		Risk: operation.OperationRiskMutation,
	}
	if _, err := (operation.OperationTargetResolver{}).Resolve(descriptor, stale); err == nil ||
		!strings.Contains(err.Error(), "target source") {
		t.Fatalf("Resolve invalid source error = %v, want a target source failure", err)
	}
}

func TestOperationRegistryValidatesCapabilities(t *testing.T) {
	registry := operation.NewOperationRegistry()

	unknownCapability := operationTestDescriptor("task.typo", "Typo")
	unknownCapability.RequiredCapabilities = operation.NewCapabilitySet("scop:project")
	if err := registry.Register(unknownCapability); err == nil ||
		!strings.Contains(err.Error(), "capability") {
		t.Fatalf("unknown capability error = %v, want a capability rejection", err)
	}

}

func TestCommandContextCloneIsolatesTypedSelection(t *testing.T) {
	ctx := foundationTestContext()
	clone := cloneCommandContext(ctx)
	clone.Selection.EntityIdentities[0] = operation.TaskEntity("project-z", "task-9")
	clone.CurrentEntity = operation.ProjectEntity("project-z")

	if ctx.Selection.EntityIdentities[0] != operation.TaskEntity("project-a", "task-1") {
		t.Fatalf("clone mutated the source selection: %v", ctx.Selection.EntityIdentities)
	}
	if ctx.CurrentEntity != operation.TaskEntity("project-b", "task-current") {
		t.Fatalf("clone mutated the source current entity: %v", ctx.CurrentEntity)
	}
	if clone.CurrentEntity != operation.ProjectEntity("project-z") {
		t.Fatalf("clone did not keep its own current entity: %v", clone.CurrentEntity)
	}
}

func TestOperationExecutorRejectsTargetHandlersForTargetFreeOperations(t *testing.T) {
	registry := operation.NewOperationRegistry()
	descriptor := operation.OperationDescriptor{
		ID: "global.tool", Label: "Tool", Category: operation.OperationCategorySystem,
		TargetSource: operation.OperationTargetGlobal, TargetCardinality: operation.TargetCardinalityNone,
		Risk: operation.OperationRiskReadOnly,
	}
	if err := registry.Register(descriptor); err != nil {
		t.Fatalf("Register descriptor: %v", err)
	}
	executor := NewOperationExecutor(registry)

	if err := executor.Register(OperationBinding{
		OperationID: descriptor.ID,
		SingleHandler: func(OperationInvocation, operation.OperationTarget) OperationHandlerResult {
			return OperationHandlerResult{}
		},
	}); err == nil || !strings.Contains(err.Error(), "target-free handler") {
		t.Fatalf("single handler binding error = %v, want a target-free rejection", err)
	}

	if err := executor.Register(OperationBinding{
		OperationID:          descriptor.ID,
		RequiredCapabilities: operation.NewCapabilitySet("bogus"),
		Handler: func(OperationInvocation) OperationHandlerResult {
			return OperationHandlerResult{}
		},
	}); err == nil || !strings.Contains(err.Error(), "capability") {
		t.Fatalf("unknown binding capability error = %v, want a capability rejection", err)
	}
}

func TestOperationExecutorPassesTypedIdentitiesToHandlers(t *testing.T) {
	registry := operation.NewOperationRegistry()
	descriptor := operation.OperationDescriptor{
		ID: "tag.rename", Label: "Rename tag", Category: operation.OperationCategoryTag,
		TargetSource: operation.OperationTargetSelectedEntities, TargetCardinality: operation.TargetCardinalityOneOrMore,
		Risk: operation.OperationRiskMutation,
	}
	if err := registry.Register(descriptor); err != nil {
		t.Fatalf("Register descriptor: %v", err)
	}
	executor := NewOperationExecutor(registry)

	var received []operation.EntityIdentity
	if err := executor.Register(OperationBinding{
		OperationID: descriptor.ID,
		BulkHandler: func(_ OperationInvocation, targets []operation.OperationTarget) OperationHandlerResult {
			for _, target := range targets {
				received = append(received, target.Entity)
			}
			return OperationHandlerResult{Status: "renamed"}
		},
	}); err != nil {
		t.Fatalf("Register binding: %v", err)
	}

	ctx := PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	ctx.Selection.EntityIdentities = []operation.EntityIdentity{
		ProjectTagEntity("project-a", "#api"),
		WorkspaceTagEntity("review"),
	}
	result := executor.Execute(OperationExecutionRequest{ID: descriptor.ID, Context: ctx})
	if result.Err != nil {
		t.Fatalf("Execute: %v", result.Err)
	}
	want := []operation.EntityIdentity{
		WorkspaceTagEntity("review"),
		ProjectTagEntity("project-a", "api"),
	}
	if !slices.Equal(received, want) {
		t.Fatalf("handler targets = %v, want %v", received, want)
	}
}
