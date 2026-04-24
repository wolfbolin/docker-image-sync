package cmd

import (
	"context"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/wolfbolin/bolbox/pkg/log"
	"github.com/wolfbolin/sync-docker/internal/hub"
	"github.com/wolfbolin/sync-docker/internal/sync"

	"github.com/wolfbolin/sync-docker/internal/cfg"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Execute image sync",
	Run:   runSync,
}

func init() {
	syncCmd.Flags().StringP("config", "c", "", "指定同步所需配置文件")
	syncCmd.Flags().StringP("rule", "r", "", "指定同步的规则（逗号分割）")
	syncCmd.Flags().BoolP("force", "f", false, "强制比较Digest信息后同步")
	syncCmd.Flags().Bool("dry-run", false, "预览模式，不实际同步")
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) {
	configFile, _ := cmd.Flags().GetString("config")
	ruleNames, _ := cmd.Flags().GetString("rule")
	forceSync, _ := cmd.Flags().GetBool("force")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	config := cfg.LoadConfig(configFile)
	config.FilterRules(ruleNames)
	log.Info("Config loaded successfully")

	if dryRun {
		log.Info("Dry run mode - no changes will be made")
	}

	ctx := context.WithValue(context.Background(), "config", config)
	for idx, rule := range config.Rules {
		PrintRuleInfo(idx+1, len(config.Rules), &rule)

		sourceClient := hub.NewContainersClient()
		targetClient := hub.NewContainersClient()

		if rule.Proxy {
			proxyUrl, _ := url.Parse(config.Proxy)
			sourceClient.SetProxy(proxyUrl)
		}

		syncer := sync.NewSyncer(config, sourceClient, targetClient)
		tagSet, err := syncer.PrepareSyncTags(ctx, &rule, forceSync)
		if err != nil {
			log.Errorf("Prepare sync error: %+v", err)
			continue
		}
		PrintTaskStats(tagSet)

		if dryRun {
			continue
		}

		ruleSum, err := syncer.ExecuteSync(ctx, &rule, tagSet)
		if err != nil {
			log.Errorf("Execute sync error: %+v", err)
			continue
		}
		PrintTaskSummary(ruleSum)
	}
}
