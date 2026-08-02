package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

const duplicateLocatorTaskID = "duplicate-task-id"

func locatorTestTask(t *testing.T, id string, shortID int, typ string, title string) *models.Task {
	t.Helper()
	task, err := models.NewGenericTask(id, shortID, title)
	if err != nil {
		t.Fatalf("NewTask(%q): %v", title, err)
	}
	task.WorkflowState = models.StateBacklog
	return task
}

func newDuplicateLocatorEnv(t *testing.T, betaExtras ...*models.Task) (*crossProjectTestEnv, *models.Task, *models.Task) {
	t.Helper()
	alpha := locatorTestTask(t, duplicateLocatorTaskID, 1, "", "Alpha duplicate")
	beta := locatorTestTask(t, duplicateLocatorTaskID, 1, "", "Beta duplicate")
	betaTasks := append([]*models.Task{beta}, betaExtras...)
	return newCrossProjectTestEnv(t, []*models.Task{alpha}, betaTasks), alpha, beta
}

func TestTaskItemCoordinatorDuplicateIDsUseSelectedProject(t *testing.T) {
	t.Run("state", func(t *testing.T) {
		env, _, beta := newDuplicateLocatorEnv(t)
		identity := taskIdentityFor(beta, env.projects[1])

		res := runResult(t, env.taskItem.Transition(identity, models.StateDoing))
		if res.ProjectID != env.projects[1].ID {
			t.Fatalf("result project = %q, want beta %q", res.ProjectID, env.projects[1].ID)
		}
		if got := env.reload(t, 0).GetTaskByID(duplicateLocatorTaskID).State(); got != models.StateBacklog {
			t.Fatalf("alpha state = %s, want ready", got)
		}
		if got := env.reload(t, 1).GetTaskByID(duplicateLocatorTaskID).State(); got != models.StateDoing {
			t.Fatalf("beta state = %s, want in_progress", got)
		}
	})

	t.Run("priority and due date", func(t *testing.T) {
		env, _, beta := newDuplicateLocatorEnv(t)
		identity := taskIdentityFor(beta, env.projects[1])
		due := time.Date(2026, time.July, 30, 0, 0, 0, 0, time.UTC)

		if res := runResult(t, env.taskItem.SetPriority(identity, 1)); res.ProjectID != env.projects[1].ID {
			t.Fatalf("priority result project = %q, want beta", res.ProjectID)
		}
		if res := runResult(t, env.taskItem.SetDueDate(identity, &due)); res.ProjectID != env.projects[1].ID {
			t.Fatalf("due-date result project = %q, want beta", res.ProjectID)
		}
		alphaTask := env.reload(t, 0).GetTaskByID(duplicateLocatorTaskID)
		if alphaTask.Priority == 1 || alphaTask.DueDate != nil {
			t.Fatalf("alpha changed: priority=%d due=%v", alphaTask.Priority, alphaTask.DueDate)
		}
		betaTask := env.reload(t, 1).GetTaskByID(duplicateLocatorTaskID)
		if betaTask.Priority != 1 || betaTask.DueDate == nil || betaTask.DueDate.Format("2006-01-02") != due.Format("2006-01-02") {
			t.Fatalf("beta mutation missing: priority=%d due=%v", betaTask.Priority, betaTask.DueDate)
		}
	})

	t.Run("delayed reparent", func(t *testing.T) {
		parent := locatorTestTask(t, "beta-parent", 2, "", "Beta parent")
		env, _, beta := newDuplicateLocatorEnv(t, parent)
		outliner := newCrossOutlinerFromEnv(env)
		newParentID := parent.ID

		_, cmd := outliner.handleCrossProjectReparent(ParentSelectedMsg{
			ParentID:     &newParentID,
			Source:       ViewCrossProjectOutliner,
			ReparentTask: taskIdentityFor(beta, env.projects[1]),
		})
		res := runResult(t, cmd)
		if res.ProjectID != env.projects[1].ID {
			t.Fatalf("reparent result project = %q, want beta", res.ProjectID)
		}
		if got := env.reload(t, 0).GetTaskByID(duplicateLocatorTaskID).ParentID; got != nil {
			t.Fatalf("alpha parent = %v, want nil", got)
		}
		got := env.reload(t, 1).GetTaskByID(duplicateLocatorTaskID).ParentID
		if got == nil || *got != parent.ID {
			t.Fatalf("beta parent = %v, want %q", got, parent.ID)
		}
	})

	t.Run("edit", func(t *testing.T) {
		env, _, beta := newDuplicateLocatorEnv(t)
		title := "Edited beta only"
		res := runResult(t, env.taskItem.Edit(taskIdentityFor(beta, env.projects[1]), app.EditRequest{Title: &title}))
		if res.ProjectID != env.projects[1].ID {
			t.Fatalf("edit result project = %q, want beta", res.ProjectID)
		}
		if got := env.reload(t, 0).GetTaskByID(duplicateLocatorTaskID).Title; got != "Alpha duplicate" {
			t.Fatalf("alpha title = %q, want unchanged", got)
		}
		if got := env.reload(t, 1).GetTaskByID(duplicateLocatorTaskID).Title; got != title {
			t.Fatalf("beta title = %q, want %q", got, title)
		}
	})

	t.Run("attachment and subview", func(t *testing.T) {
		env, _, beta := newDuplicateLocatorEnv(t)
		features := newSecondaryCoordinators(env.svc)
		identity := taskIdentityFor(beta, env.projects[1])

		if res := runResult(t, features.Attachment.AttachURL(identity, "https://example.com/beta")); res.ProjectID != env.projects[1].ID {
			t.Fatalf("attachment result project = %q, want beta", res.ProjectID)
		}
		if res := runResult(t, features.Subview.AddComment(identity, "beta comment")); res.ProjectID != env.projects[1].ID {
			t.Fatalf("comment result project = %q, want beta", res.ProjectID)
		}
		alphaTask := env.reload(t, 0).GetTaskByID(duplicateLocatorTaskID)
		if len(alphaTask.Attachments) != 0 || len(alphaTask.Comments) != 0 {
			t.Fatalf("alpha changed: attachments=%d comments=%d", len(alphaTask.Attachments), len(alphaTask.Comments))
		}
		betaTask := env.reload(t, 1).GetTaskByID(duplicateLocatorTaskID)
		if len(betaTask.Attachments) != 1 || len(betaTask.Comments) != 1 {
			t.Fatalf("beta changes missing: attachments=%d comments=%d", len(betaTask.Attachments), len(betaTask.Comments))
		}
	})

}
