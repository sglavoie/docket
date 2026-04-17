package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

var projectExportFile string

var projectExportCmd = &cobra.Command{
	Use:   "export [slug-or-id]",
	Short: "Export a project to clean Markdown",
	Long: `Export a project to a human-readable Markdown file in ~/.tt/exports/<slug>.md.

If no project is specified, exports the current project.
Use --file to write to an arbitrary path instead.

EXAMPLES

  tt project export
  tt project export my-project
  tt project export my-project --file /tmp/my-project.md`,
	Args: cobra.MaximumNArgs(1),
	RunE: runProjectExport,
}

func init() {
	projectCmd.AddCommand(projectExportCmd)
	projectExportCmd.Flags().StringVar(&projectExportFile, "file", "", "write to this path instead of ~/.tt/exports/<slug>.md")
}

func runProjectExport(cmd *cobra.Command, args []string) error {
	project, _, _, err := LoadConfigAndProjectOrCurrent(args)
	if err != nil {
		return err
	}

	destPath, err := ExportProjectMarkdown(project, projectExportFile)
	if err != nil {
		return err
	}

	fmt.Printf("Exported %q to %s\n", project.Name, destPath)
	return nil
}

// ExportProjectMarkdown writes the project to destPath (or the default exports
// path if destPath is "") and returns the final absolute path.
// LoadProject is called on p to ensure tasks are populated before exporting.
func ExportProjectMarkdown(p *models.Project, destPath string) (string, error) {
	return storage.ExportProjectMarkdown(p, destPath)
}
