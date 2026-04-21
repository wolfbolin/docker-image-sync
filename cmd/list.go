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

		printListHeader(i+1, total, rule, tags)
		printListBody(tags)
		fmt.Println()
	}
}

func printListHeader(idx, total int, rule config.Rule, tags []string) {
	title := fmt.Sprintf("RULE %d/%d", idx, total)
	kvs := make(map[string]string)

	if rule.Name != "" {
		kvs["Name"] = rule.Name
	}
	kvs["Source"] = rule.Source
	kvs["Destination"] = rule.Dest

	if len(rule.Tags) > 0 {
		kvs["Mode"] = "tags"
	} else if rule.TagRegex != "" {
		kvs["Mode"] = "tag_regex"
		kvs["Pattern"] = rule.TagRegex
	}
	kvs["Total tags"] = fmt.Sprintf("%d", len(tags))

	logger.PrintInfoCard(title, kvs)
}

func printListBody(tags []string) {
	if len(tags) == 0 {
		fmt.Printf("  %s(no tags)%s\n", logger.ColorDim, logger.ColorReset)
		return
	}
	logger.PrintTagGroup(logger.ColorCyan+"● Tags"+logger.ColorReset, tags)
}
