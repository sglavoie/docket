package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// DiffLineKind represents the type of a diff line.
type DiffLineKind int

const (
	DiffContext DiffLineKind = iota // Unchanged line
	DiffAdd                         // Added line
	DiffRemove                      // Removed line
)

// DiffLine represents a single line in a diff.
type DiffLine struct {
	Kind    DiffLineKind
	Content string
}

// Diff styles
var (
	DiffAddStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))  // Green
	DiffRemoveStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red
	DiffContextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")) // Gray
)

// ComputeLineDiff computes a simple line-by-line diff between old and new content.
// Returns lines showing what changed: removed lines from old, added lines from new.
func ComputeLineDiff(oldContent, newContent string) []DiffLine {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// Build a map of lines in old content for quick lookup
	oldLineSet := make(map[string]int)
	for _, line := range oldLines {
		oldLineSet[line]++
	}

	// Build a map of lines in new content
	newLineSet := make(map[string]int)
	for _, line := range newLines {
		newLineSet[line]++
	}

	var result []DiffLine

	// Find removed lines (in old but not in new, or fewer occurrences in new)
	oldCounts := make(map[string]int)
	for _, line := range oldLines {
		oldCounts[line]++
	}
	newCounts := make(map[string]int)
	for _, line := range newLines {
		newCounts[line]++
	}

	// Show removed lines
	seenOld := make(map[string]int)
	for _, line := range oldLines {
		seenOld[line]++
		if seenOld[line] > newCounts[line] {
			result = append(result, DiffLine{Kind: DiffRemove, Content: line})
		}
	}

	// Show added lines
	seenNew := make(map[string]int)
	for _, line := range newLines {
		seenNew[line]++
		if seenNew[line] > oldCounts[line] {
			result = append(result, DiffLine{Kind: DiffAdd, Content: line})
		}
	}

	return result
}

// RenderDiffLines renders diff lines with appropriate styling.
func RenderDiffLines(lines []DiffLine, width int) string {
	var b strings.Builder

	for _, line := range lines {
		content := line.Content
		// Truncate if too long
		maxLen := width - 3 // Leave room for prefix
		if maxLen > 0 && len(content) > maxLen {
			content = content[:maxLen-3] + "..."
		}

		switch line.Kind {
		case DiffRemove:
			b.WriteString(DiffRemoveStyle.Render("- " + content))
		case DiffAdd:
			b.WriteString(DiffAddStyle.Render("+ " + content))
		case DiffContext:
			b.WriteString(DiffContextStyle.Render("  " + content))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// ComputeContextualDiff computes a diff with surrounding context lines.
// This provides a more readable diff by showing unchanged lines around changes.
func ComputeContextualDiff(oldContent, newContent string, contextLines int) []DiffLine {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// Use a simple LCS-based approach for better diffs
	// For now, use a simpler unified diff approach

	var result []DiffLine
	oldIdx, newIdx := 0, 0

	for oldIdx < len(oldLines) || newIdx < len(newLines) {
		if oldIdx >= len(oldLines) {
			// Remaining lines in new are additions
			result = append(result, DiffLine{Kind: DiffAdd, Content: newLines[newIdx]})
			newIdx++
		} else if newIdx >= len(newLines) {
			// Remaining lines in old are removals
			result = append(result, DiffLine{Kind: DiffRemove, Content: oldLines[oldIdx]})
			oldIdx++
		} else if oldLines[oldIdx] == newLines[newIdx] {
			// Lines match - context
			result = append(result, DiffLine{Kind: DiffContext, Content: oldLines[oldIdx]})
			oldIdx++
			newIdx++
		} else {
			// Lines differ - check if it's a removal, addition, or modification
			// Look ahead to see if old line appears later in new
			foundInNew := -1
			for i := newIdx; i < len(newLines) && i < newIdx+10; i++ {
				if oldLines[oldIdx] == newLines[i] {
					foundInNew = i
					break
				}
			}

			// Look ahead to see if new line appears later in old
			foundInOld := -1
			for i := oldIdx; i < len(oldLines) && i < oldIdx+10; i++ {
				if newLines[newIdx] == oldLines[i] {
					foundInOld = i
					break
				}
			}

			if foundInNew >= 0 && (foundInOld < 0 || foundInNew-newIdx <= foundInOld-oldIdx) {
				// New lines were added before the match
				for newIdx < foundInNew {
					result = append(result, DiffLine{Kind: DiffAdd, Content: newLines[newIdx]})
					newIdx++
				}
			} else if foundInOld >= 0 {
				// Old lines were removed before the match
				for oldIdx < foundInOld {
					result = append(result, DiffLine{Kind: DiffRemove, Content: oldLines[oldIdx]})
					oldIdx++
				}
			} else {
				// No match found - treat as removal then addition
				result = append(result, DiffLine{Kind: DiffRemove, Content: oldLines[oldIdx]})
				oldIdx++
				result = append(result, DiffLine{Kind: DiffAdd, Content: newLines[newIdx]})
				newIdx++
			}
		}
	}

	return result
}

// FilterDiffToChanges returns only the add/remove lines from a diff.
func FilterDiffToChanges(lines []DiffLine) []DiffLine {
	var result []DiffLine
	for _, line := range lines {
		if line.Kind != DiffContext {
			result = append(result, line)
		}
	}
	return result
}

// SummarizeDiff returns a brief summary of changes.
func SummarizeDiff(lines []DiffLine) (added, removed int) {
	for _, line := range lines {
		switch line.Kind {
		case DiffAdd:
			added++
		case DiffRemove:
			removed++
		}
	}
	return
}
