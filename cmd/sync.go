package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wolfbolin/sync-docker/internal/config"
	"github.com/wolfbolin/sync-docker/internal/logger"
	"github.com/wolfbolin/sync-docker/internal/registry"
	"github.com/wolfbolin/sync-docker/internal/syncer"
)

var syncRuleNames string

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Execute image sync",
	Run:   runSync,
}

func init() {
	syncCmd.Flags().StringVarP(&syncRuleNames, "rule", "r", "", "Rule names to sync (comma-separated)")
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		logger.Fatal("Failed to load config: %v", err)
	}

	rules, err := cfg.FilterRules(syncRuleNames)
	if err != nil {
		logger.Fatal("Failed to filter rules: %v", err)
	}

	logger.Info("Config loaded successfully")

	sourceClient := registry.NewSourceClient()

	s := syncer.NewSyncer(sourceClient, nil, &cfg.Sync, cfg)
	defer s.Close()

	logger.Info("Start sync, %d rules in total", len(rules))

	var totalStats syncer.SyncStats

	for i, rule := range rules {
		destPr := config.ParseRef(rule.Dest)
		harbor := registry.NewHarborClient(destPr.Registry)
		s.SetHarborClient(harbor)

		result, err := s.SyncRuleDetailed(rule)
		if err != nil {
			label := rule.Name
			if label == "" {
				label = rule.Source
			}
			logger.Error("[Rule %d/%d] %s => %s - Error: %v", i+1, len(rules), label, rule.Dest, err)
			fmt.Println()
			continue
		}

		printSyncHeader(i+1, len(rules), rule, result)
		printSyncBody(result)
		fmt.Println()

		totalStats.Add(result.Stats)
	}

	logger.Info("Sync complete: Success=%d Failed=%d Exist=%d Schema1=%d",
		totalStats.Success, totalStats.Failed, totalStats.Exist, totalStats.Schema1)

	if totalStats.Failed > 0 {
		fmt.Println()
		logger.Error("There were %d failed syncs", totalStats.Failed)
	}
}

func printSyncHeader(idx, total int, rule config.Rule, result *syncer.SyncResult) {
	printRuleBoxHeader(idx, total, rule, headerOptions{
		ShowSource: true,
		TagMode:    result.TagMode,
		ModeSuffix: "(exact match)",
		TagRegex:   result.TagRegex,
		TotalTags:  result.TotalTags,
	})
}

func printSyncBody(result *syncer.SyncResult) {
	printTagGroup(cGreen+"✓ Synced"+cReset, result.ToSync, cGreen)
	printTagGroup(cMagenta+"↻ Updated"+cReset, result.Updated, cMagenta)
	printTagGroup(cYellow+"● Already exist"+cReset, result.Exist, cYellow)
	printTagGroup(cRed+"⊘ Schema1 skipped"+cReset, result.Schema1, cRed)
}
