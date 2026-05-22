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
		category  models.DecisionCategory
		resolved  bool
	}

	var current *decisionDraft
	collectingRationale := false

	flush := func() {
		if current == nil {
			return
		}
		text := strings.TrimSpace(current.text.String())
		if text == "" {
			current = nil
			collectingRationale = false
			return
		}
		task.Decisions = append(task.Decisions, &models.Decision{
			Text:      unescapeBlockText(text),
			Rationale: unescapeBlockText(strings.TrimSpace(current.rationale.String())),
			Category:  current.category,
			CreatedAt: current.createdAt,
			Resolved:  current.resolved,
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

			// Parse fields by recognized prefix rather than position so
			// that a header missing the category (e.g. "<!-- decision: ts,
			// resolved -->") still records the resolved flag instead of
			// misassigning "resolved" to category.
			if len(parts) >= 1 {
				if t, ok := ParseTimeField(parts[0]); ok {
					current.createdAt = t
				}
			}
			for _, p := range parts[1:] {
				switch {
				case strings.HasPrefix(p, "category:"):
					catStr := strings.TrimPrefix(p, "category:")
					current.category = models.DecisionCategory(strings.TrimSpace(catStr))
				case p == "resolved":
					current.resolved = true
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
func WriteDecisions(task *models.Task) string {
	if len(task.Decisions) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("<!-- DECISIONS -->\n")

	for _, d := range task.Decisions {
		resolvedSuffix := ""
		if d.Resolved {
			resolvedSuffix = ", resolved"
		}
		fmt.Fprintf(&b, "<!-- decision: %s, category:%s%s -->\n",
			d.CreatedAt.Format(util.DateTimeFormat),
			string(d.Category),
			resolvedSuffix,
		)
		b.WriteString(escapeBlockText(d.Text))
		b.WriteString("\n")
		b.WriteString("<!-- rationale -->\n")
		b.WriteString(escapeBlockText(d.Rationale))
		b.WriteString("\n")
	}

	b.WriteString("<!-- /DECISIONS -->\n")
	return b.String()
}
