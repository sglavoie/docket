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
	"github.com/sglavoie/tt/internal/models"
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
	candidates := &captureCandidates{used: used, partial: payload.Text[start+len(def.Trigger) : caret], seen: map[string]bool{}}
	for _, value := range def.Values {
		candidates.add(value, def.Trigger+value)
	}
	for _, project := range projects {
		if project == nil || project.Archived {
			continue
		}
		if def.Field == textgrammar.FieldProject {
			candidates.add(project.Slug, "%"+project.Slug+" · "+project.Name)
		}
		for _, task := range project.Tasks {
			if task != nil {
				candidates.addTask(def, project, task, effective, projects)
			}
		}
	}
	candidates.addTemplates(def)
	sort.SliceStable(candidates.list, func(i, j int) bool {
		if candidates.list[i].score != candidates.list[j].score {
			return candidates.list[i].score > candidates.list[j].score
		}
		return candidates.list[i].value < candidates.list[j].value
	})
	for _, candidate := range candidates.list {
		result.Suggestions = append(result.Suggestions, candidates.suggestion(payload.Text, start, end, def, candidate))
	}
	return result, nil
}

type captureCandidate struct {
	value, label string
	score        int
}

// captureCandidates collects the values one sigil at the caret could complete
// to, each once, skipping what the line already carries.
type captureCandidates struct {
	used    []textgrammar.Token
	partial string
	seen    map[string]bool
	list    []captureCandidate
}

func (c *captureCandidates) add(value, label string) {
	if c.seen[value] {
		return
	}
	for _, token := range c.used {
		if strings.EqualFold(token.Value, value) {
			return
		}
	}
	score := fuzzy.Score(value+" "+label, c.partial)
	if score > 0 {
		c.seen[value] = true
		c.list = append(c.list, captureCandidate{value, label, score})
	}
}

// addTask offers what one task contributes to def's field.
func (c *captureCandidates) addTask(def textgrammar.Definition, project *models.Project, task *models.Task, effective *models.Project, projects []*models.Project) {
	switch def.Field { //nolint:exhaustive // Other fields use registry values or templates.
	case textgrammar.FieldTag:
		for _, tag := range task.Tags {
			c.add(tag, "#"+tag)
		}
	case textgrammar.FieldContext:
		if task.Context != "" {
			c.add(task.Context, "##"+task.Context)
		}
	case textgrammar.FieldParent, textgrammar.FieldChild, textgrammar.FieldDependency:
		if task.IsComplete() || effective == nil || (def.Field != textgrammar.FieldDependency && project.ID != effective.ID) {
			return
		}
		for _, token := range c.used {
			ref, resolveErr := capturetext.ResolveDependencyReference(token.Value, effective, projects)
			if resolveErr == nil && ref.Resolved && ref.Locator.ProjectID == project.ID && ref.Locator.TaskID == task.ID {
				return
			}
		}
		value := fmt.Sprint(task.ShortID)
		if project.ID != effective.ID {
			value = project.Slug + ":" + value
		}
		c.add(value, def.Trigger+value+" · "+task.Title+" · "+project.Name)
	case textgrammar.FieldAttachment:
		for _, attachment := range task.Attachments {
			if attachment.IsURL {
				c.add(attachment.Filename, "&"+attachment.Filename)
			}
		}
	}
}

// addTemplates offers the values no task stores: a link scheme to start from
// and the comment's closing delimiter.
func (c *captureCandidates) addTemplates(def textgrammar.Definition) {
	if def.Field == textgrammar.FieldAttachment {
		for _, scheme := range []string{"https://", "http://"} {
			if strings.HasPrefix(scheme, c.partial) {
				c.add(scheme, "&"+scheme+" — add a link")
			}
		}
	}
	if def.Field == textgrammar.FieldComment {
		if c.partial == "" {
			c.add("comment;", "Add a comment")
		} else if !strings.Contains(c.partial, ";") {
			c.add(c.partial+";", "Finish comment")
		}
	}
}

// suggestion is text with the span from start to end replaced by candidate.
func (c *captureCandidates) suggestion(text string, start, end int, def textgrammar.Definition, candidate captureCandidate) captureSuggestion {
	token := def.Trigger + candidate.value
	insert := token + " "
	replaceEnd := end
	if replaceEnd < len(text) && text[replaceEnd] == ' ' {
		replaceEnd++
	}
	if def.Field == textgrammar.FieldAttachment && (candidate.value == "https://" || candidate.value == "http://") {
		insert = token
		replaceEnd = end
	}
	prefix := text[:start] + insert
	suggestion := captureSuggestion{Token: token, Label: candidate.label, Text: prefix + text[replaceEnd:], Caret: utf16Length(prefix)}
	if def.Field == textgrammar.FieldComment && c.partial == "" {
		suggestion.Caret = utf16Length(text[:start]) + 1
		suggestion.SelectionLength = len("comment")
	}
	return suggestion
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
