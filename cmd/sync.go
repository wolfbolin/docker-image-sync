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

	proxy := cfg.GetProxy()

	dockerHub := registry.NewDockerHubClient(proxy)
	s := syncer.NewSyncer(dockerHub, nil, &cfg.Sync, cfg)
	defer s.Close()

	logger.Info("Start sync, %d rules in total", len(rules))

	var totalStats syncer.SyncStats

	for i, rule := range rules {
		destPr := config.ParseRef(rule.Dest)
		harbor := registry.NewHarborClient(destPr.Registry, proxy)
		s.SetHarborClient(harbor)

		label := rule.Name
		if label == "" {
			label = rule.Source
		}
		logger.Info("[Rule %d/%d] %s => %s", i+1, len(rules), label, rule.Dest)

		stats := s.SyncRule(rule)
		totalStats.Add(stats)
	}

	logger.Info("Sync complete: Success=%d Failed=%d Exist=%d Skip=%d",
		totalStats.Success, totalStats.Failed, totalStats.Exist, totalStats.Skipped)

	if totalStats.Failed > 0 {
		fmt.Println()
		logger.Error("There were %d failed syncs", totalStats.Failed)
	}
}
