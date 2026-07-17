package format

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

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
// into the task's stored WorkflowState field (via Task.ApplyWorkflowState, which
// also reconstitutes the legacy Status/Section shims so domain code that still
// reads them keeps working during the migration window). Its writer is
// intentionally a no-op: the shared (v1) writer never emits `state`, and the v2
// writer injects it explicitly in appendWrittenPartsV2 from Task.State().
// Registering the codec here is what makes the metadata splitter recognize
// `state:` as a known key boundary when parsing a v2 document.
func metadataStateFieldCodec() metadataFieldCodec {
	return metadataField("state",
		func(task *models.Task, value string, state *metadataParseState, warnings *[]ParseWarning) {
			parsed, ok := models.ParseWorkflowState(strings.TrimSpace(value))
			if !ok {
				addValidationWarning(warnings, "state", "invalid state %q, ignoring", value)
				return
			}
			task.ApplyWorkflowState(parsed)
			state.stateSeen = true
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
			if status, ok := models.ParseTaskStatus(value); ok {
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
	for _, family := range r.families {
		family.write(task, parts)
	}
}

// WriteMetadataV2 generates the schema-v2 metadata comment for a task: the
// canonical `state` field plus all non-lifecycle metadata, with the legacy
// status/section/stage/active fields removed.
func WriteMetadataV2(task *models.Task) string {
	var parts []string
	metadataFieldRegistry.appendWrittenPartsV2(task, &parts)
	return fmt.Sprintf("<!-- %s -->", strings.Join(parts, ", "))
}
