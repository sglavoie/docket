package format

import (
	"regexp"
	"strings"
)

// BlockLine represents a single line found inside a block.
type BlockLine struct {
	Trimmed  string // Whitespace-trimmed version
	Original string // Original line (preserving indentation)
}

// ExtractBlock scans lines of text for a delimited block (start/end regex markers).
// It returns the lines found inside the block and the cleaned description with the block removed.
// Lines matching startRe and endRe are consumed (not included in either output).
func ExtractBlock(description string, startRe, endRe *regexp.Regexp) (blockLines []BlockLine, cleanDesc string) {
	lines := strings.Split(description, "\n")
	var clean strings.Builder
	var inBlock bool

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if startRe.MatchString(trimmed) {
			inBlock = true
			continue
		}

		if inBlock && endRe.MatchString(trimmed) {
			inBlock = false
			continue
		}

		if inBlock {
			blockLines = append(blockLines, BlockLine{Trimmed: trimmed, Original: line})
		} else {
			clean.WriteString(line)
			clean.WriteString("\n")
		}
	}

	return blockLines, strings.TrimSpace(clean.String())
}
