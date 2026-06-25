package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

// TestKanbanDenseCardOmitsSecondaryMetadata verifies the compact card hierarchy:
// status/type/title only, with checklist, description, and GitHub issue
// indicators surfaced in the detail pane instead of crowding the card row.
func TestKanbanDenseCardOmitsSecondaryMetadata(t *testing.T) {
	project := testutil.NewTestProject("p")
	task := testutil.NewTestTask("Rich card with secondary metadata", models.TaskTypeTask)
	task.Description = "A descriptive note that the detail pane carries."
	task.Checklist = []models.ChecklistItem{
		{Text: "one", Done: true},
		{Text: "two", Done: false},
	}
	task.Attachments = []models.Attachment{
		{Filename: "https://github.com/acme/roadmap/issues/123", IsURL: true},
	}

	col := KanbanColumn{
		ID:    "inbox",
		Label: "inbox",
		Icon:  "·",
		Tasks: []KanbanCard{{Task: task, Project: project}},
	}
	m := newRenderKanban(80, 24, []KanbanColumn{col})
	got := ansi.Strip(kanbanRender(m))

	// The card row contains the title; isolate it and assert the secondary
	// indicators are absent from that line specifically.
	var cardLine string
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, task.Title) {
			cardLine = line
			break
		}
	}
	if cardLine == "" {
		t.Fatalf("expected to find card row containing title %q in:\n%s", task.Title, got)
	}

	if strings.Contains(cardLine, ansi.Strip(renderChecklistIndicatorCompact(task))) {
		t.Fatalf("dense card row must not carry compact checklist indicator inline, got %q", cardLine)
	}
	if strings.Contains(cardLine, IconDescription()) {
		t.Fatalf("dense card row must not carry description glyph inline, got %q", cardLine)
	}
	if strings.Contains(cardLine, "GH acme/roadmap#123") {
		t.Fatalf("dense card row must not carry GitHub issue indicator inline, got %q", cardLine)
	}
}

// TestKanbanDenseCardHeroDemotion guards the shared hero policy on cards: when
// a task has an urgency hero (e.g., overdue), the non-hero badges (status, type)
// are rendered through demoteBadge so only the hero reads at full saturation.
func TestKanbanDenseCardHeroDemotion(t *testing.T) {
	yesterday := time.Now().AddDate(0, 0, -1)

	project := testutil.NewTestProject("p")
	// Use a non-cursor anchor first so the focused-row styling does not mask
	// the demoted badge color on the overdue card under test.
	anchor := testutil.NewTestTask("Anchor card", models.TaskTypeTask)
	anchor.Status = models.StatusTodo

	overdue := testutil.NewTestTask("Overdue card", models.TaskTypeTask)
	overdue.Status = models.StatusTodo
	overdue.DueDate = &yesterday

	plain := testutil.NewTestTask("Plain card", models.TaskTypeTask)
	plain.Status = models.StatusTodo

	col := KanbanColumn{
		ID:    "inbox",
		Label: "inbox",
		Icon:  "·",
		Tasks: []KanbanCard{
			{Task: anchor, Project: project},
			{Task: overdue, Project: project},
			{Task: plain, Project: project},
		},
	}
	m := newRenderKanban(80, 24, []KanbanColumn{col})
	rendered := kanbanRender(m)

	rawStatusBadge := GetStatusBadge(string(plain.Status), plain.Status.Icon())
	rawTypeBadge := GetTypeBadge(string(plain.Type), plain.Type.Icon())
	demotedStatus := demoteBadge(GetStatusBadge(string(overdue.Status), overdue.Status.Icon()))
	demotedType := demoteBadge(GetTypeBadge(string(overdue.Type), overdue.Type.Icon()))

	// Overdue card line: badges must appear demoted.
	var overdueLine, plainLine string
	for _, line := range strings.Split(rendered, "\n") {
		switch {
		case strings.Contains(ansi.Strip(line), overdue.Title):
			overdueLine = line
		case strings.Contains(ansi.Strip(line), plain.Title):
			plainLine = line
		}
	}
	if overdueLine == "" || plainLine == "" {
		t.Fatalf("expected to find both card rows; overdue=%q plain=%q in:\n%s",
			overdueLine, plainLine, ansi.Strip(rendered))
	}

	if !strings.Contains(overdueLine, demotedStatus) {
		t.Fatalf("overdue card should render demoted status badge; want substring %q in line %q",
			demotedStatus, overdueLine)
	}
	if !strings.Contains(overdueLine, demotedType) {
		t.Fatalf("overdue card should render demoted type badge; want substring %q in line %q",
			demotedType, overdueLine)
	}
	// Plain card has no hero — badges keep full saturation styling.
	if !strings.Contains(plainLine, rawStatusBadge) {
		t.Fatalf("plain card should keep undemoted status badge; want substring %q in line %q",
			rawStatusBadge, plainLine)
	}
	if !strings.Contains(plainLine, rawTypeBadge) {
		t.Fatalf("plain card should keep undemoted type badge; want substring %q in line %q",
			rawTypeBadge, plainLine)
	}
}

// TestKanbanDetailPaneSurfacesMovedFields ensures the detail pane carries the
// indicators that were dropped from the compact card row. This is the kanban
// analogue of the outliner / project-tree / focus-board metadata strip.
func TestKanbanDetailPaneSurfacesMovedFields(t *testing.T) {
	project := testutil.NewTestProject("p")
	task := testutil.NewTestTask("Detail pane carries moved fields", models.TaskTypeTask)
	task.Description = "Detail text the pane must surface."
	task.Checklist = []models.ChecklistItem{
		{Text: "one", Done: true},
		{Text: "two", Done: false},
		{Text: "three", Done: false},
	}
	task.Attachments = []models.Attachment{
		{Filename: "https://github.com/acme/roadmap/issues/456", IsURL: true},
	}

	col := KanbanColumn{
		ID:    "inbox",
		Label: "inbox",
		Icon:  "·",
		Tasks: []KanbanCard{{Task: task, Project: project}},
	}
	m := newRenderKanban(96, 24, []KanbanColumn{col})
	m.detailPaneVisible = true

	plain := strings.Join(strippedDetailPaneLines(t, m), "\n")

	if !strings.Contains(plain, ansi.Strip(renderChecklistIndicator(task))) {
		t.Fatalf("detail pane should surface checklist indicator, got:\n%s", plain)
	}
	if !strings.Contains(plain, IconDescription()) {
		t.Fatalf("detail pane should surface description glyph, got:\n%s", plain)
	}
	if !strings.Contains(plain, "GH acme/roadmap#456") {
		t.Fatalf("detail pane should surface GitHub issue indicator, got:\n%s", plain)
	}
}

// TestKanbanDenseCardWidthStableAcrossSecondaryMetadata locks in the dense
// hierarchy's stability promise on kanban cards: varying secondary metadata
// across cards must not change rendered card widths.
func TestKanbanDenseCardWidthStableAcrossSecondaryMetadata(t *testing.T) {
	project := testutil.NewTestProject("p")
	titles := []string{
		"Sibling task A",
		"Sibling task B",
		"Sibling task C",
		"Sibling task D",
	}
	tasks := make([]*models.Task, 0, len(titles))
	for _, title := range titles {
		t := testutil.NewTestTask(title, models.TaskTypeTask)
		tasks = append(tasks, t)
	}
	tasks[1].Description = "A long enough description that would have pressured the row width."
	tasks[2].Checklist = []models.ChecklistItem{
		{Text: "one", Done: true},
		{Text: "two", Done: false},
	}
	tasks[3].Attachments = []models.Attachment{
		{Filename: "https://github.com/acme/roadmap/issues/789", IsURL: true},
	}

	col := KanbanColumn{ID: "inbox", Label: "inbox", Icon: "·"}
	for _, task := range tasks {
		col.Tasks = append(col.Tasks, KanbanCard{Task: task, Project: project})
	}

	m := newRenderKanban(80, 24, []KanbanColumn{col})
	rendered := kanbanRender(m)

	widths := make(map[int]int)
	for _, line := range strings.Split(rendered, "\n") {
		plain := ansi.Strip(line)
		for _, title := range titles {
			if strings.Contains(plain, title) {
				widths[lipgloss.Width(plain)]++
				break
			}
		}
	}
	if len(widths) == 0 {
		t.Fatalf("expected at least one card row, got no matches in:\n%s", ansi.Strip(rendered))
	}
	if len(widths) != 1 {
		t.Fatalf("expected uniform dense-card widths across varied secondary metadata, got %v", widths)
	}
}

// TestKanbanDenseCardNarrowColumnRendersWithoutOverflow guards narrow-column
// behavior: a card in a minimum-width column still renders inside the column
// budget and does not overflow because the secondary indicators are gone.
func TestKanbanDenseCardNarrowColumnRendersWithoutOverflow(t *testing.T) {
	project := testutil.NewTestProject("p")
	task := testutil.NewTestTask("Title that would have triggered overflow with extras", models.TaskTypeTask)
	task.Description = "Has description."
	task.Checklist = []models.ChecklistItem{
		{Text: "a", Done: true},
		{Text: "b", Done: false},
	}
	task.Attachments = []models.Attachment{
		{Filename: "https://github.com/acme/roadmap/issues/321", IsURL: true},
	}

	col := KanbanColumn{
		ID:    "inbox",
		Label: "inbox",
		Icon:  "·",
		Tasks: []KanbanCard{{Task: task, Project: project}},
	}

	// width=36 → maxVisible = (36+1)/26 = 1 → narrow single-column mode.
	// Mirrors TestKanbanGitHubIssueIndicatorDoesNotOverflowCard.
	m := newRenderKanban(36, 12, []KanbanColumn{col})
	rendered := kanbanRender(m)

	// The card row contains the title; verify it specifically fits the column
	// budget after the secondary indicators were dropped.
	plain := ansi.Strip(rendered)
	var cardLine string
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, "Title that would have") {
			cardLine = line
			break
		}
	}
	if cardLine == "" {
		t.Fatalf("expected to find card row containing title in:\n%s", plain)
	}
	if got := lipgloss.Width(cardLine); got > m.width {
		t.Fatalf("narrow card row exceeds width %d: width=%d, content=%q", m.width, got, cardLine)
	}
}
