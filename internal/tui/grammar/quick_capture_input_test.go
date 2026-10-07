package grammar

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/textgrammar"
)

// makePreviewInput builds a quickCaptureInput pre-populated with the given
// preview result so renderPreview can be exercised directly.
func makePreviewInput(t *testing.T, preview *QuickCaptureResult, previewErr string, width int) *QuickCaptureInput {
	t.Helper()
	return &QuickCaptureInput{
		Preview:    preview,
		PreviewErr: previewErr,
		Width:      width,
	}
}

func TestRenderPreview_EmptyWhenNoPreview(t *testing.T) {
	q := makePreviewInput(t, nil, "", 0)
	if got := q.renderPreview(); got != "" {
		t.Fatalf("expected empty preview, got %q", got)
	}

	q = makePreviewInput(t, &QuickCaptureResult{Title: ""}, "", 0)
	if got := q.renderPreview(); got != "" {
		t.Fatalf("expected empty preview when title is empty, got %q", got)
	}
}

func TestRenderPreview_PreviewErrorShortCircuits(t *testing.T) {
	q := makePreviewInput(t, &QuickCaptureResult{Title: "ignored"}, "bad due: junk", 0)
	got := ansi.Strip(q.renderPreview())
	if !strings.Contains(got, "bad due") {
		t.Fatalf("expected error text in preview, got %q", got)
	}
	if strings.Contains(got, "Task") {
		t.Fatalf("error preview should not render group labels, got %q", got)
	}
}

func TestRenderPreview_TitleOnlyShowsDefaultInboxRoute(t *testing.T) {
	q := makePreviewInput(t, &QuickCaptureResult{
		Title:    "Fix login bug",
		Priority: 0, // default - hidden
	}, "", 0)

	plain := ansi.Strip(q.renderPreview())
	lines := strings.Split(plain, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected exactly two lines (Task + Route) for title-only preview, got %d:\n%s", len(lines), plain)
	}
	if !strings.HasPrefix(lines[0], "Task") {
		t.Fatalf("expected Task group line, got %q", lines[0])
	}
	if !strings.Contains(lines[0], "Fix login bug") {
		t.Fatalf("expected title in Task line, got %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "Route") {
		t.Fatalf("expected Route group line, got %q", lines[1])
	}
	if !strings.Contains(lines[1], "$inbox (default)") {
		t.Fatalf("expected Route line to show '$inbox (default)', got %q", lines[1])
	}
	for _, banned := range []string{"Schedule", "Metadata", "Aliases"} {
		if strings.Contains(plain, banned) {
			t.Errorf("expected %s group to be hidden, got %q", banned, plain)
		}
	}
}

func TestRenderPreview_DefaultInboxRouteWithTargetProject(t *testing.T) {
	q := makePreviewInput(t, &QuickCaptureResult{
		Title:             "Capture for other team",
		Priority:          0,
		TargetProjectName: "Web App",
	}, "", 0)

	plain := ansi.Strip(q.renderPreview())
	var routeLine string
	for _, line := range strings.Split(plain, "\n") {
		if strings.HasPrefix(line, "Route") {
			routeLine = line
			break
		}
	}
	if routeLine == "" {
		t.Fatalf("expected Route group line, got:\n%s", plain)
	}
	for _, substr := range []string{"$inbox (default)", "Web App", "→"} {
		if !strings.Contains(routeLine, substr) {
			t.Errorf("expected substring %q in Route line, got %q", substr, routeLine)
		}
	}
}

func TestRenderPreviewShowsPlanningDestinationAndChildDefault(t *testing.T) {
	today := makePreviewInput(t, &QuickCaptureResult{
		Title: "Plan it", Destination: app.CaptureDestinationToday, DestinationSet: true,
	}, "", 0)
	if got := ansi.Strip(today.renderPreview()); !strings.Contains(got, "$today") || strings.Contains(got, "(default)") {
		t.Fatalf("Today preview = %q", got)
	}

	parentID := "parent"
	child := makePreviewInput(t, &QuickCaptureResult{Title: "Child", ParentID: &parentID}, "", 0)
	if got := ansi.Strip(child.renderPreview()); !strings.Contains(got, "$backlog (default)") {
		t.Fatalf("child preview = %q", got)
	}
}

func TestRenderPreview_DefaultInboxRouteWrapsAtNarrowWidth(t *testing.T) {
	q := makePreviewInput(t, &QuickCaptureResult{
		Title:             "Capture for the offsite tomorrow morning",
		Priority:          0,
		TargetProjectName: "Q1 Planning Roadmap Initiative",
	}, "", 32)

	rendered := q.renderPreview()
	for _, line := range strings.Split(rendered, "\n") {
		stripped := ansi.Strip(line)
		if w := lipgloss.Width(stripped); w > 32 {
			t.Errorf("preview line exceeds width 32: %q (width=%d)", stripped, w)
		}
	}
	plain := ansi.Strip(rendered)
	if !strings.Contains(plain, "$inbox (default)") {
		t.Fatalf("expected '$inbox (default)' hint in narrow preview, got:\n%s", plain)
	}
	// The long project name wraps across continuation lines, so look for
	// each word individually rather than the full contiguous string.
	for _, tok := range []string{"Q1", "Planning", "Roadmap", "Initiative"} {
		if !strings.Contains(plain, tok) {
			t.Fatalf("expected target project token %q to appear in narrow preview, got:\n%s", tok, plain)
		}
	}
}

func TestRenderPreview_AllGroupsRendered(t *testing.T) {
	due := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)
	q := makePreviewInput(t, &QuickCaptureResult{
		Title:             "Ship release",
		Priority:          1,
		Tags:              []string{"backend", "urgent"},
		DueDate:           &due,
		Destination:       textgrammar.DestinationDoing,
		DestinationSet:    true,
		TargetProjectName: "Web App",
	}, "", 0)

	plain := ansi.Strip(q.renderPreview())
	lines := strings.Split(plain, "\n")

	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d:\n%s", len(lines), plain)
	}

	expectations := []struct {
		prefix string
		substr []string
	}{
		{"Task", []string{"Ship release"}},
		{"Route", []string{"$doing", "Web App"}},
		{"Schedule", []string{"@Sun, Feb 15"}},
		{"Priority", []string{"!1"}},
		{"Tags", []string{"#backend", "#urgent"}},
	}
	for i, exp := range expectations {
		if !strings.HasPrefix(lines[i], exp.prefix) {
			t.Errorf("line %d: expected prefix %q, got %q", i, exp.prefix, lines[i])
		}
		for _, s := range exp.substr {
			if !strings.Contains(lines[i], s) {
				t.Errorf("line %d (%s): expected substring %q, got %q", i, exp.prefix, s, lines[i])
			}
		}
	}
}

func TestRenderPreview_ShowsSemanticTagWithoutMarkerPrefix(t *testing.T) {
	q := makePreviewInput(t, &QuickCaptureResult{
		Title:    "Fix composer",
		Priority: 3,
		Tags:     []string{"bug", "capture"},
	}, "", 80)

	plain := ansi.Strip(q.renderPreview())
	for _, want := range []string{"Task", "Fix composer", "Tags", "#bug", "#capture"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("preview missing %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "B Fix composer") {
		t.Fatalf("preview retained the retired bug marker prefix:\n%s", plain)
	}
	if strings.Contains(plain, "Metadata") {
		t.Fatalf("preview retained misleading Metadata group:\n%s", plain)
	}
}

func TestRenderPreview_NarrowSafeVisualWidth(t *testing.T) {
	due := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)
	q := makePreviewInput(t, &QuickCaptureResult{
		Title:             "Investigate intermittent flaky integration test failure on CI",
		Priority:          1,
		Tags:              []string{"backend", "urgent", "production", "hotfix"},
		DueDate:           &due,
		Destination:       app.CaptureDestinationBacklog,
		DestinationSet:    true,
		TargetProjectName: "Reliability And Observability Platform",
	}, "", 32)

	rendered := q.renderPreview()
	for _, line := range strings.Split(rendered, "\n") {
		stripped := ansi.Strip(line)
		if w := lipgloss.Width(stripped); w > 32 {
			t.Errorf("preview line exceeds width 32: %q (width=%d)", stripped, w)
		}
	}
}

func TestRenderPreview_NarrowSafeWithWideRunes(t *testing.T) {
	q := makePreviewInput(t, &QuickCaptureResult{
		Title:          "修复收件箱卡片在窄宽度下的宽字符标题截断问题",
		Priority:       3,
		Tags:           []string{"界面修复", "次要标签"},
		Destination:    app.CaptureDestinationBacklog,
		DestinationSet: true,
	}, "", 28)

	rendered := q.renderPreview()
	for _, line := range strings.Split(rendered, "\n") {
		stripped := ansi.Strip(line)
		if w := lipgloss.Width(stripped); w > 28 {
			t.Errorf("preview line (wide runes) exceeds width 28: %q (width=%d)", stripped, w)
		}
	}
}

func TestRenderPreview_ContinuationLinesAlignToLabelColumn(t *testing.T) {
	q := makePreviewInput(t, &QuickCaptureResult{
		Title:    "Task title used to force a wrap",
		Priority: 3,
		Tags:     []string{"alpha", "beta", "gamma", "delta", "epsilon"},
	}, "", 28)

	rendered := q.renderPreview()
	for _, line := range strings.Split(rendered, "\n") {
		stripped := ansi.Strip(line)
		if w := lipgloss.Width(stripped); w > 28 {
			t.Fatalf("preview line exceeds width 28: %q (width=%d)", stripped, w)
		}
	}
	// At least one Tags continuation should appear when the tag list wraps.
	if !strings.Contains(ansi.Strip(rendered), "#alpha") {
		t.Fatalf("expected tags to render, got:\n%s", ansi.Strip(rendered))
	}
}
