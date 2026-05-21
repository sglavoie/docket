package format

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestSplitMetadataPairs_NoMisSplitOnValuesContainingKeyPatterns(t *testing.T) {
	tests := []struct {
		name    string
		metaStr string
		wantKey string
		wantVal string
		field   string
	}{
		{
			name:    "URL containing comma-id pattern",
			metaStr: "id: t1, status: todo, priority: 2, url: http://example.com/path?id: foo",
			wantKey: "t1",
			wantVal: "http://example.com/path?id: foo",
			field:   "url",
		},
		{
			name:    "URL containing comma-status pattern",
			metaStr: "id: t1, status: todo, priority: 2, url: http://example.com?status: active",
			wantKey: "t1",
			wantVal: "http://example.com?status: active",
			field:   "url",
		},
		{
			name:    "source containing comma-priority pattern",
			metaStr: "id: t1, status: todo, priority: 2, source: from app, priority: high context",
			wantKey: "t1",
			wantVal: "from app, priority: high context",
			field:   "source",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{}
			ParseMetadata(task, tc.metaStr)

			if task.ID != tc.wantKey {
				t.Errorf("ID = %q, want %q", task.ID, tc.wantKey)
			}

			var got string
			switch tc.field {
			case "url":
				got = task.URL
			case "source":
				got = task.Source
			}

			if got != tc.wantVal {
				t.Errorf("%s = %q, want %q", tc.field, got, tc.wantVal)
			}
		})
	}
}

func TestLooksLikeMetadata_RecognizesRegistryKeys(t *testing.T) {
	tests := []struct {
		name    string
		metaStr string
		want    bool
	}{
		{name: "core key", metaStr: "id: task-1", want: true},
		{name: "recurrence subfield", metaStr: "recur.original_day: 31", want: true},
		{name: "type specific key", metaStr: "epic.scope_baselined: 2025-01-01", want: true},
		{name: "valid score key", metaStr: "score.impact: 5", want: true},
		{name: "invalid score key", metaStr: "score.foo-bar: 5", want: false},
		{name: "unknown key", metaStr: "unknown: value", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := LooksLikeMetadata(tc.metaStr); got != tc.want {
				t.Errorf("LooksLikeMetadata(%q) = %v, want %v", tc.metaStr, got, tc.want)
			}
		})
	}
}
