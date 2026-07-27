package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRetiredLifecycleCommandsReturnCanonicalReplacements(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"section"}, "tt task list --state <state>"},
		{[]string{"sec"}, "tt task list --state <state>"},
	} {
		err := validateRootArgs(&cobra.Command{Use: "tt"}, test.args)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Errorf("validateRootArgs(%v) = %v, want replacement %q", test.args, err, test.want)
		}
	}

	for retired, replacement := range map[string]string{
		"block":   "tt task wait",
		"standby": "tt task pause",
		"maybe":   "tt task someday",
	} {
		err := validateTaskArgs(&cobra.Command{Use: "task"}, []string{retired, "1"})
		if err == nil || !strings.Contains(err.Error(), replacement) {
			t.Errorf("validateTaskArgs(%q) = %v, want replacement %q", retired, err, replacement)
		}
	}
}

func TestRetiredLifecycleFlagsReturnCanonicalReplacements(t *testing.T) {
	for _, retired := range []string{"--status", "--section"} {
		err := lifecycleFlagError(nil, errors.New("unknown flag: "+retired))
		if err == nil || !strings.Contains(err.Error(), "use --state <state>") {
			t.Errorf("lifecycleFlagError(%q) = %v", retired, err)
		}
	}

	original := errors.New("unknown flag: --bogus")
	if err := lifecycleFlagError(nil, original); !errors.Is(err, original) {
		t.Errorf("unrelated flag error = %v, want original", err)
	}
}

func TestRetiredStateValuesReturnCanonicalReplacements(t *testing.T) {
	for retired, replacement := range map[string]string{
		"blocked": "waiting",
		"standby": "paused",
		"maybe":   "someday",
		"start":   "in_progress",
	} {
		_, err := parseCanonicalWorkflowStateInput("state", retired)
		if err == nil || !strings.Contains(err.Error(), replacement) {
			t.Errorf("parseCanonicalWorkflowStateInput(%q) = %v, want replacement %q", retired, err, replacement)
		}
	}
}
