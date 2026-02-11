package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

var noteListTag string

var noteListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls", "l"},
	Short:   "List notes",
	Long: `List all notes in the current project, sorted by modification date.

Examples:
  tt note list
  tt note list --tag design`,
	RunE: runNoteList,
}

func init() {
	noteCmd.AddCommand(noteListCmd)
	noteListCmd.Flags().StringVarP(&noteListTag, "tag", "t", "", "filter by tag")
}

func runNoteList(cmd *cobra.Command, args []string) error {
	var notes []*models.Note
	var err error

	if noteGlobal {
		dir, _, gErr := globalNotesDir()
		if gErr != nil {
			return gErr
		}
		notes, err = storage.ListNotesFrom(dir)
	} else {
		var project *models.Project
		project, _, _, err = LoadCurrentProject()
		if err != nil {
			return err
		}
		notes, err = storage.ListNotes(project)
	}
	if err != nil {
		return fmt.Errorf("failed to list notes: %w", err)
	}

	if len(notes) == 0 {
		fmt.Println("No notes found. Create one with: tt note add \"Title\"")
		return nil
	}

	for _, note := range notes {
		if noteListTag != "" {
			found := false
			for _, t := range note.Tags {
				if t == noteListTag {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		pinIcon := " "
		if note.Pinned {
			pinIcon = "P"
		}
		tags := ""
		if len(note.Tags) > 0 {
			tags = " [" + strings.Join(note.Tags, ", ") + "]"
		}
		linked := ""
		if len(note.LinkedTasks) > 0 {
			linked = fmt.Sprintf(" (%d linked)", len(note.LinkedTasks))
		}
		age := formatAge(note.ModifiedAt)
		fmt.Printf("%s %-40s %s%s%s\n", pinIcon, note.Title, age, tags, linked)
	}

	return nil
}

func formatAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < util.Day:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
