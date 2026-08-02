package storage

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage/format"
)

func TestV5ProjectDocumentRoundTripUsesGenericHeadingsAndNoCatalog(t *testing.T) {
	now := time.Date(2026, 7, 28, 9, 0, 0, 123, time.FixedZone("CST", -6*60*60))
	document := V5ProjectDocument{
		ProjectName: "Codec Project",
		Tasks: []format.V5TaskRecord{
			{
				ID:          "root",
				ShortID:     1,
				Title:       "Root record",
				Description: "Description with\n## Bug: legacy-looking text",
				State:       "ready",
				Priority:    3,
				Tags:        []string{"story", "source:github"},
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			{
				ID:        "child",
				ShortID:   2,
				Title:     "Child record",
				State:     "in_progress",
				Priority:  1,
				ParentID:  "root",
				Tags:      []string{"team:platform"},
				CreatedAt: now.Add(time.Minute),
				UpdatedAt: now.Add(2 * time.Minute),
			},
		},
	}

	content, err := WriteV5ProjectDocument(document)
	if err != nil {
		t.Fatalf("WriteV5ProjectDocument: %v", err)
	}
	for _, want := range []string{
		"<!-- tt-schema: 5 -->",
		"## Task: Root record",
		"### Task: Child record",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("v5 document missing %q:\n%s", want, content)
		}
	}
	for _, line := range strings.Split(content, "\n") {
		for _, forbidden := range []string{
			"<!-- tags:", "<!-- pinned_tags:",
			"## Epic:", "## Story:", "## Bug:", "## Idea:", "## Action:", "## Note:",
		} {
			if strings.HasPrefix(line, forbidden) {
				t.Errorf("v5 document contains retired/catalog syntax %q:\n%s", forbidden, content)
			}
		}
	}

	parsed, err := ParseV5ProjectDocument(content)
	if err != nil {
		t.Fatalf("ParseV5ProjectDocument: %v\ncontent:\n%s", err, content)
	}
	if parsed.ProjectName != document.ProjectName || len(parsed.Tasks) != len(document.Tasks) {
		t.Fatalf("parsed document summary = %#v, want %#v", parsed, document)
	}
	for i := range document.Tasks {
		gotBody, gotErr := format.WriteV5TaskBody(parsed.Tasks[i])
		wantBody, wantErr := format.WriteV5TaskBody(document.Tasks[i])
		if gotErr != nil || wantErr != nil || gotBody != wantBody {
			t.Errorf("task %d mismatch:\n got: %#v\nwant: %#v", i, parsed.Tasks[i], document.Tasks[i])
		}
	}

	second, err := WriteV5ProjectDocument(parsed)
	if err != nil {
		t.Fatalf("second WriteV5ProjectDocument: %v", err)
	}
	if second != content {
		t.Fatalf("v5 project document is not a fixed point:\nfirst:\n%s\nsecond:\n%s", content, second)
	}
}

func TestV5ProjectDocumentRoundTripRetainsSchema6RetiredValues(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	document := V5ProjectDocument{
		ProjectName: "Retained Legacy Values",
		Tasks: []format.V5TaskRecord{
			{
				ID:        "task-one",
				ShortID:   1,
				Title:     "Legacy-rich task",
				State:     "ready",
				Priority:  3,
				CreatedAt: now,
				UpdatedAt: now,
				DependsOn: []string{"task-two", "project-beta:task-three"},
				Recurrence: &format.V5TaskRecurrence{
					Interval: 2,
					Unit:     "week",
				},
				Decisions: []format.V5TaskDecision{
					{Text: "Keep the adapter explicit", Rationale: "avoid startup ambiguity", CreatedAt: now},
				},
			},
			{ID: "task-two", ShortID: 2, Title: "Local blocker", State: "done", Priority: 3, CreatedAt: now, UpdatedAt: now, CompletedAt: &now},
		},
	}

	encoded, err := WriteV5ProjectDocument(document)
	if err != nil {
		t.Fatalf("WriteV5ProjectDocument: %v", err)
	}
	decoded, err := ParseV5ProjectDocument(encoded)
	if err != nil {
		t.Fatalf("ParseV5ProjectDocument: %v", err)
	}
	if len(decoded.Tasks) != 2 {
		t.Fatalf("decoded task count = %d, want 2", len(decoded.Tasks))
	}
	retained := decoded.Tasks[0]
	if strings.Join(retained.DependsOn, "|") != "task-two|project-beta:task-three" {
		t.Fatalf("dependencies = %v, want both retained", retained.DependsOn)
	}
	if retained.Recurrence == nil || retained.Recurrence.Interval != 2 || retained.Recurrence.Unit != "week" {
		t.Fatalf("recurrence = %+v, want retained", retained.Recurrence)
	}
	if len(retained.Decisions) != 1 || retained.Decisions[0].Text != "Keep the adapter explicit" || retained.Decisions[0].Rationale != "avoid startup ambiguity" {
		t.Fatalf("decisions = %+v, want retained", retained.Decisions)
	}
	second, err := WriteV5ProjectDocument(decoded)
	if err != nil {
		t.Fatalf("second WriteV5ProjectDocument: %v", err)
	}
	if second != encoded {
		t.Fatalf("retired schema-5 values are not a fixed point:\nfirst:\n%s\nsecond:\n%s", encoded, second)
	}
}

func TestParseV5ProjectDocumentRejectsLegacyAndCatalogSyntax(t *testing.T) {
	now := "2026-07-28T12:00:00Z"
	tests := map[string]string{
		"schema v2": "# Project: Legacy\n<!-- tt-schema: 2 -->\n",
		"typed heading": "# Project: Legacy\n<!-- tt-schema: 5 -->\n\n" +
			"## Bug: Typed\n<!-- id: typed, short_id: 1, state: ready, priority: 3, created: " + now + ", updated: " + now + " -->\n",
		"tag catalog": "# Project: Legacy\n<!-- tt-schema: 5 -->\n<!-- tags: catalog-only -->\n",
		"retired metadata": "# Project: Legacy\n<!-- tt-schema: 5 -->\n\n" +
			"## Task: Generic\n<!-- id: generic, short_id: 1, state: ready, priority: 3, created: " + now + ", updated: " + now + ", source: github -->\n",
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseV5ProjectDocument(content); err == nil {
				t.Fatalf("ParseV5ProjectDocument accepted invalid content:\n%s", content)
			}
		})
	}
}

func TestValidateV5ProjectDocumentRejectsMissingParentsAndIdentityCollisions(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	base := format.V5TaskRecord{
		ID:        "one",
		ShortID:   1,
		Title:     "One",
		State:     "ready",
		Priority:  3,
		CreatedAt: now,
		UpdatedAt: now,
	}
	tests := map[string][]format.V5TaskRecord{
		"missing parent": {
			func() format.V5TaskRecord {
				task := base
				task.ParentID = "missing"
				return task
			}(),
		},
		"duplicate id": {
			base,
			func() format.V5TaskRecord {
				task := base
				task.ShortID = 2
				task.Title = "Two"
				return task
			}(),
		},
		"duplicate short id": {
			base,
			func() format.V5TaskRecord {
				task := base
				task.ID = "two"
				task.Title = "Two"
				return task
			}(),
		},
	}
	for name, tasks := range tests {
		t.Run(name, func(t *testing.T) {
			if err := ValidateV5ProjectDocument(V5ProjectDocument{ProjectName: "Invalid", Tasks: tasks}); err == nil {
				t.Fatalf("ValidateV5ProjectDocument accepted %s", name)
			}
		})
	}
}

func TestValidateV5ProjectDocumentEnforcesMaximumHierarchyDepth(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	tasks := make([]format.V5TaskRecord, models.MaxNestingDepth+2)
	for i := range tasks {
		tasks[i] = format.V5TaskRecord{
			ID:        fmt.Sprintf("task-%d", i),
			ShortID:   i + 1,
			Title:     fmt.Sprintf("Task %d", i),
			State:     "ready",
			Priority:  3,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if i > 0 {
			tasks[i].ParentID = tasks[i-1].ID
		}
	}
	if err := ValidateV5ProjectDocument(V5ProjectDocument{ProjectName: "Too Deep", Tasks: tasks}); err == nil {
		t.Fatal("ValidateV5ProjectDocument accepted a hierarchy deeper than MaxNestingDepth")
	}

	tasks = tasks[:models.MaxNestingDepth+1]
	if err := ValidateV5ProjectDocument(V5ProjectDocument{ProjectName: "At Limit", Tasks: tasks}); err != nil {
		t.Fatalf("ValidateV5ProjectDocument rejected hierarchy at MaxNestingDepth: %v", err)
	}
}
