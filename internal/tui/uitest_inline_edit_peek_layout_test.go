package tui

import "testing"

// TestUITestScenarioInlineEditUnderPeekFitsTheFrame holds the inline row
// editor, its tag completions, and a wrapping draft inside the frame while the
// Peek dock stands open, down to the 60x20 minimum: the completions cover list
// rows rather than adding any.
func TestUITestScenarioInlineEditUnderPeekFitsTheFrame(t *testing.T) {
	for _, size := range []string{"60x20", "72x20", "100x22", "120x24"} {
		for _, draft := range []string{" x", " #", " a draft long enough to wrap the inline editor over several rows #"} {
			t.Run(size+draft, func(t *testing.T) {
				sc := UITestScenario{
					Name:    "inline edit under Peek",
					Fixture: "rich",
					Size:    size,
					Actions: []UITestAction{
						{Type: UITestActionKey, Key: "b"},
						{Type: UITestActionKey, Key: "e"},
						{Type: UITestActionAssert, Assert: "contains", Value: "EDIT TASK"},
						{Type: UITestActionText, Text: draft},
						{Type: UITestActionAssert, Assert: "no_layout_issues"},
					},
				}
				result, ws, err := RunScenarioFixture(sc, RunScenarioOptions{})
				if err != nil {
					t.Fatalf("RunScenarioFixture error: %v", err)
				}
				t.Cleanup(func() { _ = ws.Cleanup() })
				if !result.OK {
					t.Fatalf("inline edit under Peek at %s: %+v", size, result.Error)
				}
			})
		}
	}
}
