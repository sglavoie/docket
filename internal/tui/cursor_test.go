package tui

import "testing"

func TestClampCursor(t *testing.T) {
	tests := []struct {
		name   string
		cursor int
		length int
		want   int
	}{
		{
			name:   "negative clamped to zero",
			cursor: -5,
			length: 10,
			want:   0,
		},
		{
			name:   "beyond length clamped to last",
			cursor: 15,
			length: 10,
			want:   9,
		},
		{
			name:   "valid cursor unchanged",
			cursor: 5,
			length: 10,
			want:   5,
		},
		{
			name:   "empty list returns zero",
			cursor: 5,
			length: 0,
			want:   0,
		},
		{
			name:   "cursor at last position unchanged",
			cursor: 9,
			length: 10,
			want:   9,
		},
		{
			name:   "cursor at first position unchanged",
			cursor: 0,
			length: 10,
			want:   0,
		},
		{
			name:   "cursor equals length clamped",
			cursor: 10,
			length: 10,
			want:   9,
		},
		{
			name:   "negative cursor empty list returns zero",
			cursor: -1,
			length: 0,
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClampCursor(tt.cursor, tt.length)
			if got != tt.want {
				t.Errorf("ClampCursor(%d, %d) = %d, want %d", tt.cursor, tt.length, got, tt.want)
			}
		})
	}
}
