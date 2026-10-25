package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/help"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/presentation"
)

func TestHelpFooterStaysOnLastTerminalRow(t *testing.T) {
	for _, size := range [][2]int{{60, 20}, {80, 24}, {100, 35}, {120, 40}, {60, 20}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			model := newTestAppModel()
			model.viewNav.resize(size[0], size[1])
			view := help.NewHelpViewModel([]operation.HelpItem{{Key: "a", Description: "Capture", Category: "Tasks"}}, "Outliner")
			model.showView(presentation.ViewHelp, view)
			for _, feedback := range []string{"", "Saved", ""} {
				model.setInfoStatus(feedback)
				content := ansi.Strip(model.View().Content)
				if got := lipgloss.Height(content); got != size[1] {
					t.Fatalf("height=%d, want %d\n%s", got, size[1], content)
				}
				rows := strings.Split(content, "\n")
				if !strings.Contains(rows[len(rows)-1], "close") {
					t.Fatalf("footer floats above bottom:\n%s", content)
				}
			}
		})
	}
}

func TestSurfaceFooterHeightPreservesControlsAndBodyAnchor(t *testing.T) {
	footer := chrome.RenderGuidance(chrome.GuidanceSpec{Hints: chrome.RequiredGuidanceHints("Esc back"), Rows: 1}, 60)
	for _, height := range []int{10, 20, 5} {
		result := fitSurfaceFooterHeight("first visible row\nsecond row\n"+footer, height)
		if lipgloss.Height(result) != height {
			t.Fatalf("height %d: %q", height, result)
		}
		if !strings.HasPrefix(result, "first visible row\n") || !strings.HasSuffix(result, footer) {
			t.Fatalf("body anchor or controls moved: %q", result)
		}
		feedback := composeSurfaceFooterFeedback(result, "Saved")
		if lipgloss.Height(feedback) != height || !strings.HasSuffix(feedback, footer) {
			t.Fatalf("feedback displaced controls: %q", feedback)
		}
	}
}

func TestDailySurfacesKeepFooterThroughFeedbackAndResize(t *testing.T) {
	for _, surface := range []struct{ name, key string }{{"outline", ""}, {"notes", "ctrl+n"}, {"projects", "P"}, {"help", "?"}} {
		t.Run(surface.name, func(t *testing.T) {
			d := polishDriver(t)
			if surface.key != "" {
				if err := d.SendKey(surface.key); err != nil {
					t.Fatal(err)
				}
			}
			m := d.App()
			for _, size := range [][2]int{{60, 20}, {100, 35}, {80, 24}, {120, 40}, {60, 20}} {
				d.Resize(size[0], size[1])
				for _, feedback := range []string{"", "Saved", ""} {
					m.setInfoStatus(feedback)
					frame := ansi.Strip(m.View().Content)
					rows := strings.Split(frame, "\n")
					if len(rows) != size[1] {
						t.Fatalf("%v %q: height=%d", size, feedback, len(rows))
					}
					if strings.TrimSpace(rows[len(rows)-1]) == "" {
						t.Fatalf("%v: footer left a blank bottom row\n%s", size, frame)
					}
					for _, row := range rows {
						if lipgloss.Width(row) > size[0] {
							t.Fatalf("%v: overflow %q", size, row)
						}
					}
				}
			}
		})
	}
}
