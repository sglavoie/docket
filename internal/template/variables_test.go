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

func TestExtractVariables_LaterDefaultBackfillsFirstOccurrence(t *testing.T) {
	tmpl := &TaskTemplate{
		Name:        "{{env}} deployment",
		Description: "Deploy to {{env:staging}}",
	}

	vars := ExtractVariables(tmpl)
	if len(vars) != 1 {
		t.Fatalf("expected 1 variable, got %d", len(vars))
	}
	if vars[0].Default != "staging" {
		t.Fatalf("Default = %q, want %q", vars[0].Default, "staging")
	}
}

func TestExtractVariables_LaterMissingDefaultDoesNotClearExistingDefault(t *testing.T) {
	tmpl := &TaskTemplate{
		Name:        "{{env:staging}} deployment",
		Description: "Deploy to {{env}}",
	}

	vars := ExtractVariables(tmpl)
	if len(vars) != 1 {
		t.Fatalf("expected 1 variable, got %d", len(vars))
	}
	if vars[0].Default != "staging" {
		t.Fatalf("Default = %q, want %q", vars[0].Default, "staging")
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

func TestExtractVariables_ExplicitEmptyDefaultPreventsLaterBackfill(t *testing.T) {
	tmpl := &TaskTemplate{
		Name:        "{{env:}} deployment",
		Description: "Deploy to {{env:staging}}",
	}

	vars := ExtractVariables(tmpl)
	if len(vars) != 1 {
		t.Fatalf("expected 1 variable, got %d", len(vars))
	}
	if vars[0].Default != "" {
		t.Fatalf("Default = %q, want empty (first explicit empty default should win)", vars[0].Default)
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

func TestValidateTokens_NestedTokenInDefaultRejected(t *testing.T) {
	// Regression: the valid token pattern allows "{" in defaults, so
	// "{{outer:{{inner}}}}" was accepted as outer's default "{{inner" plus a
	// trailing "}}". Applying variables could then leave a fresh "{{inner}}"
	// token in the rendered template.
	tmpl := &TaskTemplate{
		Description: "{{outer:{{inner}}}}",
	}
	err := ValidateTokens(tmpl)
	if err == nil {
		t.Fatal("expected error for nested-looking token in default, got nil")
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

func TestValidateTokens_StrayClosingBraceInsideTokenRejected(t *testing.T) {
	// Regression: valid variable defaults cannot contain "}", so a token like
	// this must fail validation instead of being left unresolved later.
	tmpl := &TaskTemplate{
		Description: "{{env:stag}ing}}",
	}
	err := ValidateTokens(tmpl)
	if err == nil {
		t.Fatal("expected error for token with stray closing brace, got nil")
	}
	if !strings.Contains(err.Error(), "invalid variable token") {
		t.Errorf("error %q should mention invalid variable token", err.Error())
	}
}

func TestValidateTokens_MultilineInvalidTokenRejected(t *testing.T) {
	// Regression: invalidVarPattern used "." without dot-all mode, so malformed
	// tokens spanning a newline were skipped by validation and left unresolved.
	tmpl := &TaskTemplate{
		Description: "{{bad\nname}}",
	}
	err := ValidateTokens(tmpl)
	if err == nil {
		t.Fatal("expected error for multiline invalid token, got nil")
	}
	if !strings.Contains(err.Error(), "invalid variable token") {
		t.Errorf("error %q should mention invalid variable token", err.Error())
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

func TestExtractVariables_NilTemplate(t *testing.T) {
	if vars := ExtractVariables(nil); vars != nil {
		t.Errorf("expected nil for nil template, got %v", vars)
	}
}

func TestHasVariables_NilTemplate(t *testing.T) {
	if HasVariables(nil) {
		t.Error("nil template must not have variables")
	}
}

func TestValidateTokens_NilTemplate(t *testing.T) {
	if err := ValidateTokens(nil); err != nil {
		t.Errorf("nil template should not error, got %v", err)
	}
}

func TestVariableNames_NilTemplate(t *testing.T) {
	names := VariableNames(nil)
	if len(names) != 0 {
		t.Errorf("expected empty names for nil template, got %v", names)
	}
}

func TestExtractVariables_ConflictingNonEmptyDefaults_FirstWins(t *testing.T) {
	// Regression note: when the same variable has two different non-empty
	// defaults, the first encountered wins and the conflict is silent. This
	// documents (and pins) current behavior so any future change is deliberate.
	tmpl := &TaskTemplate{
		Description: "from {{env:staging}} to {{env:production}}",
	}

	vars := ExtractVariables(tmpl)
	if len(vars) != 1 {
		t.Fatalf("expected 1 variable, got %d", len(vars))
	}
	if vars[0].Default != "staging" {
		t.Errorf("Default = %q, want %q (first non-empty default should win)", vars[0].Default, "staging")
	}
}

func TestApplyVariables_ValuesNil(t *testing.T) {
	// Passing a nil values map must not panic; behaves like an empty map so
	// each token falls back to its default (or "").
	tmpl := TaskTemplate{
		Name:        "{{a:fallback}}",
		Description: "{{b}}",
	}
	result := ApplyVariables(tmpl, nil)
	if result.Name != "fallback" {
		t.Errorf("Name: got %q, want %q", result.Name, "fallback")
	}
	if result.Description != "" {
		t.Errorf("Description: got %q, want empty", result.Description)
	}
}

func TestApplyVariables_EmptyStringValueBeatsDefault(t *testing.T) {
	// An explicit empty-string value in the map is still a value: it overrides
	// the default. Callers that want the default to win for missing input must
	// omit the key from values entirely.
	tmpl := TaskTemplate{Description: "{{env:staging}}"}
	result := ApplyVariables(tmpl, map[string]string{"env": ""})
	if result.Description != "" {
		t.Errorf("Description: got %q, want empty (explicit empty wins over default)", result.Description)
	}
}

func TestApplyVariables_DefaultWithColon(t *testing.T) {
	// The default capture is greedy up to "}}", so a colon inside the default
	// is preserved verbatim (only the first colon delimits name from default).
	tmpl := TaskTemplate{Description: "{{url:https://example.com:8080/path}}"}
	result := ApplyVariables(tmpl, nil)
	if result.Description != "https://example.com:8080/path" {
		t.Errorf("Description: got %q", result.Description)
	}
}

func TestApplyVariables_RoundTripWithExtract(t *testing.T) {
	tmpl := TaskTemplate{
		Name:        "{{a}} - {{b:beta}}",
		Description: "{{a}} also {{c}}",
		Section:     "{{a}}",
		Tags:        []string{"{{c}}"},
		Checklist:   []string{"{{b}} {{a}}"},
		Children:    []TemplateChild{{Title: "{{c}} for {{a}}"}},
	}
	vars := ExtractVariables(&tmpl)
	if len(vars) != 3 {
		t.Fatalf("expected 3 unique vars (a,b,c), got %d", len(vars))
	}
	values := map[string]string{"a": "ALPHA", "c": "GAMMA"} // omit b so default applies
	result := ApplyVariables(tmpl, values)
	if HasVariables(&result) {
		t.Errorf("result should have no remaining tokens, got %+v", result)
	}
	if result.Name != "ALPHA - beta" {
		t.Errorf("Name: got %q", result.Name)
	}
	if result.Checklist[0] != "beta ALPHA" {
		t.Errorf("Checklist[0]: got %q", result.Checklist[0])
	}
	if result.Children[0].Title != "GAMMA for ALPHA" {
		t.Errorf("Children[0].Title: got %q", result.Children[0].Title)
	}
}

func TestApplyVariables_CrossFieldDefaultFallback(t *testing.T) {
	// Bug: previously, "{{b:beta}}" in Name and "{{b}}" in another field with
	// no values supplied would resolve to "beta" in Name but "" elsewhere —
	// the same variable expanding to two different strings within one template.
	// ExtractVariables already consolidates the default; ApplyVariables must do
	// the same so the rendering is consistent.
	tmpl := TaskTemplate{
		Name:        "{{b:beta}} release",
		Description: "Notes for {{b}}",
		Section:     "{{b}}",
		Tags:        []string{"{{b}}"},
		Checklist:   []string{"Ship {{b}}"},
		Children:    []TemplateChild{{Title: "Verify {{b}}"}},
	}
	result := ApplyVariables(tmpl, nil)
	if result.Name != "beta release" {
		t.Errorf("Name: got %q", result.Name)
	}
	if result.Description != "Notes for beta" {
		t.Errorf("Description: got %q (cross-field default should resolve)", result.Description)
	}
	if result.Section != "beta" {
		t.Errorf("Section: got %q", result.Section)
	}
	if result.Tags[0] != "beta" {
		t.Errorf("Tags[0]: got %q", result.Tags[0])
	}
	if result.Checklist[0] != "Ship beta" {
		t.Errorf("Checklist[0]: got %q", result.Checklist[0])
	}
	if result.Children[0].Title != "Verify beta" {
		t.Errorf("Children[0].Title: got %q", result.Children[0].Title)
	}
}

func TestApplyVariables_ExplicitValueBeatsCrossFieldDefault(t *testing.T) {
	tmpl := TaskTemplate{
		Name:        "{{b:beta}}",
		Description: "{{b}}",
	}
	result := ApplyVariables(tmpl, map[string]string{"b": "gamma"})
	if result.Name != "gamma" || result.Description != "gamma" {
		t.Errorf("explicit value must beat both local and cross-field defaults; got Name=%q Desc=%q", result.Name, result.Description)
	}
}

func TestApplyVariables_LocalDefaultBeatsCrossFieldDefault(t *testing.T) {
	// "first non-empty default wins" applies even when later occurrences have
	// their own defaults; the local token's own default still takes precedence
	// over the cross-field lookup (they happen to agree when the global is the
	// local, so we test by giving the second occurrence a different default).
	tmpl := TaskTemplate{
		Name:        "{{b:first}}",
		Description: "{{b:second}}",
	}
	result := ApplyVariables(tmpl, nil)
	if result.Name != "first" {
		t.Errorf("Name: got %q, want %q", result.Name, "first")
	}
	if result.Description != "second" {
		t.Errorf("Description: got %q, want %q (local default wins)", result.Description, "second")
	}
}

func TestApplyVariables_ExplicitEmptyDefaultBeatsCrossField(t *testing.T) {
	// Regression: "{{x:}}" is an explicit empty default. It must render empty,
	// not silently inherit a non-empty default declared for the same variable
	// in another field.
	tmpl := TaskTemplate{
		Name:        "{{x:}}",
		Description: "{{x:fallback}}",
	}
	result := ApplyVariables(tmpl, nil)
	if result.Name != "" {
		t.Errorf("Name: got %q, want empty (explicit empty default must win)", result.Name)
	}
	if result.Description != "fallback" {
		t.Errorf("Description: got %q, want %q", result.Description, "fallback")
	}
}

func TestApplyVariables_ExplicitEmptyDefaultAcrossAllFields(t *testing.T) {
	// Across every field type, "{{x:}}" must render empty even when other
	// occurrences carry a non-empty default. Bare "{{x}}" still inherits the
	// cross-field default — only the explicit empty token suppresses fallback.
	tmpl := TaskTemplate{
		Name:        "{{x:}}",
		Description: "bare {{x}}",
		Section:     "{{x:}}",
		Tags:        []string{"{{x:}}"},
		Checklist:   []string{"{{x:fallback}}"},
		Children:    []TemplateChild{{Title: "{{x:}}"}},
	}
	result := ApplyVariables(tmpl, nil)
	if result.Name != "" {
		t.Errorf("Name: got %q, want empty", result.Name)
	}
	if result.Description != "bare fallback" {
		t.Errorf("Description: got %q, want %q (bare token falls back)", result.Description, "bare fallback")
	}
	if result.Section != "" {
		t.Errorf("Section: got %q, want empty", result.Section)
	}
	if result.Tags[0] != "" {
		t.Errorf("Tags[0]: got %q, want empty", result.Tags[0])
	}
	if result.Checklist[0] != "fallback" {
		t.Errorf("Checklist[0]: got %q, want %q", result.Checklist[0], "fallback")
	}
	if result.Children[0].Title != "" {
		t.Errorf("Children[0].Title: got %q, want empty", result.Children[0].Title)
	}
}

func TestApplyVariables_ExplicitValueBeatsExplicitEmptyDefault(t *testing.T) {
	// A provided value still wins over the explicit empty default — values
	// always take precedence.
	tmpl := TaskTemplate{Name: "{{x:}}"}
	result := ApplyVariables(tmpl, map[string]string{"x": "from-values"})
	if result.Name != "from-values" {
		t.Errorf("Name: got %q, want %q", result.Name, "from-values")
	}
}

func TestApplyVariables_MultipleVarsSameField(t *testing.T) {
	tmpl := TaskTemplate{
		Checklist: []string{"{{a}} then {{b}} then {{c:fallback}}"},
	}
	result := ApplyVariables(tmpl, map[string]string{"a": "1", "b": "2"})
	if result.Checklist[0] != "1 then 2 then fallback" {
		t.Errorf("Checklist[0]: got %q", result.Checklist[0])
	}
}
