package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readmeDemoStep is one beat of the README animation: the input it sends, how
// long the resulting frame holds, and the caption naming what was pressed.
type readmeDemoStep struct {
	action  UITestAction
	holdMS  int
	caption string
}

// readmeDemoFrame is one exported frame of docs/images/demo.svg.
type readmeDemoFrame struct {
	ANSI    string `json:"ansi"`
	HoldMS  int    `json:"hold_ms"`
	Caption string `json:"caption"`
}

func demoKey(key string, holdMS int, caption string) readmeDemoStep {
	return readmeDemoStep{UITestAction{Type: UITestActionKey, Key: key}, holdMS, caption}
}

func demoText(text string, holdMS int, caption string) readmeDemoStep {
	return readmeDemoStep{UITestAction{Type: UITestActionText, Text: text}, holdMS, caption}
}

// readmeDemoScript is the tour the README animation plays: capture a task with
// sigils, read it in Peek, start it, search, plan another for Today, finish it
// there, and undo, ending back on All so the loop closes where it began. It names no date, so the frames
// do not change with the day they are regenerated on.
var readmeDemoScript = []readmeDemoStep{
	demoKey("a", 700, "a  capture"),
	demoText("Draft the release notes", 450, "a  capture"),
	demoText(" %website", 450, "%project routes it"),
	demoText(" !1", 450, "!1 sets priority"),
	demoText(" #docs", 1800, "#tag adds a tag"),
	demoKey("enter", 1800, "Enter  save"),
	demoKey("j", 300, "j  move"),
	demoKey("j", 300, "j  move"),
	demoKey("j", 300, "j  move"),
	demoKey("j", 300, "j  move"),
	demoKey("j", 900, "j  move"),
	demoKey("b", 2200, "b  peek"),
	demoKey("w", 1800, "w  workflow chord"),
	demoKey("g", 2000, "w g  start it"),
	demoKey("/", 700, "/  search"),
	demoText("land", 2000, "/  search"),
	demoKey("esc", 1200, "Esc  back"),
	demoKey("w", 900, "w  workflow chord"),
	demoKey("t", 1800, "w t  plan for Today"),
	demoKey("tab", 2000, "Tab  next mode"),
	demoKey("w", 900, "w  workflow chord"),
	demoKey("d", 2000, "w d  done"),
	demoKey("u", 2000, "u  undo"),
	demoKey("shift+tab", 1500, "Shift+Tab  back to All"),
}

// TestReadmeDemo plays readmeDemoScript over the README fixture and exports
// every frame for scripts/render_terminal_svg.py to animate. Each frame must be
// free of layout issues, since the animation is published as the product.
func TestReadmeDemo(t *testing.T) {
	driver := launchReadmeFixture(t)
	opts := ObserveOptions{IncludeANSI: true}
	first := driver.Observe(opts)
	frames := []readmeDemoFrame{{ANSI: first.ANSIScreen, HoldMS: 2500, Caption: "tt  all your work, one outline"}}
	for i, step := range readmeDemoScript {
		obs, err := driver.ApplyAction(step.action, opts)
		if err != nil {
			t.Fatalf("step %d (%s): %v", i, step.caption, err)
		}
		if len(obs.LayoutIssues) != 0 || strings.Contains(obs.PlainScreen, "✕") {
			t.Fatalf("step %d (%s) is not clean:\n%s\n%+v", i, step.caption, obs.PlainScreen, obs.LayoutIssues)
		}
		if os.Getenv("DOCKET_README_DEMO_PRINT") != "" {
			t.Logf("step %d (%s):\n%s", i, step.caption, obs.PlainScreen)
		}
		frames = append(frames, readmeDemoFrame{ANSI: obs.ANSIScreen, HoldMS: step.holdMS, Caption: step.caption})
	}
	data, err := json.MarshalIndent(frames, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(readmeScreenshotDir(t), "demo.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}
