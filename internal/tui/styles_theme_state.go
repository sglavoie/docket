package tui

import "charm.land/lipgloss/v2"

// themeBgColor holds the current TUI background color (empty = none/terminal default).
var themeBgColor string

// colorNormalText tracks whether NormalText palette override is set.
var colorNormalText string

// zebraEnabled controls whether alternating row backgrounds are rendered.
var zebraEnabled bool

// ZebraRowStyle is the background style applied to odd-indexed rows when zebra is enabled.
var ZebraRowStyle = lipgloss.NewStyle().Background(ColorZebraBg)
