package format

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestParseMetadata_ScoreDimensions(t *testing.T) {
	tests := []struct {
		name    string
		metaStr string
		want    map[string]int
	}{
		{
			name:    "single score dimension",
			metaStr: "id: s1, status: todo, priority: 2, score.impact: 4",
			want:    map[string]int{"impact": 4},
		},
		{
			name:    "multiple score dimensions",
			metaStr: "id: s2, status: todo, priority: 2, score.impact: 4, score.confidence: 3, score.reach: 8",
			want:    map[string]int{"impact": 4, "confidence": 3, "reach": 8},
		},
		{
			name:    "score dimensions mixed with other metadata",
			metaStr: "id: s3, status: in_progress, priority: 1, tags: urgent, score.effort: 2, section: Next, score.value: 5",
			want:    map[string]int{"effort": 2, "value": 5},
		},
		{
			name:    "score dimension with underscore in name",
			metaStr: "id: s4, status: todo, priority: 2, score.business_value: 7",
			want:    map[string]int{"business_value": 7},
		},
		{
			name:    "no score dimensions",
			metaStr: "id: s5, status: todo, priority: 2",
			want:    nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{}
			ParseMetadata(task, tc.metaStr)

			if tc.want == nil {
				if task.ScoreDimensions != nil {
					t.Errorf("ScoreDimensions = %v, want nil", task.ScoreDimensions)
				}
				return
			}
			if task.ScoreDimensions == nil {
				t.Fatal("ScoreDimensions is nil")
			}
			for k, v := range tc.want {
				if got, ok := task.ScoreDimensions[k]; !ok {
					t.Errorf("missing dimension %q", k)
				} else if got != v {
					t.Errorf("dimension %q = %d, want %d", k, got, v)
				}
			}
			if len(task.ScoreDimensions) != len(tc.want) {
				t.Errorf("ScoreDimensions has %d entries, want %d", len(task.ScoreDimensions), len(tc.want))
			}
		})
	}
}

func TestWriteMetadata_ScoreDimensions(t *testing.T) {
	task := &models.Task{
		ID:              "score-1",
		Status:          models.StatusTodo,
		Priority:        2,
		ScoreDimensions: map[string]int{"confidence": 3, "impact": 4},
	}

	got := WriteMetadata(task)
	// Keys should appear in sorted order
	if !containsString(got, "score.confidence: 3") {
		t.Errorf("WriteMetadata() = %q, should contain %q", got, "score.confidence: 3")
	}
	if !containsString(got, "score.impact: 4") {
		t.Errorf("WriteMetadata() = %q, should contain %q", got, "score.impact: 4")
	}
}

func TestScoreDimensions_RoundTrip(t *testing.T) {
	task := &models.Task{
		ID:       "score-rt",
		Type:     models.TaskTypeTask,
		Status:   models.StatusTodo,
		Priority: 2,
		ScoreDimensions: map[string]int{
			"impact":     4,
			"confidence": 3,
			"reach":      8,
		},
	}

	metaStr := WriteMetadata(task)
	content := metaStr[5 : len(metaStr)-4]

	parsed := &models.Task{}
	ParseMetadata(parsed, content)

	if parsed.ScoreDimensions == nil {
		t.Fatal("ScoreDimensions is nil after roundtrip")
	}
	for k, v := range task.ScoreDimensions {
		if got, ok := parsed.ScoreDimensions[k]; !ok {
			t.Errorf("missing dimension %q after roundtrip", k)
		} else if got != v {
			t.Errorf("dimension %q = %d after roundtrip, want %d", k, got, v)
		}
	}
}

func TestScoreDimensions_EmptyNotWritten(t *testing.T) {
	task := &models.Task{
		ID:       "score-empty",
		Status:   models.StatusTodo,
		Priority: 2,
	}

	got := WriteMetadata(task)
	if containsString(got, "score.") {
		t.Errorf("WriteMetadata() = %q, should not contain score dimensions for nil map", got)
	}

	task.ScoreDimensions = map[string]int{}
	got = WriteMetadata(task)
	if containsString(got, "score.") {
		t.Errorf("WriteMetadata() = %q, should not contain score dimensions for empty map", got)
	}
}
