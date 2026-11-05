// Package icons is the one glyph registry. Get answers a key from the plain
// Unicode set unless the process switched to the Nerd Font set, so every caller
// reads the switch by construction and none branches on it.
//
// The switch is process-wide and off by default. Only the interactive TUI shell
// turns it on (see SetNerdGlyphs), so non-interactive CLI output, which never
// does, keeps the Unicode set regardless of any preference or environment.
package icons

import "sync/atomic"

var registry = map[string]string{
	"date":         "⊡",
	"status.inbox": "◌", "status.backlog": "□", "status.doing": "◐",
	"status.backlog.bold": "▣",
	"status.waiting":      "⧗", "status.paused": "‖", "status.someday": "¿",
	"status.done": "✓", "status.canceled": "✕",
	"priority.1": "◉", "priority.2": "●", "priority.3": "◆",
	"github_issue":     "GH",
	"checkbox.checked": "☑", "checkbox.unchecked": "☐",
	"warning": "⚠", "success": "✓", "error": "✕",
	"marker.pin": "*", "marker.attachment": "📎", "marker.description": "▤",
}

// nerdRegistry overrides registry keys with Nerd Font Material Design Icons
// (the nf-md-* range, U+F0001 and up). Each is one terminal cell under the
// width function Docket measures with, which a test holds; a key it lacks
// falls back to the Unicode glyph.
var nerdRegistry = map[string]string{
	"date":                "\U000F0B67", // nf-md-calendar_outline
	"status.inbox":        "\U000F0120", // nf-md-tray_arrow_down
	"status.backlog":      "\U000F14FC", // nf-md-square_rounded_outline
	"status.backlog.bold": "\U000F14FC", // nf-md-square_rounded_outline
	"status.doing":        "\U000F040D", // nf-md-play_circle_outline
	"status.waiting":      "\U000F051F", // nf-md-timer_sand
	"status.paused":       "\U000F03E6", // nf-md-pause_circle_outline
	"status.someday":      "\U000F07F7", // nf-md-thought_bubble_outline
	"status.done":         "\U000F05E1", // nf-md-check_circle_outline
	"status.canceled":     "\U000F015A", // nf-md-close_circle_outline
	"priority.1":          "\U000F0DBC", // nf-md-chevron_triple_up
	"priority.2":          "\U000F013F", // nf-md-chevron_double_up
	"priority.3":          "\U000F0143", // nf-md-chevron_up
	"github_issue":        "\U000F02A4", // nf-md-github
	"checkbox.checked":    "\U000F0135", // nf-md-checkbox_marked_outline
	"checkbox.unchecked":  "\U000F0131", // nf-md-checkbox_blank_outline
	"warning":             "\U000F002A", // nf-md-alert_outline
	"success":             "\U000F012C", // nf-md-check
	"error":               "\U000F0156", // nf-md-close
	"marker.pin":          "\U000F0403", // nf-md-pin
	"marker.attachment":   "\U000F03E2", // nf-md-paperclip
	"marker.description":  "\U000F09ED", // nf-md-text_box_outline
}

var nerd atomic.Bool

// SetNerdGlyphs selects the Nerd Font set (true) or the Unicode set (false) for
// every later Get in this process.
func SetNerdGlyphs(on bool) { nerd.Store(on) }

// Get returns the glyph for key in the selected set, or "" for an unknown key.
func Get(key string) string {
	if nerd.Load() {
		if glyph, ok := nerdRegistry[key]; ok {
			return glyph
		}
	}
	return registry[key]
}
