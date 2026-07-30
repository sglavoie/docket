package format

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// Matches decision block start marker "<!-- DECISIONS -->"
var decisionsStartRegex = regexp.MustCompile(`^<!--\s*DECISIONS\s*-->$`)

// Matches decision block end marker "<!-- /DECISIONS -->"
var decisionsEndRegex = regexp.MustCompile(`^<!--\s*/DECISIONS\s*-->$`)

// Matches individual decision header like "<!-- decision: 2026-05-07 14:30, category:design -->"
// or "<!-- decision: 2026-05-07 14:30, category:trade-off, resolved -->"
var decisionHeaderRegex = regexp.MustCompile(`^<!--\s*decision:\s*(.+?)\s*-->$`)

// Matches the rationale separator "<!-- rationale -->"
var rationaleMarkerRegex = regexp.MustCompile(`^<!--\s*rationale\s*-->$`)

// ExtractDecisionsFromDescription parses the DECISIONS block from task description.
// It extracts decisions and returns the cleaned description (without the decisions block).
func ExtractDecisionsFromDescription(task *models.Task) {
	if task.Description == "" {
		return
	}

	blockLines, cleanDesc := ExtractBlock(task.Description, decisionsStartRegex, decisionsEndRegex)
	task.Description = cleanDesc

	type decisionDraft struct {
		text      strings.Builder
		rationale strings.Builder
		createdAt time.Time
	}

	var current *decisionDraft
	collectingRationale := false

	flush := func() {
		if current == nil {
			return
		}
		text := current.text.String()
		if strings.TrimSpace(text) == "" {
			current = nil
			collectingRationale = false
			return
		}
		rationale := current.rationale.String()
		if strings.TrimSpace(rationale) == "" {
			rationale = ""
		}
		task.Decisions = append(task.Decisions, &models.Decision{
			Text:      unescapeBlockText(text),
			Rationale: unescapeBlockText(rationale),
			CreatedAt: current.createdAt,
		})
		current = nil
		collectingRationale = false
	}

	for _, bl := range blockLines {
		if matches := decisionHeaderRegex.FindStringSubmatch(bl.Trimmed); matches != nil {
			flush()
			current = &decisionDraft{}
			collectingRationale = false

			headerValue := matches[1]
			parts := strings.Split(headerValue, ",")
			for i := range parts {
				parts[i] = strings.TrimSpace(parts[i])
			}

			// Legacy headers may carry retired category and resolved fields.
			// Parse only the timestamp so live records normalize to the lean
			// text/rationale/timestamp shape.
			if len(parts) >= 1 {
				if t, ok := ParseTimeField(parts[0]); ok {
					current.createdAt = t
				}
			}
			continue
		}

		if rationaleMarkerRegex.MatchString(bl.Trimmed) {
			collectingRationale = true
			continue
		}

		if current == nil {
			continue
		}

		if collectingRationale {
			if current.rationale.Len() > 0 {
				current.rationale.WriteString("\n")
			}
			current.rationale.WriteString(bl.Original)
		} else {
			if current.text.Len() > 0 {
				current.text.WriteString("\n")
			}
			current.text.WriteString(bl.Original)
		}
	}

	flush()
}

// WriteDecisions generates the decisions block for a task.
// Decisions with empty (whitespace-only) text are skipped: the parser drops
// them on flush, so emitting them would silently lose the entry on the next save.
func WriteDecisions(task *models.Task) string {
	if len(task.Decisions) == 0 {
		return ""
	}

	var b strings.Builder
	headerWritten := false

	for _, d := range task.Decisions {
		if d == nil {
			continue
		}
		text := d.Text
		if strings.TrimSpace(text) == "" {
			continue
		}
		rationale := d.Rationale
		if strings.TrimSpace(rationale) == "" {
			rationale = ""
		}
		if !headerWritten {
			b.WriteString("<!-- DECISIONS -->\n")
			headerWritten = true
		}
		fmt.Fprintf(&b, "<!-- decision: %s -->\n", d.CreatedAt.Format(util.DateTimeFormat))
		b.WriteString(escapeBlockText(text))
		b.WriteString("\n")
		b.WriteString("<!-- rationale -->\n")
		b.WriteString(escapeBlockText(rationale))
		b.WriteString("\n")
	}

	if !headerWritten {
		return ""
	}
	b.WriteString("<!-- /DECISIONS -->\n")
	return b.String()
}
