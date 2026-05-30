package tui

import "strings"

const projectTreeIndentUnit = "  "

// buildProjectTreePrefix returns a compact depth-only prefix for Project Tree.
func buildProjectTreePrefix(level int) string {
	if level <= 0 {
		return ""
	}
	return strings.Repeat(projectTreeIndentUnit, level)
}
