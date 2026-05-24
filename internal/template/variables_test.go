package template

import (
	"strings"
	"testing"
)

func TestExtractVariables_Basic(t *testing.T) {
	tmpl := &TaskTemplate{
		Name:        "Setup for {{client}}",
		Description: "Onboard {{client}} with {{plan}}",
	}

	vars := ExtractVariables(tmpl)
	if len(vars) != 2 {
		t.Fatalf("expected 2 variables, got %d", len(vars))
	}
	if vars[0].Name != "client" || vars[0].Default != "" {
		t.Errorf("var 0: got %+v, want {Name:client Default:}", vars[0])
	}
	if vars[1].Name != "plan" || vars[1].Default != "" {
		t.Errorf("var 1: got %+v, want {Name:plan Default:}", vars[1])
	}
}

func TestExtractVariables_WithDefaults(t *testing.T) {
	tmpl := &TaskTemplate{
		Description: "Deploy to {{env:staging}} for {{client:Acme Corp}}",
	}

	vars := ExtractVariables(tmpl)
	if len(vars) != 2 {
		t.Fatalf("expected 2 variables, got %d", len(vars))
	}
	if vars[0].Name != "env" || vars[0].Default != "staging" {
		t.Errorf("var 0: got %+v, want {Name:env Default:staging}", vars[0])
	}
	if vars[1].Name != "client" || vars[1].Default != "Acme Corp" {
		t.Errorf("var 1: got %+v, want {Name:client Default:Acme Corp}", vars[1])
	}
}

func TestExtractVariables_Deduplication(t *testing.T) {
	tmpl := &TaskTemplate{
		Name:        "{{client}} setup",
		Description: "Welcome {{client}} to {{plan}}",
		Section:     "{{client}}",
	}

	vars := ExtractVariables(tmpl)
	if len(vars) != 2 {
		t.Fatalf("expected 2 unique variables, got %d", len(vars))
	}
	if vars[0].Name != "client" {
		t.Errorf("var 0 name: got %q, want %q", vars[0].Name, "client")
	}
	if vars[1].Name != "plan" {
		t.Errorf("var 1 name: got %q, want %q", vars[1].Name, "plan")
	}
}

func TestExtractVariables_AllFields(t *testing.T) {
	tmpl := &TaskTemplate{
		Name:        "{{f1}}",
		Description: "{{f2}}",
		Section:     "{{f3}}",
		Tags:        []string{"{{f4}}", "{{f5}}"},
		Checklist:   []string{"{{f6}}", "{{f7}}"},
		Children: []TemplateChild{
			{Title: "{{f8}}"},
			{Title: "{{f9}}"},
		},
	}

	vars := ExtractVariables(tmpl)
	if len(vars) != 9 {
		t.Fatalf("expected 9 variables, got %d", len(vars))
	}
	for i, v := range vars {
		expected := "f" + string(rune('1'+i))
		if v.Name != expected {
			t.Errorf("var %d: got %q, want %q", i, v.Name, expected)
		}
	}
}

func TestExtractVariables_Ordering(t *testing.T) {
	tmpl := &TaskTemplate{
		Description: "{{zebra}} then {{alpha}} then {{middle}}",
	}

	vars := ExtractVariables(tmpl)
	if len(vars) != 3 {
		t.Fatalf("expected 3 variables, got %d", len(vars))
	}
	if vars[0].Name != "zebra" || vars[1].Name != "alpha" || vars[2].Name != "middle" {
		t.Errorf("wrong order: got %v", vars)
	}
}

func TestExtractVariables_NoVariables(t *testing.T) {
	tmpl := &TaskTemplate{
		Name:        "Plain template",
		Description: "No variables here",
	}

	vars := ExtractVariables(tmpl)
	if len(vars) != 0 {
		t.Errorf("expected 0 variables, got %d", len(vars))
	}
}

func TestExtractVariables_UnderscoresAndNumbers(t *testing.T) {
	tmpl := &TaskTemplate{
		Description: "{{my_var_1}} and {{_private}} and {{A2B}}",
	}

	vars := ExtractVariables(tmpl)
	if len(vars) != 3 {
		t.Fatalf("expected 3 variables, got %d", len(vars))
	}
	if vars[0].Name != "my_var_1" {
		t.Errorf("var 0: got %q, want %q", vars[0].Name, "my_var_1")
	}
	if vars[1].Name != "_private" {
		t.Errorf("var 1: got %q, want %q", vars[1].Name, "_private")
	}
	if vars[2].Name != "A2B" {
		t.Errorf("var 2: got %q, want %q", vars[2].Name, "A2B")
	}
}

func TestApplyVariables_AllFields(t *testing.T) {
	tmpl := TaskTemplate{
		Name:        "Setup {{client}}",
		Description: "Onboard {{client}} with {{plan}}",
		Section:     "{{section_name}}",
		Tags:        []string{"{{tag1}}", "static", "{{tag2}}"},
		Checklist:   []string{"Do {{task}} first", "Then {{task}} again"},
		Children: []TemplateChild{
			{Title: "Child for {{client}}"},
		},
	}

	result := ApplyVariables(tmpl, map[string]string{
		"client":       "Acme",
		"plan":         "enterprise",
		"section_name": "Projects",
		"tag1":         "important",
		"tag2":         "urgent",
		"task":         "review",
	})

	if result.Name != "Setup Acme" {
		t.Errorf("Name: got %q, want %q", result.Name, "Setup Acme")
	}
	if result.Description != "Onboard Acme with enterprise" {
		t.Errorf("Description: got %q", result.Description)
	}
	if result.Section != "Projects" {
		t.Errorf("Section: got %q", result.Section)
	}
	if result.Tags[0] != "important" || result.Tags[1] != "static" || result.Tags[2] != "urgent" {
		t.Errorf("Tags: got %v", result.Tags)
	}
	if result.Checklist[0] != "Do review first" || result.Checklist[1] != "Then review again" {
		t.Errorf("Checklist: got %v", result.Checklist)
	}
	if result.Children[0].Title != "Child for Acme" {
		t.Errorf("Children[0].Title: got %q", result.Children[0].Title)
	}
}

func TestApplyVariables_MissingKeysEmpty(t *testing.T) {
	tmpl := TaskTemplate{
		Name:        "Setup {{client}}",
		Description: "Plan: {{plan}}",
	}

	result := ApplyVariables(tmpl, map[string]string{})

	if result.Name != "Setup " {
		t.Errorf("Name: got %q, want %q", result.Name, "Setup ")
	}
	if result.Description != "Plan: " {
		t.Errorf("Description: got %q, want %q", result.Description, "Plan: ")
	}
}

func TestApplyVariables_DefaultsUsedWhenMissing(t *testing.T) {
	tmpl := TaskTemplate{
		Description: "Deploy to {{env:staging}}",
	}

	result := ApplyVariables(tmpl, map[string]string{})

	if result.Description != "Deploy to staging" {
		t.Errorf("Description: got %q, want %q", result.Description, "Deploy to staging")
	}
}

func TestApplyVariables_DefaultsOverriddenByValue(t *testing.T) {
	tmpl := TaskTemplate{
		Description: "Deploy to {{env:staging}}",
	}

	result := ApplyVariables(tmpl, map[string]string{"env": "production"})

	if result.Description != "Deploy to production" {
		t.Errorf("Description: got %q, want %q", result.Description, "Deploy to production")
	}
}

func TestApplyVariables_DoesNotMutateOriginal(t *testing.T) {
	original := TaskTemplate{
		Name:      "{{client}}",
		Tags:      []string{"{{tag}}"},
		Checklist: []string{"{{item}}"},
		Children:  []TemplateChild{{Title: "{{child}}"}},
	}

	_ = ApplyVariables(original, map[string]string{
		"client": "Acme",
		"tag":    "new",
		"item":   "done",
		"child":  "resolved",
	})

	if original.Name != "{{client}}" {
		t.Error("original Name was mutated")
	}
	if original.Tags[0] != "{{tag}}" {
		t.Error("original Tags was mutated")
	}
	if original.Checklist[0] != "{{item}}" {
		t.Error("original Checklist was mutated")
	}
	if original.Children[0].Title != "{{child}}" {
		t.Error("original Children was mutated")
	}
}

func TestApplyVariables_NilSlices(t *testing.T) {
	tmpl := TaskTemplate{
		Name: "{{client}}",
	}

	result := ApplyVariables(tmpl, map[string]string{"client": "Acme"})

	if result.Name != "Acme" {
		t.Errorf("Name: got %q", result.Name)
	}
	if result.Tags != nil {
		t.Error("Tags should remain nil")
	}
	if result.Checklist != nil {
		t.Error("Checklist should remain nil")
	}
	if result.Children != nil {
		t.Error("Children should remain nil")
	}
}

func TestValidateTokens_Valid(t *testing.T) {
	tests := []struct {
		name string
		tmpl *TaskTemplate
	}{
		{"no variables", &TaskTemplate{Name: "Plain"}},
		{"simple variable", &TaskTemplate{Name: "{{client}}"}},
		{"variable with default", &TaskTemplate{Name: "{{env:staging}}"}},
		{"multiple variables", &TaskTemplate{Description: "{{a}} and {{b:default}}"}},
		{"underscores", &TaskTemplate{Description: "{{my_var}}"}},
		{"all fields", &TaskTemplate{
			Name:        "{{a}}",
			Description: "{{b}}",
			Section:     "{{c}}",
			Tags:        []string{"{{d}}"},
			Checklist:   []string{"{{e}}"},
			Children:    []TemplateChild{{Title: "{{f}}"}},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateTokens(tt.tmpl); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateTokens_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		tmpl    *TaskTemplate
		wantErr string
	}{
		{
			"empty braces",
			&TaskTemplate{Name: "{{}}"},
			"invalid variable token",
		},
		{
			"spaces inside",
			&TaskTemplate{Description: "{{ }}"},
			"invalid variable token",
		},
		{
			"space in name",
			&TaskTemplate{Description: "{{foo bar}}"},
			"invalid variable token",
		},
		{
			"starts with number",
			&TaskTemplate{Description: "{{1abc}}"},
			"invalid variable token",
		},
		{
			"special characters",
			&TaskTemplate{Description: "{{foo-bar}}"},
			"invalid variable token",
		},
		{
			"in tag",
			&TaskTemplate{Tags: []string{"{{bad token}}"}},
			"invalid variable token",
		},
		{
			"in checklist",
			&TaskTemplate{Checklist: []string{"{{}}"}},
			"invalid variable token",
		},
		{
			"in child title",
			&TaskTemplate{Children: []TemplateChild{{Title: "{{ }}"}}},
			"invalid variable token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTokens(tt.tmpl)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateTokens_LiteralBraces(t *testing.T) {
	tmpl := &TaskTemplate{
		Description: "Use { and } normally, or even {{ without closing",
	}
	if err := ValidateTokens(tmpl); err != nil {
		t.Errorf("literal braces should not cause error: %v", err)
	}
}

func TestValidateTokens_TripleBraces(t *testing.T) {
	tmpl := &TaskTemplate{
		Description: "{{{foo}}}",
	}
	// The inner {{foo}} is valid; the outer braces are just literal characters
	if err := ValidateTokens(tmpl); err != nil {
		t.Errorf("triple braces with valid inner should not error: %v", err)
	}
}

func TestHasVariables(t *testing.T) {
	tests := []struct {
		name string
		tmpl *TaskTemplate
		want bool
	}{
		{"no variables", &TaskTemplate{Name: "Plain"}, false},
		{"in name", &TaskTemplate{Name: "{{x}}"}, true},
		{"in description", &TaskTemplate{Description: "{{x}}"}, true},
		{"in section", &TaskTemplate{Section: "{{x}}"}, true},
		{"in tag", &TaskTemplate{Tags: []string{"{{x}}"}}, true},
		{"in checklist", &TaskTemplate{Checklist: []string{"{{x}}"}}, true},
		{"in child", &TaskTemplate{Children: []TemplateChild{{Title: "{{x}}"}}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasVariables(tt.tmpl); got != tt.want {
				t.Errorf("HasVariables() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVariableNames(t *testing.T) {
	tmpl := &TaskTemplate{
		Description: "{{alpha}} and {{beta:default}} and {{alpha}}",
	}

	names := VariableNames(tmpl)
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}
	if names[0] != "alpha" || names[1] != "beta" {
		t.Errorf("got %v, want [alpha beta]", names)
	}
}

func TestExtractVariables_EmptyDefault(t *testing.T) {
	tmpl := &TaskTemplate{
		Description: "{{name:}}",
	}

	vars := ExtractVariables(tmpl)
	if len(vars) != 1 {
		t.Fatalf("expected 1 variable, got %d", len(vars))
	}
	// Empty default is still a default (empty string)
	if vars[0].Name != "name" || vars[0].Default != "" {
		t.Errorf("got %+v", vars[0])
	}
}

func TestValidateTokens_NestedWrapperRejected(t *testing.T) {
	// Regression: "{{outer {{inner}}" looks like a valid {{inner}} wrapped in
	// garbage. The validator must reject the outer wrapper because the user's
	// intent is ambiguous and ApplyVariables would silently emit
	// "{{outer INNERVAL}}" — not what they typed.
	tmpl := &TaskTemplate{
		Description: "{{outer {{inner}}}}",
	}
	err := ValidateTokens(tmpl)
	if err == nil {
		t.Fatal("expected error for nested-looking wrapper, got nil")
	}
	if !strings.Contains(err.Error(), "invalid variable token") {
		t.Errorf("error %q should mention invalid variable token", err.Error())
	}
}

func TestValidateTokens_InvalidWithEmbeddedValidRejected(t *testing.T) {
	// Regression: a malformed token that happens to contain a valid token as
	// a substring must still be rejected.
	tmpl := &TaskTemplate{
		Description: "{{bad space {{ok}}",
	}
	err := ValidateTokens(tmpl)
	if err == nil {
		t.Fatal("expected error for invalid wrapper containing a valid token, got nil")
	}
}

func TestApplyVariables_MultipleOccurrences(t *testing.T) {
	tmpl := TaskTemplate{
		Description: "{{x}} and {{x}} and {{x}}",
	}

	result := ApplyVariables(tmpl, map[string]string{"x": "val"})
	if result.Description != "val and val and val" {
		t.Errorf("got %q", result.Description)
	}
}
