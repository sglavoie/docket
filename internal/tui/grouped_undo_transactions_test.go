package tui

import (
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/workflow"
)

func TestTUITaskMoveRecordsGroupedUndoAndRepairsConfigOnRestore(t *testing.T) {
	source, moved := newGroupedUndoProject(t, "source", "Source", "move-task-1", "Move me")
	dest, _ := newGroupedUndoProject(t, "dest", "Dest", "", "")
	cfg, mgr := newGroupedUndoConfig(t, source, dest)
	cfg.FocusQueue = []config.FocusQueueItem{{TaskID: moved.ID, ProjectID: source.ID}}
	cfg.DailyPlan = []config.DailyPlanItem{{TaskID: moved.ID, ProjectID: source.ID, PlannedDate: "2026-06-18"}}
	cfg.RecentTaskVisits = []config.RecentTaskVisit{{TaskID: moved.ID, ProjectID: source.ID}}
	if err := mgr.Save(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	app := &AppModel{
		config:           cfg,
		configManager:    mgr,
		persister:        NewProjectService(),
		focusQueue:       []FocusTask{{Task: moved, Project: source}},
		focusQueueIDs:    map[string]bool{moved.ID: true},
		dailyPlan:        []FocusTask{{Task: moved, Project: source}},
		dailyPlanIDs:     map[string]bool{moved.ID: true},
		recentTaskVisits: []RecentTaskVisitEntry{{Task: moved, Project: source}},
	}

	handled, cmd := app.handleMoveTaskToProject(MoveTaskToProjectMsg{
		Task:          moved,
		SourceProject: source,
		DestProjectID: dest.ID,
	})
	if !handled {
		t.Fatal("move message was not handled")
	}
	msgs := drainTUITestCmd(t, cmd)
	foundMoveMsg := false
	for _, msg := range msgs {
		switch msg := msg.(type) {
		case SaveErrorMsg:
			t.Fatalf("move save failed: %v", msg.Err)
		case TaskMovedToProjectMsg:
			foundMoveMsg = true
		}
	}
	if !foundMoveMsg {
		t.Fatalf("expected TaskMovedToProjectMsg, got %#v", msgs)
	}

	assertGroupedTopUndo(t, source, dest)
	assertConfigRefsProject(t, cfg, moved.ID, dest.ID)
	if app.focusQueue[0].Project.ID != dest.ID || app.dailyPlan[0].Project.ID != dest.ID || app.recentTaskVisits[0].Project.ID != dest.ID {
		t.Fatalf("in-memory refs should point to dest after move")
	}

	undoMsg := performUndoForProjectsCmd(app.persister, source, cfg.Projects, app.repairTaskProjectRefsAfterRestore)()
	if errMsg, ok := undoMsg.(ErrorMsg); ok {
		t.Fatalf("undo failed: %v", errMsg.Err)
	}
	if _, ok := undoMsg.(UndoPerformedMsg); !ok {
		t.Fatalf("expected UndoPerformedMsg, got %T", undoMsg)
	}
	assertConfigRefsProject(t, cfg, moved.ID, source.ID)
	if app.focusQueue[0].Project.ID != source.ID || app.dailyPlan[0].Project.ID != source.ID || app.recentTaskVisits[0].Project.ID != source.ID {
		t.Fatalf("in-memory refs should point back to source after undo")
	}

	redoMsg := performRedoForProjectsCmd(app.persister, source, cfg.Projects, app.repairTaskProjectRefsAfterRestore)()
	if errMsg, ok := redoMsg.(ErrorMsg); ok {
		t.Fatalf("redo failed: %v", errMsg.Err)
	}
	if _, ok := redoMsg.(RedoPerformedMsg); !ok {
		t.Fatalf("expected RedoPerformedMsg, got %T", redoMsg)
	}
	assertConfigRefsProject(t, cfg, moved.ID, dest.ID)
	if app.focusQueue[0].Project.ID != dest.ID || app.dailyPlan[0].Project.ID != dest.ID || app.recentTaskVisits[0].Project.ID != dest.ID {
		t.Fatalf("in-memory refs should point to dest after redo")
	}
}

func TestProcessTUIDeferredMovesRecordsGroupedUndo(t *testing.T) {
	source, moved := newGroupedUndoProject(t, "source", "Source", "workflow-task-1", "Workflow move")
	dest, _ := newGroupedUndoProject(t, "dest", "Dest", "", "")
	cfg, mgr := newGroupedUndoConfig(t, source, dest)
	cfg.FocusQueue = []config.FocusQueueItem{{TaskID: moved.ID, ProjectID: source.ID}}
	if err := mgr.Save(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	svc := NewProjectService()
	recorded, err := processTUIDeferredMoves(svc, source, []workflow.DeferredMove{
		{TaskID: moved.ID, Destination: dest.ID},
	}, cfg, mgr, tuiDeferredMoveUndoOptions{
		Action:  "Workflow Move to Project",
		Context: "#1 Workflow move",
		Depth:   *cfg.UndoHistoryDepth,
	})
	if err != nil {
		t.Fatalf("process deferred moves: %v", err)
	}
	if !recorded {
		t.Fatal("expected grouped undo snapshot recording")
	}
	if err := svc.Backup(source); err != nil {
		t.Fatalf("backup source: %v", err)
	}
	if err := svc.Save(source); err != nil {
		t.Fatalf("save source: %v", err)
	}

	assertGroupedTopUndo(t, source, dest)
	assertConfigRefsProject(t, cfg, moved.ID, dest.ID)

	undoMsg := performUndoForProjectsCmd(svc, source, cfg.Projects, repairConfigTaskRefsAfterRestore(mgr, cfg))()
	if errMsg, ok := undoMsg.(ErrorMsg); ok {
		t.Fatalf("undo failed: %v", errMsg.Err)
	}
	if _, ok := undoMsg.(UndoPerformedMsg); !ok {
		t.Fatalf("expected UndoPerformedMsg, got %T", undoMsg)
	}
	assertConfigRefsProject(t, cfg, moved.ID, source.ID)
	if source.GetTaskByID(moved.ID) == nil {
		t.Fatal("source task should be restored after undo")
	}
	if dest.GetTaskByID(moved.ID) != nil {
		t.Fatal("destination task should be removed after undo")
	}
}

func newGroupedUndoProject(t *testing.T, id, name, taskID, taskTitle string) (*models.Project, *models.Task) {
	t.Helper()
	project, err := models.NewProject(id, name, t.TempDir())
	if err != nil {
		t.Fatalf("new project: %v", err)
	}
	var task *models.Task
	if taskID != "" {
		task, err = models.NewTask(taskID, 1, models.TaskTypeTask, taskTitle)
		if err != nil {
			t.Fatalf("new task: %v", err)
		}
		project.Tasks = []*models.Task{task}
		project.NextShortID = 2
	}
	project.BuildTaskTree()
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("save project: %v", err)
	}
	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("load project: %v", err)
	}
	if taskID != "" {
		task = project.GetTaskByID(taskID)
		if task == nil {
			t.Fatalf("task %s missing after load", taskID)
		}
	}
	return project, task
}

func newGroupedUndoConfig(t *testing.T, projects ...*models.Project) (*config.Config, *config.Manager) {
	t.Helper()
	cfg := config.NewConfig()
	depth := 5
	cfg.UndoHistoryDepth = &depth
	cfg.Projects = projects
	cfg.CurrentProject = projects[0].ID
	mgr := config.NewManager(filepath.Join(t.TempDir(), "config.json"))
	return cfg, mgr
}

func assertGroupedTopUndo(t *testing.T, first, second *models.Project) {
	t.Helper()
	firstEntries, err := storage.LoadUndoHistory(first)
	if err != nil {
		t.Fatalf("load first undo history: %v", err)
	}
	secondEntries, err := storage.LoadUndoHistory(second)
	if err != nil {
		t.Fatalf("load second undo history: %v", err)
	}
	if len(firstEntries) == 0 || len(secondEntries) == 0 {
		t.Fatalf("expected undo history for both projects, got %d and %d", len(firstEntries), len(secondEntries))
	}
	if firstEntries[0].GroupID == "" || firstEntries[0].GroupID != secondEntries[0].GroupID {
		t.Fatalf("group IDs should match and be non-empty, got %q and %q", firstEntries[0].GroupID, secondEntries[0].GroupID)
	}
	gotIDs := firstEntries[0].GroupProjectIDs
	wantIDs := map[string]bool{first.ID: true, second.ID: true}
	if len(gotIDs) != 2 || !wantIDs[gotIDs[0]] || !wantIDs[gotIDs[1]] || gotIDs[0] == gotIDs[1] {
		t.Fatalf("group project IDs = %v, want %s and %s", gotIDs, first.ID, second.ID)
	}
}

func assertConfigRefsProject(t *testing.T, cfg *config.Config, taskID, projectID string) {
	t.Helper()
	if len(cfg.FocusQueue) != 1 || cfg.FocusQueue[0].TaskID != taskID || cfg.FocusQueue[0].ProjectID != projectID {
		t.Fatalf("FocusQueue = %+v, want task %s in project %s", cfg.FocusQueue, taskID, projectID)
	}
	if len(cfg.DailyPlan) > 0 && (cfg.DailyPlan[0].TaskID != taskID || cfg.DailyPlan[0].ProjectID != projectID) {
		t.Fatalf("DailyPlan = %+v, want task %s in project %s", cfg.DailyPlan, taskID, projectID)
	}
	if len(cfg.RecentTaskVisits) > 0 && (cfg.RecentTaskVisits[0].TaskID != taskID || cfg.RecentTaskVisits[0].ProjectID != projectID) {
		t.Fatalf("RecentTaskVisits = %+v, want task %s in project %s", cfg.RecentTaskVisits, taskID, projectID)
	}
}

func drainTUITestCmd(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		msgs := make([]tea.Msg, 0, len(batch))
		for _, batchCmd := range batch {
			if batchCmd == nil {
				continue
			}
			msgs = append(msgs, batchCmd())
		}
		return msgs
	}
	return []tea.Msg{msg}
}
