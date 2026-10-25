package lens

import (
	"strings"
	"testing"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/presentation"
)

func TestLensPickerNoticeWrapsWithinBudget(t *testing.T) {
	for _, notice := range []string{
		`Cannot apply lens "Planning for the next release": its project has been archived. Unarchive the project first.`,
		strings.Repeat("界", 75),
		strings.Repeat("A very long refusal with more detail. ", 100),
	} {
		t.Run(notice[:min(len(notice), 12)], func(t *testing.T) {
			model, err := NewNamedLensPickerModel(operation.NewOperationExecutor(operation.NewOperationRegistry()),
				presentation.ViewCrossProjectOutliner, NamedLensPickerSnapshot{Lenses: config.StarterNamedLenses()})
			if err != nil {
				t.Fatal(err)
			}
			model.notice = notice
			model.Searching = true
			rendered := ansi.Strip(model.RenderWithin(chrome.MinTerminalWidth, chrome.MinTerminalHeight))
			if !utf8.ValidString(rendered) || lipgloss.Width(rendered) > chrome.MinTerminalWidth || lipgloss.Height(rendered) > chrome.MinTerminalHeight {
				t.Fatalf("notice exceeds terminal or breaks Unicode:\n%s", rendered)
			}
			for _, control := range []string{"Search:", "Enter apply search", "Esc cancel search", "Save Current View"} {
				if !strings.Contains(rendered, control) {
					t.Fatalf("missing %q:\n%s", control, rendered)
				}
			}
			if len(notice) < 500 {
				// Strip framing and whitespace so word wrapping is immaterial.
				normalize := func(s string) string {
					return strings.Join(strings.Fields(strings.ReplaceAll(s, "│", "")), "")
				}
				if !strings.Contains(normalize(rendered), normalize(notice)) {
					t.Fatalf("notice was cut off:\n%s", rendered)
				}
			}
		})
	}
}
