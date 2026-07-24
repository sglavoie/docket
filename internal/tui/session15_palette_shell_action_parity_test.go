package tui

import (
	"testing"
)

func findParityCommand(t *testing.T, cmds []Command, name string) Command {
	t.Helper()
	for _, command := range cmds {
		if command.Name == name {
			return command
		}
	}
	t.Fatalf("palette catalog missing command %q", name)
	return Command{}
}
