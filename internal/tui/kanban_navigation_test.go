package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestFindNextNonEmptyColumn(t *testing.T) {
	// Helper to create a task for populating columns
	makeTask := func() *models.Task {
		return &models.Task{Title: "test task", Status: models.StatusTodo}
	}

	tests := []struct {
		name          string
		activeColumn  int
		direction     int
		taskCounts    map[string]int // section -> task count
		expectedIndex int
	}{
		{
			name:         "search right finds next non-empty",
			activeColumn: 0,
			direction:    1,
			taskCounts: map[string]int{
				KanbanSectionActive:  1,
				KanbanSectionNext:    2,
				KanbanSectionInbox:   0,
				KanbanSectionWaiting: 0,
				KanbanSectionSomeday: 0,
			},
			expectedIndex: 1,
		},
		{
			name:         "search right skips empty columns",
			activeColumn: 0,
			direction:    1,
			taskCounts: map[string]int{
				KanbanSectionActive:  1,
				KanbanSectionNext:    0,
				KanbanSectionInbox:   0,
				KanbanSectionWaiting: 3,
				KanbanSectionSomeday: 0,
			},
			expectedIndex: 3,
		},
		{
			name:         "search left finds previous non-empty",
			activeColumn: 4,
			direction:    -1,
			taskCounts: map[string]int{
				KanbanSectionActive:  0,
				KanbanSectionNext:    0,
				KanbanSectionInbox:   2,
				KanbanSectionWaiting: 0,
				KanbanSectionSomeday: 1,
			},
			expectedIndex: 2,
		},
		{
			name:         "search left skips empty columns",
			activeColumn: 4,
			direction:    -1,
			taskCounts: map[string]int{
				KanbanSectionActive:  5,
				KanbanSectionNext:    0,
				KanbanSectionInbox:   0,
				KanbanSectionWaiting: 0,
				KanbanSectionSomeday: 1,
			},
			expectedIndex: 0,
		},
		{
			name:         "no non-empty columns to the right returns current",
			activeColumn: 1,
			direction:    1,
			taskCounts: map[string]int{
				KanbanSectionActive:  1,
				KanbanSectionNext:    2,
				KanbanSectionInbox:   0,
				KanbanSectionWaiting: 0,
				KanbanSectionSomeday: 0,
			},
			expectedIndex: 1,
		},
		{
			name:         "no non-empty columns to the left returns current",
			activeColumn: 2,
			direction:    -1,
			taskCounts: map[string]int{
				KanbanSectionActive:  0,
				KanbanSectionNext:    0,
				KanbanSectionInbox:   3,
				KanbanSectionWaiting: 0,
				KanbanSectionSomeday: 0,
			},
			expectedIndex: 2,
		},
		{
			name:         "all columns empty to the right returns current",
			activeColumn: 0,
			direction:    1,
			taskCounts: map[string]int{
				KanbanSectionActive:  1,
				KanbanSectionNext:    0,
				KanbanSectionInbox:   0,
				KanbanSectionWaiting: 0,
				KanbanSectionSomeday: 0,
			},
			expectedIndex: 0,
		},
		{
			name:         "at rightmost boundary returns current",
			activeColumn: 4,
			direction:    1,
			taskCounts: map[string]int{
				KanbanSectionActive:  1,
				KanbanSectionNext:    1,
				KanbanSectionInbox:   1,
				KanbanSectionWaiting: 1,
				KanbanSectionSomeday: 1,
			},
			expectedIndex: 4,
		},
		{
			name:         "at leftmost boundary returns current",
			activeColumn: 0,
			direction:    -1,
			taskCounts: map[string]int{
				KanbanSectionActive:  1,
				KanbanSectionNext:    1,
				KanbanSectionInbox:   1,
				KanbanSectionWaiting: 1,
				KanbanSectionSomeday: 1,
			},
			expectedIndex: 0,
		},
		{
			name:         "finds immediate neighbor when non-empty",
			activeColumn: 2,
			direction:    1,
			taskCounts: map[string]int{
				KanbanSectionActive:  0,
				KanbanSectionNext:    0,
				KanbanSectionInbox:   1,
				KanbanSectionWaiting: 1,
				KanbanSectionSomeday: 0,
			},
			expectedIndex: 3,
		},
		{
			name:         "middle column with non-empty on both sides goes in direction",
			activeColumn: 2,
			direction:    -1,
			taskCounts: map[string]int{
				KanbanSectionActive:  1,
				KanbanSectionNext:    1,
				KanbanSectionInbox:   1,
				KanbanSectionWaiting: 1,
				KanbanSectionSomeday: 1,
			},
			expectedIndex: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a KanbanViewModel with the specified task configuration
			m := &KanbanViewModel{
				Project:      &models.Project{},
				Columns:      make(map[string]*ColumnState),
				Tasks:        make(map[string][]*models.Task),
				ActiveColumn: tt.activeColumn,
			}

			// Initialize columns and populate tasks based on taskCounts
			for _, section := range KanbanColumnOrder {
				m.Columns[section] = &ColumnState{}
				count := tt.taskCounts[section]
				tasks := make([]*models.Task, count)
				for i := range tasks {
					tasks[i] = makeTask()
				}
				m.Tasks[section] = tasks
			}

			got := m.findNextNonEmptyColumn(tt.direction)
			if got != tt.expectedIndex {
				t.Errorf("findNextNonEmptyColumn(%d) = %d, want %d", tt.direction, got, tt.expectedIndex)
			}
		})
	}
}
