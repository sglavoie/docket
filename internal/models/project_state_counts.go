package models

// CountTasksByState returns task counts keyed by exact canonical workflow
// state.
func (p *Project) CountTasksByState() map[WorkflowState]int {
	counts := make(map[WorkflowState]int)
	for _, task := range p.Tasks {
		counts[task.State()]++
	}
	return counts
}

// ValidThemeColors is the set of names accepted for workflow-group colors.
var ValidThemeColors = map[string]bool{
	"gray": true, "red": true, "orange": true, "yellow": true, "green": true,
	"cyan": true, "blue": true, "purple": true, "magenta": true, "white": true,
}

// IsValidThemeColor reports whether color is a supported semantic theme color.
func IsValidThemeColor(color string) bool {
	return ValidThemeColors[color]
}
