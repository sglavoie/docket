package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// This file owns the outline buffer's own syntax: the two-space indent ladder
// and the [[tt:xxxxxxxx]] identity marker. Everything left after those are read
// off the front and back of a line is handed to the capture parser, so a line in
// an outline buffer means exactly what the same line means in the composer.

const (
	// outlineIndentWidth is the number of spaces one nesting level costs. The
	// buffer header states it, because indentation is the only parentage channel
	// an outline buffer has.
	outlineIndentWidth = 2
	// outlineMarkerPrefix and outlineMarkerSuffix bracket the identity marker a
	// dumped line carries. A line without one creates a task.
	outlineMarkerPrefix = "[[tt:"
	outlineMarkerSuffix = "]]"
	// outlineMarkerLength is how much of a task ID the dump prints. Eight
	// characters keep the marker unobtrusive while staying unique within one
	// project; the dump verifies that uniqueness before the editor opens.
	outlineMarkerLength = 8
)

// outlineBufferLine is one nonblank, non-comment buffer line after the outline
// phase has read its indentation and identity marker off it.
type outlineBufferLine struct {
	sourceLine int
	depth      int
	taskID     string
	body       string
}

// parseOutlineBuffer turns an edited buffer into application plan lines.
//
// Validation runs in two phases and both finish before any plan is proposed: the
// outline phase reads indentation and identity markers, and the line phase hands
// the remainder to the capture parser. Both collect rather than stopping at the
// first failure, because a buffer edited outside the TUI is usually wrong in
// several places at once, and reopening the editor once per error is not a
// workflow.
//
// An empty result is not an error here. Since absence deletes, a buffer holding
// no lines is a request to empty the project, and only the caller knows the
// scope that request would remove.
func parseOutlineBuffer(buffer string, project *models.Project) ([]app.OutlinePlanLine, []lineError) {
	structural, errs := parseOutlineStructure(buffer, project)
	lines, lineErrs := parseOutlineBodies(structural, project)
	errs = append(errs, lineErrs...)
	sort.SliceStable(errs, func(i, j int) bool { return errs[i].line < errs[j].line })
	if len(errs) > 0 {
		return nil, errs
	}
	return lines, nil
}

// parseOutlineStructure is the outline phase. It rejects malformed indentation
// and identity markers, and reports a line whose parent was itself rejected
// rather than silently reattaching it to a surviving ancestor.
func parseOutlineStructure(buffer string, project *models.Project) ([]outlineBufferLine, []lineError) {
	markers := outlineMarkerIndex(project)
	var (
		out           []outlineBufferLine
		errs          []lineError
		claimed       = make(map[string]int)
		previousDepth = -1
		// rejectedDepth is the depth of the shallowest still-open rejected line,
		// and rejectedLine is where it was reported. Together they let a child of a
		// rejected parent name the line that actually broke.
		rejectedDepth = -1
		rejectedLine  = 0
	)

	for index, raw := range strings.Split(buffer, "\n") {
		number := index + 1
		if outlineLineIsSkippable(raw) {
			continue
		}

		indent, body, indentErr := splitOutlineIndent(raw)
		if indentErr != nil {
			errs = append(errs, lineError{line: number, message: indentErr.Error()})
			continue
		}
		depth := indent / outlineIndentWidth

		// A rejected line is still a real indentation level, so previousDepth
		// advances and reject() opens a rejected span at this depth. Anything
		// nested under it names the line that actually broke instead of being
		// silently reattached to a surviving ancestor.
		reject := func(message string) {
			errs = append(errs, lineError{line: number, message: message})
			previousDepth = depth
			rejectedDepth, rejectedLine = depth, number
		}

		if rejectedDepth >= 0 {
			if depth > rejectedDepth {
				reject(fmt.Sprintf("its parent on line %d was rejected, so it has nowhere to attach", rejectedLine))
				continue
			}
			rejectedDepth, rejectedLine = -1, 0
		}
		if depth > previousDepth+1 {
			reject(fmt.Sprintf("indent jumps from level %d to level %d; indent one level at a time", previousDepth, depth))
			continue
		}
		if depth > models.MaxNestingDepth {
			reject(fmt.Sprintf("nests deeper than the maximum of %d levels", models.MaxNestingDepth))
			continue
		}

		marker, body, markerErr := splitOutlineMarker(body)
		if markerErr != nil {
			reject(markerErr.Error())
			continue
		}
		taskID := ""
		if marker != "" {
			resolved, known := markers[marker]
			if !known {
				reject(fmt.Sprintf("marker %s%s%s does not name a task in this project", outlineMarkerPrefix, marker, outlineMarkerSuffix))
				continue
			}
			if first, duplicate := claimed[resolved]; duplicate {
				reject(fmt.Sprintf("marker %s%s%s already claimed on line %d", outlineMarkerPrefix, marker, outlineMarkerSuffix, first))
				continue
			}
			claimed[resolved] = number
			taskID = resolved
		}

		previousDepth = depth
		out = append(out, outlineBufferLine{sourceLine: number, depth: depth, taskID: taskID, body: body})
	}
	return out, errs
}

// parseOutlineBodies is the line phase. The body left after the marker strip is
// ordinary capture text, so it goes through the same parser the composer uses
// with %project disabled: an outline buffer is scoped to one project and never
// moves a task out of it.
func parseOutlineBodies(structural []outlineBufferLine, project *models.Project) ([]app.OutlinePlanLine, []lineError) {
	var (
		lines []app.OutlinePlanLine
		errs  []lineError
	)
	for _, item := range structural {
		parsed, err := parseQuickCapture(item.body, project, nil, true)
		if err != nil {
			errs = append(errs, lineError{line: item.sourceLine, message: err.Error()})
			continue
		}
		if err := models.ValidateTaskTitle(parsed.title); err != nil {
			errs = append(errs, lineError{line: item.sourceLine, message: err.Error()})
			continue
		}
		if parsed.parentID != nil {
			errs = append(errs, lineError{
				line:    item.sourceLine,
				message: "indentation sets parentage here; remove the >parent sigil",
			})
			continue
		}
		line := app.OutlinePlanLine{
			SourceLine: item.sourceLine,
			TaskID:     item.taskID,
			Depth:      item.depth,
			Title:      parsed.title,
			Priority:   parsed.priority,
			Tags:       parsed.tags,
			Due:        parsed.dueDate,
		}
		if parsed.destinationSet {
			line.Destination = parsed.destination
		}
		lines = append(lines, line)
	}
	return lines, errs
}

// outlineLineIsSkippable reports whether a raw buffer line carries no task. A
// comment is recognized by a '#' in the first column only, so an indented #tag
// stays an ordinary (and invalid) task line rather than vanishing silently.
func outlineLineIsSkippable(raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return true
	}
	return strings.HasPrefix(raw, "#")
}

// splitOutlineIndent measures a line's nesting indent and returns the remainder.
func splitOutlineIndent(raw string) (int, string, error) {
	body := strings.TrimLeft(raw, " ")
	indent := len(raw) - len(body)
	if strings.HasPrefix(body, "\t") {
		return 0, "", fmt.Errorf("indent with spaces, not tabs")
	}
	if indent%outlineIndentWidth != 0 {
		return 0, "", fmt.Errorf("indent of %d spaces is not a multiple of %d", indent, outlineIndentWidth)
	}
	return indent, strings.TrimRight(body, " \t"), nil
}

// splitOutlineMarker removes a trailing identity marker and returns it alongside
// the capture text. The marker is anchored to the end of the line so a title may
// contain brackets without being mistaken for one.
func splitOutlineMarker(body string) (string, string, error) {
	if !strings.HasSuffix(body, outlineMarkerSuffix) {
		return "", body, nil
	}
	start := strings.LastIndex(body, outlineMarkerPrefix)
	if start < 0 {
		return "", "", fmt.Errorf("a line ending in %s must carry a %s…%s marker", outlineMarkerSuffix, outlineMarkerPrefix, outlineMarkerSuffix)
	}
	marker := body[start+len(outlineMarkerPrefix) : len(body)-len(outlineMarkerSuffix)]
	if strings.TrimSpace(marker) == "" || strings.ContainsAny(marker, " \t[]") {
		return "", "", fmt.Errorf("marker %q is malformed", body[start:])
	}
	return marker, strings.TrimRight(body[:start], " \t"), nil
}

// outlineMarkerIndex maps every marker a buffer may legally carry to the task ID
// it names. Both the printed short marker and the full task ID resolve, so a
// hand-written marker copied from elsewhere still lands. An ambiguous short
// marker is left out rather than resolved arbitrarily; the dump refuses to
// produce one, so reaching this case means the buffer was hand-authored.
func outlineMarkerIndex(project *models.Project) map[string]string {
	index := make(map[string]string)
	if project == nil {
		return index
	}
	short := make(map[string]int)
	for _, task := range project.Tasks {
		if task == nil || task.ID == "" {
			continue
		}
		index[task.ID] = task.ID
		short[outlineMarkerFor(task.ID)]++
	}
	for _, task := range project.Tasks {
		if task == nil || task.ID == "" {
			continue
		}
		if marker := outlineMarkerFor(task.ID); short[marker] == 1 {
			index[marker] = task.ID
		}
	}
	return index
}

// outlineMarkerFor is the marker the dump prints for a task ID.
func outlineMarkerFor(taskID string) string {
	if len(taskID) <= outlineMarkerLength {
		return taskID
	}
	return taskID[:outlineMarkerLength]
}
