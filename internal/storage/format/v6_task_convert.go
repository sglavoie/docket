package format

import (
	"fmt"
	"sort"
	"time"
)

// V6TaskConversionResult is the filesystem-free result of converting one
// schema-5 task. NextMember preserves the planning fact carried by the retired
// v5 next lifecycle state so the domain conversion can build ordered planning
// membership after every task has been converted.
type V6TaskConversionResult struct {
	Task       V6TaskRecord                `json:"task"`
	NextMember bool                        `json:"next_member,omitempty"`
	Audit      []V6TaskMigrationAuditEntry `json:"audit,omitempty"`
}

// V6TaskMigrationAuditEntry records one schema-5 value discarded or converted
// while building a schema-6 task. Repeated dependency fields are indexed so
// every discarded reference remains independently auditable.
type V6TaskMigrationAuditEntry struct {
	ProjectID string `json:"project_id,omitempty"`
	TaskID    string `json:"task_id"`
	Action    string `json:"action"`
	Field     string `json:"field"`
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
	Reason    string `json:"reason"`
}

// ConvertV5Task converts one validated schema-5 task without touching the
// filesystem or mutating the source record. All schema-6-retained collections
// and timestamp pointers are copied so callers can safely assemble and
// validate an isolated migration candidate.
func ConvertV5Task(source V5TaskRecord) (V6TaskConversionResult, error) {
	if err := ValidateV5TaskRecord(source); err != nil {
		return V6TaskConversionResult{}, fmt.Errorf("convert v5 task %q: %w", source.ID, err)
	}

	state, nextMember := convertV5TaskState(source.State)
	result := V6TaskConversionResult{
		Task: V6TaskRecord{
			ID:           source.ID,
			ShortID:      source.ShortID,
			Title:        source.Title,
			Description:  source.Description,
			State:        state,
			Priority:     convertV5TaskPriority(source.Priority),
			ParentID:     source.ParentID,
			Tags:         cloneStrings(source.Tags),
			CreatedAt:    source.CreatedAt,
			UpdatedAt:    source.UpdatedAt,
			CompletedAt:  cloneTimePointer(source.CompletedAt),
			DueDate:      source.DueDate,
			SnoozedUntil: source.SnoozedUntil,
			Pinned:       source.Pinned,
			PinnedAt:     cloneTimePointer(source.PinnedAt),
			Attachments:  convertV5TaskAttachments(source.Attachments),
			Checklist:    convertV5Checklist(source.Checklist),
			Comments:     mergeV5TaskComments(source.Comments, source.Decisions),
		},
		NextMember: nextMember,
	}

	for index, dependency := range source.DependsOn {
		result.Audit = append(result.Audit, V6TaskMigrationAuditEntry{
			TaskID: source.ID,
			Action: "discarded",
			Field:  fmt.Sprintf("depends_on[%d]", index),
			From:   dependency,
			Reason: "schema 6 retires task dependencies",
		})
	}
	if source.Recurrence != nil {
		result.Audit = append(result.Audit, V6TaskMigrationAuditEntry{
			TaskID: source.ID,
			Action: "discarded",
			Field:  "recurrence",
			From:   formatV5Recurrence(*source.Recurrence),
			Reason: "schema 6 retires task recurrence",
		})
	}
	if len(source.Decisions) > 0 {
		count := len(source.Decisions)
		result.Audit = append(result.Audit, V6TaskMigrationAuditEntry{
			TaskID: source.ID,
			Action: "converted",
			Field:  "decisions",
			From:   fmt.Sprintf("%d decisions", count),
			To:     fmt.Sprintf("%d comments", count),
			Reason: "schema 6 converts task decisions into comments",
		})
	}

	if err := ValidateV6TaskRecord(result.Task); err != nil {
		return V6TaskConversionResult{}, fmt.Errorf("convert v5 task %q: validate v6 candidate: %w", source.ID, err)
	}
	return result, nil
}

func convertV5TaskState(state string) (string, bool) {
	switch state {
	case "ready":
		return "backlog", false
	case "next":
		return "backlog", true
	case "in_progress":
		return "doing", false
	default:
		return state, false
	}
}

func convertV5TaskPriority(priority int) int {
	switch priority {
	case 1, 2:
		return 1
	case 4, 5:
		return 3
	default:
		return 0
	}
}

func convertV5TaskAttachments(source []V5TaskAttachment) []V6TaskAttachment {
	if source == nil {
		return nil
	}
	converted := make([]V6TaskAttachment, len(source))
	for index, attachment := range source {
		converted[index] = V6TaskAttachment{
			Kind:         attachment.Kind,
			Filename:     attachment.Filename,
			OriginalPath: attachment.OriginalPath,
			LineNumber:   attachment.LineNumber,
			AddedAt:      cloneTimePointer(attachment.AddedAt),
		}
	}
	return converted
}

func convertV5Checklist(source []V5ChecklistItem) []V6ChecklistItem {
	if source == nil {
		return nil
	}
	converted := make([]V6ChecklistItem, len(source))
	for index, item := range source {
		converted[index] = V6ChecklistItem(item)
	}
	return converted
}

func mergeV5TaskComments(comments []V5TaskComment, decisions []V5TaskDecision) []V6TaskComment {
	if comments == nil && decisions == nil {
		return nil
	}
	merged := make([]V6TaskComment, 0, len(comments)+len(decisions))
	for _, comment := range comments {
		merged = append(merged, V6TaskComment(comment))
	}
	for _, decision := range decisions {
		text := "Decision: " + decision.Text
		if decision.Rationale != "" {
			text += "\nRationale: " + decision.Rationale
		}
		merged = append(merged, V6TaskComment{
			Text:      text,
			CreatedAt: decision.CreatedAt,
		})
	}
	// Comments were appended before decisions, so a stable chronological sort
	// gives existing comments precedence at equal timestamps while retaining
	// source order within both collections.
	sort.SliceStable(merged, func(left, right int) bool {
		return merged[left].CreatedAt.Before(merged[right].CreatedAt)
	})
	return merged
}

func cloneStrings(source []string) []string {
	if source == nil {
		return nil
	}
	return append([]string(nil), source...)
}

func cloneTimePointer(source *time.Time) *time.Time {
	if source == nil {
		return nil
	}
	cloned := *source
	return &cloned
}
