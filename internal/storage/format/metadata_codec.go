package format

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

type metadataParseFunc func(task *models.Task, value string, state *metadataParseState, warnings *[]ParseWarning)
type metadataWriteFunc func(task *models.Task, parts *[]string)
type metadataDynamicParseFunc func(task *models.Task, key, value string, state *metadataParseState, warnings *[]ParseWarning)
type metadataDynamicIndexFunc func(s string) (idx int, key string)

type metadataFieldCodec struct {
	key      string
	taskType models.TaskType
	parse    metadataParseFunc
	write    metadataWriteFunc
}

type metadataFieldFamilyCodec struct {
	recognizes  func(key string) bool
	parseMatch  func(key string) bool
	parse       metadataDynamicParseFunc
	write       metadataWriteFunc
	findSplitAt metadataDynamicIndexFunc
}

type metadataRegistry struct {
	ordered  []metadataFieldCodec
	keys     []string
	byKey    map[string]metadataFieldCodec
	families []metadataFieldFamilyCodec
}

type metadataParseState struct {
	recurSubFields struct {
		skipped     int
		streak      int
		maxStreak   int
		originalDay int
		hasAny      bool
	}
}

func metadataField(key string, parse metadataParseFunc, write metadataWriteFunc) metadataFieldCodec {
	return metadataFieldCodec{
		key:   key,
		parse: parse,
		write: write,
	}
}

func typedMetadataField(key string, taskType models.TaskType, parse metadataParseFunc, write metadataWriteFunc) metadataFieldCodec {
	return metadataFieldCodec{
		key:      key,
		taskType: taskType,
		parse:    parse,
		write:    write,
	}
}

func newMetadataRegistry(fields []metadataFieldCodec, families ...metadataFieldFamilyCodec) metadataRegistry {
	keys := make([]string, 0, len(fields))
	byKey := make(map[string]metadataFieldCodec, len(fields))
	for _, field := range fields {
		keys = append(keys, field.key)
		byKey[field.key] = field
	}

	return metadataRegistry{
		ordered:  fields,
		keys:     keys,
		byKey:    byKey,
		families: families,
	}
}

func (r metadataRegistry) recognizesKey(key string) bool {
	if key == "" {
		return false
	}
	if _, ok := r.byKey[key]; ok {
		return true
	}
	for _, family := range r.families {
		if family.recognizes != nil && family.recognizes(key) {
			return true
		}
	}
	return false
}

func (r metadataRegistry) parse(task *models.Task, key, value string, state *metadataParseState, warnings *[]ParseWarning) bool {
	if field, ok := r.byKey[key]; ok {
		if field.taskType != "" && task.Type != field.taskType {
			*warnings = append(*warnings, ParseWarning{
				Type:    WarningValidation,
				Field:   key,
				Message: fmt.Sprintf("%s attribute on %s task, skipping", key, task.Type),
			})
			return true
		}
		field.parse(task, value, state, warnings)
		return true
	}

	for _, family := range r.families {
		match := family.recognizes
		if family.parseMatch != nil {
			match = family.parseMatch
		}
		if match != nil && match(key) {
			family.parse(task, key, value, state, warnings)
			return true
		}
	}

	return false
}

func (r metadataRegistry) appendWrittenParts(task *models.Task, parts *[]string) {
	for _, field := range r.ordered {
		field.write(task, parts)
	}
	for _, family := range r.families {
		family.write(task, parts)
	}
}

func (r metadataRegistry) nextSplitIndex(remaining string, seenKeys map[string]bool) int {
	splitIdx := -1

	for _, key := range r.keys {
		if seenKeys[key] {
			continue
		}
		pattern := ", " + key + ":"
		idx := strings.Index(remaining, pattern)
		if idx != -1 && (splitIdx == -1 || idx < splitIdx) {
			splitIdx = idx
		}
	}

	for _, family := range r.families {
		if family.findSplitAt == nil {
			continue
		}
		idx, key := family.findSplitAt(remaining)
		if idx != -1 && !seenKeys[key] && (splitIdx == -1 || idx < splitIdx) {
			splitIdx = idx
		}
	}

	return splitIdx
}

func appendMetadataPart(parts *[]string, key, value string) {
	*parts = append(*parts, fmt.Sprintf("%s: %s", key, value))
}

func appendTrueMetadataPart(parts *[]string, key string, value bool) {
	if value {
		appendMetadataPart(parts, key, "true")
	}
}

func parseChecklistMetadata(task *models.Task, value string) {
	items := splitChecklistItems(value)
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		done := false
		text := item
		if strings.HasSuffix(item, "[x]") {
			done = true
			text = strings.TrimSuffix(item, "[x]")
		} else if before, ok := strings.CutSuffix(item, "[ ]"); ok {
			text = before
		}
		text = unescapeChecklistText(strings.TrimSpace(text))
		if text != "" {
			task.Checklist = append(task.Checklist, models.ChecklistItem{Text: text, Done: done})
		}
	}
}

func writeAttachmentsMetadata(task *models.Task, parts *[]string) {
	if len(task.Attachments) == 0 {
		return
	}

	var attStrings []string
	for _, att := range task.Attachments {
		var attStr string
		if att.IsURL {
			attStr = "url:" + att.Filename
		} else if att.IsCodeRef {
			attStr = "code:" + att.Filename
		} else {
			attStr = att.Filename
		}
		if att.OriginalPath != "" {
			if att.IsCodeRef && att.LineNumber > 0 {
				attStr += fmt.Sprintf("||%s:%d", att.OriginalPath, att.LineNumber)
			} else {
				attStr += "||" + att.OriginalPath
			}
		}
		attStrings = append(attStrings, attStr+fmt.Sprintf("@%d", att.AddedAt.Unix()))
	}

	appendMetadataPart(parts, "attachments", strings.Join(attStrings, ";"))
}

func writeChecklistMetadata(task *models.Task, parts *[]string) {
	if len(task.Checklist) == 0 {
		return
	}

	var checklistStrings []string
	for _, item := range task.Checklist {
		marker := "[ ]"
		if item.Done {
			marker = "[x]"
		}
		checklistStrings = append(checklistStrings, escapeChecklistText(item.Text)+marker)
	}

	appendMetadataPart(parts, "checklist", strings.Join(checklistStrings, ";"))
}

func findScoreMetadataSplitIndex(s string) (int, string) {
	idx := findScoreKeyIndex(s)
	if idx == -1 {
		return -1, ""
	}
	return idx, extractScoreKey(s[idx+2:])
}

func parseScoreMetadata(task *models.Task, key, value string, warnings *[]ParseWarning) {
	dimID := strings.TrimPrefix(key, "score.")
	if !isValidDimensionID(dimID) {
		*warnings = append(*warnings, ParseWarning{
			Type:    WarningValidation,
			Field:   key,
			Message: fmt.Sprintf("invalid score dimension ID '%s' (must be 1-%d alphanumeric/underscore chars), skipping", dimID, MaxDimensionIDLength),
		})
		return
	}

	v, err := strconv.Atoi(value)
	if err != nil {
		*warnings = append(*warnings, ParseWarning{
			Type:    WarningValidation,
			Field:   key,
			Message: fmt.Sprintf("invalid score value '%s' for dimension '%s', skipping", value, dimID),
		})
		return
	}

	if task.ScoreDimensions == nil {
		task.ScoreDimensions = make(map[string]int)
	}
	task.ScoreDimensions[dimID] = v
}

func writeScoreMetadata(task *models.Task, parts *[]string) {
	if len(task.ScoreDimensions) == 0 {
		return
	}

	for _, key := range sortedKeys(task.ScoreDimensions) {
		if key == "" || !isValidDimensionID(key) {
			continue
		}
		appendMetadataPart(parts, "score."+key, strconv.Itoa(task.ScoreDimensions[key]))
	}
}

func metadataFieldCodecs() []metadataFieldCodec {
	return append(metadataCoreFieldCodecs(), metadataTypedFieldCodecs()...)
}

func metadataScoreFieldFamily() metadataFieldFamilyCodec {
	return metadataFieldFamilyCodec{
		recognizes: func(key string) bool {
			return strings.HasPrefix(key, "score.") && isValidDimensionID(strings.TrimPrefix(key, "score."))
		},
		parseMatch: func(key string) bool {
			return strings.HasPrefix(key, "score.")
		},
		parse: func(task *models.Task, key, value string, _ *metadataParseState, warnings *[]ParseWarning) {
			parseScoreMetadata(task, key, value, warnings)
		},
		write:       writeScoreMetadata,
		findSplitAt: findScoreMetadataSplitIndex,
	}
}

// metadataFieldRegistry is the single source of truth for metadata key
// recognition, parsing, and serialization order.
var metadataFieldRegistry = newMetadataRegistry(
	metadataFieldCodecs(),
	metadataScoreFieldFamily(),
)
