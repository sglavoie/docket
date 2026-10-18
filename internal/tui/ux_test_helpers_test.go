package tui

import (
	"fmt"

	"github.com/charmbracelet/x/ansi"
)

// uxDailyUseSizeName spells one size for a subtest name.
func uxDailyUseSizeName(size [2]int) string {
	return fmt.Sprintf("%dx%d", size[0], size[1])
}

// uxClarityFooter is the shell-composed bottom bar with its styling removed,
// which is where feedback and guidance either coexist or replace one another.
func uxClarityFooter(driver *UITestDriver) string {
	return ansi.Strip(driver.App().renderStatusBar())
}
