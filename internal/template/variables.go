package template

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// TemplateVariable represents a variable placeholder found in a template.
type TemplateVariable struct {
	Name    string
	Default string
}

// validVarPattern matches {{name}} or {{name:default_value}} tokens.
var validVarPattern = regexp.MustCompile(`\{\{([a-zA-Z_][a-zA-Z0-9_]*)(?::([^}]*))?\}\}`)

// invalidVarPattern matches anything that looks like {{...}} but may not be valid.
var invalidVarPattern = regexp.MustCompile(`(?s)\{\{.*?\}\}`)

func tokenHasDefault(token, name string) bool {
	idx := 2 + len(name)
	return idx < len(token)-2 && token[idx] == ':'
}

// ExtractVariables scans all text fields of a TaskTemplate and returns unique
// variables in order of first appearance.
func ExtractVariables(t *TaskTemplate) []TemplateVariable {
	if t == nil {
		return nil
	}
	seen := make(map[string]int)
	seenDefault := make(map[string]bool)
	var vars []TemplateVariable

	extract := func(s string) {
		for _, match := range validVarPattern.FindAllStringSubmatch(s, -1) {
			name := match[1]
			hasDefault := tokenHasDefault(match[0], name)
			if idx, ok := seen[name]; ok {
				if !seenDefault[name] && hasDefault {
					vars[idx].Default = match[2]
					seenDefault[name] = true
				}
				continue
			}
			seen[name] = len(vars)
			seenDefault[name] = hasDefault
			vars = append(vars, TemplateVariable{
				Name:    name,
				Default: match[2],
			})
		}
	}

	extract(t.Name)
	extract(t.Description)
	for _, tag := range t.Tags {
		extract(tag)
	}
	for _, item := range t.Checklist {
		extract(item)
	}
	for _, child := range t.Children {
		extract(child.Title)
	}

	return vars
}

// ApplyVariables returns a shallow copy of the template with all variable
// tokens replaced by the provided values. Missing keys fall back to the local
// token default, then to any default declared elsewhere in the template for
// the same variable, then to empty string. The cross-field fallback keeps
// occurrences of the same variable consistent even when only one occurrence
// declares a default (e.g. "{{b:beta}}" in Name and "{{b}}" in Checklist).
// Tags, Checklist, and Children slices are deep-copied.
func ApplyVariables(t TaskTemplate, values map[string]string) TaskTemplate {
	globalDefaults := make(map[string]string)
	extractDefaults := func(s string) {
		for _, match := range validVarPattern.FindAllStringSubmatch(s, -1) {
			if match[2] == "" {
				continue
			}
			if _, ok := globalDefaults[match[1]]; !ok {
				globalDefaults[match[1]] = match[2]
			}
		}
	}
	extractDefaults(t.Name)
	extractDefaults(t.Description)
	for _, tag := range t.Tags {
		extractDefaults(tag)
	}
	for _, item := range t.Checklist {
		extractDefaults(item)
	}
	for _, child := range t.Children {
		extractDefaults(child.Title)
	}

	replace := func(s string) string {
		return validVarPattern.ReplaceAllStringFunc(s, func(token string) string {
			match := validVarPattern.FindStringSubmatch(token)
			name := match[1]
			if v, ok := values[name]; ok {
				return v
			}
			// An explicit ":" in the token means the author chose a local default,
			// even if it is empty ("{{name:}}"). That choice must beat the
			// cross-field fallback so the rendered output matches what the user
			// typed; otherwise "{{name:}}" silently inherits another field's
			// default and renders non-empty.
			if tokenHasDefault(token, name) {
				return match[2]
			}
			if v, ok := globalDefaults[name]; ok {
				return v
			}
			return ""
		})
	}

	t.Name = replace(t.Name)
	t.Description = replace(t.Description)

	if t.Tags != nil {
		tags := make([]string, len(t.Tags))
		for i, tag := range t.Tags {
			tags[i] = replace(tag)
		}
		t.Tags = tags
	}

	if t.Checklist != nil {
		checklist := make([]string, len(t.Checklist))
		for i, item := range t.Checklist {
			checklist[i] = replace(item)
		}
		t.Checklist = checklist
	}

	if t.Children != nil {
		children := make([]TemplateChild, len(t.Children))
		for i, child := range t.Children {
			children[i] = child
			children[i].Title = replace(child.Title)
		}
		t.Children = children
	}

	return t
}

// ValidateTokens scans all text fields for anything that looks like {{...}}
// but doesn't match the valid identifier pattern. Returns a descriptive error
// for the first invalid token found.
func ValidateTokens(t *TaskTemplate) error {
	if t == nil {
		return nil
	}
	validate := func(field, s string) error {
		for _, token := range invalidVarPattern.FindAllString(s, -1) {
			// A token is OK if it is itself a valid token, OR if stripping a
			// single leading literal "{" yields a valid token. This tolerates
			// accidental literal braces like "{{{foo}}}" while still rejecting
			// malformed wrappers like "{{outer {{inner}}" where extra text
			// sits between the outer "{{" and the inner "{{".
			if validVarPattern.FindString(token) == token {
				if strings.Contains(token[2:], "{{") {
					return fmt.Errorf("invalid variable token %q in %s", token, field)
				}
				continue
			}
			if len(token) > 0 && token[0] == '{' &&
				validVarPattern.FindString(token[1:]) == token[1:] {
				continue
			}
			return fmt.Errorf("invalid variable token %q in %s", token, field)
		}
		return nil
	}

	fields := []struct {
		name  string
		value string
	}{
		{"name", t.Name},
		{"description", t.Description},
	}

	for _, f := range fields {
		if err := validate(f.name, f.value); err != nil {
			return err
		}
	}

	for i, tag := range t.Tags {
		if err := validate(fmt.Sprintf("tag %d", i+1), tag); err != nil {
			return err
		}
	}

	for i, item := range t.Checklist {
		if err := validate(fmt.Sprintf("checklist item %d", i+1), item); err != nil {
			return err
		}
	}

	for i, child := range t.Children {
		if err := validate(fmt.Sprintf("child %d title", i+1), child.Title); err != nil {
			return err
		}
	}

	return nil
}

// HasVariables returns true if the template contains any variable placeholders.
func HasVariables(t *TaskTemplate) bool {
	if t == nil {
		return false
	}
	check := func(s string) bool {
		return validVarPattern.MatchString(s)
	}

	if check(t.Name) || check(t.Description) {
		return true
	}
	if slices.ContainsFunc(t.Tags, check) {
		return true
	}
	if slices.ContainsFunc(t.Checklist, check) {
		return true
	}
	for _, child := range t.Children {
		if check(child.Title) {
			return true
		}
	}
	return false
}

// VariableNames returns just the names of all variables in the template.
func VariableNames(t *TaskTemplate) []string {
	vars := ExtractVariables(t)
	names := make([]string, len(vars))
	for i, v := range vars {
		names[i] = v.Name
	}
	return names
}
