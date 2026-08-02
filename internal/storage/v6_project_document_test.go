package storage

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage/format"
)

func TestV6ProjectDocumentRoundTripUsesGenericHeadings(t *testing.T) {
	now := time.Date(2026, 8, 1, 9, 0, 0, 123, time.UTC)
	document := V6ProjectDocument{
		ProjectName: "Codec Project",
		Tasks: []format.V6TaskRecord{
			{
				ID:          "root",
				ShortID:     1,
				Title:       "Root record",
				Description: "Description with\n## Bug: legacy-looking text",
				State:       "backlog",
				Priority:    0,
				Tags:        []string{"bug", "feature", "source:github"},
				CreatedAt:   now,
				UpdatedAt:   now,
				Comments: []format.V6TaskComment{
					{Text: "A retained comment", CreatedAt: now.Add(time.Minute)},
				},
			},
			{
				ID:        "child",
				ShortID:   2,
				Title:     "Child record",
				State:     "doing",
				Priority:  1,
				ParentID:  "root",
				Tags:      []string{"team:platform"},
				CreatedAt: now.Add(2 * time.Minute),
				UpdatedAt: now.Add(3 * time.Minute),
			},
		},
	}

	content, err := WriteV6ProjectDocument(document)
	if err != nil {
		t.Fatalf("WriteV6ProjectDocument: %v", err)
	}
	for _, want := range []string{
		"<!-- tt-schema: 6 -->",
		"## Task: Root record",
		"### Task: Child record",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("v6 document missing %q:\n%s", want, content)
		}
	}
	for _, line := range strings.Split(content, "\n") {
		for _, forbidden := range []string{
			"<!-- tags:", "<!-- pinned_tags:",
			"## Epic:", "## Story:", "## Bug:", "## Idea:", "## Action:", "## Note:",
			"<!-- DECISIONS -->",
		} {
			if strings.HasPrefix(line, forbidden) {
				t.Errorf("v6 document contains retired/catalog syntax %q:\n%s", forbidden, content)
			}
		}
	}

	parsed, err := ParseV6ProjectDocument(content)
	if err != nil {
		t.Fatalf("ParseV6ProjectDocument: %v\ncontent:\n%s", err, content)
	}
	if !reflect.DeepEqual(parsed, document) {
		t.Fatalf("parsed document mismatch:\n got: %#v\nwant: %#v", parsed, document)
	}
	second, err := WriteV6ProjectDocument(parsed)
	if err != nil {
		t.Fatalf("second WriteV6ProjectDocument: %v", err)
	}
	if second != content {
		t.Fatalf("v6 project document is not a fixed point:\nfirst:\n%s\nsecond:\n%s", content, second)
	}
}

func TestParseV6ProjectDocumentRejectsWrongSchemaAndRetiredSyntax(t *testing.T) {
	now := "2026-08-01T12:00:00Z"
	validMetadata := "<!-- id: generic, short_id: 1, state: backlog, priority: 0, created: " + now + ", updated: " + now + " -->"
	tests := map[string]string{
		"schema v5":     "# Project: Legacy\n<!-- tt-schema: 5 -->\n",
		"typed heading": "# Project: Legacy\n<!-- tt-schema: 6 -->\n\n## Bug: Typed\n" + validMetadata + "\n",
		"tag catalog":   "# Project: Legacy\n<!-- tt-schema: 6 -->\n<!-- tags: catalog-only -->\n",
		"dependency": "# Project: Legacy\n<!-- tt-schema: 6 -->\n\n## Task: Generic\n" +
			"<!-- id: generic, short_id: 1, state: backlog, priority: 0, created: " + now + ", updated: " + now + ", depends: other -->\n",
		"recurrence": "# Project: Legacy\n<!-- tt-schema: 6 -->\n\n## Task: Generic\n" +
			"<!-- id: generic, short_id: 1, state: backlog, priority: 0, created: " + now + ", updated: " + now + ", recur: 1w -->\n",
		"decision block": "# Project: Legacy\n<!-- tt-schema: 6 -->\n\n## Task: Generic\n" + validMetadata +
			"\n<!-- DECISIONS -->\n<!-- decision: " + now + " -->\nText\n<!-- rationale -->\nWhy\n<!-- /DECISIONS -->\n",
		"untrimmed title":  "# Project: Legacy\n<!-- tt-schema: 6 -->\n\n## Task: Generic  \n" + validMetadata + "\n",
		"reordered header": "<!-- tt-schema: 6 -->\n# Project: Legacy\n",
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseV6ProjectDocument(content); err == nil {
				t.Fatalf("ParseV6ProjectDocument accepted invalid content:\n%s", content)
			}
		})
	}
}

func TestValidateV6ProjectDocumentRejectsMalformedHierarchyAndIdentity(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	base := format.V6TaskRecord{
		ID:        "one",
		ShortID:   1,
		Title:     "One",
		State:     "backlog",
		Priority:  0,
		CreatedAt: now,
		UpdatedAt: now,
	}
	tests := map[string][]format.V6TaskRecord{
		"missing parent": {
			func() format.V6TaskRecord {
				task := base
				task.ParentID = "missing"
				return task
			}(),
		},
		"self parent": {
			func() format.V6TaskRecord {
				task := base
				task.ParentID = task.ID
				return task
			}(),
		},
		"parent cycle": {
			func() format.V6TaskRecord {
				task := base
				task.ParentID = "two"
				return task
			}(),
			func() format.V6TaskRecord {
				task := base
				task.ID = "two"
				task.ShortID = 2
				task.Title = "Two"
				task.ParentID = "one"
				return task
			}(),
		},
		"duplicate id": {
			base,
			func() format.V6TaskRecord {
				task := base
				task.ShortID = 2
				task.Title = "Two"
				return task
			}(),
		},
		"duplicate short id": {
			base,
			func() format.V6TaskRecord {
				task := base
				task.ID = "two"
				task.Title = "Two"
				return task
			}(),
		},
	}
	for name, tasks := range tests {
		t.Run(name, func(t *testing.T) {
			if err := ValidateV6ProjectDocument(V6ProjectDocument{ProjectName: "Invalid", Tasks: tasks}); err == nil {
				t.Fatalf("ValidateV6ProjectDocument accepted %s", name)
			}
		})
	}
}

func TestValidateV6ProjectDocumentEnforcesMaximumHierarchyDepth(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	tasks := make([]format.V6TaskRecord, models.MaxNestingDepth+2)
	for i := range tasks {
		tasks[i] = format.V6TaskRecord{
			ID:        fmt.Sprintf("task-%d", i),
			ShortID:   i + 1,
			Title:     fmt.Sprintf("Task %d", i),
			State:     "backlog",
			Priority:  0,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if i > 0 {
			tasks[i].ParentID = tasks[i-1].ID
		}
	}
	if err := ValidateV6ProjectDocument(V6ProjectDocument{ProjectName: "Too Deep", Tasks: tasks}); err == nil {
		t.Fatal("ValidateV6ProjectDocument accepted a hierarchy deeper than MaxNestingDepth")
	}

	tasks = tasks[:models.MaxNestingDepth+1]
	document := V6ProjectDocument{ProjectName: "At Limit", Tasks: tasks}
	if err := ValidateV6ProjectDocument(document); err != nil {
		t.Fatalf("ValidateV6ProjectDocument rejected hierarchy at MaxNestingDepth: %v", err)
	}
	content, err := WriteV6ProjectDocument(document)
	if err != nil {
		t.Fatalf("WriteV6ProjectDocument at depth limit: %v", err)
	}
	parsed, err := ParseV6ProjectDocument(content)
	if err != nil {
		t.Fatalf("ParseV6ProjectDocument at depth limit: %v\n%s", err, content)
	}
	if len(parsed.Tasks) != len(tasks) {
		t.Fatalf("round trip task count = %d, want %d", len(parsed.Tasks), len(tasks))
	}
}
