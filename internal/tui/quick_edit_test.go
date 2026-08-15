package tui

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestParseQuickEdit_Priority(t *testing.T) {
	project := &models.Project{Name: "test"}
	task := &models.Task{Title: "Original", Priority: 3}

	result, err := parseQuickEdit("Updated title !1", task, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.title != "Updated title" {
		t.Errorf("expected title 'Updated title', got '%s'", result.title)
	}
	if !result.prioritySet || result.priority != 1 {
		t.Errorf("expected priority 1, got %d (set=%v)", result.priority, result.prioritySet)
	}
}

func TestParseQuickEditClearPriorityWithBangZero(t *testing.T) {
	task := &models.Task{ID: "task", Title: "Original", Priority: models.PriorityHigh}
	project := &models.Project{Name: "test", Tasks: []*models.Task{task}}

	result, err := parseQuickEdit("Updated title !0", task, project)
	if err != nil {
		t.Fatalf("parseQuickEdit: %v", err)
	}
	if !result.prioritySet || result.priority != models.PriorityUnset {
		t.Fatalf("priority = %d set=%v, want Unset", result.priority, result.prioritySet)
	}
	req := taskLineEditRequest(TaskIdentity{ProjectID: "project", TaskID: task.ID}, result)
	if req.Priority == nil || *req.Priority != models.PriorityUnset {
		t.Fatalf("request priority = %v, want Unset", req.Priority)
	}
}

func TestParseQuickEdit_Tags(t *testing.T) {
	project := &models.Project{Name: "test"}
	task := &models.Task{Title: "Original", Tags: []string{"old"}}

	result, err := parseQuickEdit("Task #newtag -#old", task, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.tagsToAdd) != 1 || result.tagsToAdd[0] != "newtag" {
		t.Errorf("expected tagsToAdd=['newtag'], got %v", result.tagsToAdd)
	}
	if len(result.tagsToRemove) != 1 || result.tagsToRemove[0] != "old" {
		t.Errorf("tagsToRemove = %v, want [old]", result.tagsToRemove)
	}
	if result.title != "Task" {
		t.Errorf("title = %q, want Task", result.title)
	}
}

func TestParseQuickEdit_DueDate(t *testing.T) {
	project := &models.Project{Name: "test"}
	task := &models.Task{Title: "Original"}

	result, err := parseQuickEdit("Task @tomorrow", task, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.dueDateSet || result.dueDate == nil {
		t.Error("expected due date to be set")
	}
}

func TestParseQuickEdit_ClearDueDate(t *testing.T) {
	project := &models.Project{Name: "test"}
	now := time.Now()
	task := &models.Task{Title: "Original", DueDate: &now}

	result, err := parseQuickEdit("Task @0", task, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.dueDateSet || !result.dueDateClear {
		t.Error("expected due date clear to be set")
	}
}

func TestParseQuickEdit_State(t *testing.T) {
	project := &models.Project{Name: "test"}
	task := &models.Task{Title: "Original", WorkflowState: models.StateBacklog}

	result, err := parseQuickEdit("Task $doing", task, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.stateSet || result.state != models.StateDoing {
		t.Errorf("expected state doing, got %s (set=%v)", result.state, result.stateSet)
	}
}

func TestParseQuickEdit_RetiredSectionTokenRemainsTitleText(t *testing.T) {
	project := &models.Project{
		Name: "test",
	}
	task := &models.Task{Title: "Original"}

	result, err := parseQuickEdit("Task /inbox", task, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.title != "Task /inbox" {
		t.Errorf("retired section token parsed as metadata: title=%q", result.title)
	}
}

func TestParseQuickEdit_EmptyTitle(t *testing.T) {
	project := &models.Project{Name: "test"}
	task := &models.Task{Title: "Original"}

	_, err := parseQuickEdit("!1 #tag", task, project)
	if err == nil {
		t.Error("expected error for empty title")
	}
}

func TestParseQuickEdit_Combined(t *testing.T) {
	project := &models.Project{
		Name: "test",
	}
	task := &models.Task{Title: "Original", Priority: 3}

	result, err := parseQuickEdit("New title !2 #urgent @tomorrow $doing", task, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.title != "New title" {
		t.Errorf("expected title 'New title', got '%s'", result.title)
	}
	if result.priority != 2 {
		t.Errorf("expected priority 2, got %d", result.priority)
	}
	if len(result.tagsToAdd) != 1 || result.tagsToAdd[0] != "urgent" {
		t.Errorf("expected tag 'urgent', got %v", result.tagsToAdd)
	}
	if result.dueDate == nil {
		t.Error("expected due date to be set")
	}
	if result.state != models.StateDoing {
		t.Errorf("expected state doing, got %s", result.state)
	}
}

func TestParseQuickEdit_HyphenatedTags(t *testing.T) {
	project := &models.Project{Name: "test"}
	task := &models.Task{Title: "Original", Tags: []string{"old-tag"}}

	result, err := parseQuickEdit("Task #new-tag -#old-tag", task, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.tagsToAdd) != 1 || result.tagsToAdd[0] != "new-tag" {
		t.Errorf("expected tagsToAdd=['new-tag'], got %v", result.tagsToAdd)
	}
	if len(result.tagsToRemove) != 1 || result.tagsToRemove[0] != "old-tag" {
		t.Errorf("tagsToRemove = %v, want [old-tag]", result.tagsToRemove)
	}
	if result.title != "Task" {
		t.Errorf("expected Task title, got %q", result.title)
	}
}

func TestParseQuickEdit_DottedTags(t *testing.T) {
	project := &models.Project{Name: "test"}
	task := &models.Task{Title: "Original"}

	result, err := parseQuickEdit("Task #v2.0", task, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.tagsToAdd) != 1 || result.tagsToAdd[0] != "v2-0" {
		t.Errorf("expected canonical tagsToAdd=['v2-0'], got %v", result.tagsToAdd)
	}
	if result.title != "Task" {
		t.Errorf("expected title 'Task', got '%s'", result.title)
	}
}

func TestParseQuickEditSingletonAndConflictErrors(t *testing.T) {
	task := &models.Task{ID: "task", ShortID: 1, Title: "Original", WorkflowState: models.StateBacklog}
	project := &models.Project{ID: "project", Slug: "project", Tasks: []*models.Task{task}}
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "priority", input: "Task !1 !2", want: "duplicate priority"},
		{name: "due", input: "Task @today @tomorrow", want: "duplicate due"},
		{name: "destination", input: "Task $today $next", want: "duplicate destination"},
		{name: "project", input: "Task %project %other", want: "duplicate project"},
		{name: "parent", input: "Task >0 >1", want: "duplicate parent"},
		{name: "tag add remove", input: "Task #same -#same", want: "conflicting tag patch"},
		{name: "unknown priority", input: "Task !9", want: "invalid priority: !9"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseQuickEdit(tt.input, task, project, []*models.Project{project})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("parse error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestParseQuickEditStructuralResolutionIsOrderIndependent(t *testing.T) {
	sourceParent := &models.Task{ID: "source-parent", ShortID: 8, Title: "Source parent"}
	task := &models.Task{ID: "task", ShortID: 9, Title: "Original", ParentID: &sourceParent.ID, WorkflowState: models.StateBacklog}
	source := &models.Project{ID: "source", Slug: "source", Tasks: []*models.Task{sourceParent, task}}
	destinationParent := &models.Task{ID: "destination-parent", ShortID: 8, Title: "Destination parent"}
	destination := &models.Project{ID: "destination", Slug: "destination", Tasks: []*models.Task{destinationParent}}
	projects := []*models.Project{source, destination}

	for _, input := range []string{
		"Moved >8 %destination",
		"Moved %destination >8",
	} {
		result, err := parseQuickEdit(input, task, source, projects)
		if err != nil {
			t.Fatalf("parseQuickEdit(%q): %v", input, err)
		}
		if result.targetProject != destination || result.parent != destinationParent || result.parentMode != quickEditParentSet {
			t.Fatalf("parseQuickEdit(%q) structural patch = project %v parent %v mode %v", input, result.targetProject, result.parent, result.parentMode)
		}
	}

	result, err := parseQuickEdit("Moved %destination", task, source, projects)
	if err != nil {
		t.Fatalf("project-root parse: %v", err)
	}
	if result.parentMode != quickEditParentRoot || result.preview.ParentID != nil {
		t.Fatalf("project-only parent patch = mode %v preview parent %v, want root", result.parentMode, result.preview.ParentID)
	}
}

func TestParseQuickEditProjectChangeInvalidatesEarlierParent(t *testing.T) {
	parent := &models.Task{ID: "source-parent", ShortID: 8, Title: "Source parent"}
	task := &models.Task{ID: "task", ShortID: 9, Title: "Original", WorkflowState: models.StateBacklog}
	source := &models.Project{ID: "source", Slug: "source", Tasks: []*models.Task{parent, task}}
	destination := &models.Project{ID: "destination", Slug: "destination", Tasks: []*models.Task{}}

	_, err := parseQuickEdit("Moved >8 %destination", task, source, []*models.Project{source, destination})
	if err == nil || !strings.Contains(err.Error(), "parent not found") || !strings.Contains(err.Error(), "destination") {
		t.Fatalf("error = %v, want effective-project parent refusal", err)
	}
}

func TestParseQuickEditParentValidation(t *testing.T) {
	task := &models.Task{ID: "task", ShortID: 1, Title: "Root", WorkflowState: models.StateBacklog}
	child := &models.Task{ID: "child", ShortID: 2, Title: "Child", ParentID: &task.ID, WorkflowState: models.StateBacklog}
	ambiguousA := &models.Task{ID: "ambiguous-a", ShortID: 3, Title: "Shared alpha", WorkflowState: models.StateBacklog}
	ambiguousB := &models.Task{ID: "ambiguous-b", ShortID: 4, Title: "Shared beta", WorkflowState: models.StateBacklog}
	project := &models.Project{ID: "project", Slug: "project", Tasks: []*models.Task{task, child, ambiguousA, ambiguousB}}

	tests := []struct {
		input string
		want  string
	}{
		{input: "Task >1", want: "own parent"},
		{input: "Task >2", want: "inside the edited task's subtree"},
		{input: "Task >Shared", want: "ambiguous"},
		{input: "Task >999", want: "parent not found"},
	}
	for _, tt := range tests {
		_, err := parseQuickEdit(tt.input, task, project)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("parseQuickEdit(%q) error = %v, want %q", tt.input, err, tt.want)
		}
	}

	stale := &models.Task{ID: "stale", ShortID: 99, Title: "Stale"}
	if reason := invalidQuickEditParent(task, project, project, stale); !strings.Contains(reason, "stale") {
		t.Fatalf("stale parent reason = %q", reason)
	}
}

func TestValidQuickEditParentsFiltersDescendantsTerminalAndDepth(t *testing.T) {
	task := &models.Task{ID: "task", ShortID: 1, Title: "Root", WorkflowState: models.StateBacklog}
	child := &models.Task{ID: "child", ShortID: 2, Title: "Child", ParentID: &task.ID, WorkflowState: models.StateBacklog}
	valid := &models.Task{ID: "valid", ShortID: 3, Title: "Valid", WorkflowState: models.StateBacklog}
	done := &models.Task{ID: "done", ShortID: 4, Title: "Done", WorkflowState: models.StateDone}
	deep := &models.Task{ID: "deep-0", ShortID: 10, Title: "Deep 0", WorkflowState: models.StateBacklog}
	tasks := []*models.Task{task, child, valid, done, deep}
	previous := deep
	for depth := 1; depth <= models.MaxNestingDepth; depth++ {
		node := &models.Task{ID: fmt.Sprintf("deep-%d", depth), ShortID: 10 + depth, Title: fmt.Sprintf("Deep %d", depth), ParentID: &previous.ID, WorkflowState: models.StateBacklog}
		tasks = append(tasks, node)
		previous = node
	}
	project := &models.Project{ID: "project", Slug: "project", Tasks: tasks}
	candidates := validQuickEditParents(task, project, project)
	ids := make([]string, len(candidates))
	for i, candidate := range candidates {
		ids[i] = candidate.ID
	}
	if slices.Contains(ids, task.ID) || slices.Contains(ids, child.ID) || slices.Contains(ids, done.ID) || slices.Contains(ids, previous.ID) {
		t.Fatalf("invalid candidates survived: %v", ids)
	}
	if !slices.Contains(ids, valid.ID) {
		t.Fatalf("valid parent missing: %v", ids)
	}
}

func TestQuickEditParentCandidatesUseEffectiveProject(t *testing.T) {
	task := &models.Task{ID: "task", ShortID: 1, Title: "Task", WorkflowState: models.StateBacklog}
	sourceParent := &models.Task{ID: "source-parent", ShortID: 2, Title: "Source", WorkflowState: models.StateBacklog}
	source := &models.Project{ID: "source", Slug: "source", Tasks: []*models.Task{task, sourceParent}}
	destinationParent := &models.Task{ID: "destination-parent", ShortID: 7, Title: "Destination", WorkflowState: models.StateBacklog}
	destination := &models.Project{ID: "destination", Slug: "destination", Tasks: []*models.Task{destinationParent}}

	candidates, errMsg := quickEditParentCandidates("Task >anything %destination", task, source, []*models.Project{source, destination})
	if errMsg != "" {
		t.Fatalf("completion error = %q", errMsg)
	}
	if len(candidates) != 1 || candidates[0] != destinationParent {
		t.Fatalf("effective-project candidates = %+v", candidates)
	}
}

func TestQuickEditPreviewIsPureAndUsesCanonicalDestinationEffects(t *testing.T) {
	future := time.Now().AddDate(0, 0, 7)
	task := &models.Task{
		ID: "task", ShortID: 1, Title: "Original", Priority: models.PriorityLow,
		Tags: []string{"old"}, WorkflowState: models.StateWaiting, DeferredUntil: &future,
	}
	project := &models.Project{ID: "project", Slug: "project", Tasks: []*models.Task{task}}
	originalUpdated := task.UpdatedAt

	result, err := parseQuickEdit("Preview !1 #new -#old @tomorrow $today >0", task, project)
	if err != nil {
		t.Fatalf("parseQuickEdit: %v", err)
	}
	if task.Title != "Original" || task.Priority != models.PriorityLow || !slices.Equal(task.Tags, []string{"old"}) || task.State() != models.StateWaiting || task.DeferredUntil != &future || task.UpdatedAt != originalUpdated {
		t.Fatalf("source task mutated during preview: %+v", task)
	}
	preview := result.preview
	if preview.Title != "Preview" || preview.Priority != models.PriorityHigh || !slices.Equal(preview.Tags, []string{"new"}) || preview.DueDate == nil || preview.State() != models.StateBacklog || preview.DeferredUntil != nil || preview.ParentID != nil {
		t.Fatalf("preview = %+v", preview)
	}
}

func TestQuickEditPreviewDestinationMatrix(t *testing.T) {
	tests := []struct {
		name      string
		from      models.WorkflowState
		dest      string
		wantState models.WorkflowState
		wantErr   string
	}{
		{name: "today", from: models.StateWaiting, dest: "today", wantState: models.StateBacklog},
		{name: "next", from: models.StateDoing, dest: "next", wantState: models.StateBacklog},
		{name: "backlog", from: models.StateDoing, dest: "backlog", wantState: models.StateBacklog},
		{name: "terminal", from: models.StateWaiting, dest: "done", wantState: models.StateDone},
		{name: "terminal cannot enter planning", from: models.StateDone, dest: "next", wantErr: "terminal tasks cannot be added to Next"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &models.Task{ID: "task", ShortID: 1, Title: "Original", WorkflowState: tt.from}
			project := &models.Project{ID: "project", Slug: "project", Tasks: []*models.Task{task}}
			result, err := parseQuickEdit("Original $"+tt.dest, task, project)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseQuickEdit: %v", err)
			}
			if result.preview.State() != tt.wantState {
				t.Fatalf("preview state = %s, want %s", result.preview.State(), tt.wantState)
			}
			if tt.wantState.IsTerminal() && result.preview.CompletedAt == nil {
				t.Fatal("terminal preview did not reconcile CompletedAt")
			}
		})
	}
}

func TestParseQuickEdit_PausedStateAndRetiredAliases(t *testing.T) {
	project := &models.Project{Name: "test"}
	task := &models.Task{Title: "Original", WorkflowState: models.StateDoing}

	result, err := parseQuickEdit("Task $paused", task, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.stateSet || result.state != models.StatePaused {
		t.Errorf("expected state paused, got %s (set=%v)", result.state, result.stateSet)
	}

	for _, retired := range []string{"$standby", "$hold"} {
		if _, err = parseQuickEdit("Task "+retired, task, project); err == nil {
			t.Errorf("expected retired alias %s to be rejected", retired)
		}
	}
}

func TestTaskLineEditRequestMapsValidatedPatch(t *testing.T) {
	task := &models.Task{ID: "test-task-id"}
	tomorrow := time.Now().AddDate(0, 0, 1)
	result := &quickEditResult{
		title:        "Updated",
		priority:     1,
		prioritySet:  true,
		tagsToAdd:    []string{"new"},
		tagsToRemove: []string{"old"},
		contextValue: "platform",
		contextSet:   true,
		dueDate:      &tomorrow,
		dueDateSet:   true,
		state:        models.StateDoing,
		stateSet:     true,
	}

	req := taskLineEditRequest(TaskIdentity{ProjectID: "project", TaskID: task.ID}, result)
	if req.Title != "Updated" || req.Priority == nil || *req.Priority != 1 {
		t.Fatalf("request content = %+v", req)
	}
	if !slices.Equal(req.TagsToAdd, []string{"new"}) || !slices.Equal(req.TagsToRemove, []string{"old"}) {
		t.Fatalf("request tag patches = +%v -%v", req.TagsToAdd, req.TagsToRemove)
	}
	if !req.DueSet || req.Due == nil || !req.Due.Equal(tomorrow) {
		t.Fatalf("request due patch = set %v due %v", req.DueSet, req.Due)
	}
	if req.Context == nil || *req.Context != "platform" {
		t.Fatalf("request context patch = %v, want platform", req.Context)
	}
}

func TestParseQuickEditInvalidStatusDoesNotMutate(t *testing.T) {
	task := &models.Task{
		ID:            "test-task-id",
		Title:         "Original",
		Priority:      3,
		Tags:          []string{"old"},
		WorkflowState: models.StateDone, // completed task cannot be started
	}
	now := task.UpdatedAt
	project := &models.Project{
		Tasks: []*models.Task{task},
	}

	_, err := parseQuickEdit("Changed Title !1 #new $doing", task, project)
	if err == nil {
		t.Fatal("expected status error for done->in_progress transition")
	}

	// Verify no fields were mutated
	if task.Title != "Original" {
		t.Errorf("title should not change on status error, got %q", task.Title)
	}
	if task.Priority != 3 {
		t.Errorf("priority should not change on status error, got %d", task.Priority)
	}
	if len(task.Tags) != 1 || task.Tags[0] != "old" {
		t.Errorf("tags should not change on status error, got %v", task.Tags)
	}
	if task.UpdatedAt != now {
		t.Error("UpdatedAt should not change on status error")
	}
}

func TestQuickEditComponent_CanonicalStatePreservedInOriginal(t *testing.T) {
	task := &models.Task{
		ID:            "test-id",
		Title:         "Test Task",
		Priority:      3,
		WorkflowState: models.StateWaiting,
	}

	comp := NewQuickEditComponent()
	comp.Enter(task, false)

	if comp.GetOriginal().state != models.StateWaiting {
		t.Errorf("expected original.state=%q, got %q", models.StateWaiting, comp.GetOriginal().state)
	}
}
