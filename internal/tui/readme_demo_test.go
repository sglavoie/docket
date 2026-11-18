package tui

import (
	"encoding/json"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// The README animation is drawn at this size, against this background:
// scripts/render_terminal_svg.py paints the same columns on the same colour.
const (
	demoCols = 100
	demoRows = 16
)

var demoBackground = color.RGBA{0x18, 0x1c, 0x22, 0xff}

// demoStep is one beat of the README animation: the keys it presses (a
// quoted token is typed, as press reads it), how long the resulting frame
// holds, and the caption naming what was pressed.
type demoStep struct {
	keys    []string
	holdMS  int
	caption string
}

// demoFrame is one exported frame of docs/images/demo.svg.
type demoFrame struct {
	ANSI    string `json:"ansi"`
	HoldMS  int    `json:"hold_ms"`
	Caption string `json:"caption"`
}

func beat(holdMS int, caption string, keys ...string) demoStep {
	return demoStep{keys, holdMS, caption}
}

// demoScript is the tour the README animation plays, from the poster inside
// docket-rewrite: capture a row with sigils into another project, pick a
// task for Today, finish one, filter by a tag, fold, visit the lists from
// the sidebar, and undo. The fixture names its days relative to the
// harness clock, so the frames do not change with the day they are made.
var demoScript = []demoStep{
	beat(700, "a  capture", "a"),
	beat(450, "a  capture", `"Draft the release notes"`),
	beat(450, "%project files it", `" %Launch"`),
	beat(450, "! flags it", `" !"`),
	beat(1800, "#tag tags it", `" #docs"`),
	beat(1200, "Enter  keep, and offer another", "enter"),
	beat(1500, "Esc  done capturing", "esc"),
	beat(1200, "j  move", "j"),
	beat(1800, "t  pick it for Today", "t"),
	beat(350, "j  move", "j"),
	beat(1200, "j  move", "j"),
	beat(1800, "x  close it", "x"),
	beat(700, "/  filter", "/"),
	beat(1800, "/  filter", `"#ux"`),
	beat(1500, "Enter  keep the filter", "enter"),
	beat(1200, "Esc  clear it", "esc"),
	beat(400, "k  move", "k"),
	beat(1500, "[  previous heading", "["),
	beat(1500, "h  fold", "h"),
	beat(1500, "l  unfold", "l"),
	beat(1200, "Tab  sidebar", "tab"),
	beat(1800, "k  Today", "k", "k", "k", "k"),
	beat(1800, "j  Waiting", "j"),
	beat(1800, "j  website", "j", "j", "j", "j"),
	beat(1200, "Enter  back to the pane", "enter"),
	beat(700, "f  go to a project by name", "f", `"dock"`),
	beat(1500, "f  go to a project by name", "enter"),
	beat(1800, "u  undo", "u"),
}

// demoFixture is a handful of projects with something to say in the
// sidebar: a deadline, nothing left to do, and one left alone for 12 days.
func demoFixture(t *testing.T) *harness {
	h := start(t)
	h.m.Update(tea.WindowSizeMsg{Width: demoCols, Height: demoRows})
	h.m.Update(tea.BackgroundColorMsg{Color: demoBackground})
	h.clock = h.clock.Add(-12 * 24 * time.Hour)
	h.pasteText(strings.Join([]string{
		"docket-rewrite ~teal",
		"website ~orange",
		"\tFix broken links in the footer $waiting",
		"tax-2026",
		"homelab",
		"\tNAS drive layout $note",
		"blog",
		"\tOutline the v2 post",
		"\tPick a theme #writing",
	}, "\n"))
	h.clock = h.clock.Add(7 * 24 * time.Hour)
	fill := func(project string, rows ...string) {
		h.press("f", `"`+project+`"`, "enter")
		h.pasteText(strings.Join(rows, "\n"))
	}
	fill("homelab", "Back up the NAS before the upgrade $waiting")
	h.clock = h.clock.Add(5 * 24 * time.Hour)
	fill("website",
		"Compress hero images #est=30m",
	)
	fill("tax-2026", "File the return @+3d #est=2h", "Gather receipts $done")
	fill("docket-rewrite",
		"Storage:",
		"\tDecide op-log format ! @fri",
		"\t\tper-field, clock-stamped",
		"\tPrototype row parser $doing",
		"\tAudit grammar reuse $done",
		"Launch:",
		"\tSidebar counts #ux",
		"\tCapture line #ux #est=1h",
		"\tHelp screen sections #ux",
	)
	h.press("g", "g")
	return h
}

// TestReadmeDemo plays demoScript over demoFixture and exports every frame
// for scripts/render_terminal_svg.py to animate. Each frame must fit the
// screen it is drawn on, since the animation is published as the product.
// DOCKET_README_DEMO_DIR names where demo.json goes; without it the frames
// are only checked.
func TestReadmeDemo(t *testing.T) {
	h := demoFixture(t)
	frame := func(caption string, holdMS int) demoFrame {
		t.Helper()
		view := h.m.View().Content
		lines := strings.Split(ansi.Strip(view), "\n")
		if len(lines) != demoRows {
			t.Fatalf("%s: %d lines, want %d", caption, len(lines), demoRows)
		}
		for _, l := range lines {
			if w := ansi.StringWidth(l); w > demoCols {
				t.Fatalf("%s: a line is %d columns wide: %q", caption, w, l)
			}
		}
		if os.Getenv("DOCKET_README_DEMO_PRINT") != "" {
			t.Logf("%s:\n%s", caption, strings.Join(lines, "\n"))
		}
		return demoFrame{ANSI: view, HoldMS: holdMS, Caption: caption}
	}
	frames := []demoFrame{frame("Docket  one outline for every project", 3000)}
	for _, step := range demoScript {
		h.press(step.keys...)
		frames = append(frames, frame(step.caption, step.holdMS))
	}
	dir := os.Getenv("DOCKET_README_DEMO_DIR")
	if dir == "" {
		return
	}
	data, err := json.MarshalIndent(frames, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "demo.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}
