package tui

import "testing"

func TestColumnStateValidate(t *testing.T) {
	tests := []struct {
		name           string
		initial        ColumnState
		taskCount      int
		expectedCursor int
		expectedScroll int
	}{
		{
			name:           "empty column resets to zero",
			initial:        ColumnState{Cursor: 5, ScrollOffset: 3},
			taskCount:      0,
			expectedCursor: 0,
			expectedScroll: 0,
		},
		{
			name:           "cursor past end clamped to last",
			initial:        ColumnState{Cursor: 10, ScrollOffset: 0},
			taskCount:      5,
			expectedCursor: 4,
			expectedScroll: 0,
		},
		{
			name:           "negative cursor clamped to zero",
			initial:        ColumnState{Cursor: -3, ScrollOffset: 0},
			taskCount:      5,
			expectedCursor: 0,
			expectedScroll: 0,
		},
		{
			name:           "valid cursor unchanged",
			initial:        ColumnState{Cursor: 2, ScrollOffset: 0},
			taskCount:      5,
			expectedCursor: 2,
			expectedScroll: 0,
		},
		{
			name:           "scroll offset exceeding cursor adjusted",
			initial:        ColumnState{Cursor: 2, ScrollOffset: 5},
			taskCount:      10,
			expectedCursor: 2,
			expectedScroll: 2,
		},
		{
			name:           "negative scroll offset adjusted",
			initial:        ColumnState{Cursor: 2, ScrollOffset: -3},
			taskCount:      10,
			expectedCursor: 2,
			expectedScroll: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := tt.initial
			cs.Validate(tt.taskCount)
			if cs.Cursor != tt.expectedCursor {
				t.Errorf("Cursor = %d, want %d", cs.Cursor, tt.expectedCursor)
			}
			if cs.ScrollOffset != tt.expectedScroll {
				t.Errorf("ScrollOffset = %d, want %d", cs.ScrollOffset, tt.expectedScroll)
			}
		})
	}
}

func TestColumnStateInBounds(t *testing.T) {
	tests := []struct {
		name      string
		state     ColumnState
		taskCount int
		want      bool
	}{
		{"valid cursor", ColumnState{Cursor: 2}, 5, true},
		{"cursor at start", ColumnState{Cursor: 0}, 5, true},
		{"cursor at end", ColumnState{Cursor: 4}, 5, true},
		{"cursor past end", ColumnState{Cursor: 5}, 5, false},
		{"negative cursor", ColumnState{Cursor: -1}, 5, false},
		{"empty list", ColumnState{Cursor: 0}, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.InBounds(tt.taskCount); got != tt.want {
				t.Errorf("InBounds(%d) = %v, want %v", tt.taskCount, got, tt.want)
			}
		})
	}
}

func TestColumnStateAdjustScrollForViewport(t *testing.T) {
	tests := []struct {
		name           string
		initial        ColumnState
		taskCount      int
		viewportHeight int
		expectedScroll int
	}{
		{
			name:           "cursor visible no change",
			initial:        ColumnState{Cursor: 2, ScrollOffset: 0},
			taskCount:      10,
			viewportHeight: 5,
			expectedScroll: 0,
		},
		{
			name:           "cursor below viewport scrolls down",
			initial:        ColumnState{Cursor: 7, ScrollOffset: 0},
			taskCount:      10,
			viewportHeight: 5,
			expectedScroll: 3,
		},
		{
			name:           "cursor above viewport scrolls up",
			initial:        ColumnState{Cursor: 1, ScrollOffset: 5},
			taskCount:      10,
			viewportHeight: 5,
			expectedScroll: 1,
		},
		{
			name:           "empty list resets scroll",
			initial:        ColumnState{Cursor: 0, ScrollOffset: 5},
			taskCount:      0,
			viewportHeight: 5,
			expectedScroll: 0,
		},
		{
			name:           "viewport larger than list",
			initial:        ColumnState{Cursor: 2, ScrollOffset: 3},
			taskCount:      3,
			viewportHeight: 10,
			expectedScroll: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := tt.initial
			cs.AdjustScrollForViewport(tt.taskCount, tt.viewportHeight)
			if cs.ScrollOffset != tt.expectedScroll {
				t.Errorf("ScrollOffset = %d, want %d", cs.ScrollOffset, tt.expectedScroll)
			}
		})
	}
}
