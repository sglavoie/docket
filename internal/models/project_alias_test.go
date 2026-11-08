package models

import (
	"strings"
	"testing"
)

// TestValidateProjectAlias pins the one gate: empty is absent and valid, and a
// set alias is one canonical token of at most twelve terminal columns that
// carries none of the brackets the row draws around it.
func TestValidateProjectAlias(t *testing.T) {
	for _, tc := range []struct {
		alias   string
		wantErr string
	}{
		{alias: ""},
		{alias: "IA"},
		{alias: "web"},
		{alias: "twelve-chars"},
		{alias: "日本語漢字六"}, // six wide characters fill exactly twelve columns
		{alias: "thirteen-char", wantErr: "too wide"},
		{alias: "日本語漢字六七", wantErr: "too wide"},
		{alias: " IA", wantErr: "whitespace"},
		{alias: "IA ", wantErr: "whitespace"},
		{alias: "I A", wantErr: "whitespace"},
		{alias: "I\tA", wantErr: "whitespace"},
		{alias: "I\u0007A", wantErr: "control"},
		{alias: "[IA", wantErr: "brackets"},
		{alias: "IA]", wantErr: "brackets"},
		{alias: "́", wantErr: "no visible width"},
	} {
		err := ValidateProjectAlias(tc.alias)
		switch {
		case tc.wantErr == "" && err != nil:
			t.Errorf("ValidateProjectAlias(%q) = %v, want valid", tc.alias, err)
		case tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)):
			t.Errorf("ValidateProjectAlias(%q) = %v, want an error containing %q", tc.alias, err, tc.wantErr)
		}
	}
}

// TestNormalizeProjectAlias trims input, so an edit surface stores what the
// catalog accepts, and still refuses interior whitespace rather than joining
// two words into a label nobody typed.
func TestNormalizeProjectAlias(t *testing.T) {
	if got, err := NormalizeProjectAlias("  IA \t"); err != nil || got != "IA" {
		t.Fatalf("NormalizeProjectAlias = %q, %v; want %q", got, err, "IA")
	}
	if got, err := NormalizeProjectAlias("   "); err != nil || got != "" {
		t.Fatalf("NormalizeProjectAlias(blank) = %q, %v; want the empty alias", got, err)
	}
	if _, err := NormalizeProjectAlias("two words"); err == nil {
		t.Fatal("NormalizeProjectAlias accepted interior whitespace")
	}
}

// TestProjectDisplayLabel prefers the alias and falls back to the slug.
func TestProjectDisplayLabel(t *testing.T) {
	project := &Project{Slug: "infrastructure-automation"}
	if label, isAlias := ProjectDisplayLabel(project); label != "infrastructure-automation" || isAlias {
		t.Fatalf("ProjectDisplayLabel = %q, %v; want the slug", label, isAlias)
	}
	project.Alias = "IA"
	if label, isAlias := ProjectDisplayLabel(project); label != "IA" || !isAlias {
		t.Fatalf("ProjectDisplayLabel = %q, %v; want the alias", label, isAlias)
	}
	if label, isAlias := ProjectDisplayLabel(nil); label != "" || isAlias {
		t.Fatalf("ProjectDisplayLabel(nil) = %q, %v; want nothing", label, isAlias)
	}
}
