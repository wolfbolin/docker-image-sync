package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wolfbolin/sync-docker/internal/config"
	"github.com/wolfbolin/sync-docker/internal/logger"
	"github.com/wolfbolin/sync-docker/internal/registry"
	"github.com/wolfbolin/sync-docker/internal/syncer"
)

var (
	deleteRuleNames string
	deleteDryRun    bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete unmatched and schema1 images from target registry",
	Run:   runDelete,
}

func init() {
	deleteCmd.Flags().StringVarP(&deleteRuleNames, "rule", "r", "", "Rule names to delete (comma-separated)")
	deleteCmd.Flags().BoolVarP(&deleteDryRun, "dry-run", "d", false, "Dry run: show what would be deleted without actually deleting")
	rootCmd.AddCommand(deleteCmd)
}

func runDelete(cmd *cobra.Command, args []string) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		logger.Fatal("Failed to load config: %v", err)
	}

	rules, err := cfg.FilterRules(deleteRuleNames)
	if err != nil {
		logger.Fatal("Failed to filter rules: %v", err)
	}

	logger.Info("Config loaded successfully")

	sourceClient := registry.NewSourceClient()

	s := syncer.NewSyncer(sourceClient, nil, &cfg.Sync, cfg)
	defer s.Close()

	if deleteDryRun {
		logger.Info("Dry run mode - no changes will be made")
	}

	logger.Info("Start delete, %d rules in total", len(rules))

	var totalStats syncer.DeleteStats

	for i, rule := range rules {
		destPr := config.ParseRef(rule.Dest)
		harbor := registry.NewHarborClient(destPr.Registry)
		s.SetHarborClient(harbor)

		result, err := s.AnalyzeDeleteRule(rule)
		if err != nil {
			label := rule.Name
			if label == "" {
				label = rule.Source
			}
			logger.Error("[Rule %d/%d] %s => %s - Error: %v", i+1, len(rules), label, rule.Dest, err)
			fmt.Println()
			continue
		}

		printDeleteHeader(i+1, len(rules), rule, result)

		if deleteDryRun {
			printDeleteDryRun(result)
		} else {
			stats := s.DeleteRule(rule, false)
			totalStats.Add(stats)
			printDeleteResult(result, stats)
		}

		fmt.Println()
	}

	if !deleteDryRun {
		logger.Info("Delete complete: Deleted=%d Failed=%d Skipped=%d",
			totalStats.Deleted, totalStats.Failed, totalStats.Skipped)

		if totalStats.Failed > 0 {
			fmt.Println()
			logger.Error("There were %d failed deletes", totalStats.Failed)
		}
	}
}

func printDeleteHeader(idx, total int, rule config.Rule, result *syncer.DeleteResult) {
	var extra []string
	modeSuffix := "(keep listed only)"
	if result.TagMode == "tag_regex" {
		modeSuffix = "(keep matching only)"
	}
	if result.TagMode == "tags" {
		extra = append(extra, fmt.Sprintf("│ %sKeep tags:%s   %s", cBold, cReset, formatTagList(result.Tags)))
	}
	extra = append(extra, fmt.Sprintf("│ %sTotal tags:%s  %d", cBold, cReset, result.TotalTags))
	if deleteDryRun {
		extra = append(extra, fmt.Sprintf("│ %sDry run:%s     %strue%s (no changes)", cBold, cReset, cYellow, cReset))
	}

	printRuleBoxHeader(idx, total, rule, headerOptions{
		ShowSource: false,
		TagMode:    result.TagMode,
		ModeSuffix: modeSuffix,
		TagRegex:   result.TagRegex,
		TotalTags:  result.TotalTags,
		ExtraLines: extra,
	})
}

func printDeleteDryRun(result *syncer.DeleteResult) {
	schema1Names := make([]string, len(result.Schema1))
	for i, item := range result.Schema1 {
		schema1Names[i] = item.TagName
	}
	unmatchedNames := make([]string, len(result.Unmatched))
	for i, item := range result.Unmatched {
		unmatchedNames[i] = item.TagName
	}

	printTagGroup(cRed+"✗ Schema1 (will delete)"+cReset, schema1Names, cRed)
	printTagGroup(cRed+"✗ Unmatched (will delete)"+cReset, unmatchedNames, cRed)
	printTagGroup(cGreen+"✓ Kept"+cReset, result.Kept, cGreen)
}

func printDeleteResult(result *syncer.DeleteResult, stats syncer.DeleteStats) {
	schema1Names := make([]string, len(result.Schema1))
	for i, item := range result.Schema1 {
		schema1Names[i] = item.TagName
	}
	unmatchedNames := make([]string, len(result.Unmatched))
	for i, item := range result.Unmatched {
		unmatchedNames[i] = item.TagName
	}

	printTagGroup(fmt.Sprintf("%s✗ Schema1: deleted=%d%s", cRed, stats.Deleted, cReset), schema1Names, cRed)
	printTagGroup(fmt.Sprintf("%s✗ Unmatched: deleted=%d%s", cRed, stats.Deleted, cReset), unmatchedNames, cRed)
	printTagGroup(cGreen+"✓ Kept"+cReset, result.Kept, cGreen)
}
