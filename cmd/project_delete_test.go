package cmd

import (
	"strings"
	"testing"
)

func TestRunProjectDeleteRejectsExtraArgs(t *testing.T) {
	err := runProjectDelete(projectDeleteCmd, []string{"first-project", "second-project"})
	if err == nil {
		t.Fatal("expected error for extra project delete argument")
	}
	if !strings.Contains(err.Error(), "expected exactly one project slug") {
		t.Fatalf("expected extra-argument error, got %v", err)
	}
}
