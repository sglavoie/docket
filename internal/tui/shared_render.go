package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// TaskLine holds the configuration for rendering a single task line.
// Callers set the fields they need and call Render() to produce the line string.
// The core badge sequence (status → type → priority → title) is always rendered.
type TaskLine struct {
	Task       *models.Task
	IsSelected bool
	MaxTitle   int // maximum title width; clamped to minimum 10

	// Optional elements
	Prefix    string // before cursor (e.g., star indicator "★ ")
	PreBadges string // after cursor, before status badge (e.g., project name, checkbox, rank)
	PostTitle string // after title (e.g., tags, score badge, custom section)
	DueStr    string // right-aligned due date string (pre-styled by caller)
	ViewWidth int    // total view width for due date right-alignment
}

// Render assembles the task line string from the configured fields.
func (tl TaskLine) Render() string {
	cursor := "  "
	if tl.IsSelected {
		cursor = "> "
	}

	statusIcon := GetStatusBadge(string(tl.Task.Status), tl.Task.Status.Icon())
	typeIcon := GetTypeBadge(string(tl.Task.Type), tl.Task.Type.Icon())
	priorityBadge := renderPriorityBadge(tl.Task.Priority)

	maxTitleLen := tl.MaxTitle
	if maxTitleLen < 10 {
		maxTitleLen = 10
	}
	title := util.Truncate(tl.Task.Title, maxTitleLen)

	line := fmt.Sprintf("%s%s%s%s %s %s %s%s", tl.Prefix, cursor, tl.PreBadges, statusIcon, typeIcon, priorityBadge, title, tl.PostTitle)
	if tl.DueStr != "" {
		line = fmt.Sprintf("%-*s %s", tl.ViewWidth-15, line, tl.DueStr)
	}

	if tl.IsSelected {
		line = SelectedStyle.Render(line)
	}

	return line
}
