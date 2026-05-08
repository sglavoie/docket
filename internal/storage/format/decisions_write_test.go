package format

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestWriteDecisions(t *testing.T) {
	tests := []struct {
		name      string
		decisions []*models.Decision
		wantHas   []string
		wantEmpty bool
	}{
		{
			name:      "no decisions",
			decisions: nil,
			wantEmpty: true,
		},
		{
			name: "single decision",
			decisions: []*models.Decision{{
				Text:      "Use PostgreSQL",
				Rationale: "Better for complex queries",
				Category:  models.DecisionCategoryDesign,
				CreatedAt: time.Date(2026, 5, 7, 14, 30, 0, 0, time.Local),
			}},
			wantHas: []string{
				"<!-- DECISIONS -->",
				"<!-- decision: 2026-05-07 14:30, category:design -->",
				"Use PostgreSQL",
				"<!-- rationale -->",
				"Better for complex queries",
				"<!-- /DECISIONS -->",
			},
		},
		{
			name: "resolved decision",
			decisions: []*models.Decision{{
				Text:      "Chosen approach",
				Rationale: "Agreed by team",
				Category:  models.DecisionCategoryArchitecture,
				CreatedAt: time.Date(2026, 5, 7, 15, 0, 0, 0, time.Local),
				Resolved:  true,
			}},
			wantHas: []string{
				"<!-- decision: 2026-05-07 15:00, category:architecture, resolved -->",
			},
		},
		{
			name: "decision with HTML markers in text",
			decisions: []*models.Decision{{
				Text:      "Use <!-- inject --> safely",
				Rationale: "Prevents XSS",
				Category:  models.DecisionCategorySecurity,
				CreatedAt: time.Date(2026, 5, 7, 16, 0, 0, 0, time.Local),
			}},
			wantHas: []string{
				"&lt;!--",
				"--&gt;",
			},
		},
		{
			name: "trade-off category",
			decisions: []*models.Decision{{
				Text:      "Speed vs correctness",
				Rationale: "Chosen speed",
				Category:  models.DecisionCategoryTradeOff,
				CreatedAt: time.Date(2026, 5, 7, 10, 0, 0, 0, time.Local),
			}},
			wantHas: []string{
				"<!-- decision: 2026-05-07 10:00, category:trade-off -->",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &models.Task{Decisions: tt.decisions}
			got := WriteDecisions(task)

			if tt.wantEmpty {
				if got != "" {
					t.Errorf("WriteDecisions() = %q, want empty string", got)
				}
				return
			}

			for _, want := range tt.wantHas {
				if !strings.Contains(got, want) {
					t.Errorf("WriteDecisions() missing %q\ngot:\n%s", want, got)
				}
			}
		})
	}
}
