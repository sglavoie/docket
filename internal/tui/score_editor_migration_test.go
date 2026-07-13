package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/scoring"
)

// TestScoreEditorSaveReplacesThroughCoordinator pins the score-editor migration
// (session 33.7.3.4.5): save() commits the manual dimension set through the
// TaskItemCoordinator (one ledger-undoable ChangeSet, no raw save) with REPLACE
// semantics — a dimension the user reset to derived/default is dropped, not
// merged back in.
func TestScoreEditorSaveReplacesThroughCoordinator(t *testing.T) {
	task := seedTask(t, 1, "Scored", models.StateReady)
	task.ScoreDimensions = map[string]int{"impact": 1, "reach": 5}
	env := newKanbanBulkEnv(t, []*models.Task{task})
	coords := newSecondaryCoordinators(env.svc, nil)

	m := &ScoreEditorModel{
		task:        task,
		project:     env.projects[0],
		coordinator: coords.TaskItem,
		profile: &scoring.ScoringProfile{
			Dimensions: []scoring.ScoringDimension{
				{ID: "impact", Min: 0, Max: 9},
				{ID: "reach", Min: 0, Max: 9},
			},
		},
		values:  []int{4, 3},
		sources: []string{"manual", "default"}, // reach reset to default → dropped on save
	}

	cmd := m.save()
	if cmd == nil {
		t.Fatal("save returned nil command")
	}
	msg := cmd()
	done, ok := msg.(ScoreEditorDoneMsg)
	if !ok {
		t.Fatalf("msg = %T, want ScoreEditorDoneMsg", msg)
	}
	if !done.Saved {
		t.Fatal("ScoreEditorDoneMsg.Saved = false, want true")
	}

	reloaded := env.reload(t, 0).GetTaskByID(task.ID)
	if len(reloaded.ScoreDimensions) != 1 || reloaded.ScoreDimensions["impact"] != 4 {
		t.Fatalf("committed dims = %v, want exactly {impact:4} (reach dropped)", reloaded.ScoreDimensions)
	}

	// The commit rode the workspace ledger, so a global undo restores the prior
	// manual dimensions.
	env.undo(t)
	restored := env.reload(t, 0).GetTaskByID(task.ID)
	if restored.ScoreDimensions["impact"] != 1 || restored.ScoreDimensions["reach"] != 5 {
		t.Fatalf("dims after ledger undo = %v, want {impact:1, reach:5}", restored.ScoreDimensions)
	}
}
