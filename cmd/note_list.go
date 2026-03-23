package cmd

import (
	"encoding/json"
	"fmt"
	"slices"
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
	noteListCmd.Flags().StringVarP(&noteListTag, "tag", "T", "", "filter by tag")
	addOutputFlags(noteListCmd)
}

func runNoteList(cmd *cobra.Command, args []string) error {
	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	var notes []*models.Note

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

	// Apply tag filter
	var filtered []*models.Note
	for _, note := range notes {
		if noteListTag != "" && !slices.Contains(note.Tags, noteListTag) {
			continue
		}
		filtered = append(filtered, note)
	}

	if out.Quiet {
		return nil
	}

	if out.JSON {
		type noteJSONEntry struct {
			Title       string    `json:"title"`
			Filename    string    `json:"filename"`
			Tags        []string  `json:"tags"`
			CreatedAt   time.Time `json:"created_at"`
			ModifiedAt  time.Time `json:"modified_at"`
			LinkedTasks []string  `json:"linked_tasks"`
			Pinned      bool      `json:"pinned"`
		}
		entries := make([]noteJSONEntry, len(filtered))
		for i, n := range filtered {
			entries[i] = noteJSONEntry{
				Title:       n.Title,
				Filename:    n.Filename,
				Tags:        n.Tags,
				CreatedAt:   n.CreatedAt,
				ModifiedAt:  n.ModifiedAt,
				LinkedTasks: n.LinkedTasks,
				Pinned:      n.Pinned,
			}
		}
		data, err := json.MarshalIndent(entries, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal notes to JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	if len(notes) == 0 {
		fmt.Println(NoItemsMsg("notes", "tt note add \"Title\""))
		return nil
	}

	if len(filtered) == 0 {
		fmt.Println("No notes match the filters.")
		return nil
	}

	for _, note := range filtered {
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
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < util.Day:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
