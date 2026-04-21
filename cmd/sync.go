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
		printSyncHeader(i+1, len(rules), rule)

		destPr := config.ParseRef(rule.Dest)
		harbor := registry.NewHarborClient(destPr.Registry)
		s.SetHarborClient(harbor)

		result, err := s.PrepareSync(rule)
		if err != nil {
			label := rule.Name
			if label == "" {
				label = rule.Source
			}
			logger.Error("[Rule %d/%d] %s => %s - Error: %v", i+1, len(rules), label, rule.Dest, err)
			fmt.Println()
			continue
		}

		printSyncTaskStats(result)
		s.ExecuteSync(result)
		printSyncResult(result)
		printSyncBody(result)
		fmt.Println()

		totalStats.Add(result.Stats)
	}

	logger.Info("Sync complete: Success=%d Failed=%d Exist=%d",
		totalStats.Success, totalStats.Failed, totalStats.Exist)

	if totalStats.Failed > 0 {
		fmt.Println()
		logger.Error("There were %d failed syncs", totalStats.Failed)
	}
}

func printSyncHeader(idx, total int, rule config.Rule) {
	title := fmt.Sprintf("RULE %d/%d", idx, total)
	kvs := make(map[string]string)

	if rule.Name != "" {
		kvs["Name"] = rule.Name
	}
	kvs["Source"] = rule.Source
	kvs["Destination"] = rule.Dest

	if len(rule.Tags) > 0 {
		kvs["Mode"] = "tags (exact match)"
	} else if rule.TagRegex != "" {
		patternDisplay := rule.TagRegex
		if len(patternDisplay) > 40 {
			patternDisplay = patternDisplay[:37] + "..."
		}
		kvs["Mode"] = "tag_regex (exact match)"
		kvs["Pattern"] = patternDisplay
	}

	logger.PrintInfoCard(title, kvs)
}

func printSyncTaskStats(result *syncer.SyncResult) {
	syncedCount := len(result.ToSync) + len(result.Updated)
	existedCount := len(result.Exist)

	kvs := map[string]string{
		"Total tags":   fmt.Sprintf("%d", result.TotalTags),
		"Synced tags":  fmt.Sprintf("%d", syncedCount),
		"Existed tags": fmt.Sprintf("%d", existedCount),
	}
	logger.PrintInfoCard("TASK STATS", kvs)
}

func printSyncResult(result *syncer.SyncResult) {
	kvs := map[string]string{
		"New":     fmt.Sprintf("%d", len(result.ToSync)),
		"Updated": fmt.Sprintf("%d", len(result.Updated)),
		"Failed":  fmt.Sprintf("%d", result.Stats.Failed),
	}
	logger.PrintInfoCard("RESULT", kvs)
}

func printSyncBody(result *syncer.SyncResult) {
	logger.PrintTagGroup(logger.ColorGreen+"[+] Synced"+logger.ColorReset, result.ToSync)
	logger.PrintTagGroup(logger.ColorMagenta+"[~] Updated"+logger.ColorReset, result.Updated)
	logger.PrintTagGroup(logger.ColorYellow+"[=] Already exist"+logger.ColorReset, result.Exist)
}
