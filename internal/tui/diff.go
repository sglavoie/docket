package tui

import (
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
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

// ComputeContextualDiff computes a diff with surrounding context lines using Myers algorithm.
// When contextLines > 0, only shows changed lines plus the specified number of surrounding context lines.
func ComputeContextualDiff(oldContent, newContent string, contextLines int) []DiffLine {
	dmp := diffmatchpatch.New()

	// Use line-based diffing for better results
	// Step 1: Convert lines to characters for efficient diffing
	chars1, chars2, lineArray := dmp.DiffLinesToChars(oldContent, newContent)

	// Step 2: Compute diff on the encoded strings (Myers algorithm)
	diffs := dmp.DiffMain(chars1, chars2, false)

	// Step 3: Convert back to lines
	diffs = dmp.DiffCharsToLines(diffs, lineArray)

	// Step 4: Convert to DiffLine format
	var result []DiffLine
	for _, d := range diffs {
		// Each diff.Text may contain multiple lines (ending with \n)
		text := strings.TrimSuffix(d.Text, "\n")
		if text == "" && d.Text == "" {
			continue
		}
		lines := strings.Split(text, "\n")

		var kind DiffLineKind
		switch d.Type {
		case diffmatchpatch.DiffEqual:
			kind = DiffContext
		case diffmatchpatch.DiffInsert:
			kind = DiffAdd
		case diffmatchpatch.DiffDelete:
			kind = DiffRemove
		}

		for _, line := range lines {
			result = append(result, DiffLine{Kind: kind, Content: line})
		}
	}

	// Apply context filtering if requested
	if contextLines > 0 {
		result = filterWithContext(result, contextLines)
	}

	return result
}

// filterWithContext filters diff lines to show only changes plus surrounding context.
// This produces output similar to unified diff format.
func filterWithContext(lines []DiffLine, contextLines int) []DiffLine {
	if len(lines) == 0 {
		return lines
	}

	// First pass: mark which lines should be included
	include := make([]bool, len(lines))

	// Mark all non-context lines and their surrounding context
	for i, line := range lines {
		if line.Kind != DiffContext {
			// Include this line and surrounding context
			start := max(i-contextLines, 0)
			end := min(i+contextLines+1, len(lines))
			for j := start; j < end; j++ {
				include[j] = true
			}
		}
	}

	// Second pass: build filtered result
	var result []DiffLine
	for i, line := range lines {
		if include[i] {
			result = append(result, line)
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
		default:
		}
	}
	return
}
