package bridge

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/sglavoie/tt/internal/capturetext"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/textgrammar"
)

type captureSuggestionsPayload struct {
	Text      string `json:"text"`
	Caret     int    `json:"caret"` // UTF-16, as used by native text selections.
	ProjectID string `json:"project_id,omitempty"`
}

type captureSuggestion struct {
	Token           string `json:"token"`
	Label           string `json:"label"`
	Text            string `json:"text"`
	Caret           int    `json:"caret"`
	SelectionLength int    `json:"selection_length"`
}

type captureSuggestionsResult struct {
	Suggestions []captureSuggestion `json:"suggestions"`
}

// queryCaptureSuggestions is read-only. The engine owns grammar, vocabulary,
// and replacement boundaries; the native editor only applies a current reply.
func (b *Bridge) queryCaptureSuggestions(request Request) (any, error) {
	var payload captureSuggestionsPayload
	if err := decodePayload(request.Payload, &payload); err != nil {
		return nil, err
	}
	caret, ok := captureCaretByte(payload.Text, payload.Caret)
	if !ok {
		return nil, newError(CodeInvalidRequest, "caret must be a UTF-16 character boundary in text")
	}
	result := captureSuggestionsResult{Suggestions: []captureSuggestion{}}
	start, end, def := captureSuggestionSpan(payload.Text, caret)
	if def.Trigger == "" {
		return result, nil
	}
	if err := b.workspace.runtime.Refresh(); err != nil {
		return nil, err
	}
	source, err := b.captureSource(payload.ProjectID)
	if err != nil {
		return nil, err
	}
	snapshot, err := b.workspace.runtime.Snapshot()
	if err != nil {
		return nil, err
	}
	projects := snapshot.Projects()
	lineStart := strings.LastIndex(payload.Text[:start], "\n") + 1
	lineEnd := len(payload.Text)
	if next := strings.IndexByte(payload.Text[end:], '\n'); next >= 0 {
		lineEnd = end + next
	}
	parsed := textgrammar.Parse(payload.Text[lineStart:start]+payload.Text[end:lineEnd], textgrammar.SurfaceCapture)
	used := parsed.TokensFor(def.Field)
	if def.Cardinality == textgrammar.CardinalitySingleton && len(used) > 0 {
		return result, nil
	}
	effective := source
	if routes := parsed.TokensFor(textgrammar.FieldProject); len(routes) > 0 {
		effective = capturetext.ResolveProject(routes[len(routes)-1].Value, projects)
	}
	partial := payload.Text[start+len(def.Trigger) : caret]
	type candidate struct {
		value, label string
		score        int
	}
	var candidates []candidate
	seen := map[string]bool{}
	add := func(value, label string) {
		if seen[value] {
			return
		}
		for _, token := range used {
			if strings.EqualFold(token.Value, value) {
				return
			}
		}
		score := fuzzy.Score(value+" "+label, partial)
		if score > 0 {
			seen[value] = true
			candidates = append(candidates, candidate{value, label, score})
		}
	}
	for _, value := range def.Values {
		add(value, def.Trigger+value)
	}
	for _, project := range projects {
		if project == nil || project.Archived {
			continue
		}
		if def.Field == textgrammar.FieldProject {
			add(project.Slug, "%"+project.Slug+" · "+project.Name)
		}
		for _, task := range project.Tasks {
			if task == nil {
				continue
			}
			switch def.Field { //nolint:exhaustive // Other fields use registry values or templates below.
			case textgrammar.FieldTag:
				for _, tag := range task.Tags {
					add(tag, "#"+tag)
				}
			case textgrammar.FieldContext:
				if task.Context != "" {
					add(task.Context, "##"+task.Context)
				}
			case textgrammar.FieldParent, textgrammar.FieldChild, textgrammar.FieldDependency:
				if task.IsComplete() || effective == nil || (def.Field != textgrammar.FieldDependency && project.ID != effective.ID) {
					continue
				}
				alreadyUsed := false
				for _, token := range used {
					ref, resolveErr := capturetext.ResolveDependencyReference(token.Value, effective, projects)
					if resolveErr == nil && ref.Resolved && ref.Locator.ProjectID == project.ID && ref.Locator.TaskID == task.ID {
						alreadyUsed = true
					}
				}
				if alreadyUsed {
					continue
				}
				value := fmt.Sprint(task.ShortID)
				if project.ID != effective.ID {
					value = project.Slug + ":" + value
				}
				add(value, def.Trigger+value+" · "+task.Title+" · "+project.Name)
			case textgrammar.FieldAttachment:
				for _, attachment := range task.Attachments {
					if attachment.IsURL {
						add(attachment.Filename, "&"+attachment.Filename)
					}
				}
			}
		}
	}
	if def.Field == textgrammar.FieldAttachment {
		for _, scheme := range []string{"https://", "http://"} {
			if strings.HasPrefix(scheme, partial) {
				add(scheme, "&"+scheme+" — add a link")
			}
		}
	}
	if def.Field == textgrammar.FieldComment {
		if partial == "" {
			add("comment;", "Add a comment")
		} else if !strings.Contains(partial, ";") {
			add(partial+";", "Finish comment")
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].value < candidates[j].value
	})
	for _, candidate := range candidates {
		token := def.Trigger + candidate.value
		insert := token + " "
		replaceEnd := end
		if replaceEnd < len(payload.Text) && payload.Text[replaceEnd] == ' ' {
			replaceEnd++
		}
		if def.Field == textgrammar.FieldAttachment && (candidate.value == "https://" || candidate.value == "http://") {
			insert = token
			replaceEnd = end
		}
		prefix := payload.Text[:start] + insert
		suggestion := captureSuggestion{Token: token, Label: candidate.label, Text: prefix + payload.Text[replaceEnd:], Caret: utf16Length(prefix)}
		if def.Field == textgrammar.FieldComment && partial == "" {
			suggestion.Caret = utf16Length(payload.Text[:start]) + 1
			suggestion.SelectionLength = len("comment")
		}
		result.Suggestions = append(result.Suggestions, suggestion)
	}
	return result, nil
}

func utf16Length(value string) int { return len(utf16.Encode([]rune(value))) }

func captureCaretByte(text string, caret int) (int, bool) {
	units := 0
	for offset, r := range text {
		if units == caret {
			return offset, true
		}
		units += utf16.RuneLen(r)
	}
	return len(text), units == caret
}

// Scan one logical line, protecting comment and quoted attachment spans. Date
// spans come from the grammar so multiword dates replace as a single token.
func captureSuggestionSpan(text string, caret int) (int, int, textgrammar.Definition) {
	lineStart := strings.LastIndex(text[:caret], "\n") + 1
	lineEnd := len(text)
	if next := strings.IndexByte(text[caret:], '\n'); next >= 0 {
		lineEnd = caret + next
	}
	definitions := textgrammar.Definitions(textgrammar.SurfaceCapture)
	for start := lineStart; start < lineEnd; {
		r, size := utf8.DecodeRuneInString(text[start:lineEnd])
		if unicode.IsSpace(r) {
			start += size
			continue
		}
		end := lineEnd
		for offset, r := range text[start:lineEnd] {
			if unicode.IsSpace(r) {
				end = start + offset
				break
			}
		}
		var def textgrammar.Definition
		previous, _ := utf8.DecodeLastRuneInString(text[:start])
		if start == lineStart || unicode.IsSpace(previous) {
			for _, candidate := range definitions {
				if strings.HasPrefix(text[start:lineEnd], candidate.Trigger) && len(candidate.Trigger) > len(def.Trigger) {
					def = candidate
				}
			}
		}
		switch def.Field { //nolint:exhaustive // Only these families span whitespace.
		case textgrammar.FieldComment:
			end = lineEnd
			if close := strings.IndexByte(text[start+1:lineEnd], ';'); close >= 0 {
				end = start + 1 + close + 1
			}
		case textgrammar.FieldAttachment:
			end = captureAttachmentEnd(text, start+1, lineEnd)
		case textgrammar.FieldDue, textgrammar.FieldDefer:
			for _, token := range textgrammar.Parse(text[start:lineEnd], textgrammar.SurfaceCapture).TokensFor(def.Field) {
				if token.Start == 0 {
					end = start + len(token.Form) + len(token.Value)
					break
				}
			}
		}
		if start < caret && caret <= end {
			if def.Field == textgrammar.FieldComment && (caret < end || (end > start+1 && text[end-1] == ';')) {
				return 0, 0, textgrammar.Definition{}
			}
			if caret < start+len(def.Trigger) {
				return 0, 0, textgrammar.Definition{}
			}
			return start, end, def
		}
		start = end
	}
	return 0, 0, textgrammar.Definition{}
}

func captureAttachmentEnd(text string, start, limit int) int {
	var quote byte
	if start < limit && (text[start] == '\'' || text[start] == '"') {
		quote = text[start]
		start++
	}
	for offset := start; offset < limit; {
		if text[offset] == '\\' && offset+1 < limit {
			_, size := utf8.DecodeRuneInString(text[offset+1 : limit])
			offset += 1 + size
			continue
		}
		if quote != 0 && text[offset] == quote {
			return offset + 1
		}
		r, size := utf8.DecodeRuneInString(text[offset:limit])
		if quote == 0 && unicode.IsSpace(r) {
			return offset
		}
		offset += size
	}
	return limit
}
