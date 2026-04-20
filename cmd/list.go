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

	fmt.Printf("List images for %d rule(s):\n", len(rules))
	fmt.Println("========================================")

	for _, rule := range rules {
		destPr := config.ParseRef(rule.Dest)
		harbor := registry.NewHarborClient(destPr.Registry)

		label := rule.Name
		if label == "" {
			label = rule.Source
		}
		fmt.Printf("\n  %s (%s => %s)\n", label, rule.Source, rule.Dest)

		tags, err := harbor.ListTags(destPr.Project, destPr.Name)
		if err != nil {
			logger.Warn("  Failed to fetch tags: %v", err)
			continue
		}

		if len(tags) == 0 {
			fmt.Println("    (no tags)")
			continue
		}

		for _, tag := range tags {
			fmt.Printf("    - %s\n", tag)
		}
	}

	fmt.Println("\n========================================")
}
