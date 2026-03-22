package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
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
	Args: cobra.MinimumNArgs(1),
	RunE: runSearch,
}

func init() {
	rootCmd.AddCommand(searchCmd)

	searchCmd.Flags().BoolVarP(&searchIncludeArchived, "all", "a", false, "include archived projects")
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "l", 0, "limit number of results (0 = no limit)")
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := strings.TrimSpace(args[0])
	if query == "" {
		return fmt.Errorf("search query cannot be empty")
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

	var results []models.SearchResult
	queryLower := strings.ToLower(query)

	for _, project := range projects {
		if err := storage.LoadProject(project); err != nil {
			logging.Warn("skipping project %s: %v", project.Name, err)
			fmt.Fprintf(os.Stderr, "warning: skipping project %q: %v\n", project.Name, err)
			continue
		}

		for _, task := range project.Tasks {
			if task.MatchesSearchQuery(queryLower) {
				results = append(results, models.SearchResult{
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
		fmt.Println(NoItemsMsg("tasks", "tt task add <title>"))
		return nil
	}

	displaySearchResults(results, query)
	return nil
}

func displaySearchResults(results []models.SearchResult, query string) {
	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"Project", "ID", "Type", "Status", "Title"})

	for _, r := range results {
		t.AppendRow(table.Row{
			r.Project.Slug,
			r.Task.ShortID,
			r.Task.Type.Icon(),
			r.Task.Status.Icon(),
			util.Truncate(r.Task.Title, 40),
		})
	}

	fmt.Printf("Found %s matching \"%s\":\n\n", Pluralize(len(results), "task", "tasks"), query)
	fmt.Println(t.Render())
}
