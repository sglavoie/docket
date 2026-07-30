package format

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

type legacyTaskStatus string

const (
	legacyStatusTodo       legacyTaskStatus = "todo"
	legacyStatusInProgress legacyTaskStatus = "in_progress"
	legacyStatusReview     legacyTaskStatus = "review"
	legacyStatusStandby    legacyTaskStatus = "standby"
	legacyStatusDone       legacyTaskStatus = "done"
	legacyStatusBlocked    legacyTaskStatus = "blocked"
	legacyStatusCanceled   legacyTaskStatus = "canceled"
	legacyStatusMaybe      legacyTaskStatus = "maybe"
)

const (
	legacySectionInbox     = "Inbox"
	legacySectionNext      = "Next"
	legacySectionWaiting   = "Waiting"
	legacySectionStandby   = "Standby"
	legacySectionReview    = "Review"
	legacySectionSomeday   = "Someday"
	legacySectionCompleted = "Completed"
)

func parseLegacyTaskStatus(value string) (legacyTaskStatus, bool) {
	status := legacyTaskStatus(value)
	switch status {
	case legacyStatusTodo, legacyStatusInProgress, legacyStatusReview,
		legacyStatusStandby, legacyStatusDone, legacyStatusBlocked,
		legacyStatusCanceled, legacyStatusMaybe:
		return status, true
	default:
		return "", false
	}
}

func isLegacyGTDSection(value string) bool {
	switch value {
	case legacySectionInbox, legacySectionNext, legacySectionWaiting,
		legacySectionStandby, legacySectionReview, legacySectionSomeday,
		legacySectionCompleted:
		return true
	default:
		return false
	}
}

func deriveLegacyWorkflowState(status legacyTaskStatus, section, stage string) models.WorkflowState {
	switch {
	case status == legacyStatusCanceled:
		return models.StateCanceled
	case status == legacyStatusDone || section == legacySectionCompleted:
		return models.StateDone
	case status == legacyStatusReview || section == legacySectionReview || strings.EqualFold(stage, legacySectionReview):
		return models.StateReview
	case status == legacyStatusInProgress:
		return models.StateInProgress
	case status == legacyStatusBlocked || section == legacySectionWaiting:
		return models.StateWaiting
	case status == legacyStatusStandby || section == legacySectionStandby:
		return models.StatePaused
	case status == legacyStatusMaybe || section == legacySectionSomeday:
		return models.StateSomeday
	case section == legacySectionInbox:
		return models.StateInbox
	case section == legacySectionNext:
		return models.StateNext
	default:
		return models.StateReady
	}
}

// legacyLifecycleKeys are the metadata keys that schema v2 replaces with a
// single canonical `state` field. They are still parsed (and still written by
// the legacy v1 writer) during the migration window, but the v2 writer omits
// them in favor of `state`.
var legacyLifecycleKeys = map[string]bool{
	"status":  true,
	"section": true,
	"stage":   true,
	"active":  true,
}

// metadataStateFieldCodec parses the canonical schema-v2 `state` field straight
// into the task's stored WorkflowState field. Its writer is intentionally a
// no-op: the shared writer never emits `state`, and the v2
// writer injects it explicitly in appendWrittenPartsV2 from Task.State().
// Registering the codec here is what makes the metadata splitter recognize
// `state:` as a known key boundary when parsing a v2 document.
func metadataStateFieldCodec() metadataFieldCodec {
	return metadataField("state",
		func(task *models.Task, value string, state *metadataParseState, warnings *[]ParseWarning) {
			raw := strings.TrimSpace(value)
			if parsed, ok := models.ParseWorkflowState(raw); ok {
				task.ApplyWorkflowState(parsed)
				state.stateSeen = true
				return
			}
			if parsed, ok := models.ParseLegacyWorkflowState(raw); ok && parsed == models.StateReview {
				task.ApplyWorkflowState(models.StateReady)
				state.stateSeen = true
				state.legacyReviewSeen = true
				addLegacyReviewWarning(warnings)
				return
			}
			addValidationWarning(warnings, "state", "invalid state %q, ignoring", value)
		},
		noopMetadataWriter,
	)
}

// noopMetadataWriter is the shared writer for keys that the v2 writer never
// emits: the canonical `state` field (injected explicitly by
// appendWrittenPartsV2) and the retired schema-v1 lifecycle keys
// (status/section/stage/active). These codecs stay registered only so the
// metadata splitter still recognizes the keys as boundaries when parsing a
// schema-v1 document.
func noopMetadataWriter(_ *models.Task, _ *[]string) {}

// legacyStatusFieldCodec parses the schema-v1 `status` field into the parse
// accumulator instead of writing task.Status directly, so finalizeParsedMetadata
// controls when the legacy fields are applied and the canonical state derived.
// The v2 writer emits `state` in this key's position, so the writer is a no-op.
func legacyStatusFieldCodec() metadataFieldCodec {
	return metadataField("status",
		func(_ *models.Task, value string, state *metadataParseState, warnings *[]ParseWarning) {
			if status, ok := parseLegacyTaskStatus(value); ok {
				state.legacy.status = status
				state.legacy.statusSeen = true
				return
			}
			addValidationWarning(warnings, "status", "invalid status %q, using default", value)
		},
		noopMetadataWriter,
	)
}

// legacySectionFieldCodec parses the schema-v1 `section` field into the parse
// accumulator. No-op writer: v2 encodes section membership in `state`.
func legacySectionFieldCodec() metadataFieldCodec {
	return metadataField("section",
		func(_ *models.Task, value string, state *metadataParseState, _ *[]ParseWarning) {
			state.legacy.section = value
			state.legacy.sectionSeen = true
		},
		noopMetadataWriter,
	)
}

// legacyStageFieldCodec parses the schema-v1 `stage` field into the parse
// accumulator (unescaping the scalar as the old codec did). No-op writer: v2
// retires custom stages entirely.
func legacyStageFieldCodec() metadataFieldCodec {
	return metadataField("stage",
		func(_ *models.Task, value string, state *metadataParseState, _ *[]ParseWarning) {
			state.legacy.stage = unescapeMetadataScalar(value)
			state.legacy.stageSeen = true
		},
		noopMetadataWriter,
	)
}

// legacyActiveFieldCodec recognizes the schema-v1 `active` flag and discards its
// value. Keeping the no-op codec registered preserves metadata boundaries for
// old task files without reintroducing a queue migration or task-level meaning.
func legacyActiveFieldCodec() metadataFieldCodec {
	return metadataField("active",
		func(_ *models.Task, _ string, _ *metadataParseState, _ *[]ParseWarning) {},
		noopMetadataWriter,
	)
}

// appendWrittenPartsV2 renders a task's metadata parts for schema v2. It emits
// the canonical `state` field in the position the legacy `status` field would
// have occupied, drops every legacy lifecycle key (status/section/stage/active),
// and writes all other fields identically to v1. Ordering is deterministic so
// v2 output is a write→parse→write fixed point.
func (r metadataRegistry) appendWrittenPartsV2(task *models.Task, parts *[]string) {
	task = targetTaskForMetadataWrite(task)
	for _, field := range r.ordered {
		if field.key == "status" {
			appendMetadataPart(parts, "state", string(task.State()))
			continue
		}
		if legacyLifecycleKeys[field.key] {
			continue
		}
		field.write(task, parts)
	}
}

// targetTaskForMetadataWrite keeps the transitional v2 writer inside the
// target vocabulary even if an unremoved caller constructed a v4 Review task
// directly. The caller's task is never mutated.
func targetTaskForMetadataWrite(task *models.Task) *models.Task {
	if task == nil || task.State() != models.StateReview {
		return task
	}
	normalized := *task
	normalized.WorkflowState = models.StateReady
	normalized.Tags = append([]string(nil), task.Tags...)
	if !normalized.HasTag(models.LegacyReviewTag) {
		normalized.Tags = append(normalized.Tags, models.LegacyReviewTag)
	}
	return &normalized
}

func addLegacyReviewWarning(warnings *[]ParseWarning) {
	*warnings = append(*warnings, ParseWarning{
		Type:    WarningDeprecation,
		Field:   "state",
		Message: "retired Review state converted to Ready plus the review tag",
	})
}

// WriteMetadataV2 generates the schema-v2 metadata comment for a task: the
// canonical `state` field plus all non-lifecycle metadata, with the legacy
// status/section/stage/active fields removed.
func WriteMetadataV2(task *models.Task) string {
	var parts []string
	metadataFieldRegistry.appendWrittenPartsV2(task, &parts)
	return fmt.Sprintf("<!-- %s -->", strings.Join(parts, ", "))
}
