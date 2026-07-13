package app_test

import (
	"errors"
	"testing"

	"github.com/sglavoie/tt/internal/app"
)

func TestAddAndRemoveProjectTag(t *testing.T) {
	repo := newFakeRepo(newProject("p1", "Alpha", newTask("t1", 1, "Task")))
	svc := newTestService(repo)

	res, err := svc.AddProjectTag("p1", " #backend ")
	if err != nil {
		t.Fatalf("AddProjectTag: %v", err)
	}
	if res.Tag != "backend" || !res.Changed || res.Revision == "" {
		t.Fatalf("res = %+v, want normalized changed commit", res)
	}
	if cs, ok := repo.lastCommit(); !ok || cs.UndoAction != "Add Project Tag" {
		t.Fatalf("lastCommit = %+v ok=%v, want Add Project Tag", cs, ok)
	}

	if _, dupErr := svc.AddProjectTag("p1", "backend"); !errors.Is(dupErr, app.ErrInvalidInput) {
		t.Fatalf("duplicate add err = %v, want ErrInvalidInput", dupErr)
	}

	removed, err := svc.RemoveProjectTag("p1", "backend")
	if err != nil {
		t.Fatalf("RemoveProjectTag: %v", err)
	}
	if removed.Tag != "backend" || !removed.Changed {
		t.Fatalf("removed = %+v, want changed backend", removed)
	}
	if _, err := svc.RemoveProjectTag("p1", "backend"); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("missing remove err = %v, want ErrInvalidInput", err)
	}
}

func TestRenameProjectTagRewritesTaskUsages(t *testing.T) {
	task := newTask("t1", 1, "Task")
	task.AddTag("backend")
	project := newProject("p1", "Alpha", task)
	if _, err := project.AddTag("backend"); err != nil {
		t.Fatalf("seed AddTag: %v", err)
	}
	repo := newFakeRepo(project)
	svc := newTestService(repo)

	res, err := svc.RenameProjectTag("p1", "backend", "platform")
	if err != nil {
		t.Fatalf("RenameProjectTag: %v", err)
	}
	if !res.Changed || res.Tag != "platform" || res.OldTag != "backend" {
		t.Fatalf("res = %+v, want changed rename", res)
	}
	if len(task.Tags) != 1 || task.Tags[0] != "platform" {
		t.Fatalf("task tags = %v, want [platform]", task.Tags)
	}
	if cs, ok := repo.lastCommit(); !ok || cs.UndoAction != "Rename Tag" {
		t.Fatalf("lastCommit = %+v ok=%v, want Rename Tag", cs, ok)
	}

	// Renaming an unknown tag changes nothing and must not commit.
	commits := len(repo.commits)
	noop, err := svc.RenameProjectTag("p1", "ghost", "spirit")
	if err != nil {
		t.Fatalf("noop rename: %v", err)
	}
	if noop.Changed || len(repo.commits) != commits {
		t.Fatalf("noop = %+v commits %d->%d, want unchanged and uncommitted", noop, commits, len(repo.commits))
	}

	if _, err := svc.RenameProjectTag("p1", "platform", "   "); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("empty new name err = %v, want ErrInvalidInput", err)
	}
}

func TestRenameWorkspaceTagAcrossProjects(t *testing.T) {
	// Alpha: catalog tag + one task usage.
	aTask := newTask("t1", 1, "Alpha Task")
	aTask.AddTag("backend")
	alpha := newProject("p1", "Alpha", aTask)
	if _, err := alpha.AddTag("backend"); err != nil {
		t.Fatalf("seed alpha AddTag: %v", err)
	}
	// Beta: two task usages + a pinned tag, no catalog entry.
	bTask1 := newTask("t2", 1, "Beta Task 1")
	bTask1.AddTag("backend")
	bTask2 := newTask("t3", 2, "Beta Task 2")
	bTask2.AddTag("backend")
	beta := newProject("p2", "Beta", bTask1, bTask2)
	if _, err := beta.PinTag("backend"); err != nil {
		t.Fatalf("seed beta PinTag: %v", err)
	}
	// Gamma: unrelated, must be left untouched (not committed).
	gTask := newTask("t4", 1, "Gamma Task")
	gTask.AddTag("frontend")
	gamma := newProject("p3", "Gamma", gTask)

	repo := newFakeRepo(alpha, beta, gamma)
	svc := newTestService(repo)

	res, err := svc.RenameWorkspaceTag(" #backend ", "platform")
	if err != nil {
		t.Fatalf("RenameWorkspaceTag: %v", err)
	}
	if res.OldTag != "backend" || res.NewTag != "platform" {
		t.Fatalf("res tags = %q -> %q, want backend -> platform", res.OldTag, res.NewTag)
	}
	if res.ProjectCount != 2 {
		t.Fatalf("ProjectCount = %d, want 2 (alpha, beta)", res.ProjectCount)
	}
	if res.TaskCount != 3 {
		t.Fatalf("TaskCount = %d, want 3 (1 alpha + 2 beta)", res.TaskCount)
	}
	if len(res.TouchedProjectIDs) != 2 {
		t.Fatalf("TouchedProjectIDs = %v, want two projects", res.TouchedProjectIDs)
	}

	// Task usages and pins rewritten in place.
	if len(aTask.Tags) != 1 || aTask.Tags[0] != "platform" {
		t.Fatalf("alpha task tags = %v, want [platform]", aTask.Tags)
	}
	if len(bTask2.Tags) != 1 || bTask2.Tags[0] != "platform" {
		t.Fatalf("beta task tags = %v, want [platform]", bTask2.Tags)
	}
	if !beta.IsTagPinned("platform") || beta.IsTagPinned("backend") {
		t.Fatalf("beta pins = %v, want platform pinned and backend gone", beta.PinnedTags)
	}
	if len(gTask.Tags) != 1 || gTask.Tags[0] != "frontend" {
		t.Fatalf("gamma task tags = %v, want untouched [frontend]", gTask.Tags)
	}

	// One atomic grouped transaction over exactly the two touched projects.
	if len(repo.commits) != 1 || len(repo.commits[0]) != 2 {
		t.Fatalf("commits = %d calls (last %d changesets), want one atomic call of two", len(repo.commits), len(repo.commits[len(repo.commits)-1]))
	}
	for _, cs := range repo.commits[0] {
		if cs.UndoAction != "Rename Global Tag" || cs.UndoContext != "backend -> platform" {
			t.Fatalf("changeset undo = %q/%q, want Rename Global Tag / backend -> platform", cs.UndoAction, cs.UndoContext)
		}
	}
}

func TestRenameWorkspaceTagNoUsagesDoesNotCommit(t *testing.T) {
	repo := newFakeRepo(newProject("p1", "Alpha", newTask("t1", 1, "Task")))
	svc := newTestService(repo)

	res, err := svc.RenameWorkspaceTag("ghost", "spirit")
	if err != nil {
		t.Fatalf("RenameWorkspaceTag: %v", err)
	}
	if res.ProjectCount != 0 || res.TaskCount != 0 || len(res.TouchedProjectIDs) != 0 {
		t.Fatalf("res = %+v, want zero touched", res)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("commits = %d, want none for a no-op rename", len(repo.commits))
	}

	if _, err := svc.RenameWorkspaceTag("backend", "  "); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("empty new name err = %v, want ErrInvalidInput", err)
	}
}

func TestTogglePinnedProjectTag(t *testing.T) {
	task := newTask("t1", 1, "Task")
	task.AddTag("backend")
	repo := newFakeRepo(newProject("p1", "Alpha", task))
	svc := newTestService(repo)

	res, err := svc.TogglePinnedProjectTag("p1", "backend")
	if err != nil {
		t.Fatalf("TogglePinnedProjectTag: %v", err)
	}
	if !res.Pinned {
		t.Fatalf("res = %+v, want pinned", res)
	}
	if cs, ok := repo.lastCommit(); !ok || cs.UndoAction != "Pin Tag" {
		t.Fatalf("lastCommit = %+v ok=%v, want Pin Tag", cs, ok)
	}

	res, err = svc.TogglePinnedProjectTag("p1", "backend")
	if err != nil {
		t.Fatalf("TogglePinnedProjectTag unpin: %v", err)
	}
	if res.Pinned {
		t.Fatalf("res = %+v, want unpinned", res)
	}
	if cs, ok := repo.lastCommit(); !ok || cs.UndoAction != "Unpin Tag" {
		t.Fatalf("lastCommit = %+v ok=%v, want Unpin Tag", cs, ok)
	}
}

func TestAddRelatedBidirectionalCrossProject(t *testing.T) {
	src := newTask("t1", 1, "Source")
	tgt := newTask("t2", 1, "Target")
	repo := newFakeRepo(newProject("p1", "Alpha", src), newProject("p2", "Beta", tgt))
	svc := newTestService(repo)

	res, err := svc.AddRelatedBidirectional("t1", "p2:t2")
	if err != nil {
		t.Fatalf("AddRelatedBidirectional: %v", err)
	}
	if len(res.TouchedProjectIDs) != 2 {
		t.Fatalf("touched = %v, want both projects", res.TouchedProjectIDs)
	}
	if len(src.RelatedTo) != 1 || src.RelatedTo[0] != "p2:t2" {
		t.Fatalf("source related = %v, want [p2:t2]", src.RelatedTo)
	}
	if len(tgt.RelatedTo) != 1 || tgt.RelatedTo[0] != "p1:t1" {
		t.Fatalf("target related = %v, want qualified back-reference [p1:t1]", tgt.RelatedTo)
	}
	if len(repo.commits) != 1 || len(repo.commits[0]) != 2 {
		t.Fatalf("commits = %d changesets in %d calls, want one atomic call of two", len(repo.commits[0]), len(repo.commits))
	}
}

func TestAddRelatedBidirectionalSameProjectUsesBareIDs(t *testing.T) {
	src := newTask("t1", 1, "Source")
	tgt := newTask("t2", 2, "Target")
	repo := newFakeRepo(newProject("p1", "Alpha", src, tgt))
	svc := newTestService(repo)

	res, err := svc.AddRelatedBidirectional("t1", "t2")
	if err != nil {
		t.Fatalf("AddRelatedBidirectional: %v", err)
	}
	if len(res.TouchedProjectIDs) != 1 {
		t.Fatalf("touched = %v, want one project", res.TouchedProjectIDs)
	}
	if len(tgt.RelatedTo) != 1 || tgt.RelatedTo[0] != "t1" {
		t.Fatalf("target related = %v, want bare back-reference [t1]", tgt.RelatedTo)
	}

	if _, err := svc.AddRelatedBidirectional("t1", "t1"); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("self link err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.AddRelatedBidirectional("t1", "ghost"); !errors.Is(err, app.ErrTaskNotFound) {
		t.Fatalf("missing target err = %v, want ErrTaskNotFound", err)
	}
}
