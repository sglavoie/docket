package app_test

import (
	"slices"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// taggedTask builds a leaf task carrying an exact stored tag list, bypassing
// AddTag so the fixture's timestamps stay the ones newTask stamped.
func taggedTask(id string, shortID int, title string, tags ...string) *models.Task {
	task := newTask(id, shortID, title)
	task.Tags = tags
	return task
}

func contextTask(id string, shortID int, title, context string) *models.Task {
	task := newTask(id, shortID, title)
	task.Context = context
	return task
}

// seedRenameCatalog gives the fake repo a catalog artifact so its census reports
// a real revision, which is what the commit-boundary guard is measured against.
func seedRenameCatalog(repo *fakeRepo, content string) {
	repo.files[fakeCatalogPath] = []byte(content)
}

func projectTaskByID(t *testing.T, repo *fakeRepo, projectID, taskID string) *models.Task {
	t.Helper()
	for _, project := range repo.projects {
		if project.ID != projectID {
			continue
		}
		for _, task := range project.Tasks {
			if task.ID == taskID {
				return task
			}
		}
	}
	t.Fatalf("repo has no task %s in project %s", taskID, projectID)
	return nil
}

func renameLocators(preview app.RenameTaskNamePreview) []string {
	names := make([]string, 0, len(preview.Tasks))
	for _, locator := range preview.Tasks {
		names = append(names, locator.ProjectID+"/"+locator.TaskID)
	}
	return names
}

// TestPreviewTaskNameRenameFreezesAffectedIdentitiesAndCounts covers the preview
// contract: exactly one census, the affected identities in census order, the
// owning projects deduplicated, and not a single write.
func TestPreviewTaskNameRenameFreezesAffectedIdentitiesAndCounts(t *testing.T) {
	alpha := newProject("alpha", "Alpha",
		taggedTask("a1", 1, "tagged", "work"),
		taggedTask("a2", 2, "untagged", "ops"),
		taggedTask("a3", 3, "tagged twice", "work", "ops"),
	)
	beta := newProject("beta", "Beta", taggedTask("b1", 1, "tagged", "work"))
	gamma := newProject("gamma", "Gamma", taggedTask("g1", 1, "unrelated", "ops"))
	repo := newFakeRepo(alpha, beta, gamma)
	svc := newTestService(repo)

	preview, err := svc.PreviewTaskNameRename(app.RenameTaskNameRequest{
		Namespace: app.TaskTagNamespace,
		From:      "work",
		To:        "effort",
	})
	if err != nil {
		t.Fatalf("PreviewTaskNameRename: %v", err)
	}
	wantTasks := []string{"alpha/a1", "alpha/a3", "beta/b1"}
	if got := renameLocators(preview); !slices.Equal(got, wantTasks) {
		t.Fatalf("preview tasks = %v, want %v", got, wantTasks)
	}
	if want := []string{"alpha", "beta"}; !slices.Equal(preview.ProjectIDs, want) {
		t.Fatalf("preview project IDs = %v, want %v", preview.ProjectIDs, want)
	}
	if preview.Merge {
		t.Fatalf("preview reported a merge, but no task carries the destination name")
	}
	if preview.Namespace != app.TaskTagNamespace || preview.From != "work" || preview.To != "effort" {
		t.Fatalf("preview request = %+v, want the normalized tag rename", preview)
	}
	if repo.censusCalls != 1 {
		t.Fatalf("census calls = %d, want exactly one frozen read set", repo.censusCalls)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("preview committed %d transactions, want none", len(repo.commits))
	}
}

// TestPreviewTaskNameRenameReportsMergeWhenDestinationExists distinguishes a
// respelling from a collapse: the destination already exists somewhere in the
// workspace, in a project the rename does not even touch.
func TestPreviewTaskNameRenameReportsMergeWhenDestinationExists(t *testing.T) {
	alpha := newProject("alpha", "Alpha", taggedTask("a1", 1, "source", "work"))
	beta := newProject("beta", "Beta", taggedTask("b1", 1, "destination", "effort"))
	svc := newTestService(newFakeRepo(alpha, beta))

	preview, err := svc.PreviewTaskNameRename(app.RenameTaskNameRequest{
		Namespace: app.TaskTagNamespace,
		From:      "work",
		To:        "effort",
	})
	if err != nil {
		t.Fatalf("PreviewTaskNameRename: %v", err)
	}
	if !preview.Merge {
		t.Fatalf("preview merge = false, want true when the destination name already exists")
	}
	if want := []string{"alpha"}; !slices.Equal(preview.ProjectIDs, want) {
		t.Fatalf("preview project IDs = %v, want only the project carrying the source", preview.ProjectIDs)
	}
}

// TestRenameTaskNameMergesDeduplicatingAndPreservesUnrelatedMemberships is the
// merge contract on the task itself: the destination appears once, in the
// source's position, and every membership the rename never named survives.
func TestRenameTaskNameMergesDeduplicatingAndPreservesUnrelatedMemberships(t *testing.T) {
	alpha := newProject("alpha", "Alpha",
		taggedTask("a1", 1, "both", "work", "urgent", "effort"),
		taggedTask("a2", 2, "source only", "work", "urgent"),
	)
	repo := newFakeRepo(alpha)
	svc := newTestService(repo)

	result, err := svc.RenameTaskName(app.RenameTaskNameRequest{
		Namespace: app.TaskTagNamespace,
		From:      "work",
		To:        "effort",
	})
	if err != nil {
		t.Fatalf("RenameTaskName: %v", err)
	}
	if len(result.Tasks) != 2 || len(result.TouchedProjectIDs) != 1 || result.TransactionID == "" {
		t.Fatalf("result = %+v, want two tasks in one project under one transaction", result)
	}
	if got := projectTaskByID(t, repo, "alpha", "a1").Tags; !slices.Equal(got, []string{"effort", "urgent"}) {
		t.Fatalf("merged tags = %v, want the destination once with the unrelated tag kept", got)
	}
	if got := projectTaskByID(t, repo, "alpha", "a2").Tags; !slices.Equal(got, []string{"effort", "urgent"}) {
		t.Fatalf("renamed tags = %v, want the destination in the source position", got)
	}
}

// TestRenameTaskNamePreservesEveryTaskTimestamp holds the rename to being a
// vocabulary change: an advancing clock proves the service never stamps the
// renamed tasks, so neither the touch time nor the state clock moves.
func TestRenameTaskNamePreservesEveryTaskTimestamp(t *testing.T) {
	alpha := newProject("alpha", "Alpha",
		taggedTask("a1", 1, "tagged", "work"),
		contextTask("a2", 2, "contextual", "work"),
	)
	repo := newFakeRepo(alpha)
	clock := &advancingClock{t: time.Date(2027, 1, 1, 9, 0, 0, 0, time.UTC), step: time.Hour}
	svc := newTestServiceWithClock(repo, clock)

	before := make(map[string][2]time.Time)
	for _, task := range alpha.Tasks {
		before[task.ID] = [2]time.Time{task.UpdatedAt, task.StateChangedAt}
	}

	if _, err := svc.RenameTaskName(app.RenameTaskNameRequest{
		Namespace: app.TaskTagNamespace,
		From:      "work",
		To:        "effort",
	}); err != nil {
		t.Fatalf("RenameTaskName: %v", err)
	}
	for id, stamps := range before {
		task := projectTaskByID(t, repo, "alpha", id)
		if !task.UpdatedAt.Equal(stamps[0]) {
			t.Fatalf("task %s UpdatedAt = %s, want the untouched %s", id, task.UpdatedAt, stamps[0])
		}
		if !task.StateChangedAt.Equal(stamps[1]) {
			t.Fatalf("task %s StateChangedAt = %s, want the untouched %s", id, task.StateChangedAt, stamps[1])
		}
	}
}

// TestRenameTaskNameKeepsNamespacesIndependent renames one axis and proves the
// identically spelled value on the other axis is neither matched nor written.
func TestRenameTaskNameKeepsNamespacesIndependent(t *testing.T) {
	tagged := taggedTask("a1", 1, "tag only", "work")
	contextual := contextTask("a2", 2, "context only", "work")
	both := taggedTask("a3", 3, "both axes", "work")
	both.Context = "work"
	repo := newFakeRepo(newProject("alpha", "Alpha", tagged, contextual, both))
	svc := newTestService(repo)

	preview, err := svc.PreviewTaskNameRename(app.RenameTaskNameRequest{
		Namespace: app.TaskContextNamespace,
		From:      "work",
		To:        "effort",
	})
	if err != nil {
		t.Fatalf("PreviewTaskNameRename: %v", err)
	}
	if got := renameLocators(preview); !slices.Equal(got, []string{"alpha/a2", "alpha/a3"}) {
		t.Fatalf("context rename selected %v, want only the context carriers", got)
	}
	if _, err := svc.ApplyTaskNameRename(preview); err != nil {
		t.Fatalf("ApplyTaskNameRename: %v", err)
	}
	if got := projectTaskByID(t, repo, "alpha", "a1"); got.Context != "" || !slices.Equal(got.Tags, []string{"work"}) {
		t.Fatalf("tag-only task = %+v, want its tag untouched by a context rename", got)
	}
	if got := projectTaskByID(t, repo, "alpha", "a2").Context; got != "effort" {
		t.Fatalf("context-only task context = %q, want the renamed value", got)
	}
	renamedBoth := projectTaskByID(t, repo, "alpha", "a3")
	if renamedBoth.Context != "effort" || !slices.Equal(renamedBoth.Tags, []string{"work"}) {
		t.Fatalf("dual-axis task = %+v, want the context renamed and the tag kept", renamedBoth)
	}
}

// TestRenameTaskNameRenamesParkedAndTerminalTasks proves the census is not the
// daily read model: a parked project and tasks in terminal states are ordinary
// members of a vocabulary rename.
func TestRenameTaskNameRenamesParkedAndTerminalTasks(t *testing.T) {
	done := taggedTask("a1", 1, "finished", "work")
	done.WorkflowState = models.StateDone
	canceled := taggedTask("a2", 2, "abandoned", "work")
	canceled.WorkflowState = models.StateCanceled
	parked := newProject("parked", "Parked", taggedTask("p1", 1, "shelved", "work"))
	parked.Tags = []string{"parked"}
	repo := newFakeRepo(newProject("alpha", "Alpha", done, canceled), parked)
	svc := newTestService(repo)

	result, err := svc.RenameTaskName(app.RenameTaskNameRequest{
		Namespace: app.TaskTagNamespace,
		From:      "work",
		To:        "effort",
	})
	if err != nil {
		t.Fatalf("RenameTaskName: %v", err)
	}
	if len(result.Tasks) != 3 {
		t.Fatalf("renamed %d tasks, want the terminal and parked ones included", len(result.Tasks))
	}
	for _, locator := range []app.TaskLocator{{ProjectID: "alpha", TaskID: "a1"}, {ProjectID: "alpha", TaskID: "a2"}, {ProjectID: "parked", TaskID: "p1"}} {
		if got := projectTaskByID(t, repo, locator.ProjectID, locator.TaskID).Tags; !slices.Equal(got, []string{"effort"}) {
			t.Fatalf("%s/%s tags = %v, want the renamed tag", locator.ProjectID, locator.TaskID, got)
		}
	}
}
