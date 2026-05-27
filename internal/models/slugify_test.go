package models

import "testing"

func TestSlugify_SpacesToHyphens(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello-world"},
		{"My Project Name", "my-project-name"},
		{"One Two Three Four", "one-two-three-four"},
		{"Single", "single"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := Slugify(tt.input)
			if result != tt.expected {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSlugify_SpecialCharactersRemoved(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"exclamation mark", "Hello World!", "hello-world"},
		{"question mark", "What Now?", "what-now"},
		{"at symbol", "user@domain", "userdomain"},
		{"hash symbol", "Issue #123", "issue-123"},
		{"dollar and percent", "Price $100 (50%)", "price-100-50"},
		{"ampersand", "Tom & Jerry", "tom-jerry"},
		{"asterisk", "Important*", "important"},
		{"parentheses", "Project (v2)", "project-v2"},
		{"brackets", "Item [draft]", "item-draft"},
		{"curly braces", "Config {test}", "config-test"},
		{"colon and semicolon", "Task: Done; Next", "task-done-next"},
		{"quotes", `Say "Hello"`, "say-hello"},
		{"single quotes", "It's working", "its-working"},
		{"slash", "A/B Testing", "ab-testing"},
		{"backslash", "Path\\To\\File", "pathtofile"},
		{"pipe", "Option|Choice", "optionchoice"},
		{"caret", "Power^2", "power2"},
		{"tilde", "Home~user", "homeuser"},
		{"underscore", "snake_case", "snakecase"},
		{"plus", "C++ Code", "c-code"},
		{"equals", "a=b", "ab"},
		{"comma", "One, Two, Three", "one-two-three"},
		{"period", "file.txt", "filetxt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Slugify(tt.input)
			if result != tt.expected {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSlugify_MultipleHyphensCollapse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"double space", "Hello  World", "hello-world"},
		{"triple space", "Hello   World", "hello-world"},
		{"multiple special chars", "Hello!!!World", "helloworld"},
		{"mixed spaces and special", "Hello  --  World", "hello-world"},
		{"leading special chars collapse", "---Hello", "hello"},
		{"trailing special chars collapse", "Hello---", "hello"},
		{"space plus hyphen", "Hello - World", "hello-world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Slugify(tt.input)
			if result != tt.expected {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSlugify_LeadingTrailingHyphensTrimmed(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"leading hyphen", "-hello", "hello"},
		{"trailing hyphen", "hello-", "hello"},
		{"both leading and trailing", "-hello-", "hello"},
		{"multiple leading", "---hello", "hello"},
		{"multiple trailing", "hello---", "hello"},
		{"leading space", " hello", "hello"},
		{"trailing space", "hello ", "hello"},
		{"both spaces", " hello ", "hello"},
		{"leading special char", "!hello", "hello"},
		{"trailing special char", "hello!", "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Slugify(tt.input)
			if result != tt.expected {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSlugify_UnicodeHandling(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"accented chars", "Café Project", "caf-project"},
		{"german umlaut", "München", "mnchen"},
		{"spanish tilde", "Español", "espaol"},
		{"french cedilla", "Façade", "faade"},
		{"chinese characters", "Hello 世界", "hello"},
		{"japanese hiragana", "Test テスト", "test"},
		{"emoji", "Project 🚀", "project"},
		{"greek letters", "Alpha Ω Beta", "alpha-beta"},
		{"cyrillic", "Привет", ""},
		{"mixed unicode", "Naïve résumé", "nave-rsum"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Slugify(tt.input)
			if result != tt.expected {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSlugify_CaseConversion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"UPPERCASE", "uppercase"},
		{"lowercase", "lowercase"},
		{"MixedCase", "mixedcase"},
		{"camelCase", "camelcase"},
		{"PascalCase", "pascalcase"},
		{"SCREAMING_SNAKE", "screamingsnake"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := Slugify(tt.input)
			if result != tt.expected {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSlugify_NumbersPreserved(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Version 2", "version-2"},
		{"Project 123", "project-123"},
		{"2024 Goals", "2024-goals"},
		{"v1.2.3", "v123"},
		{"Issue #42", "issue-42"},
		{"100 Tasks", "100-tasks"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := Slugify(tt.input)
			if result != tt.expected {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSlugify_EmptyAndEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"only spaces", "   ", ""},
		{"only special chars", "!@#$%", ""},
		{"only hyphens", "---", ""},
		{"single char", "a", "a"},
		{"single number", "5", "5"},
		{"hyphen only input", "-", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Slugify(tt.input)
			if result != tt.expected {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSlugify_WhitespaceCharactersAsSeparators(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"tab", "Hello\tWorld", "hello-world"},
		{"newline", "Hello\nWorld", "hello-world"},
		{"carriage return", "Hello\rWorld", "hello-world"},
		{"mixed whitespace", "One\tTwo Three\nFour", "one-two-three-four"},
		{"tab collapses with hyphen", "Hello\t-World", "hello-world"},
		{"leading tab trimmed", "\thello", "hello"},
		{"trailing newline trimmed", "hello\n", "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Slugify(tt.input)
			if result != tt.expected {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSlugify_RealWorldExamples(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"My Awesome Project", "my-awesome-project"},
		{"API Integration v2", "api-integration-v2"},
		{"Bug Fix: Login Issue", "bug-fix-login-issue"},
		{"Feature Request #123", "feature-request-123"},
		{"Q1 2024 Goals", "q1-2024-goals"},
		{"User & Admin Dashboard", "user-admin-dashboard"},
		{"TODO: Refactor Code", "todo-refactor-code"},
		{"(WIP) New Feature", "wip-new-feature"},
		{"[DRAFT] Documentation", "draft-documentation"},
		{"Release v1.0.0-beta", "release-v100-beta"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := Slugify(tt.input)
			if result != tt.expected {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
