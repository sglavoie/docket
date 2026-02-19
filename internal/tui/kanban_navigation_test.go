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
				KanbanSectionReview:  0,
				KanbanSectionStandby: 0,
				KanbanSectionNext:    2,
				KanbanSectionInbox:   0,
				KanbanSectionWaiting: 0,
				KanbanSectionSomeday: 0,
			},
			expectedIndex: 3,
		},
		{
			name:         "search right skips empty columns",
			activeColumn: 0,
			direction:    1,
			taskCounts: map[string]int{
				KanbanSectionActive:  1,
				KanbanSectionReview:  0,
				KanbanSectionStandby: 0,
				KanbanSectionNext:    0,
				KanbanSectionInbox:   0,
				KanbanSectionWaiting: 3,
				KanbanSectionSomeday: 0,
			},
			expectedIndex: 5,
		},
		{
			name:         "search left finds previous non-empty",
			activeColumn: 6,
			direction:    -1,
			taskCounts: map[string]int{
				KanbanSectionActive:  0,
				KanbanSectionReview:  0,
				KanbanSectionStandby: 0,
				KanbanSectionNext:    0,
				KanbanSectionInbox:   2,
				KanbanSectionWaiting: 0,
				KanbanSectionSomeday: 1,
			},
			expectedIndex: 4,
		},
		{
			name:         "search left skips empty columns",
			activeColumn: 6,
			direction:    -1,
			taskCounts: map[string]int{
				KanbanSectionActive:  5,
				KanbanSectionReview:  0,
				KanbanSectionStandby: 0,
				KanbanSectionNext:    0,
				KanbanSectionInbox:   0,
				KanbanSectionWaiting: 0,
				KanbanSectionSomeday: 1,
			},
			expectedIndex: 0,
		},
		{
			name:         "no non-empty columns to the right returns current",
			activeColumn: 3,
			direction:    1,
			taskCounts: map[string]int{
				KanbanSectionActive:  1,
				KanbanSectionReview:  0,
				KanbanSectionStandby: 0,
				KanbanSectionNext:    2,
				KanbanSectionInbox:   0,
				KanbanSectionWaiting: 0,
				KanbanSectionSomeday: 0,
			},
			expectedIndex: 3,
		},
		{
			name:         "no non-empty columns to the left returns current",
			activeColumn: 4,
			direction:    -1,
			taskCounts: map[string]int{
				KanbanSectionActive:  0,
				KanbanSectionReview:  0,
				KanbanSectionStandby: 0,
				KanbanSectionNext:    0,
				KanbanSectionInbox:   3,
				KanbanSectionWaiting: 0,
				KanbanSectionSomeday: 0,
			},
			expectedIndex: 4,
		},
		{
			name:         "all columns empty to the right returns current",
			activeColumn: 0,
			direction:    1,
			taskCounts: map[string]int{
				KanbanSectionActive:  1,
				KanbanSectionReview:  0,
				KanbanSectionStandby: 0,
				KanbanSectionNext:    0,
				KanbanSectionInbox:   0,
				KanbanSectionWaiting: 0,
				KanbanSectionSomeday: 0,
			},
			expectedIndex: 0,
		},
		{
			name:         "at rightmost boundary returns current",
			activeColumn: 6,
			direction:    1,
			taskCounts: map[string]int{
				KanbanSectionActive:  1,
				KanbanSectionReview:  1,
				KanbanSectionStandby: 1,
				KanbanSectionNext:    1,
				KanbanSectionInbox:   1,
				KanbanSectionWaiting: 1,
				KanbanSectionSomeday: 1,
			},
			expectedIndex: 6,
		},
		{
			name:         "at leftmost boundary returns current",
			activeColumn: 0,
			direction:    -1,
			taskCounts: map[string]int{
				KanbanSectionActive:  1,
				KanbanSectionReview:  1,
				KanbanSectionStandby: 1,
				KanbanSectionNext:    1,
				KanbanSectionInbox:   1,
				KanbanSectionWaiting: 1,
				KanbanSectionSomeday: 1,
			},
			expectedIndex: 0,
		},
		{
			name:         "finds immediate neighbor when non-empty",
			activeColumn: 4,
			direction:    1,
			taskCounts: map[string]int{
				KanbanSectionActive:  0,
				KanbanSectionReview:  0,
				KanbanSectionStandby: 0,
				KanbanSectionNext:    0,
				KanbanSectionInbox:   1,
				KanbanSectionWaiting: 1,
				KanbanSectionSomeday: 0,
			},
			expectedIndex: 5,
		},
		{
			name:         "middle column with non-empty on both sides goes in direction",
			activeColumn: 4,
			direction:    -1,
			taskCounts: map[string]int{
				KanbanSectionActive:  1,
				KanbanSectionReview:  1,
				KanbanSectionStandby: 1,
				KanbanSectionNext:    1,
				KanbanSectionInbox:   1,
				KanbanSectionWaiting: 1,
				KanbanSectionSomeday: 1,
			},
			expectedIndex: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a KanbanViewModel with the specified task configuration
			m := &KanbanViewModel{
				Project:      &models.Project{},
				columnDefs:   ColumnsForGroupBy(GroupBySection, nil),
				Columns:      make(map[string]*ColumnState),
				Tasks:        make(map[string][]*models.Task),
				Nodes:        make(map[string][]*KanbanNode),
				ExpandState:  make(map[string]bool),
				ActiveColumn: tt.activeColumn,
			}

			// Initialize columns and populate tasks/nodes based on taskCounts
			for _, def := range m.columnDefs {
				section := def.Key
				m.Columns[section] = &ColumnState{}
				count := tt.taskCounts[section]
				tasks := make([]*models.Task, count)
				nodes := make([]*KanbanNode, count)
				for i := range tasks {
					tasks[i] = makeTask()
					nodes[i] = &KanbanNode{
						Task:    tasks[i],
						Visible: true,
					}
				}
				m.Tasks[section] = tasks
				m.Nodes[section] = nodes
			}

			got := m.findNextNonEmptyColumn(tt.direction)
			if got != tt.expectedIndex {
				t.Errorf("findNextNonEmptyColumn(%d) = %d, want %d", tt.direction, got, tt.expectedIndex)
			}
		})
	}
}
