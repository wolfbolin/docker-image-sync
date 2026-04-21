package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wolfbolin/sync-docker/internal/config"
	"github.com/wolfbolin/sync-docker/internal/logger"
	"github.com/wolfbolin/sync-docker/internal/registry"
)

var listRuleNames string

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List synced images and tags for rules",
	Run:   runList,
}

func init() {
	listCmd.Flags().StringVarP(&listRuleNames, "rule", "r", "", "Rule names to list (comma-separated)")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		logger.Fatal("Failed to load config: %v", err)
	}

	rules, err := cfg.FilterRules(listRuleNames)
	if err != nil {
		logger.Fatal("Failed to filter rules: %v", err)
	}

	total := len(rules)

	for i, rule := range rules {
		destPr := config.ParseRef(rule.Dest)
		harbor := registry.NewHarborClient(destPr.Registry)

		tags, err := harbor.ListTags(destPr.Project, destPr.Name)
		if err != nil {
			logger.Warn("  Failed to fetch tags: %v", err)
			continue
		}

		tagMode := ""
		if len(rule.Tags) > 0 {
			tagMode = "tags"
		} else if rule.TagRegex != "" {
			tagMode = "tag_regex"
		}

		printListHeader(i+1, total, rule, tagMode, len(tags))
		printListBody(tags)
		fmt.Println()
	}
}

func printListHeader(idx, total int, rule config.Rule, tagMode string, tagCount int) {
	opts := headerOptions{
		ShowSource: true,
		TagMode:    tagMode,
		TotalTags:  tagCount,
	}
	if tagMode == "tag_regex" {
		opts.TagRegex = rule.TagRegex
	}
	printRuleBoxHeader(idx, total, rule, opts)
}

func printListBody(tags []string) {
	if len(tags) == 0 {
		fmt.Printf("  %s(no tags)%s\n", cDim, cReset)
		return
	}
	printTagGroup(cCyan+"● Tags"+cReset, tags, cCyan)
}
