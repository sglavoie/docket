package tui

import "fmt"

// PriorityInfo represents a selectable priority option.
type PriorityInfo struct {
	Priority    int
	Description string
}

func (p PriorityInfo) FilterText() string {
	return fmt.Sprintf("%d %s", p.Priority, p.Description)
}

func (p PriorityInfo) DisplayText() string {
	return fmt.Sprintf("Priority %d - %s", p.Priority, p.Description)
}

func (p PriorityInfo) CountValue() int {
	return 0
}

func (p PriorityInfo) ItemID() string {
	return fmt.Sprintf("%d", p.Priority)
}
