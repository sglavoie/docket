package cmd

import (
	"bytes"
	"strings"
	"testing"
)

// Each script registers the alias where it registers the binary, so dkt
// completes like docket from the cask, `just install`, or a sourced script.
func TestCompletionScriptsRegisterTheAlias(t *testing.T) {
	cases := map[string][]string{
		"bash": {"complete -o default -F __start_docket docket dkt\n", "complete -o default -o nospace -F __start_docket docket dkt\n"},
		"zsh":  {"#compdef docket dkt\n", "compdef _docket docket dkt\n"},
		"fish": {"complete -c dkt -w docket\n"},
	}
	for shell, wants := range cases {
		t.Run(shell, func(t *testing.T) {
			var out bytes.Buffer
			if err := writeCompletion(&out, shell); err != nil {
				t.Fatal(err)
			}
			for _, want := range wants {
				if !strings.Contains(out.String(), want) {
					t.Errorf("%s completion lacks %q", shell, want)
				}
			}
		})
	}
}
