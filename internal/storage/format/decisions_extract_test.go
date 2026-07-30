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
}

func TestExtractDecisionsFromDescription_DiscardsLegacyHeaderFields(t *testing.T) {
	description := `Task description

<!-- DECISIONS -->
<!-- decision: 2026-05-07 14:30, category:design, resolved -->
Use SQL
<!-- rationale -->
Team already knows it
<!-- /DECISIONS -->`

	task := &models.Task{Description: description}
	ExtractDecisionsFromDescription(task)

	if len(task.Decisions) != 1 || task.Decisions[0].Text != "Use SQL" ||
		task.Decisions[0].Rationale != "Team already knows it" {
		t.Fatalf("decisions = %+v, want retained lean record", task.Decisions)
	}
	if rewritten := WriteDecisions(task); strings.Contains(rewritten, "category:") || strings.Contains(rewritten, "resolved") {
		t.Fatalf("lean rewrite retained legacy decision fields:\n%s", rewritten)
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
