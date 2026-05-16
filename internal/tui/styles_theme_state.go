package tui

import "charm.land/lipgloss/v2"

// activeThemeName holds the current theme name ("light" or "dark"; empty = not set).
var activeThemeName string

// themeBgColor holds the current TUI background color (empty = none/terminal default).
var themeBgColor string

// colorNormalText tracks whether NormalText palette override is set.
var colorNormalText string

// zebraEnabled controls whether alternating row backgrounds are rendered.
var zebraEnabled bool

// ZebraRowStyle is the background style applied to odd-indexed rows when zebra is enabled.
var ZebraRowStyle = lipgloss.NewStyle().Background(ColorZebraBg)

// FirstPlanItemStyle is the subtle background tint applied to the first today plan item when unselected.
var FirstPlanItemStyle = lipgloss.NewStyle().Background(ColorFirstPlanItemBg)

// TreeGroupOddBgStyle is applied to nodes in odd top-level groups for visual region separation.
var TreeGroupOddBgStyle = lipgloss.NewStyle().Background(ColorTreeGroupBg)
