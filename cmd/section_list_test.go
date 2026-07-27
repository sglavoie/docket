package cmd

import "testing"

func TestSectionCommandSurfaceRemoved(t *testing.T) {
	for _, child := range rootCmd.Commands() {
		if child.Name() == "section" {
			t.Fatal("root command still exposes retired tt section")
		}
		for _, alias := range child.Aliases {
			if alias == "section" || alias == "sec" {
				t.Fatalf("root command %q still exposes retired section alias %q", child.Name(), alias)
			}
		}
	}
}
