package cmd

import (
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/spf13/cobra"
)

var (
	searchIncludeArchived bool
	searchLimit           int
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search tasks across all projects",
	Long: `Search for tasks by matching the query against task titles and descriptions.

The search is case-insensitive and matches partial text.

Examples:
  tt search "login"           # Find tasks mentioning "login"
  tt search "bug fix"         # Find tasks with "bug fix"
  tt search --all "auth"      # Include archived projects
  tt search --limit 10 "api"  # Limit to 10 results`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

func init() {
	rootCmd.AddCommand(searchCmd)

	searchCmd.Flags().BoolVarP(&searchIncludeArchived, "all", "a", false, "include archived projects")
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "l", 0, "limit number of results (0 = no limit)")
}

// searchResult holds a task and its parent project for display.
type searchResult struct {
	Task    *models.Task
	Project *models.Project
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := strings.TrimSpace(args[0])
	if query == "" {
		fmt.Println("Search query cannot be empty.")
		return nil
	}

	cfg, _, err := LoadConfig()
	if err != nil {
		return err
	}

	var projects []*models.Project
	if searchIncludeArchived {
		projects = cfg.Projects
	} else {
		projects = cfg.GetActiveProjects()
	}

	if len(projects) == 0 {
		fmt.Println("No projects found.")
		return nil
	}

	var results []searchResult
	queryLower := strings.ToLower(query)

	for _, project := range projects {
		if err := storage.LoadProject(project); err != nil {
			continue
		}

		for _, task := range project.Tasks {
			if matchesSearchQuery(task, queryLower) {
				results = append(results, searchResult{
					Task:    task,
					Project: project,
				})

				if searchLimit > 0 && len(results) >= searchLimit {
					break
				}
			}
		}

		if searchLimit > 0 && len(results) >= searchLimit {
			break
		}
	}

	if len(results) == 0 {
		fmt.Printf("No tasks found matching \"%s\".\n", query)
		return nil
	}

	displaySearchResults(results, query)
	return nil
}

// matchesSearchQuery checks if a task matches the search query.
func matchesSearchQuery(task *models.Task, queryLower string) bool {
	if strings.Contains(strings.ToLower(task.Title), queryLower) {
		return true
	}
	if strings.Contains(strings.ToLower(task.Description), queryLower) {
		return true
	}
	return false
}

func displaySearchResults(results []searchResult, query string) {
	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"Project", "ID", "Type", "Status", "Title"})

	for _, r := range results {
		t.AppendRow(table.Row{
			r.Project.Slug,
			r.Task.ShortID,
			r.Task.Type.Icon(),
			r.Task.Status.Icon(),
			truncate(r.Task.Title, 40),
		})
	}

	fmt.Printf("Found %d task(s) matching \"%s\":\n\n", len(results), query)
	fmt.Println(t.Render())
}
