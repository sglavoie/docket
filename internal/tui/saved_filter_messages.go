package tui

import "github.com/sglavoie/tt/internal/config"

type SaveFilterMsg struct {
	Name string
}

type LoadSavedFilterMsg struct {
	Filter config.SavedFilter
}

type DeleteSavedFilterMsg struct {
	ID string
}
