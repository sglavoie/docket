package format

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestExtractDecisionsFromDescription(t *testing.T) {
	tests := []struct {
		name            string
		description     string
		wantDescription string
		wantDecisions   int
	}{
		{
			name:            "no decisions",
			description:     "This is a regular description",
			wantDescription: "This is a regular description",
			wantDecisions:   0,
		},
		{
			name: "single decision",
			description: `Task description here

<!-- DECISIONS -->
<!-- decision: 2026-05-07 14:30, category:design -->
Use PostgreSQL
<!-- rationale -->
Better for complex queries
<!-- /DECISIONS -->`,
			wantDescription: "Task description here",
			wantDecisions:   1,
		},
		{
			name: "multiple decisions",
			description: `Task description

<!-- DECISIONS -->
<!-- decision: 2026-05-07 14:30, category:design -->
First decision
<!-- rationale -->
First rationale
<!-- decision: 2026-05-07 15:00, category:scope -->
Second decision
<!-- rationale -->
Second rationale
<!-- /DECISIONS -->`,
			wantDescription: "Task description",
			wantDecisions:   2,
		},
		{
			name: "decision with multi-line text and rationale",
			description: `Task description

<!-- DECISIONS -->
<!-- decision: 2026-05-07 14:30, category:architecture -->
Use microservices
for scalability
<!-- rationale -->
Allows independent
deployment cycles
<!-- /DECISIONS -->`,
			wantDescription: "Task description",
			wantDecisions:   1,
		},
		{
			name: "empty decision text",
			description: `Task description

<!-- DECISIONS -->
<!-- decision: 2026-05-07 14:30, category:design -->

<!-- rationale -->
Some rationale
<!-- /DECISIONS -->`,
			wantDescription: "Task description",
			wantDecisions:   0,
		},
		{
			name: "decision without rationale",
			description: `Task description

<!-- DECISIONS -->
<!-- decision: 2026-05-07 14:30, category:ux -->
Simplified flow
<!-- rationale -->
<!-- /DECISIONS -->`,
			wantDescription: "Task description",
			wantDecisions:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &models.Task{Description: tt.description}

			ExtractDecisionsFromDescription(task)

			gotDesc := strings.TrimSpace(task.Description)
			wantDesc := strings.TrimSpace(tt.wantDescription)
			if gotDesc != wantDesc {
				t.Errorf("Description = %q, want %q", gotDesc, wantDesc)
			}

			if len(task.Decisions) != tt.wantDecisions {
				t.Errorf("got %d decisions, want %d", len(task.Decisions), tt.wantDecisions)
			}
		})
	}
}

func TestExtractDecisionsFromDescription_Fields(t *testing.T) {
	description := `Task description

<!-- DECISIONS -->
<!-- decision: 2026-05-07 14:30, category:trade-off -->
Use caching
<!-- rationale -->
Reduces latency
<!-- /DECISIONS -->`

	task := &models.Task{Description: description}
	ExtractDecisionsFromDescription(task)

	if len(task.Decisions) != 1 {
		t.Fatalf("got %d decisions, want 1", len(task.Decisions))
	}
	d := task.Decisions[0]

	if d.Text != "Use caching" {
		t.Errorf("Text = %q, want %q", d.Text, "Use caching")
	}
	if d.Rationale != "Reduces latency" {
		t.Errorf("Rationale = %q, want %q", d.Rationale, "Reduces latency")
	}
	if d.Category != models.DecisionCategoryTradeOff {
		t.Errorf("Category = %q, want %q", d.Category, models.DecisionCategoryTradeOff)
	}
	if d.Resolved {
		t.Error("expected decision to not be resolved")
	}
}

func TestExtractDecisionsFromDescription_Resolved(t *testing.T) {
	t.Run("parses resolved decision", func(t *testing.T) {
		description := `Task description

<!-- DECISIONS -->
<!-- decision: 2026-05-07 14:30, category:design, resolved -->
Use SQL
<!-- rationale -->
Team already knows it
<!-- /DECISIONS -->`

		task := &models.Task{Description: description}
		ExtractDecisionsFromDescription(task)

		if len(task.Decisions) != 1 {
			t.Fatalf("got %d decisions, want 1", len(task.Decisions))
		}
		if !task.Decisions[0].Resolved {
			t.Error("expected decision to be resolved")
		}
	})

	t.Run("mixed resolved and unresolved", func(t *testing.T) {
		description := `Task description

<!-- DECISIONS -->
<!-- decision: 2026-05-07 14:30, category:design -->
First
<!-- rationale -->
First rationale
<!-- decision: 2026-05-07 15:00, category:scope, resolved -->
Second
<!-- rationale -->
Second rationale
<!-- decision: 2026-05-07 16:00, category:ux -->
Third
<!-- rationale -->
Third rationale
<!-- /DECISIONS -->`

		task := &models.Task{Description: description}
		ExtractDecisionsFromDescription(task)

		if len(task.Decisions) != 3 {
			t.Fatalf("got %d decisions, want 3", len(task.Decisions))
		}
		if task.Decisions[0].Resolved {
			t.Error("decision 0 should be unresolved")
		}
		if !task.Decisions[1].Resolved {
			t.Error("decision 1 should be resolved")
		}
		if task.Decisions[2].Resolved {
			t.Error("decision 2 should be unresolved")
		}
	})
}

// TestExtractDecisionsFromDescription_ResolvedNoCategory ensures the parser
// recognizes the "resolved" suffix when no category clause is present.
// Previously the parser positionally assigned parts[1] to category, which
// caused "resolved" to be misassigned as the category string and silently
// dropped the resolved flag.
func TestExtractDecisionsFromDescription_ResolvedNoCategory(t *testing.T) {
	description := `Task description

<!-- DECISIONS -->
<!-- decision: 2026-05-07 14:30, resolved -->
Decision without category
<!-- rationale -->
Rationale text
<!-- /DECISIONS -->`

	task := &models.Task{Description: description}
	ExtractDecisionsFromDescription(task)

	if len(task.Decisions) != 1 {
		t.Fatalf("got %d decisions, want 1", len(task.Decisions))
	}
	d := task.Decisions[0]
	if !d.Resolved {
		t.Error("expected decision to be resolved")
	}
	if d.Category != "" {
		t.Errorf("Category = %q, want empty", d.Category)
	}
}

// TestExtractDecisionsFromDescription_FieldsAnyOrder ensures the parser
// recognizes "category:" and "resolved" regardless of their position
// in the comma-separated decision header.
func TestExtractDecisionsFromDescription_FieldsAnyOrder(t *testing.T) {
	description := `<!-- DECISIONS -->
<!-- decision: 2026-05-07 14:30, resolved, category:design -->
Reorder test
<!-- rationale -->
Rationale
<!-- /DECISIONS -->`

	task := &models.Task{Description: description}
	ExtractDecisionsFromDescription(task)

	if len(task.Decisions) != 1 {
		t.Fatalf("got %d decisions, want 1", len(task.Decisions))
	}
	d := task.Decisions[0]
	if !d.Resolved {
		t.Error("expected decision to be resolved")
	}
	if d.Category != models.DecisionCategoryDesign {
		t.Errorf("Category = %q, want %q", d.Category, models.DecisionCategoryDesign)
	}
}

func TestExtractDecisionsFromDescription_EscapedText(t *testing.T) {
	description := `Task description

<!-- DECISIONS -->
<!-- decision: 2026-05-07 14:30, category:design -->
Decision with &lt;!-- and --&gt; markers
<!-- rationale -->
Rationale with &lt;!-- markers too --&gt;
<!-- /DECISIONS -->`

	task := &models.Task{Description: description}
	ExtractDecisionsFromDescription(task)

	if len(task.Decisions) != 1 {
		t.Fatalf("got %d decisions, want 1", len(task.Decisions))
	}
	if task.Decisions[0].Text != "Decision with <!-- and --> markers" {
		t.Errorf("Text = %q", task.Decisions[0].Text)
	}
	if task.Decisions[0].Rationale != "Rationale with <!-- markers too -->" {
		t.Errorf("Rationale = %q", task.Decisions[0].Rationale)
	}
}
