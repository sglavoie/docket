package grammar

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/capturetext"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/textgrammar"
)

// This file owns the outline buffer's own syntax: the two-space indent ladder
// and the [[tt:xxxxxxxx]] identity marker. Everything left after those are read
// off the front and back of a line is handed to the capture parser, so a line in
// an outline buffer means exactly what the same line means in the composer.

const (
	// OutlineIndentWidth is the number of spaces one nesting level costs. The
	// buffer header states it, because indentation is the only parentage channel
	// an outline buffer has.
	OutlineIndentWidth = 2
	// OutlineMarkerPrefix and OutlineMarkerSuffix bracket the identity marker a
	// dumped line carries. A line without one creates a task.
	OutlineMarkerPrefix = "[[tt:"
	OutlineMarkerSuffix = "]]"
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

// ParseOutlineBuffer turns an edited buffer into application plan lines.
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
// projects is the loaded population a ~task reference is addressed against; the
// scope project alone would leave a dumped cross-project dependency unresolvable
// on the way back in.
func ParseOutlineBuffer(buffer string, project *models.Project, projects []*models.Project) ([]app.OutlinePlanLine, []LineError) {
	structural, errs := parseOutlineStructure(buffer, project)
	lines, lineErrs := parseOutlineBodies(structural, project, projects)
	errs = append(errs, lineErrs...)
	sort.SliceStable(errs, func(i, j int) bool { return errs[i].Line < errs[j].Line })
	if len(errs) > 0 {
		return nil, errs
	}
	return lines, nil
}

// parseOutlineStructure is the outline phase. It rejects malformed indentation
// and identity markers, and reports a line whose parent was itself rejected
// rather than silently reattaching it to a surviving ancestor.
func parseOutlineStructure(buffer string, project *models.Project) ([]outlineBufferLine, []LineError) {
	markers := OutlineMarkerIndex(project)
	var (
		out           []outlineBufferLine
		errs          []LineError
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
			errs = append(errs, LineError{Line: number, Message: indentErr.Error()})
			continue
		}
		depth := indent / OutlineIndentWidth

		// A rejected line is still a real indentation level, so previousDepth
		// advances and reject() opens a rejected span at this depth. Anything
		// nested under it names the line that actually broke instead of being
		// silently reattached to a surviving ancestor.
		reject := func(message string) {
			errs = append(errs, LineError{Line: number, Message: message})
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

		marker, body, markerErr := SplitOutlineMarker(body)
		if markerErr != nil {
			reject(markerErr.Error())
			continue
		}
		taskID := ""
		if marker != "" {
			resolved, known := markers[marker]
			if !known {
				reject(fmt.Sprintf("marker %s%s%s does not name a task in this project", OutlineMarkerPrefix, marker, OutlineMarkerSuffix))
				continue
			}
			if first, duplicate := claimed[resolved]; duplicate {
				reject(fmt.Sprintf("marker %s%s%s already claimed on line %d", OutlineMarkerPrefix, marker, OutlineMarkerSuffix, first))
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
// parsed through the shared outline surface. Structural capture tokens receive
// explicit guidance because the buffer owns project and parent placement.
func parseOutlineBodies(structural []outlineBufferLine, project *models.Project, projects []*models.Project) ([]app.OutlinePlanLine, []LineError) {
	var (
		lines []app.OutlinePlanLine
		errs  []LineError
	)
	for _, item := range structural {
		structuralTokens := textgrammar.Parse(item.body, textgrammar.SurfaceCapture)
		forbidden := false
		if len(structuralTokens.TokensFor(textgrammar.FieldProject)) > 0 {
			errs = append(errs, LineError{
				Line:    item.sourceLine,
				Message: "the outline buffer owns the project; remove the %project sigil",
			})
			forbidden = true
		}
		if len(structuralTokens.TokensFor(textgrammar.FieldParent)) > 0 {
			errs = append(errs, LineError{
				Line:    item.sourceLine,
				Message: "indentation sets parentage here; remove the >parent sigil",
			})
			forbidden = true
		}
		if len(structuralTokens.TokensFor(textgrammar.FieldChild)) > 0 {
			errs = append(errs, LineError{
				Line:    item.sourceLine,
				Message: "indentation sets parentage here; remove the <child sigil",
			})
			forbidden = true
		}
		if len(structuralTokens.TokensFor(textgrammar.FieldDefer)) > 0 {
			errs = append(errs, LineError{
				Line:    item.sourceLine,
				Message: "the outline buffer cannot defer; remove the @@defer sigil",
			})
			forbidden = true
		}
		// The buffer edits tasks that already exist, and an attachment is a
		// creation-time or panel act, so a dumped &link would either duplicate an
		// existing attachment or silently vanish on the round trip.
		if len(structuralTokens.TokensFor(textgrammar.FieldAttachment)) > 0 {
			errs = append(errs, LineError{
				Line:    item.sourceLine,
				Message: "the outline buffer cannot attach; remove the &link sigil",
			})
			forbidden = true
		}
		// The buffer holds no comment either: a dumped task's comments are not in
		// the line, so a ;comment; span could only ever append a duplicate on every
		// round trip.
		if len(structuralTokens.TokensFor(textgrammar.FieldComment)) > 0 {
			errs = append(errs, LineError{
				Line:    item.sourceLine,
				Message: "the outline buffer cannot comment; remove the ;comment; sigil",
			})
			forbidden = true
		}
		if forbidden {
			continue
		}

		parsed, err := capturetext.ParseQuickCaptureSurface(item.body, project, projects, true, textgrammar.SurfaceOutline)
		if err != nil {
			errs = append(errs, LineError{Line: item.sourceLine, Message: err.Error()})
			continue
		}
		if err := models.ValidateTaskTitle(parsed.Title); err != nil {
			errs = append(errs, LineError{Line: item.sourceLine, Message: err.Error()})
			continue
		}
		line := app.OutlinePlanLine{
			SourceLine: item.sourceLine,
			TaskID:     item.taskID,
			Depth:      item.depth,
			Title:      parsed.Title,
			Priority:   parsed.Priority,
			Tags:       parsed.Tags,
			Context:    parsed.Context,
			Due:        parsed.DueDate,
			Pinned:     parsed.Pinned,
			Review:     parsed.Review,
			// The parser has already folded -~task and ~0 in, so this is the final
			// set the line declares rather than a patch over the stored one.
			Dependencies: capturetext.DependencyLocatorsOf(parsed.DependencyRefs),
		}
		if parsed.DestinationSet {
			line.Destination = parsed.Destination
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
	if indent%OutlineIndentWidth != 0 {
		return 0, "", fmt.Errorf("indent of %d spaces is not a multiple of %d", indent, OutlineIndentWidth)
	}
	return indent, strings.TrimRight(body, " \t"), nil
}

// SplitOutlineMarker removes a trailing identity marker and returns it alongside
// the capture text. The marker is anchored to the end of the line so a title may
// contain brackets without being mistaken for one.
func SplitOutlineMarker(body string) (string, string, error) {
	if !strings.HasSuffix(body, OutlineMarkerSuffix) {
		return "", body, nil
	}
	start := strings.LastIndex(body, OutlineMarkerPrefix)
	if start < 0 {
		return "", "", fmt.Errorf("a line ending in %s must carry a %s…%s marker", OutlineMarkerSuffix, OutlineMarkerPrefix, OutlineMarkerSuffix)
	}
	marker := body[start+len(OutlineMarkerPrefix) : len(body)-len(OutlineMarkerSuffix)]
	if strings.TrimSpace(marker) == "" || strings.ContainsAny(marker, " \t[]") {
		return "", "", fmt.Errorf("marker %q is malformed", body[start:])
	}
	return marker, strings.TrimRight(body[:start], " \t"), nil
}

// OutlineMarkerIndex maps every marker a buffer may legally carry to the task ID
// it names. Both the printed short marker and the full task ID resolve, so a
// hand-written marker copied from elsewhere still lands. An ambiguous short
// marker is left out rather than resolved arbitrarily; the dump refuses to
// produce one, so reaching this case means the buffer was hand-authored.
func OutlineMarkerIndex(project *models.Project) map[string]string {
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
		short[OutlineMarkerFor(task.ID)]++
	}
	for _, task := range project.Tasks {
		if task == nil || task.ID == "" {
			continue
		}
		if marker := OutlineMarkerFor(task.ID); short[marker] == 1 {
			index[marker] = task.ID
		}
	}
	return index
}

// OutlineMarkerFor is the marker the dump prints for a task ID.
func OutlineMarkerFor(taskID string) string {
	if len(taskID) <= outlineMarkerLength {
		return taskID
	}
	return taskID[:outlineMarkerLength]
}
