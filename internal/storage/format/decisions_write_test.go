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
			name:      "only nil decisions",
			decisions: []*models.Decision{nil},
			wantEmpty: true,
		},
		{
			name: "single decision",
			decisions: []*models.Decision{{
				Text:      "Use PostgreSQL",
				Rationale: "Better for complex queries",
				CreatedAt: time.Date(2026, 5, 7, 14, 30, 0, 0, time.Local),
			}},
			wantHas: []string{
				"<!-- DECISIONS -->",
				"<!-- decision: 2026-05-07 14:30 -->",
				"Use PostgreSQL",
				"<!-- rationale -->",
				"Better for complex queries",
				"<!-- /DECISIONS -->",
			},
		},
		{
			name: "decision without rationale",
			decisions: []*models.Decision{{
				Text:      "Chosen approach",
				CreatedAt: time.Date(2026, 5, 7, 15, 0, 0, 0, time.Local),
			}},
			wantHas: []string{
				"<!-- decision: 2026-05-07 15:00 -->",
			},
		},
		{
			name: "decision with HTML markers in text",
			decisions: []*models.Decision{{
				Text:      "Use <!-- inject --> safely",
				Rationale: "Prevents XSS",
				CreatedAt: time.Date(2026, 5, 7, 16, 0, 0, 0, time.Local),
			}},
			wantHas: []string{
				"&lt;!--",
				"--&gt;",
			},
		},
		{
			name: "another timestamp",
			decisions: []*models.Decision{{
				Text:      "Speed vs correctness",
				Rationale: "Chosen speed",
				CreatedAt: time.Date(2026, 5, 7, 10, 0, 0, 0, time.Local),
			}},
			wantHas: []string{
				"<!-- decision: 2026-05-07 10:00 -->",
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
