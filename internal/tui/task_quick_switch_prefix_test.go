package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func TestQuickSwitchPrefixDispatch(t *testing.T) {
	t.Run("status prefix changes task status", func(t *testing.T) {
		app, _ := newTestAppWithPersisterForQuickSwitch()
		task := &models.Task{ID: "t1", Title: "Task 1", ShortID: 1, Status: models.StatusTodo}
		p := &models.Project{ID: "p1", Name: "Project 1", Tasks: []*models.Task{task}}
		app.recentTaskVisits = []RecentTaskVisitEntry{{Task: task, Project: p}}
		initQuickSwitchFilter(app)
		app.showQuickSwitch = true
		app.quickSwitchPendingPrefix = ","

		msg := syntheticKey('s')
		app.handleQuickSwitchKey(msg)

		if app.quickSwitchPendingPrefix != "" {
			t.Error("expected pending prefix to be cleared")
		}
		if task.Status != models.StatusInProgress {
			t.Errorf("expected status in_progress, got %s", task.Status)
		}
		if app.statusMessage == "" {
			t.Error("expected status message to be set")
		}
	})

	t.Run("priority prefix changes task priority", func(t *testing.T) {
		app, _ := newTestAppWithPersisterForQuickSwitch()
		task := &models.Task{ID: "t1", Title: "Task 1", ShortID: 1, Priority: 3}
		p := &models.Project{ID: "p1", Name: "Project 1", Tasks: []*models.Task{task}}
		app.recentTaskVisits = []RecentTaskVisitEntry{{Task: task, Project: p}}
		initQuickSwitchFilter(app)
		app.showQuickSwitch = true
		app.quickSwitchPendingPrefix = "!"

		msg := syntheticKey('1')
		app.handleQuickSwitchKey(msg)

		if app.quickSwitchPendingPrefix != "" {
			t.Error("expected pending prefix to be cleared")
		}
		if task.Priority != 1 {
			t.Errorf("expected priority 1, got %d", task.Priority)
		}
	})

	t.Run("esc cancels pending prefix", func(t *testing.T) {
		app := newTestAppForQuickSwitch()
		p := &models.Project{ID: "p1", Name: "Project 1"}
		app.recentTaskVisits = []RecentTaskVisitEntry{{Task: &models.Task{ID: "t1", Title: "Task 1"}, Project: p}}
		initQuickSwitchFilter(app)
		app.showQuickSwitch = true
		app.quickSwitchPendingPrefix = ","

		msg := syntheticSpecialKey(tea.KeyEscape)
		app.handleQuickSwitchKey(msg)

		if app.quickSwitchPendingPrefix != "" {
			t.Error("expected pending prefix to be cleared on esc")
		}
		if !app.showQuickSwitch {
			t.Error("popup should still be open after canceling prefix")
		}
	})

	t.Run("invalid prefix key is ignored", func(t *testing.T) {
		app, _ := newTestAppWithPersisterForQuickSwitch()
		task := &models.Task{ID: "t1", Title: "Task 1", ShortID: 1, Status: models.StatusTodo, Priority: 3}
		p := &models.Project{ID: "p1", Name: "Project 1", Tasks: []*models.Task{task}}
		app.recentTaskVisits = []RecentTaskVisitEntry{{Task: task, Project: p}}
		initQuickSwitchFilter(app)
		app.showQuickSwitch = true
		app.quickSwitchPendingPrefix = ","

		msg := syntheticKey('z')
		app.handleQuickSwitchKey(msg)

		if app.quickSwitchPendingPrefix != "" {
			t.Error("expected pending prefix to be cleared")
		}
		if task.Status != models.StatusTodo {
			t.Errorf("expected status unchanged (todo), got %s", task.Status)
		}
	})
}