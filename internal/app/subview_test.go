package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestAddCommentAppendsAndCommits(t *testing.T) {
	task := newTask("t1", 1, "Commented")
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	res, err := svc.AddComment("t1", "  root cause found  ")
	if err != nil {
		t.Fatalf("AddComment: %v", err)
	}
	if res.Count != 1 || res.Index != 0 {
		t.Fatalf("res = %+v, want count 1 index 0", res)
	}
	if len(task.Comments) != 1 || task.Comments[0].Text != "root cause found" {
		t.Fatalf("comments = %+v, want one trimmed comment", task.Comments)
	}
	cs, ok := repo.lastCommit()
	if !ok || cs.UndoAction != "Add Comment" {
		t.Fatalf("lastCommit = %+v ok=%v, want Add Comment", cs, ok)
	}
	if res.Task.TaskID != "t1" || res.Task.Revision == "" {
		t.Fatalf("task result = %+v, want t1 with revision", res.Task)
	}
}

func TestAddCommentRejectsEmptyText(t *testing.T) {
	repo := newFakeRepo(newProject("p1", "Alpha", newTask("t1", 1, "Task")))
	svc := newTestService(repo)

	_, err := svc.AddComment("t1", "   ")
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 {
		t.Fatal("empty comment must not commit")
	}
}

func TestEditCommentPreservesTimestamp(t *testing.T) {
	task := newTask("t1", 1, "Commented")
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	task.Comments = []*models.Comment{{Text: "old", CreatedAt: created}}
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	res, err := svc.EditComment("t1", 0, "new text")
	if err != nil {
		t.Fatalf("EditComment: %v", err)
	}
	if task.Comments[0].Text != "new text" || !task.Comments[0].CreatedAt.Equal(created) {
		t.Fatalf("comment = %+v, want new text with preserved timestamp", task.Comments[0])
	}
	if res.Index != 0 || res.Count != 1 {
		t.Fatalf("res = %+v, want index 0 count 1", res)
	}

	if _, err := svc.EditComment("t1", 5, "text"); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("out-of-range err = %v, want ErrInvalidInput", err)
	}
}

func TestRemoveAndClearComments(t *testing.T) {
	task := newTask("t1", 1, "Commented")
	task.Comments = []*models.Comment{{Text: "first"}, {Text: "second"}, {Text: "third"}}
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	res, err := svc.RemoveComment("t1", 0)
	if err != nil {
		t.Fatalf("RemoveComment: %v", err)
	}
	if res.Count != 2 || task.Comments[0].Text != "second" {
		t.Fatalf("after remove: res=%+v comments=%+v", res, task.Comments)
	}

	if _, rangeErr := svc.RemoveComment("t1", 9); !errors.Is(rangeErr, app.ErrInvalidInput) {
		t.Fatalf("out-of-range err = %v, want ErrInvalidInput", rangeErr)
	}

	cleared, err := svc.ClearComments("t1")
	if err != nil {
		t.Fatalf("ClearComments: %v", err)
	}
	if cleared.Count != 2 || len(task.Comments) != 0 {
		t.Fatalf("after clear: res=%+v comments=%+v", cleared, task.Comments)
	}
}

func TestToggleCommentResolvedReportsState(t *testing.T) {
	task := newTask("t1", 1, "Commented")
	task.Comments = []*models.Comment{{Text: "first"}}
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	res, err := svc.ToggleCommentResolved("t1", 0)
	if err != nil {
		t.Fatalf("ToggleCommentResolved: %v", err)
	}
	if !res.Resolved || !task.Comments[0].Resolved {
		t.Fatal("first toggle should resolve the comment")
	}

	res, err = svc.ToggleCommentResolved("t1", 0)
	if err != nil {
		t.Fatalf("second toggle: %v", err)
	}
	if res.Resolved || task.Comments[0].Resolved {
		t.Fatal("second toggle should unresolve the comment")
	}
}

func TestBaselineEpicRecordsScope(t *testing.T) {
	epic, err := models.NewTask("e1", 1, models.TaskTypeEpic, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	child := newTask("c1", 2, "Child")
	child.ParentID = &epic.ID
	repo := newFakeRepo(newProject("p1", "Alpha", epic, child))
	svc := newTestService(repo)

	res, err := svc.BaselineEpic("e1")
	if err != nil {
		t.Fatalf("BaselineEpic: %v", err)
	}
	if res.ChildCount != 1 {
		t.Fatalf("ChildCount = %d, want 1", res.ChildCount)
	}
	if epic.EpicAttrs == nil || epic.EpicAttrs.InitialChildCount != 1 || epic.EpicAttrs.ScopeBaselinedAt == nil {
		t.Fatalf("epic attrs = %+v, want baselined", epic.EpicAttrs)
	}
	cs, ok := repo.lastCommit()
	if !ok || cs.UndoAction != "Baseline Epic" {
		t.Fatalf("lastCommit = %+v ok=%v, want Baseline Epic", cs, ok)
	}
}

func TestBaselineEpicRejectsNonEpicAndChildless(t *testing.T) {
	leaf := newTask("t1", 1, "Leaf")
	epic, err := models.NewTask("e1", 2, models.TaskTypeEpic, "Empty Epic")
	if err != nil {
		t.Fatal(err)
	}
	repo := newFakeRepo(newProject("p1", "Alpha", leaf, epic))
	svc := newTestService(repo)

	if _, err := svc.BaselineEpic("t1"); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("non-epic err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.BaselineEpic("e1"); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("childless err = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 {
		t.Fatal("rejected baselines must not commit")
	}
}

func TestSetScoreDimensionsMerges(t *testing.T) {
	task := newTask("t1", 1, "Scored")
	task.ScoreDimensions = map[string]int{"impact": 1, "reach": 2}
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	_, err := svc.SetScoreDimensions("t1", map[string]int{"impact": 4, "confidence": 3})
	if err != nil {
		t.Fatalf("SetScoreDimensions: %v", err)
	}
	want := map[string]int{"impact": 4, "reach": 2, "confidence": 3}
	for k, v := range want {
		if task.ScoreDimensions[k] != v {
			t.Fatalf("ScoreDimensions = %v, want %v", task.ScoreDimensions, want)
		}
	}
	cs, ok := repo.lastCommit()
	if !ok || cs.UndoAction != "Set Score" || cs.UndoContext != "confidence=3 impact=4" {
		t.Fatalf("lastCommit = %+v ok=%v, want Set Score with sorted context", cs, ok)
	}

	if _, err := svc.SetScoreDimensions("t1", nil); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("empty dims err = %v, want ErrInvalidInput", err)
	}
}

func TestPinAndUnpinProjectTag(t *testing.T) {
	project := newProject("p1", "Alpha", newTask("t1", 1, "Task"))
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	res, err := svc.PinProjectTag("p1", "#backend")
	if err != nil {
		t.Fatalf("PinProjectTag: %v", err)
	}
	if !res.Changed || !res.Pinned || res.Tag != "backend" {
		t.Fatalf("pin res = %+v, want changed pinned backend", res)
	}
	if got := project.PinnedTagList(); len(got) != 1 || got[0] != "backend" {
		t.Fatalf("PinnedTagList = %v, want [backend]", got)
	}

	again, err := svc.PinProjectTag("p1", "backend")
	if err != nil {
		t.Fatalf("re-pin: %v", err)
	}
	if again.Changed {
		t.Fatal("re-pinning must report Changed=false")
	}

	unpin, err := svc.UnpinProjectTag("p1", "backend")
	if err != nil {
		t.Fatalf("UnpinProjectTag: %v", err)
	}
	if !unpin.Changed || unpin.Pinned {
		t.Fatalf("unpin res = %+v, want changed unpinned", unpin)
	}
	if got := project.PinnedTagList(); len(got) != 0 {
		t.Fatalf("PinnedTagList = %v, want empty", got)
	}

	if _, err := svc.PinProjectTag("p1", "a,b"); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("invalid tag err = %v, want ErrInvalidInput", err)
	}
}

func TestResetSandboxClearsEverything(t *testing.T) {
	task := newTask("t1", 1, "Doomed")
	project := newProject("p1", "Sandbox", task)
	project.Sandbox = true
	project.Tags = []string{"catalog"}
	project.PinnedTags = []string{"catalog"}
	project.SectionColors = map[string]string{models.SectionNext: "blue"}
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	res, err := svc.ResetSandbox("p1")
	if err != nil {
		t.Fatalf("ResetSandbox: %v", err)
	}
	if res.Cleared.Tasks != 1 {
		t.Fatalf("Cleared.Tasks = %d, want 1", res.Cleared.Tasks)
	}
	cs, ok := repo.lastCommit()
	if !ok || cs.UndoAction != "Reset Sandbox" {
		t.Fatalf("lastCommit = %+v ok=%v, want Reset Sandbox", cs, ok)
	}
	committed := cs.Project
	if len(committed.Tasks) != 0 || committed.NextShortID != 1 {
		t.Fatalf("project not cleared: tasks=%d nextShortID=%d", len(committed.Tasks), committed.NextShortID)
	}
	if len(committed.Tags) != 0 || len(committed.PinnedTags) != 0 || len(committed.SectionColors) != 0 {
		t.Fatal("tags/pinned tags/section colors must be cleared")
	}
	if len(committed.Sections) != 1 || committed.Sections[0] != models.SectionNext {
		t.Fatalf("Sections = %v, want [Next]", committed.Sections)
	}
}

func TestApplyTemplateCreatesRootAndChildrenInOneCommit(t *testing.T) {
	project := newProject("p1", "Alpha")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	art := storage.ArtifactChange{Path: "/tmp/config.json", Data: []byte("{}")}
	res, err := svc.ApplyTemplate(app.ApplyTemplateRequest{
		AddTaskRequest: app.AddTaskRequest{
			ProjectID: "p1",
			Title:     "My Epic",
			Type:      models.TaskTypeEpic,
			Priority:  2,
			Tags:      []string{"backend"},
			Checklist: []string{"Step 1"},
			Artifacts: []storage.ArtifactChange{art},
		},
		TemplateName: "Epic Template",
		Children: []app.TemplateChildSpec{
			{Title: "Child 1", Type: models.TaskTypeTask},
			{Title: "Child 2"},
		},
	})
	if err != nil {
		t.Fatalf("ApplyTemplate: %v", err)
	}
	if res.SubtasksCreated != 2 {
		t.Fatalf("SubtasksCreated = %d, want 2", res.SubtasksCreated)
	}
	if len(project.Tasks) != 3 {
		t.Fatalf("project has %d tasks, want 3", len(project.Tasks))
	}
	root := project.Tasks[0]
	if root.Title != "My Epic" || len(root.Checklist) != 1 {
		t.Fatalf("root = %+v, want epic with checklist", root)
	}
	if root.State() != models.StateInbox {
		t.Fatalf("root state = %q, want canonical Inbox default", root.State())
	}
	for _, child := range project.Tasks[1:] {
		if child.ParentID == nil || *child.ParentID != root.ID {
			t.Fatalf("child %q not parented to root", child.Title)
		}
		if child.Priority != 2 || !child.HasTag("backend") {
			t.Fatalf("child %q did not inherit priority/tags", child.Title)
		}
		if child.State() != root.State() {
			t.Fatalf("child %q state = %q, want inherited %q", child.Title, child.State(), root.State())
		}
	}
	if len(repo.commits) != 1 {
		t.Fatalf("commits = %d, want exactly one transaction", len(repo.commits))
	}
	cs, ok := repo.lastCommit()
	if !ok || cs.UndoAction != "Use Template" || len(cs.Artifacts) != 1 {
		t.Fatalf("lastCommit = %+v ok=%v, want Use Template with artifact", cs, ok)
	}
}

func TestApplyTemplateSkipsChildrenForLeafType(t *testing.T) {
	project := newProject("p1", "Alpha")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	res, err := svc.ApplyTemplate(app.ApplyTemplateRequest{
		AddTaskRequest: app.AddTaskRequest{ProjectID: "p1", Title: "Leaf"},
		TemplateName:   "Leaf Template",
		Children:       []app.TemplateChildSpec{{Title: "Ignored"}},
	})
	if err != nil {
		t.Fatalf("ApplyTemplate: %v", err)
	}
	if res.SubtasksCreated != 0 || len(project.Tasks) != 1 {
		t.Fatalf("res=%+v tasks=%d, want leaf only", res, len(project.Tasks))
	}
}

func TestApplyTemplateRejectsInvalidChildTitleBeforeMutation(t *testing.T) {
	project := newProject("p1", "Alpha")
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	_, err := svc.ApplyTemplate(app.ApplyTemplateRequest{
		AddTaskRequest: app.AddTaskRequest{ProjectID: "p1", Title: "Epic", Type: models.TaskTypeEpic},
		TemplateName:   "Bad Template",
		Children:       []app.TemplateChildSpec{{Title: "   "}},
	})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if len(project.Tasks) != 0 || len(repo.commits) != 0 {
		t.Fatal("rejected template must not mutate or commit")
	}
}
