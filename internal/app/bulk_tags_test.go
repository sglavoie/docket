package app_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

func TestBulkSetTagsReplacesTagSet(t *testing.T) {
	t1 := newTask("t1", 1, "first")
	t1.Tags = []string{"old", "stale"}
	t2 := newTask("t2", 2, "second")
	projA := newProject("proj-a", "Alpha", t1, t2)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{Operation: app.TaskOperationSetTags, Targets: []app.TaskLocator{repo.loc("t1"), repo.loc("t2")}, Tags: []string{"#backend", "urgent", "backend"}}); err != nil {
		t.Fatalf("BulkSetTags: %v", err)
	}
	// "#backend" normalizes to "backend"; the duplicate "backend" is dropped.
	want := []string{"backend", "urgent"}
	for _, task := range []*models.Task{t1, t2} {
		if len(task.Tags) != len(want) {
			t.Fatalf("task %s tags = %v, want %v", task.ID, task.Tags, want)
		}
		for i, tag := range want {
			if task.Tags[i] != tag {
				t.Errorf("task %s tag[%d] = %q, want %q", task.ID, i, task.Tags[i], tag)
			}
		}
	}
	// Each task holds its own tag slice, not a shared backing array.
	t1.Tags[0] = "mutated"
	if t2.Tags[0] == "mutated" {
		t.Error("tasks share the same tag backing slice")
	}
}

func TestBulkSetTagsEmptyClearsTags(t *testing.T) {
	t1 := newTask("t1", 1, "first")
	t1.Tags = []string{"keep", "me"}
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	if _, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{Operation: app.TaskOperationSetTags, Targets: []app.TaskLocator{repo.loc("t1")}, Tags: nil}); err != nil {
		t.Fatalf("BulkSetTags empty: %v", err)
	}
	if len(t1.Tags) != 0 {
		t.Errorf("t1 tags = %v, want empty", t1.Tags)
	}
}

func TestBulkSetTagsRejectsInvalidTag(t *testing.T) {
	t1 := newTask("t1", 1, "first")
	projA := newProject("proj-a", "Alpha", t1)
	repo := newFakeRepo(projA)
	svc := newTestService(repo)

	_, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{Operation: app.TaskOperationSetTags, Targets: []app.TaskLocator{repo.loc("t1")}, Tags: []string{"ok", "!!!"}})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput (tag has no ASCII alphanumeric)", err)
	}
	if len(repo.commits) != 0 {
		t.Errorf("Commit called %d times, want 0 (invalid tag aborts before commit)", len(repo.commits))
	}
}

func TestBulkSetTagsAllowsIndependentSemanticTags(t *testing.T) {
	task := newTask("t1", 1, "first")
	project := newProject("proj-a", "Alpha", task)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{Operation: app.TaskOperationSetTags, Targets: []app.TaskLocator{repo.loc(task.ID)}, Tags: []string{"bug", "idea"}})
	if err != nil {
		t.Fatalf("BulkSetTags: %v", err)
	}
	if !slices.Equal(task.Tags, []string{"bug", "idea"}) || len(repo.commits) != 1 {
		t.Fatalf("semantic tags=%v commits=%d", task.Tags, len(repo.commits))
	}
}

func TestBulkPatchTagsPreservesSomeMembershipAndCommitsOnce(t *testing.T) {
	t1 := newTask("t1", 1, "first")
	t1.Tags = []string{"common", "first-only"}
	t2 := newTask("t2", 1, "second")
	t2.Tags = []string{"common", "second-only"}
	projA := newProject("proj-a", "Alpha", t1)
	projB := newProject("proj-b", "Beta", t2)
	repo := newFakeRepo(projA, projB)
	svc := newTestService(repo)

	_, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationSetTags,
		Targets: []app.TaskLocator{
			{ProjectID: projA.ID, TaskID: t1.ID},
			{ProjectID: projB.ID, TaskID: t2.ID},
		},
		TagPatch:     true,
		TagsToAdd:    []string{"first-only"},
		TagsToRemove: []string{"common"},
	})
	if err != nil {
		t.Fatalf("patch tags: %v", err)
	}
	if got := t1.Tags; !slices.Equal(got, []string{"first-only"}) {
		t.Fatalf("first tags = %v, want [first-only]", got)
	}
	if got := t2.Tags; !slices.Equal(got, []string{"second-only", "first-only"}) {
		t.Fatalf("second tags = %v, want its partial tag plus first-only", got)
	}
	if len(repo.commits) != 1 || len(repo.commits[0]) != 2 {
		t.Fatalf("tag patch commits = %+v, want one two-project transaction", repo.commits)
	}
}

func TestSetTagsWritesTagsAndContextInOneTransaction(t *testing.T) {
	task := newTask("task", 1, "task")
	task.Tags = []string{"keep"}
	task.Context = "office"
	project := newProject("project", "Project", task)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	result, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation:  app.TaskOperationSetTags,
		Targets:    []app.TaskLocator{{ProjectID: project.ID, TaskID: task.ID}},
		TagPatch:   true,
		TagsToAdd:  []string{"bug"},
		ContextSet: true,
		Context:    "Deep Work",
	})
	if err != nil {
		t.Fatalf("ExecuteTaskOperation: %v", err)
	}
	if !slices.Equal(task.Tags, []string{"keep", "bug"}) {
		t.Fatalf("tags = %v, want [keep bug]", task.Tags)
	}
	if task.Context != "deep-work" {
		t.Fatalf("context = %q, want the normalized deep-work", task.Context)
	}
	if len(repo.commits) != 1 || result.TransactionID == "" {
		t.Fatalf("commits = %+v, transaction = %q, want one transaction",
			repo.commits, result.TransactionID)
	}
}

func TestSetTagsClearsContextAndRejectsAnInvalidOneBeforeMutation(t *testing.T) {
	task := newTask("task", 1, "task")
	task.Tags = []string{"keep"}
	task.Context = "office"
	project := newProject("project", "Project", task)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation:  app.TaskOperationSetTags,
		Targets:    []app.TaskLocator{{ProjectID: project.ID, TaskID: task.ID}},
		TagPatch:   true,
		TagsToAdd:  []string{"bug"},
		ContextSet: true,
		Context:    "!!!",
	})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	if !slices.Equal(task.Tags, []string{"keep"}) || task.Context != "office" || len(repo.commits) != 0 {
		t.Fatalf("refused edit mutated the task: tags=%v context=%q commits=%d",
			task.Tags, task.Context, len(repo.commits))
	}

	if _, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation:  app.TaskOperationSetTags,
		Targets:    []app.TaskLocator{{ProjectID: project.ID, TaskID: task.ID}},
		TagPatch:   true,
		ContextSet: true,
	}); err != nil {
		t.Fatalf("ExecuteTaskOperation: %v", err)
	}
	if task.Context != "" {
		t.Fatalf("context = %q, want the field cleared", task.Context)
	}
}

func TestBulkPatchTagsRejectsConflictingPatchBeforeMutation(t *testing.T) {
	task := newTask("task", 1, "task")
	task.Tags = []string{"keep"}
	project := newProject("project", "Project", task)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation:    app.TaskOperationSetTags,
		Targets:      []app.TaskLocator{{ProjectID: project.ID, TaskID: task.ID}},
		TagPatch:     true,
		TagsToAdd:    []string{"same"},
		TagsToRemove: []string{"same"},
	})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("error = %v, want ErrInvalidInput", err)
	}
	if !slices.Equal(task.Tags, []string{"keep"}) || len(repo.commits) != 0 {
		t.Fatalf("conflicting patch mutated task or committed: tags=%v commits=%d", task.Tags, len(repo.commits))
	}
}

func TestBulkPatchTagsAddsSemanticTagAlongsideExisting(t *testing.T) {
	task := newTask("task", 1, "task")
	task.Tags = []string{"idea", "keep"}
	project := newProject("project", "Project", task)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationSetTags,
		Targets:   []app.TaskLocator{{ProjectID: project.ID, TaskID: task.ID}},
		TagPatch:  true,
		TagsToAdd: []string{"bug"},
	})
	if err != nil {
		t.Fatalf("patch tags: %v", err)
	}
	if !slices.Equal(task.Tags, []string{"idea", "keep", "bug"}) || len(repo.commits) != 1 {
		t.Fatalf("patched tags=%v commits=%d", task.Tags, len(repo.commits))
	}
}

func TestBulkPatchTagsExplicitlyRemovesAndAddsIndependentSemanticTags(t *testing.T) {
	bug := newTask("bug", 1, "bug")
	bug.Tags = []string{"bug", "keep"}
	idea := newTask("idea", 2, "idea")
	idea.Tags = []string{"idea", "keep"}
	project := newProject("project", "Project", bug, idea)
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation: app.TaskOperationSetTags,
		Targets: []app.TaskLocator{
			{ProjectID: project.ID, TaskID: bug.ID},
			{ProjectID: project.ID, TaskID: idea.ID},
		},
		TagPatch:     true,
		TagsToAdd:    []string{"story"},
		TagsToRemove: []string{"bug", "idea"},
	})
	if err != nil {
		t.Fatalf("patch tags: %v", err)
	}
	for _, task := range []*models.Task{bug, idea} {
		if !slices.Equal(task.Tags, []string{"keep", "story"}) {
			t.Fatalf("task %s tags = %v, want [keep story]", task.ID, task.Tags)
		}
	}
}

func TestBulkToggleTagUsesAllOrNoneMembershipAndOneCommit(t *testing.T) {
	first := newTask("t1", 1, "first")
	second := newTask("t2", 2, "second")
	first.AddTag("review")
	project := newProject("proj-a", "Alpha", first, second)
	repo := newFakeRepo(project)
	svc := newTestService(repo)
	locators := []app.TaskLocator{
		{ProjectID: project.ID, TaskID: first.ID},
		{ProjectID: project.ID, TaskID: second.ID},
	}

	if _, err := svc.BulkToggleTag(locators, "Review"); err != nil {
		t.Fatalf("add review: %v", err)
	}
	if !first.HasTag("review") || !second.HasTag("review") || len(repo.commits) != 1 {
		t.Fatalf("first=%v second=%v commits=%d, want both reviewed in one commit", first.Tags, second.Tags, len(repo.commits))
	}
	if _, err := svc.BulkToggleTag(locators, "#review"); err != nil {
		t.Fatalf("remove review: %v", err)
	}
	if first.HasTag("review") || second.HasTag("review") || len(repo.commits) != 2 {
		t.Fatalf("first=%v second=%v commits=%d, want review removed in second commit", first.Tags, second.Tags, len(repo.commits))
	}
}
