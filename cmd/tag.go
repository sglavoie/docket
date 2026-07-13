package cmd

import (
	"fmt"
	"sort"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tterrors"
)

var tagCmd = &cobra.Command{
	Use:     "tag",
	Aliases: []string{"tags"},
	Short:   "Manage project tags",
	Long: `Manage project tags and pinned tag tabs.

Tags group tasks across the built-in GTD sections. Pinned tags appear as
quick-access tabs in project and cross-project views.

Examples:
  tt tag list
  tt tag list --pinned
  tt tag pin backend
  tt tag unpin backend
  tt tag pin --global urgent
  tt tag list --global --pinned`,
	RunE: runTagList,
}

var tagPinGlobal bool
var tagUnpinGlobal bool
var tagListPinned bool
var tagListGlobal bool

var tagPinCmd = &cobra.Command{
	Use:   "pin <tag>",
	Short: "Pin a tag as a quick-access tab",
	Long: `Pin a tag as a quick-access tab.

By default this pins the tag in the current project. Use --global to pin it
for cross-project tag tabs.

Examples:
  tt tag pin backend
  tt tag pin --global urgent`,
	RunE: runTagPin,
}

var tagUnpinCmd = &cobra.Command{
	Use:   "unpin <tag>",
	Short: "Unpin a tag quick-access tab",
	Long: `Unpin a tag quick-access tab.

By default this unpins the tag in the current project. Use --global to unpin it
from cross-project tag tabs.

Examples:
  tt tag unpin backend
  tt tag unpin --global urgent`,
	RunE: runTagUnpin,
}

var tagListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls", "l"},
	Short:   "List project tags",
	Long: `List tags in the current project.

Use --pinned to show only pinned tag tabs. Use --global with --pinned to show
cross-project pinned tag tabs.

Examples:
  tt tag list
  tt tag list --pinned
  tt tag list --global --pinned`,
	RunE: runTagList,
}

func init() {
	rootCmd.AddCommand(tagCmd)

	tagCmd.AddCommand(tagPinCmd)
	tagCmd.AddCommand(tagUnpinCmd)
	tagCmd.AddCommand(tagListCmd)

	tagPinCmd.Flags().BoolVarP(&tagPinGlobal, "global", "g", false, "pin as a global cross-project tag tab")
	tagUnpinCmd.Flags().BoolVarP(&tagUnpinGlobal, "global", "g", false, "unpin from global cross-project tag tabs")
	tagListCmd.Flags().BoolVarP(&tagListPinned, "pinned", "p", false, "show only pinned tag tabs")
	tagListCmd.Flags().BoolVarP(&tagListGlobal, "global", "g", false, "list global cross-project pinned tag tabs")

	addOutputFlags(tagPinCmd)
	addOutputFlags(tagUnpinCmd)
	addOutputFlags(tagListCmd)
	addOutputFlags(tagCmd)
}

func runTagPin(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return tterrors.NewRequiredFieldError("tag")
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	tag, err := normalizeCLIProjectTag(args[0])
	if err != nil {
		return err
	}

	if tagPinGlobal {
		return pinGlobalTag(out, tag)
	}
	return pinProjectTag(out, tag)
}

func runTagUnpin(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return tterrors.NewRequiredFieldError("tag")
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	tag, err := normalizeCLIProjectTag(args[0])
	if err != nil {
		return err
	}

	if tagUnpinGlobal {
		return unpinGlobalTag(out, tag)
	}
	return unpinProjectTag(out, tag)
}

func runTagList(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("tag list does not accept positional arguments")
	}

	out, err := getOutputOpts(cmd)
	if err != nil {
		return err
	}

	if tagListGlobal {
		if !tagListPinned {
			return fmt.Errorf("global tag list currently supports pinned tags only; use --global --pinned")
		}
		return listGlobalPinnedTags(out)
	}
	return listProjectTags(out, tagListPinned)
}

func normalizeCLIProjectTag(input string) (string, error) {
	tag := models.NormalizeTag(input)
	if err := models.ValidateTag(tag); err != nil {
		return "", err
	}
	return tag, nil
}

func pinProjectTag(out MutateOutput, tag string) error {
	project, cfg, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	svc := newAppService(cfg)
	res, err := withResultRetry(func() (app.ProjectTagResult, error) {
		return svc.PinProjectTag(project.ID, tag)
	})
	if err != nil {
		return translateAppError(err)
	}

	return outputTagMutation(out, tagMutationResult{
		Tag:     res.Tag,
		Pinned:  true,
		Changed: res.Changed,
		Project: res.ProjectName,
	})
}

func unpinProjectTag(out MutateOutput, tag string) error {
	project, cfg, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	svc := newAppService(cfg)
	res, err := withResultRetry(func() (app.ProjectTagResult, error) {
		return svc.UnpinProjectTag(project.ID, tag)
	})
	if err != nil {
		return translateAppError(err)
	}

	return outputTagMutation(out, tagMutationResult{
		Tag:     res.Tag,
		Pinned:  false,
		Changed: res.Changed,
		Project: res.ProjectName,
	})
}

func pinGlobalTag(out MutateOutput, tag string) error {
	cfg, configManager, err := LoadConfig()
	if err != nil {
		return err
	}

	changed, err := cfg.PinGlobalTag(tag)
	if err != nil {
		return err
	}

	if err := saveConfig(configManager, cfg); err != nil {
		return err
	}

	return outputTagMutation(out, tagMutationResult{
		Tag:     tag,
		Pinned:  true,
		Changed: changed,
		Global:  true,
	})
}

func unpinGlobalTag(out MutateOutput, tag string) error {
	cfg, configManager, err := LoadConfig()
	if err != nil {
		return err
	}

	changed := cfg.UnpinGlobalTag(tag)

	if err := saveConfig(configManager, cfg); err != nil {
		return err
	}

	return outputTagMutation(out, tagMutationResult{
		Tag:     tag,
		Pinned:  false,
		Changed: changed,
		Global:  true,
	})
}

type tagMutationResult struct {
	Tag     string `json:"tag"`
	Pinned  bool   `json:"pinned"`
	Changed bool   `json:"changed"`
	Project string `json:"project,omitempty"`
	Global  bool   `json:"global,omitempty"`
}

func outputTagMutation(out MutateOutput, result tagMutationResult) error {
	if out.Quiet {
		return nil
	}
	if out.JSON {
		return printJSON(result)
	}

	scope := "project"
	if result.Global {
		scope = "global"
	}
	verb := "Pinned"
	if !result.Pinned {
		verb = "Unpinned"
	}
	if !result.Changed {
		state := "already pinned"
		if !result.Pinned {
			state = "not pinned"
		}
		fmt.Printf("Tag '#%s' is %s in %s scope.\n", result.Tag, state, scope)
		return nil
	}
	fmt.Printf("%s tag '#%s' in %s scope.\n", verb, result.Tag, scope)
	return nil
}

type tagListEntry struct {
	Tag    string `json:"tag"`
	Tasks  int    `json:"tasks,omitempty"`
	Pinned bool   `json:"pinned"`
	Global bool   `json:"global,omitempty"`
}

func listProjectTags(out MutateOutput, pinnedOnly bool) error {
	project, _, _, err := LoadCurrentProject()
	if err != nil {
		return err
	}

	entries := buildProjectTagListEntries(project, pinnedOnly)
	if out.JSON {
		return printJSON(entries)
	}
	if out.Quiet {
		return nil
	}
	if len(entries) == 0 {
		thing := "tags"
		if pinnedOnly {
			thing = "pinned tags"
		}
		fmt.Println(NoItemsMsg(thing, "tt tag pin <tag>"))
		return nil
	}

	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"Tag", "Tasks", "Pinned"})
	for _, entry := range entries {
		pinned := ""
		if entry.Pinned {
			pinned = "yes"
		}
		t.AppendRow(table.Row{"#" + entry.Tag, entry.Tasks, pinned})
	}
	fmt.Println(t.Render())
	return nil
}

func listGlobalPinnedTags(out MutateOutput) error {
	cfg, _, err := LoadConfig()
	if err != nil {
		return err
	}

	entries := buildGlobalPinnedTagListEntries(cfg.GlobalPinnedTagList())
	if out.JSON {
		return printJSON(entries)
	}
	if out.Quiet {
		return nil
	}
	if len(entries) == 0 {
		fmt.Println(NoItemsMsg("global pinned tags", "tt tag pin --global <tag>"))
		return nil
	}

	t := table.NewWriter()
	t.SetStyle(table.StyleRounded)
	t.AppendHeader(table.Row{"Tag", "Pinned"})
	for _, entry := range entries {
		t.AppendRow(table.Row{"#" + entry.Tag, "yes"})
	}
	fmt.Println(t.Render())
	return nil
}

func buildProjectTagListEntries(project *models.Project, pinnedOnly bool) []tagListEntry {
	counts := project.TagCounts()
	pinned := make(map[string]bool)
	for _, tag := range project.PinnedTagList() {
		pinned[tag] = true
	}

	if pinnedOnly {
		entries := make([]tagListEntry, 0, len(pinned))
		for _, tag := range project.PinnedTagList() {
			entries = append(entries, tagListEntry{
				Tag:    tag,
				Tasks:  counts[tag],
				Pinned: true,
			})
		}
		return entries
	}

	seen := make(map[string]bool)
	tags := project.AllTags()
	for _, tag := range project.PinnedTagList() {
		if _, ok := counts[tag]; !ok {
			tags = append(tags, tag)
		}
	}
	sort.Strings(tags)

	entries := make([]tagListEntry, 0, len(tags))
	for _, tag := range tags {
		if seen[tag] {
			continue
		}
		seen[tag] = true
		entries = append(entries, tagListEntry{
			Tag:    tag,
			Tasks:  counts[tag],
			Pinned: pinned[tag],
		})
	}
	return entries
}

func buildGlobalPinnedTagListEntries(tags []string) []tagListEntry {
	entries := make([]tagListEntry, 0, len(tags))
	for _, tag := range tags {
		entries = append(entries, tagListEntry{
			Tag:    tag,
			Pinned: true,
			Global: true,
		})
	}
	return entries
}
