package app_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

func taskLineDestination(t *testing.T, raw string) *app.CaptureDestination {
	t.Helper()
	destination, ok := app.ParseCaptureDestination(raw)
	if !ok {
		t.Fatalf("ParseCaptureDestination(%q) failed", raw)
	}
	return &destination
}

func TestTaskLineEditAdoptsChildrenAdditively(t *testing.T) {
	task := newTask("task", 1, "Edited")
	kept := newTask("kept", 2, "Kept child")
	kept.ParentID = &task.ID
	adopted := newTask("adopted", 3, "Adopted")
	grandchild := newTask("grandchild", 4, "Grandchild")
	grandchild.ParentID = &adopted.ID
	former := newTask("former", 5, "Former parent")
	reparented := newTask("reparented", 6, "Reparented child")
	reparented.ParentID = &former.ID
	project := newProject("project", "Project", task, kept, adopted, grandchild, former, reparented)
	repo := newFakeRepo(project)

	result, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
		Source: app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Title:  "Edited",
		Children: []app.TaskLocator{
			{ProjectID: project.ID, TaskID: adopted.ID},
			{ProjectID: project.ID, TaskID: reparented.ID},
			{ProjectID: project.ID, TaskID: adopted.ID},
		},
	})
	if err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}
	for _, child := range []*models.Task{kept, adopted, reparented} {
		if child.ParentID == nil || *child.ParentID != task.ID {
			t.Fatalf("child %q parent = %v, want the edited task", child.ID, child.ParentID)
		}
	}
	if grandchild.ParentID == nil || *grandchild.ParentID != adopted.ID {
		t.Fatalf("adopted subtree changed: %v", grandchild.ParentID)
	}
	if len(former.Children) != 0 {
		t.Fatalf("former parent kept %d children", len(former.Children))
	}
	if !result.Changed || result.TransactionID == "" || result.MovedDescendantCount != 0 {
		t.Fatalf("result = %+v, want a committed edit that relocated no descendants", result)
	}
	if !slices.Equal(result.AffectedProjectIDs, []string{project.ID}) || len(repo.commits) != 1 || len(repo.commits[0]) != 1 {
		t.Fatalf("affected projects/commits = %v/%+v", result.AffectedProjectIDs, repo.commits)
	}
}

func TestTaskLineEditAlreadyDirectChildDoesNotTransact(t *testing.T) {
	task := newTask("task", 1, "Same")
	child := newTask("child", 2, "Child")
	child.ParentID = &task.ID
	project := newProject("project", "Project", task, child)
	repo := newFakeRepo(project)

	result, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
		Source:   app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Title:    "Same",
		Children: []app.TaskLocator{{ProjectID: project.ID, TaskID: child.ID}},
	})
	if err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}
	if result.Changed || result.TransactionID != "" || len(result.AffectedProjectIDs) != 0 {
		t.Fatalf("no-op result = %+v", result)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("no-op committed %d transaction(s)", len(repo.commits))
	}
}

func TestTaskLineEditAppliesParentAndChildEdgesAsOneBatch(t *testing.T) {
	parent := newTask("parent", 1, "Parent")
	task := newTask("task", 2, "Inverted")
	task.ParentID = &parent.ID
	child := newTask("child", 3, "Child")
	child.ParentID = &task.ID
	project := newProject("project", "Project", parent, task, child)
	repo := newFakeRepo(project)

	result, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
		Source:   app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		Title:    "Inverted",
		Parent:   app.TaskLineParentPatch{Mode: app.TaskLineParentRoot},
		Children: []app.TaskLocator{{ProjectID: project.ID, TaskID: parent.ID}},
	})
	if err != nil {
		t.Fatalf("EditTaskLine: %v", err)
	}
	if task.ParentID != nil {
		t.Fatalf("edited task parent = %v, want root", task.ParentID)
	}
	if parent.ParentID == nil || *parent.ParentID != task.ID {
		t.Fatalf("inverted parent = %v, want the edited task", parent.ParentID)
	}
	if child.ParentID == nil || *child.ParentID != task.ID {
		t.Fatalf("existing child = %v, want retained", child.ParentID)
	}
	if result.MovedDescendantCount != 1 {
		t.Fatalf("MovedDescendantCount = %d, want the edited subtree only", result.MovedDescendantCount)
	}
}

func TestTaskLineEditRejectsChildAssignmentsWithoutMutation(t *testing.T) {
	t.Run("adopting the retained parent", func(t *testing.T) {
		parent := newTask("parent", 1, "Parent")
		task := newTask("task", 2, "Edited")
		task.ParentID = &parent.ID
		project := newProject("project", "Project", parent, task)
		repo := newFakeRepo(project)

		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:   app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
			Title:    "Edited",
			Children: []app.TaskLocator{{ProjectID: project.ID, TaskID: parent.ID}},
		})
		if !errors.Is(err, app.ErrInvalidInput) {
			t.Fatalf("error = %v, want cycle ErrInvalidInput", err)
		}
		if parent.ParentID != nil || task.ParentID == nil || *task.ParentID != parent.ID || len(repo.commits) != 0 {
			t.Fatalf("rejected cycle mutated hierarchy: parent=%v task=%v commits=%d", parent.ParentID, task.ParentID, len(repo.commits))
		}
	})

	t.Run("adopting itself", func(t *testing.T) {
		task := newTask("task", 1, "Edited")
		project := newProject("project", "Project", task)
		repo := newFakeRepo(project)

		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:   app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
			Title:    "Edited",
			Children: []app.TaskLocator{{ProjectID: project.ID, TaskID: task.ID}},
		})
		if !errors.Is(err, app.ErrInvalidInput) {
			t.Fatalf("error = %v, want self-parent ErrInvalidInput", err)
		}
		if task.ParentID != nil || len(repo.commits) != 0 {
			t.Fatalf("rejected self-adoption mutated state: %v/%d", task.ParentID, len(repo.commits))
		}
	})

	t.Run("adoption past the depth cap", func(t *testing.T) {
		task := newTask("task", 1, "Edited")
		tasks := []*models.Task{task}
		var head, previous *models.Task
		for level := 0; level <= models.MaxNestingDepth; level++ {
			node := newTask(fmt.Sprintf("chain-%d", level), level+2, fmt.Sprintf("Chain %d", level))
			if previous == nil {
				head = node
			} else {
				node.ParentID = &previous.ID
			}
			tasks = append(tasks, node)
			previous = node
		}
		project := newProject("project", "Project", tasks...)
		repo := newFakeRepo(project)

		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:   app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
			Title:    "Edited",
			Children: []app.TaskLocator{{ProjectID: project.ID, TaskID: head.ID}},
		})
		if !errors.Is(err, app.ErrInvalidInput) {
			t.Fatalf("error = %v, want depth ErrInvalidInput", err)
		}
		if head.ParentID != nil || len(repo.commits) != 0 {
			t.Fatalf("rejected depth overflow mutated state: %v/%d", head.ParentID, len(repo.commits))
		}
	})
}

func TestTaskLineEditRejectsUnresolvableChildLocators(t *testing.T) {
	t.Run("stale task id", func(t *testing.T) {
		task := newTask("task", 1, "Edited")
		project := newProject("project", "Project", task)
		repo := newFakeRepo(project)

		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:   app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
			Title:    "Edited",
			Children: []app.TaskLocator{{ProjectID: project.ID, TaskID: "deleted"}},
		})
		if !errors.Is(err, app.ErrTaskNotFound) {
			t.Fatalf("error = %v, want ErrTaskNotFound", err)
		}
		if len(repo.commits) != 0 {
			t.Fatalf("stale child committed %d transaction(s)", len(repo.commits))
		}
	})

	t.Run("unqualified locator", func(t *testing.T) {
		task := newTask("task", 1, "Edited")
		project := newProject("project", "Project", task)
		repo := newFakeRepo(project)

		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:   app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
			Title:    "Edited",
			Children: []app.TaskLocator{{ProjectID: project.ID}},
		})
		if !errors.Is(err, app.ErrInvalidInput) {
			t.Fatalf("error = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("child outside the effective project", func(t *testing.T) {
		task := newTask("task", 1, "Edited")
		stranded := newTask("stranded", 2, "Stranded")
		source := newProject("source", "Source", task, stranded)
		destination := newProject("destination", "Destination")
		repo := newFakeRepo(source, destination)

		_, err := newTestService(repo).EditTaskLine(app.TaskLineEditRequest{
			Source:               app.TaskLocator{ProjectID: source.ID, TaskID: task.ID},
			Title:                "Edited",
			DestinationProjectID: destination.ID,
			Children:             []app.TaskLocator{{ProjectID: source.ID, TaskID: stranded.ID}},
		})
		if !errors.Is(err, app.ErrInvalidInput) {
			t.Fatalf("error = %v, want effective-project ErrInvalidInput", err)
		}
		if source.GetTaskByID(task.ID) != task || len(repo.commits) != 0 {
			t.Fatalf("rejected cross-project adoption moved the subtree: %+v/%d", source.Tasks, len(repo.commits))
		}
	})
}
