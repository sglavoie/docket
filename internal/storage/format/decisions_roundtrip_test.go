package format

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

func TestWriteDecisions_RoundTrip(t *testing.T) {
	decisions := []*models.Decision{
		{
			Text:      "Use microservices",
			Rationale: "Independent deployments",
			CreatedAt: time.Date(2026, 5, 7, 14, 30, 0, 0, time.Local),
		},
		{
			Text:      "Multi-line\ndecision text",
			Rationale: "Multi-line\nrationale text",
			CreatedAt: time.Date(2026, 5, 7, 15, 0, 0, 0, time.Local),
		},
		{
			Text:      "Decision with <!-- and --> markers",
			Rationale: "Rationale with <!-- markers -->",
			CreatedAt: time.Date(2026, 5, 7, 16, 0, 0, 0, time.Local),
		},
	}

	task := &models.Task{
		Description: "Task description",
		Decisions:   decisions,
	}

	block := WriteDecisions(task)

	task2 := &models.Task{
		Description: "Task description\n\n" + block,
	}

	ExtractDecisionsFromDescription(task2)

	if len(task2.Decisions) != len(decisions) {
		t.Fatalf("got %d decisions, want %d", len(task2.Decisions), len(decisions))
	}

	for i, got := range task2.Decisions {
		want := decisions[i]
		if got.Text != want.Text {
			t.Errorf("Decision[%d].Text = %q, want %q", i, got.Text, want.Text)
		}
		if got.Rationale != want.Rationale {
			t.Errorf("Decision[%d].Rationale = %q, want %q", i, got.Rationale, want.Rationale)
		}
		gotTime := got.CreatedAt.Format(util.DateTimeFormat)
		wantTime := want.CreatedAt.Format(util.DateTimeFormat)
		if gotTime != wantTime {
			t.Errorf("Decision[%d].CreatedAt = %s, want %s", i, gotTime, wantTime)
		}
	}

	if !strings.Contains(task2.Description, "Task description") {
		t.Errorf("Description missing original content: %q", task2.Description)
	}
	if strings.Contains(task2.Description, "<!-- DECISIONS -->") {
		t.Errorf("Description still contains decisions block")
	}
}

func TestWriteDecisions_AmpersandRoundTrip(t *testing.T) {
	decisions := []*models.Decision{
		{
			Text:      "Use &lt;!-- for HTML comments",
			Rationale: "Tom & Jerry approach",
			CreatedAt: time.Date(2026, 5, 7, 10, 0, 0, 0, time.Local),
		},
		{
			Text:      "Code: <!-- inject --> end",
			Rationale: "Sanitize all inputs",
			CreatedAt: time.Date(2026, 5, 7, 11, 0, 0, 0, time.Local),
		},
	}

	task := &models.Task{
		Description: "Task description",
		Decisions:   decisions,
	}

	block := WriteDecisions(task)
	task2 := &models.Task{Description: "Task description\n\n" + block}
	ExtractDecisionsFromDescription(task2)

	if len(task2.Decisions) != len(decisions) {
		t.Fatalf("got %d decisions, want %d", len(task2.Decisions), len(decisions))
	}

	for i, got := range task2.Decisions {
		want := decisions[i]
		if got.Text != want.Text {
			t.Errorf("Decision[%d].Text = %q, want %q", i, got.Text, want.Text)
		}
		if got.Rationale != want.Rationale {
			t.Errorf("Decision[%d].Rationale = %q, want %q", i, got.Rationale, want.Rationale)
		}
	}
}

func TestWriteDecisions_HeadingLikeLinesRoundTrip(t *testing.T) {
	decisions := []*models.Decision{
		{
			Text:      "Choose parser, status: todo\n## Task: pseudo heading",
			Rationale: "Because # Project: embedded should stay text\n\\## Task: literal backslash",
			CreatedAt: time.Date(2026, 5, 7, 12, 0, 0, 0, time.Local),
		},
	}

	task := &models.Task{Decisions: decisions}
	block := WriteDecisions(task)

	task2 := &models.Task{Description: block}
	ExtractDecisionsFromDescription(task2)

	if len(task2.Decisions) != 1 {
		t.Fatalf("got %d decisions, want 1", len(task2.Decisions))
	}
	if got := task2.Decisions[0].Text; got != decisions[0].Text {
		t.Errorf("Decision.Text = %q, want %q", got, decisions[0].Text)
	}
	if got := task2.Decisions[0].Rationale; got != decisions[0].Rationale {
		t.Errorf("Decision.Rationale = %q, want %q", got, decisions[0].Rationale)
	}
	if rewritten := WriteDecisions(task2); rewritten != block {
		t.Fatalf("decisions block did not reach fixed point:\n--- block ---\n%s\n--- rewritten ---\n%s", block, rewritten)
	}
}

func TestWriteDecisions_PreservesOuterWhitespaceRoundTrip(t *testing.T) {
	decisions := []*models.Decision{
		{
			Text:      "    keep leading indentation\ntrailing spaces stay  \n",
			Rationale: "  rationale starts indented\n\nends after blank line\n",
			CreatedAt: time.Date(2026, 5, 7, 13, 0, 0, 0, time.Local),
		},
	}

	task := &models.Task{Decisions: decisions}
	block := WriteDecisions(task)

	task2 := &models.Task{Description: block}
	ExtractDecisionsFromDescription(task2)

	if len(task2.Decisions) != 1 {
		t.Fatalf("got %d decisions, want 1", len(task2.Decisions))
	}
	if got := task2.Decisions[0].Text; got != decisions[0].Text {
		t.Errorf("Decision.Text = %q, want %q", got, decisions[0].Text)
	}
	if got := task2.Decisions[0].Rationale; got != decisions[0].Rationale {
		t.Errorf("Decision.Rationale = %q, want %q", got, decisions[0].Rationale)
	}
}
