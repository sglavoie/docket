package app_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

func TestTaskLineEditRollsBackPlanningPersistenceAndCommitFailures(t *testing.T) {
	t.Run("planning artifact failure", func(t *testing.T) {
		task := newTask("task", 1, "Before")
		project := newProject("project", "Project", task)
		repo := newFakeRepo(project)
		destination := app.CaptureDestinationToday
		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:      app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
			Title:       "After",
			Destination: &destination,
			PlanningArtifacts: func(app.PlanningReconcileRequest) ([]storage.ArtifactChange, error) {
				return nil, errors.New("planning unavailable")
			},
		})
		if err == nil {
			t.Fatal("expected planning failure")
		}
		if task.Title != "Before" || len(repo.commits) != 0 {
			t.Fatalf("planning failure left partial edit: title=%q commits=%d", task.Title, len(repo.commits))
		}
	})

	t.Run("repository commit failure", func(t *testing.T) {
		task := newTask("task", 1, "Before")
		parent := newTask("parent", 2, "Parent")
		project := newProject("project", "Project", task, parent)
		repo := newFakeRepo(project)
		repo.commitErr = errors.New("disk full")
		priority := models.PriorityHigh
		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:   app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
			Title:    "After",
			Priority: &priority,
			Parent: app.TaskLineParentPatch{
				Mode:   app.TaskLineParentSet,
				Parent: app.TaskLocator{ProjectID: project.ID, TaskID: parent.ID},
			},
		})
		if err == nil {
			t.Fatal("expected commit failure")
		}
		if task.Title != "Before" || task.Priority != models.PriorityUnset || task.ParentID != nil {
			t.Fatalf("commit failure left partial task: %+v", task)
		}
		if len(repo.commits) != 0 {
			t.Fatalf("failed repository recorded %d commit(s)", len(repo.commits))
		}
	})
}

func TestTaskLineEditCrossProjectFailuresRestoreBothProjects(t *testing.T) {
	assertRestored := func(t *testing.T, source, destination *models.Project, root, child *models.Task, commits int) {
		t.Helper()
		if source.GetTaskByID(root.ID) != root || source.GetTaskByID(child.ID) != child {
			t.Fatalf("source subtree not restored: %+v", source.Tasks)
		}
		if destination.GetTaskByID(root.ID) != nil || destination.GetTaskByID(child.ID) != nil {
			t.Fatalf("destination retained partial subtree: %+v", destination.Tasks)
		}
		if root.Title != "Before" || root.ShortID != 1 || root.ParentID != nil || child.ShortID != 2 || child.ParentID == nil || *child.ParentID != root.ID {
			t.Fatalf("task identity/content not restored: root=%+v child=%+v", root, child)
		}
		if commits != 0 {
			t.Fatalf("failed transaction recorded %d commit(s)", commits)
		}
	}

	newWorkspace := func() (*models.Task, *models.Task, *models.Project, *models.Project, *fakeRepo) {
		root := newTask("root", 1, "Before")
		child := newTask("child", 2, "Child")
		child.ParentID = &root.ID
		source := newProject("source", "Source", root, child)
		destination := newProject("destination", "Destination")
		repo := newFakeRepo(source, destination)
		return root, child, source, destination, repo
	}

	t.Run("planning state", func(t *testing.T) {
		root, child, source, destination, repo := newWorkspace()
		wantErr := errors.New("planning unavailable")
		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:               app.TaskLocator{ProjectID: source.ID, TaskID: root.ID},
			Title:                "After",
			DestinationProjectID: destination.ID,
			PlanningArtifacts: func(app.PlanningReconcileRequest) ([]storage.ArtifactChange, error) {
				return nil, wantErr
			},
		})
		if !errors.Is(err, wantErr) {
			t.Fatalf("error = %v, want %v", err, wantErr)
		}
		assertRestored(t, source, destination, root, child, len(repo.commits))
	})

	t.Run("repository transaction after internal move", func(t *testing.T) {
		root, child, source, destination, repo := newWorkspace()
		repo.commitErr = errors.New("transaction unavailable")
		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:               app.TaskLocator{ProjectID: source.ID, TaskID: root.ID},
			Title:                "After",
			DestinationProjectID: destination.ID,
		})
		if err == nil {
			t.Fatal("expected transaction failure")
		}
		assertRestored(t, source, destination, root, child, len(repo.commits))
	})

	for _, staleProject := range []string{"source", "destination"} {
		t.Run(staleProject+" document revision", func(t *testing.T) {
			root, child, source, destination, repo := newWorkspace()
			repo.preCommit = func() {
				repo.revisions[staleProject] = "concurrent-revision"
				repo.preCommit = nil
			}
			_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
				Source:               app.TaskLocator{ProjectID: source.ID, TaskID: root.ID},
				Title:                "After",
				DestinationProjectID: destination.ID,
			})
			if !errors.Is(err, storage.ErrStaleRevision) {
				t.Fatalf("error = %v, want ErrStaleRevision", err)
			}
			assertRestored(t, source, destination, root, child, len(repo.commits))
		})
	}

	t.Run("destination depth after subtree insertion", func(t *testing.T) {
		root, child, source, destination, repo := newWorkspace()
		var parent *models.Task
		for level := 0; level <= models.MaxNestingDepth; level++ {
			node := newTask(fmt.Sprintf("parent-%d", level), level+1, fmt.Sprintf("Parent %d", level))
			if parent != nil {
				node.ParentID = &parent.ID
			}
			destination.Tasks = append(destination.Tasks, node)
			parent = node
		}
		destination.BuildTaskTree()
		repo.documents[destination.ID] = documentOf(destination, nil)
		repo.revisions[destination.ID] = repo.documents[destination.ID].Revision

		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:               app.TaskLocator{ProjectID: source.ID, TaskID: root.ID},
			Title:                "After",
			DestinationProjectID: destination.ID,
			Parent: app.TaskLineParentPatch{
				Mode:   app.TaskLineParentSet,
				Parent: app.TaskLocator{ProjectID: destination.ID, TaskID: parent.ID},
			},
		})
		if !errors.Is(err, app.ErrInvalidInput) {
			t.Fatalf("error = %v, want depth ErrInvalidInput", err)
		}
		assertRestored(t, source, destination, root, child, len(repo.commits))
	})
}

func TestTaskLineEditRejectsStaleRevisionAndRestoresSnapshot(t *testing.T) {
	task := newTask("task", 1, "Before")
	project := newProject("project", "Project", task)
	repo := newFakeRepo(project)
	repo.preCommit = func() {
		repo.revisions[project.ID] = "concurrent-revision"
		repo.preCommit = nil
	}

	_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
		Source: app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Title:  "After",
	})
	if !errors.Is(err, storage.ErrStaleRevision) {
		t.Fatalf("error = %v, want ErrStaleRevision", err)
	}
	if task.Title != "Before" || len(repo.commits) != 0 {
		t.Fatalf("stale edit left partial state: title=%q commits=%d", task.Title, len(repo.commits))
	}
}
