package outliner

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/theme"
)

func TestPeekKeepsPassageThroughResizeAndEdits(t *testing.T) {
	defer theme.ApplyTheme(theme.ThemeConfig{})
	for _, palette := range []string{"dark", "light"} {
		for _, mutation := range []string{"resize", "append", "prepend", "other-section", "palette"} {
			t.Run(palette+"/"+mutation, func(t *testing.T) {
				theme.ApplyTheme(theme.ThemeConfig{Name: palette})
				task := pagedPeekTask()
				var paragraphs []string
				for i := range 20 {
					paragraphs = append(paragraphs, fmt.Sprintf("Passage %02d: 確認 équipe — keep this sentence visible while the pane changes.", i))
				}
				task.Description = strings.Join(paragraphs, "\n\n")
				m := pagedPeekOutliner(t, task, 80, 24)
				task = m.Rows[m.Cursor].Task
				doc := cursorPeekDocument(t, m)
				target := -1
				for i, line := range doc.Body {
					if strings.Contains(ansi.Strip(line), "Passage 08:") {
						target = i
						break
					}
				}
				if target < 0 || !m.scrollPeek(target) {
					t.Fatal("fixture did not reach passage")
				}
				cursor := m.Cursor
				switch mutation {
				case "resize":
					m.SetSize(60, 20)
				case "append":
					task.Description += "\n\nAn appended paragraph."
				case "prepend":
					task.Description = "A newly inserted paragraph.\n\n" + task.Description
				case "other-section":
					task.ResumeNote = strings.Repeat("Longer restart context. ", 8)
				case "palette":
					theme.ApplyTheme(theme.ThemeConfig{Name: "light"})
				}
				m.View()
				doc = cursorPeekDocument(t, m)
				visible := ansi.Strip(strings.Join(doc.Lines(peekOffset(t, m)), "\n"))
				if !strings.Contains(visible, "Passage 08:") {
					t.Fatalf("lost passage:\n%s", visible)
				}
				if m.Cursor != cursor {
					t.Fatal("reading moved the task cursor")
				}
				if mutation == "resize" {
					m.SetSize(80, 24)
					m.View()
					if peekOffset(t, m) != target {
						t.Fatal("resize round trip lost original position")
					}
				}
			})
		}
	}
}

func TestFeedbackExpiryDoesNotUndoNavigation(t *testing.T) {
	for _, change := range []string{"key", "resize", "rebuild"} {
		t.Run(change, func(t *testing.T) {
			m := dockFixtureOutliner(t, 60, 20)
			m.ViewScrollOffset = 4
			m.SetShellFrameMetrics(chrome.ShellFrameMetrics{FeedbackRows: 1, ControlRows: 1})
			switch change {
			case "key":
				m.HandleKeyMsg(tea.KeyPressMsg{Code: 'j', Text: "j"})
			case "resize":
				m.SetSize(80, 24)
			case "rebuild":
				m.Population.Invalidate()
				m.CrossProjectRebuildRows()
			}
			m.ViewScrollOffset = 7
			m.SetShellFrameMetrics(chrome.ShellFrameMetrics{ControlRows: 1})
			if m.ViewScrollOffset != 7 {
				t.Fatal("feedback expiry replaced the new window")
			}
		})
	}
}

func TestPeekRemovedPassageFallsBackAndClamps(t *testing.T) {
	task := pagedPeekTask()
	task.Description = strings.Repeat("A long paragraph to read. ", 100)
	m := pagedPeekOutliner(t, task, 60, 20)
	task = m.Rows[m.Cursor].Task
	if !m.scrollPeek(15) {
		t.Fatal("fixture did not scroll")
	}
	task.Description = "Replacement."
	doc := cursorPeekDocument(t, m)
	if offset := peekOffset(t, m); offset < 0 || offset > doc.MaxOffset() {
		t.Fatal("replacement left an invalid offset")
	}
	task.Description = ""
	if peekOffset(t, m) != 0 {
		t.Fatal("removed section retained a stale anchor")
	}
}
