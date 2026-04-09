package util

import "testing"

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"empty string", "", 10, ""},
		{"shorter than max", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"needs truncation", "hello world", 8, "hello..."},
		{"very short max", "hello", 2, "he"},
		{"max of 3", "hello", 3, "hel"},
		{"max of 4", "hello", 4, "h..."},
		{"single char max", "hello", 1, "h"},
		{"zero max", "hello", 0, ""},
		{"unicode exact rune count", "hello 世界", 8, "hello 世界"},
		{"unicode needs truncation", "hello 世界!", 8, "hello..."},
		{"unicode short max", "世界你好", 3, "世界你"},
		{"unicode exact rune count 4", "世界你好", 4, "世界你好"},
		{"unicode truncate with ellipsis", "世界你好啊", 4, "世..."},
		{"emoji truncation", "hello 🌍🌎🌏", 8, "hello..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Truncate(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}
