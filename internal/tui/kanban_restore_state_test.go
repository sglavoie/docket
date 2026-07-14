package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func newKanbanRestoreStateFixture() (*AppModel, *KanbanModel, *models.Project) {
	proj := testutil.NewTestProject("p")
	task := testutil.NewTestTask("t", models.TaskTypeTask)
	task.SetSection(models.SectionInbox)
	proj.Tasks = []*models.Task{task}

	cfg := newTestConfig()
	app := &AppModel{
		runtime:         testRuntimeAt(cfg.DataDir(), cfg),
		kanbanViewCache: make(map[string]*KanbanViewCache),
	}

	kb := &KanbanModel{
		runtime:     testRuntimeAt(cfg.DataDir(), cfg),
		project:     proj,
		mode:        KanbanModeSection,
		allCards:    []KanbanCard{{Task: task, Project: proj}},
		FilterMixin: FilterMixin{filter: newFilterState()},
	}

	return app, kb, proj
}

// TestRestoreKanbanState_ClampsOutOfBoundsScrollOffset verifies that
// restoreKanbanState clamps a cached ScrollOffset that no longer fits within
// a column's task list. Without clamping, deleting tasks while away from the
// kanban view would leave the column scrolled past its last task on return.
func TestRestoreKanbanState_ClampsOutOfBoundsScrollOffset(t *testing.T) {
	app, kb, proj := newKanbanRestoreStateFixture()

	// Cache a wildly out-of-bounds scroll offset and a stale cursor.
	app.kanbanViewCache[proj.ID] = &KanbanViewCache{
		ActiveColID:      "inbox",
		ColCursors:       map[string]int{"inbox": 99},
		ColScrollOffsets: map[string]int{"inbox": 99},
		Mode:             KanbanModeSection,
	}

	app.restoreKanbanState(kb)

	for _, col := range kb.columns {
		if col.ID != "inbox" {
			continue
		}
		if col.ScrollOffset >= len(col.Tasks) {
			t.Errorf("ScrollOffset not clamped: got %d for %d tasks", col.ScrollOffset, len(col.Tasks))
		}
		if col.Cursor >= len(col.Tasks) {
			t.Errorf("Cursor not clamped: got %d for %d tasks", col.Cursor, len(col.Tasks))
		}
	}
}

// TestRestoreKanbanState_ClampsNegativeScrollOffset verifies that a corrupted
// negative ScrollOffset in the cache is clamped to 0 on restore.
func TestRestoreKanbanState_ClampsNegativeScrollOffset(t *testing.T) {
	app, kb, proj := newKanbanRestoreStateFixture()

	app.kanbanViewCache[proj.ID] = &KanbanViewCache{
		ActiveColID:      "inbox",
		ColCursors:       map[string]int{"inbox": 0},
		ColScrollOffsets: map[string]int{"inbox": -5},
		Mode:             KanbanModeSection,
	}

	app.restoreKanbanState(kb)

	for _, col := range kb.columns {
		if col.ID != "inbox" {
			continue
		}
		if col.ScrollOffset != 0 {
			t.Errorf("negative ScrollOffset not clamped: got %d", col.ScrollOffset)
		}
	}
}
