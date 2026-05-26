package template

import (
	"fmt"
	"regexp"
	"slices"
)

// TemplateVariable represents a variable placeholder found in a template.
type TemplateVariable struct {
	Name    string
	Default string
}

// validVarPattern matches {{name}} or {{name:default_value}} tokens.
var validVarPattern = regexp.MustCompile(`\{\{([a-zA-Z_][a-zA-Z0-9_]*)(?::([^}]*))?\}\}`)

// invalidVarPattern matches anything that looks like {{...}} but may not be valid.
var invalidVarPattern = regexp.MustCompile(`\{\{[^}]*\}\}`)

// ExtractVariables scans all text fields of a TaskTemplate and returns unique
// variables in order of first appearance.
func ExtractVariables(t *TaskTemplate) []TemplateVariable {
	seen := make(map[string]int)
	var vars []TemplateVariable

	extract := func(s string) {
		for _, match := range validVarPattern.FindAllStringSubmatch(s, -1) {
			name := match[1]
			if idx, ok := seen[name]; ok {
				if vars[idx].Default == "" && match[2] != "" {
					vars[idx].Default = match[2]
				}
				continue
			}
			seen[name] = len(vars)
			vars = append(vars, TemplateVariable{
				Name:    name,
				Default: match[2],
			})
		}
	}

	extract(t.Name)
	extract(t.Description)
	extract(t.Section)
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
// tokens replaced by the provided values. Missing keys are replaced with
// empty string. Tags, Checklist, and Children slices are deep-copied.
func ApplyVariables(t TaskTemplate, values map[string]string) TaskTemplate {
	replace := func(s string) string {
		return validVarPattern.ReplaceAllStringFunc(s, func(token string) string {
			match := validVarPattern.FindStringSubmatch(token)
			name := match[1]
			if v, ok := values[name]; ok {
				return v
			}
			if match[2] != "" {
				return match[2]
			}
			return ""
		})
	}

	t.Name = replace(t.Name)
	t.Description = replace(t.Description)
	t.Section = replace(t.Section)

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
	validate := func(field, s string) error {
		for _, token := range invalidVarPattern.FindAllString(s, -1) {
			// A token is OK if it is itself a valid token, OR if stripping a
			// single leading literal "{" yields a valid token. This tolerates
			// accidental literal braces like "{{{foo}}}" while still rejecting
			// malformed wrappers like "{{outer {{inner}}" where extra text
			// sits between the outer "{{" and the inner "{{".
			if validVarPattern.FindString(token) == token {
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
		{"section", t.Section},
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
	check := func(s string) bool {
		return validVarPattern.MatchString(s)
	}

	if check(t.Name) || check(t.Description) || check(t.Section) {
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
