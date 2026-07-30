package format

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

type metadataParseFunc func(task *models.Task, value string, state *metadataParseState, warnings *[]ParseWarning)
type metadataWriteFunc func(task *models.Task, parts *[]string)

type metadataFieldCodec struct {
	key   string
	parse metadataParseFunc
	write metadataWriteFunc
}

type metadataRegistry struct {
	ordered []metadataFieldCodec
	keys    []string
	byKey   map[string]metadataFieldCodec
}

type metadataParseState struct {
	legacyRecurrenceSubFieldSeen bool
	// legacy accumulates the schema-v1 lifecycle fields (status/section/stage)
	// during parse. The v2 writer no longer emits these keys, but the parse-only
	// codecs retain enough information to derive the canonical WorkflowState when
	// a schema-v1 document carries no `state:` field. The retired `active` key is
	// recognized and discarded so old metadata continues to parse safely.
	legacy struct {
		status      legacyTaskStatus
		statusSeen  bool
		section     string
		sectionSeen bool
		stage       string
		stageSeen   bool
	}
	// stateSeen records whether a canonical schema-v2 `state:` field was parsed.
	// When true, finalizeParsedMetadata trusts the state the codec already
	// applied and skips legacy derivation.
	stateSeen bool
	// legacyReviewSeen records that either a v2 `state: review` value or the v1
	// lifecycle fields represented Review. Finalization preserves that meaning
	// as the target Ready state plus the `review` tag.
	legacyReviewSeen bool
}

func metadataField(key string, parse metadataParseFunc, write metadataWriteFunc) metadataFieldCodec {
	return metadataFieldCodec{
		key:   key,
		parse: parse,
		write: write,
	}
}

func newMetadataRegistry(fields []metadataFieldCodec) metadataRegistry {
	keys := make([]string, 0, len(fields))
	byKey := make(map[string]metadataFieldCodec, len(fields))
	for _, field := range fields {
		keys = append(keys, field.key)
		byKey[field.key] = field
	}

	return metadataRegistry{
		ordered: fields,
		keys:    keys,
		byKey:   byKey,
	}
}

func (r metadataRegistry) recognizesKey(key string) bool {
	if key == "" {
		return false
	}
	if _, ok := r.byKey[key]; ok {
		return true
	}
	return false
}

func (r metadataRegistry) parse(task *models.Task, key, value string, state *metadataParseState, warnings *[]ParseWarning) bool {
	if field, ok := r.byKey[key]; ok {
		field.parse(task, value, state, warnings)
		return true
	}
	return false
}

func (r metadataRegistry) nextSplitIndex(remaining string, seenKeys map[string]bool) int {
	splitIdx := -1

	for _, key := range r.keys {
		if seenKeys[key] {
			continue
		}
		pattern := ", " + key + ":"
		idx := findUnescapedMetadataDelimiter(remaining, pattern)
		if idx != -1 && (splitIdx == -1 || idx < splitIdx) {
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
		filename := escapeAttachmentField(att.Filename)
		var attStr string
		if att.IsURL {
			attStr = "url:" + filename
		} else if att.IsCodeRef {
			attStr = "code:" + filename
		} else if strings.HasPrefix(att.Filename, "url:") || strings.HasPrefix(att.Filename, "code:") || strings.HasPrefix(att.Filename, "file:") {
			// Disambiguate a plain attachment whose filename collides with a
			// type-prefix marker so the parser doesn't promote it to URL/code
			// on the next load.
			attStr = "file:" + filename
		} else {
			attStr = filename
		}
		if att.OriginalPath != "" {
			origEscaped := escapeAttachmentField(att.OriginalPath)
			if att.IsCodeRef && att.LineNumber > 0 {
				attStr += fmt.Sprintf("||%s:%d", origEscaped, att.LineNumber)
			} else {
				attStr += "||" + origEscaped
			}
		} else if att.IsCodeRef && att.LineNumber > 0 {
			// Preserve the line number even when OriginalPath is empty.
			// Without this the writer silently drops a non-zero LineNumber
			// because the line-number suffix lives in the OriginalPath segment.
			// Emit a sentinel empty path so the parser can still recover the
			// line number on the next load.
			attStr += fmt.Sprintf("||:%d", att.LineNumber)
		}
		attStrings = append(attStrings, attStr+fmt.Sprintf("@%d", att.AddedAt.Unix()))
	}

	appendMetadataPart(parts, "attachments", strings.Join(attStrings, ";"))
}

// escapeAttachmentField escapes backslash, semicolon, and comma in attachment
// fields (filename, OriginalPath, URL) so nested attachment delimiters and
// top-level metadata delimiters do not split a single attachment on round-trip.
// Backslash is escaped first so the unescape pass can distinguish a literal
// `\` from the escape prefix.
func escapeAttachmentField(s string) string {
	if !strings.ContainsAny(s, `\;,`) {
		return s
	}
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, ";", `\;`)
	s = strings.ReplaceAll(s, ",", `\,`)
	return s
}

// unescapeAttachmentField reverses escapeAttachmentField byte-by-byte to avoid
// the ambiguity that arises with chained ReplaceAll (`\\;` must become `\;`,
// not `\` + unescaped `;`).
func unescapeAttachmentField(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case '\\':
				b.WriteByte('\\')
				i++
				continue
			case ';':
				b.WriteByte(';')
				i++
				continue
			case ',':
				b.WriteByte(',')
				i++
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// splitAttachmentEntries splits an attachments metadata value on unescaped
// semicolons. Mirrors splitChecklistItems but yields trimmed, non-empty entries.
func splitAttachmentEntries(s string) []string {
	var entries []string
	var current strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			current.WriteByte(s[i])
			i++
			current.WriteByte(s[i])
			continue
		}
		if s[i] == ';' {
			entries = append(entries, current.String())
			current.Reset()
			continue
		}
		current.WriteByte(s[i])
	}
	entries = append(entries, current.String())
	return entries
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

func metadataFieldCodecs() []metadataFieldCodec {
	return metadataCoreFieldCodecs()
}

// metadataFieldRegistry is the single source of truth for metadata key
// recognition, parsing, and serialization order.
var metadataFieldRegistry = newMetadataRegistry(metadataFieldCodecs())
