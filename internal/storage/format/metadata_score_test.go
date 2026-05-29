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

func TestScoreDimensions_RoundTripWithScorePatternInURL(t *testing.T) {
	task := &models.Task{
		ID:       "score-url",
		Status:   models.StatusTodo,
		Priority: 2,
		URL:      "https://example.com/search?q=literal, score.impact: not-metadata",
		ScoreDimensions: map[string]int{
			"impact": 4,
		},
	}

	meta := WriteMetadata(task)
	if !containsString(meta, `url: https://example.com/search?q=literal\, score.impact: not-metadata`) {
		t.Fatalf("WriteMetadata() = %q, want escaped comma in URL", meta)
	}

	content := meta[5 : len(meta)-4]
	parsed := &models.Task{}
	warnings := ParseMetadata(parsed, content)

	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if parsed.URL != task.URL {
		t.Errorf("URL = %q, want %q", parsed.URL, task.URL)
	}
	if got := parsed.ScoreDimensions["impact"]; got != 4 {
		t.Errorf("score impact = %d, want 4 (all scores: %v)", got, parsed.ScoreDimensions)
	}
}

func TestWriteMetadata_SkipsInvalidScoreDimensionIDs(t *testing.T) {
	tests := []struct {
		name      string
		dimension string
	}{
		{"hyphen in ID", "foo-bar"},
		{"dot in ID", "foo.bar"},
		{"space in ID", "foo bar"},
		{"colon in ID", "foo:bar"},
		{"comma in ID", "foo,bar"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{
				ID:       "score-invalid",
				Type:     models.TaskTypeTask,
				Status:   models.StatusTodo,
				Priority: 2,
				URL:      "http://example.com/page",
				ScoreDimensions: map[string]int{
					tc.dimension: 5,
					"valid":      7,
				},
			}

			meta := WriteMetadata(task)
			if containsString(meta, "score."+tc.dimension) {
				t.Errorf("WriteMetadata = %q, should not contain invalid dim %q", meta, tc.dimension)
			}
			if !containsString(meta, "score.valid: 7") {
				t.Errorf("WriteMetadata = %q, should contain valid dim", meta)
			}

			// Round-trip: adjacent fields (url) must not absorb the dropped score key.
			content := meta[5 : len(meta)-4]
			parsed := &models.Task{}
			ParseMetadata(parsed, content)
			if parsed.URL != task.URL {
				t.Errorf("URL corrupted by invalid score dim: got %q, want %q", parsed.URL, task.URL)
			}
			if _, ok := parsed.ScoreDimensions[tc.dimension]; ok {
				t.Errorf("invalid dim %q should not survive round-trip", tc.dimension)
			}
			if parsed.ScoreDimensions["valid"] != 7 {
				t.Errorf("valid dim lost: got %v", parsed.ScoreDimensions)
			}
		})
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
