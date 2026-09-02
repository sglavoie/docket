package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/presentation"
)

func TestRenderAuditNoOpOutsideAudit(t *testing.T) {
	if activeRenderAudit != nil {
		t.Fatalf("activeRenderAudit should be nil at test start, got %+v", activeRenderAudit)
	}
	recordRequiredTextTruncation("orphan site", "the full required text", "the ful…")
	if activeRenderAudit != nil {
		t.Fatalf("recordRequiredTextTruncation must not create an audit outside withRenderAudit, got %+v", activeRenderAudit)
	}
}

func TestRenderAuditRecordsInsideAudit(t *testing.T) {
	out, issues := withRenderAudit(func() string {
		recordRequiredTextTruncation("footer left", "keys: quit save filter help more", "keys: quit save…")
		return "rendered screen"
	})

	if out != "rendered screen" {
		t.Errorf("withRenderAudit output = %q, want %q", out, "rendered screen")
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 audit issue, got %d: %+v", len(issues), issues)
	}
	issue := issues[0]
	if issue.Kind != requiredTextTruncatedKind {
		t.Errorf("issue.Kind = %q, want %q", issue.Kind, requiredTextTruncatedKind)
	}
	if issue.Line != 0 {
		t.Errorf("issue.Line = %d, want 0 (screen-wide)", issue.Line)
	}
	if !strings.Contains(issue.Message, "footer left") {
		t.Errorf("issue.Message = %q, want it to name the render site", issue.Message)
	}
	if activeRenderAudit != nil {
		t.Errorf("activeRenderAudit should be nil after withRenderAudit, got %+v", activeRenderAudit)
	}
}

func TestRenderAuditSnippetStripsAndClamps(t *testing.T) {
	if got := renderAuditSnippet("\x1b[31mred\x1b[0m"); got != "red" {
		t.Errorf("renderAuditSnippet stripped = %q, want %q", got, "red")
	}
	if got := renderAuditSnippet("line one\nline two"); got != "line one line two" {
		t.Errorf("renderAuditSnippet newline collapse = %q, want %q", got, "line one line two")
	}
	long := strings.Repeat("x", renderAuditSnippetMax+10)
	got := renderAuditSnippet(long)
	gotRunes := []rune(got)
	if len(gotRunes) != renderAuditSnippetMax+1 {
		t.Errorf("renderAuditSnippet clamp length = %d runes, want %d", len(gotRunes), renderAuditSnippetMax+1)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("renderAuditSnippet clamped = %q, want trailing ellipsis", got)
	}
}

type recordingAuditView struct {
	width, height int
}

func (v *recordingAuditView) Init() tea.Cmd { return nil }

func (v *recordingAuditView) Update(tea.Msg) (tea.Model, tea.Cmd) { return v, nil }

func (v *recordingAuditView) View() tea.View {
	recordRequiredTextTruncation("recording view", "the full required chrome text", "the full…")
	return tea.NewView("recording view content")
}

func (v *recordingAuditView) SetSize(width, height int) {
	v.width, v.height = width, height
}

func TestRenderAuditObserveIncludesAuditIssues(t *testing.T) {
	app := newUITestApp()
	app.views.Register(presentation.ViewCrossProjectOutliner, &recordingAuditView{})
	driver := NewUITestDriver(app, 100, 30)
	obs := driver.Observe(ObserveOptions{})
	if !hasIssueKind(obs.LayoutIssues, requiredTextTruncatedKind) {
		t.Fatalf("expected Observe to include a %q issue, got %+v", requiredTextTruncatedKind, obs.LayoutIssues)
	}
}
