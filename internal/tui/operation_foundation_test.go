package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

func foundationTestContext() CommandContext {
	ctx := PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	ctx.Entity = EntityContext{ProjectID: "project-b", TaskID: "task-current"}
	ctx.CurrentEntity = TaskEntity("project-b", "task-current")
	ctx.Selection.Tasks = []taskref.TaskIdentity{{ProjectID: "project-a", TaskID: "task-1"}}
	ctx.Selection.EntityIdentities = []EntityIdentity{TaskEntity("project-a", "task-1")}
	ctx.ScopeProjectIDs = []string{"project-a", "project-b"}
	return ctx
}

func TestFoundationInvariants(t *testing.T) {
	t.Run("every planned identity form is typed and pointer free", func(t *testing.T) {
		forms := []struct {
			name       string
			identity   EntityIdentity
			want       EntityIdentity
			wantString string
		}{
			{
				"task", TaskEntity(" project-a ", " task-1 "),
				EntityIdentity{Kind: EntityKindTask, ProjectID: "project-a", TaskID: "task-1"},
				"task:project-a/task-1",
			},
			{
				"project", ProjectEntity(" project-a "),
				EntityIdentity{Kind: EntityKindProject, ProjectID: "project-a"},
				"project:project-a",
			},
			{
				"global note", GlobalNoteEntity(" scratch.md "),
				EntityIdentity{Kind: EntityKindNote, Scope: EntityScopeGlobal, Name: "scratch.md"},
				"note:scratch.md",
			},
			{
				"project note", ProjectNoteEntity("project-a", "design.md"),
				EntityIdentity{
					Kind: EntityKindNote, Scope: EntityScopeProject,
					ProjectID: "project-a", Name: "design.md",
				},
				"note:project-a/design.md",
			},
			{
				"workspace tag", WorkspaceTagEntity(" #backend "),
				EntityIdentity{Kind: EntityKindTag, Scope: EntityScopeGlobal, Name: "backend"},
				"tag:backend",
			},
			{
				"project tag", ProjectTagEntity("project-a", "#api"),
				EntityIdentity{
					Kind: EntityKindTag, Scope: EntityScopeProject,
					ProjectID: "project-a", Name: "api",
				},
				"tag:project-a/api",
			},
			{
				"scope", ScopeEntity("project-a"),
				EntityIdentity{Kind: EntityKindScope, Name: "project-a"},
				"scope:project-a",
			},
			{
				"mode", ModeEntity(chrome.ModeInbox),
				EntityIdentity{Kind: EntityKindMode, Name: "inbox"},
				"mode:inbox",
			},
			{
				"tool", ToolEntity("weekly-review"),
				EntityIdentity{Kind: EntityKindTool, Name: "weekly-review"},
				"tool:weekly-review",
			},
			{
				"operation", OperationEntity("tui.task.edit"),
				EntityIdentity{Kind: EntityKindOperation, Name: "tui.task.edit"},
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
		if got := entityForTaskIdentity(task).Task(); got != task {
			t.Fatalf("task round trip = %#v, want %#v", got, task)
		}
		if got := ProjectEntity("project-a").Task(); got != (taskref.TaskIdentity{}) {
			t.Fatalf("non-task identity yielded task %#v", got)
		}
	})

	t.Run("malformed identities never resolve", func(t *testing.T) {
		invalid := []struct {
			name     string
			identity EntityIdentity
		}{
			{"unset kind", EntityIdentity{}},
			{"unknown kind", EntityIdentity{Kind: "widget", Name: "x"}},
			{"task without task ID", EntityIdentity{Kind: EntityKindTask, ProjectID: "project-a"}},
			{"task without project", EntityIdentity{Kind: EntityKindTask, TaskID: "task-1"}},
			{"task with foreign field", EntityIdentity{
				Kind: EntityKindTask, ProjectID: "project-a", TaskID: "task-1", Name: "extra",
			}},
			{"project without ID", EntityIdentity{Kind: EntityKindProject}},
			{"project with task", EntityIdentity{
				Kind: EntityKindProject, ProjectID: "project-a", TaskID: "task-1",
			}},
			{"note without scope", EntityIdentity{Kind: EntityKindNote, Name: "scratch.md"}},
			{"project note without project", EntityIdentity{
				Kind: EntityKindNote, Scope: EntityScopeProject, Name: "scratch.md",
			}},
			{"global note with project", EntityIdentity{
				Kind: EntityKindNote, Scope: EntityScopeGlobal,
				ProjectID: "project-a", Name: "scratch.md",
			}},
			{"tag without name", EntityIdentity{Kind: EntityKindTag, Scope: EntityScopeGlobal}},
			{"tag that normalizes away", WorkspaceTagEntity(" # ")},
			{"mode with project", EntityIdentity{
				Kind: EntityKindMode, ProjectID: "project-a", Name: "inbox",
			}},
			{"tool without name", EntityIdentity{Kind: EntityKindTool}},
		}
		for _, test := range invalid {
			if test.identity.Valid() {
				t.Fatalf("%s should be invalid: %#v", test.name, test.identity)
			}
		}

		identities := make([]EntityIdentity, 0, len(invalid))
		for _, test := range invalid {
			identities = append(identities, test.identity)
		}
		identities = append(identities, TaskEntity("project-b", "task-2"), TaskEntity("project-a", "task-1"))
		identities = append(identities, TaskEntity("project-a", "task-1"))
		want := []EntityIdentity{
			TaskEntity("project-a", "task-1"),
			TaskEntity("project-b", "task-2"),
		}
		if got := normalizedEntityIdentities(identities); !slices.Equal(got, want) {
			t.Fatalf("normalized identities = %v, want %v", got, want)
		}
	})

	t.Run("every target source resolves", func(t *testing.T) {
		// A task-category descriptor may only name a task source; the project
		// sources are exercised under the category that owns them.
		sources := []struct {
			source      OperationTargetSource
			category    OperationCategory
			cardinality TargetCardinality
			wantCount   int
		}{
			{OperationTargetGlobal, OperationCategorySystem, TargetCardinalityNone, 0},
			{OperationTargetCurrentTask, OperationCategoryTask, TargetCardinalityOne, 1},
			{OperationTargetSelectedTasks, OperationCategoryTask, TargetCardinalityOneOrMore, 1},
			{OperationTargetCurrentProject, OperationCategoryProject, TargetCardinalityOne, 1},
			{OperationTargetScopedProjects, OperationCategoryProject, TargetCardinalityOneOrMore, 2},
			{OperationTargetCurrentEntity, OperationCategoryTask, TargetCardinalityOne, 1},
			{OperationTargetSelectedEntities, OperationCategoryTask, TargetCardinalityOneOrMore, 1},
		}
		ctx := foundationTestContext()
		for _, test := range sources {
			if !validOperationTargetSource(test.source) {
				t.Fatalf("%s should be a valid target source", test.source)
			}
			descriptor := OperationDescriptor{
				ID:                OperationID("test." + string(test.source)),
				Label:             string(test.source),
				Category:          test.category,
				TargetSource:      test.source,
				TargetCardinality: test.cardinality,
				Risk:              OperationRiskReadOnly,
			}
			targets, err := (OperationTargetResolver{}).Resolve(descriptor, ctx)
			if err != nil {
				t.Fatalf("Resolve(%s): %v", test.source, err)
			}
			if got := targets.Count(); got != test.wantCount {
				t.Fatalf("Resolve(%s) count = %d, want %d", test.source, got, test.wantCount)
			}
			if test.source == OperationTargetGlobal {
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
	ctx.CurrentEntity = ProjectNoteEntity("project-a", "design.md")
	ctx.Selection.EntityIdentities = []EntityIdentity{
		WorkspaceTagEntity("#review"),
		ProjectTagEntity("project-a", "api"),
		WorkspaceTagEntity("review"),
	}

	current := OperationDescriptor{
		ID: "note.rename", Label: "Rename note", Category: OperationCategoryNote,
		TargetSource: OperationTargetCurrentEntity, TargetCardinality: TargetCardinalityOne,
		Risk: OperationRiskMutation,
	}
	targets, err := (OperationTargetResolver{}).Resolve(current, ctx)
	if err != nil {
		t.Fatalf("Resolve current entity: %v", err)
	}
	wantCurrent := []EntityIdentity{ProjectNoteEntity("project-a", "design.md")}
	if got := targets.Entities(); !slices.Equal(got, wantCurrent) {
		t.Fatalf("current entities = %v, want %v", got, wantCurrent)
	}
	if got := targets.ProjectIDs(); !slices.Equal(got, []string{"project-a"}) {
		t.Fatalf("project-scoped note should expose its project, got %v", got)
	}
	if got := targets.Tasks(); len(got) != 0 {
		t.Fatalf("note target exposed tasks %v", got)
	}

	selected := OperationDescriptor{
		ID: "tag.rename", Label: "Rename tag", Category: OperationCategoryTag,
		TargetSource: OperationTargetSelectedEntities, TargetCardinality: TargetCardinalityOneOrMore,
		Risk: OperationRiskMutation,
	}
	targets, err = (OperationTargetResolver{}).Resolve(selected, ctx)
	if err != nil {
		t.Fatalf("Resolve selected entities: %v", err)
	}
	wantSelected := []EntityIdentity{
		WorkspaceTagEntity("review"),
		ProjectTagEntity("project-a", "api"),
	}
	if got := targets.Entities(); !slices.Equal(got, wantSelected) {
		t.Fatalf("selected entities = %v, want %v", got, wantSelected)
	}
}

func TestOperationTargetResolverProjectsTaskSelectionOntoBothSources(t *testing.T) {
	ctx := PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	ctx.Entity = EntityContext{ProjectID: "project-b", TaskID: "task-current"}
	ctx.Selection.Tasks = []taskref.TaskIdentity{
		{ProjectID: "project-b", TaskID: "task-2"},
		{ProjectID: "project-a", TaskID: "task-1"},
	}

	descriptor := OperationDescriptor{
		ID: "task.bulk", Label: "Bulk", Category: OperationCategoryTask,
		TargetSource: OperationTargetSelectedTasks, TargetCardinality: TargetCardinalityOneOrMore,
		Risk: OperationRiskMutation,
	}
	targets, err := (OperationTargetResolver{}).Resolve(descriptor, ctx)
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
	if got := targets.Entities(); !slices.Equal(got, []EntityIdentity{
		TaskEntity("project-a", "task-1"),
		TaskEntity("project-b", "task-2"),
	}) {
		t.Fatalf("task targets should also carry typed identities, got %v", got)
	}

	// A context that carries only a task selection still resolves the
	// selected-entity source, so both sources name the same rows.
	descriptor.TargetSource = OperationTargetSelectedEntities
	targets, err = (OperationTargetResolver{}).Resolve(descriptor, ctx)
	if err != nil {
		t.Fatalf("Resolve selected entities: %v", err)
	}
	if got := targets.Tasks(); !slices.Equal(got, wantTasks) {
		t.Fatalf("selected-entity fallback tasks = %v, want %v", got, wantTasks)
	}
}

func TestOperationTargetResolverRejectsStaleAndEmptyTargets(t *testing.T) {
	stale := PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	stale.CurrentEntity = EntityIdentity{Kind: EntityKindNote, Name: "orphan.md"}
	stale.Selection.EntityIdentities = []EntityIdentity{{Kind: EntityKindTask, ProjectID: "gone"}}

	for _, source := range []OperationTargetSource{
		OperationTargetCurrentEntity,
		OperationTargetSelectedEntities,
		OperationTargetCurrentTask,
		OperationTargetCurrentProject,
	} {
		descriptor := OperationDescriptor{
			ID: OperationID("stale." + string(source)), Label: "Stale",
			Category: OperationCategoryTask, TargetSource: source,
			TargetCardinality: TargetCardinalityOne, Risk: OperationRiskMutation,
		}
		if _, err := (OperationTargetResolver{}).Resolve(descriptor, stale); err == nil ||
			err.Error() != operationTargetRejectionMessage(OperationCategoryTask) {
			t.Fatalf("Resolve(%s) error = %v, want the target rejection sentence", source, err)
		}
	}

	descriptor := OperationDescriptor{
		ID: "invalid.source", Label: "Invalid", Category: OperationCategoryTask,
		TargetSource: "cursor-row", TargetCardinality: TargetCardinalityOne,
		Risk: OperationRiskMutation,
	}
	if _, err := (OperationTargetResolver{}).Resolve(descriptor, stale); err == nil ||
		!strings.Contains(err.Error(), "target source") {
		t.Fatalf("Resolve invalid source error = %v, want a target source failure", err)
	}
}

func TestOperationRegistryValidatesCapabilities(t *testing.T) {
	registry := NewOperationRegistry()

	unknownCapability := operationTestDescriptor("task.typo", "Typo")
	unknownCapability.RequiredCapabilities = NewCapabilitySet("scop:project")
	if err := registry.Register(unknownCapability); err == nil ||
		!strings.Contains(err.Error(), "capability") {
		t.Fatalf("unknown capability error = %v, want a capability rejection", err)
	}

}

func TestCommandContextCloneIsolatesTypedSelection(t *testing.T) {
	ctx := foundationTestContext()
	clone := cloneCommandContext(ctx)
	clone.Selection.EntityIdentities[0] = TaskEntity("project-z", "task-9")
	clone.CurrentEntity = ProjectEntity("project-z")

	if ctx.Selection.EntityIdentities[0] != TaskEntity("project-a", "task-1") {
		t.Fatalf("clone mutated the source selection: %v", ctx.Selection.EntityIdentities)
	}
	if ctx.CurrentEntity != TaskEntity("project-b", "task-current") {
		t.Fatalf("clone mutated the source current entity: %v", ctx.CurrentEntity)
	}
	if clone.CurrentEntity != ProjectEntity("project-z") {
		t.Fatalf("clone did not keep its own current entity: %v", clone.CurrentEntity)
	}
}

func TestOperationExecutorRejectsTargetHandlersForTargetFreeOperations(t *testing.T) {
	registry := NewOperationRegistry()
	descriptor := OperationDescriptor{
		ID: "global.tool", Label: "Tool", Category: OperationCategorySystem,
		TargetSource: OperationTargetGlobal, TargetCardinality: TargetCardinalityNone,
		Risk: OperationRiskReadOnly,
	}
	if err := registry.Register(descriptor); err != nil {
		t.Fatalf("Register descriptor: %v", err)
	}
	executor := NewOperationExecutor(registry)

	if err := executor.Register(OperationBinding{
		OperationID: descriptor.ID,
		SingleHandler: func(OperationInvocation, OperationTarget) OperationHandlerResult {
			return OperationHandlerResult{}
		},
	}); err == nil || !strings.Contains(err.Error(), "target-free handler") {
		t.Fatalf("single handler binding error = %v, want a target-free rejection", err)
	}

	if err := executor.Register(OperationBinding{
		OperationID:          descriptor.ID,
		RequiredCapabilities: NewCapabilitySet("bogus"),
		Handler: func(OperationInvocation) OperationHandlerResult {
			return OperationHandlerResult{}
		},
	}); err == nil || !strings.Contains(err.Error(), "capability") {
		t.Fatalf("unknown binding capability error = %v, want a capability rejection", err)
	}
}

func TestOperationExecutorPassesTypedIdentitiesToHandlers(t *testing.T) {
	registry := NewOperationRegistry()
	descriptor := OperationDescriptor{
		ID: "tag.rename", Label: "Rename tag", Category: OperationCategoryTag,
		TargetSource: OperationTargetSelectedEntities, TargetCardinality: TargetCardinalityOneOrMore,
		Risk: OperationRiskMutation,
	}
	if err := registry.Register(descriptor); err != nil {
		t.Fatalf("Register descriptor: %v", err)
	}
	executor := NewOperationExecutor(registry)

	var received []EntityIdentity
	if err := executor.Register(OperationBinding{
		OperationID: descriptor.ID,
		BulkHandler: func(_ OperationInvocation, targets []OperationTarget) OperationHandlerResult {
			for _, target := range targets {
				received = append(received, target.Entity)
			}
			return OperationHandlerResult{Status: "renamed"}
		},
	}); err != nil {
		t.Fatalf("Register binding: %v", err)
	}

	ctx := PresentationCommandContext(presentation.ViewCrossProjectOutliner)
	ctx.Selection.EntityIdentities = []EntityIdentity{
		ProjectTagEntity("project-a", "#api"),
		WorkspaceTagEntity("review"),
	}
	result := executor.Execute(OperationExecutionRequest{ID: descriptor.ID, Context: ctx})
	if result.Err != nil {
		t.Fatalf("Execute: %v", result.Err)
	}
	want := []EntityIdentity{
		WorkspaceTagEntity("review"),
		ProjectTagEntity("project-a", "api"),
	}
	if !slices.Equal(received, want) {
		t.Fatalf("handler targets = %v, want %v", received, want)
	}
}
