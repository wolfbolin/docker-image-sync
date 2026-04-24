package cmd

import (
	"context"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/wolfbolin/bolbox/pkg/log"
	"github.com/wolfbolin/sync-docker/internal/cfg"
	"github.com/wolfbolin/sync-docker/internal/hub"
	"github.com/wolfbolin/sync-docker/internal/sync"
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete unmatched images from target registry",
	Run:   runDelete,
}

func init() {
	deleteCmd.Flags().StringP("config", "c", "", "指定同步所需配置文件")
	deleteCmd.Flags().StringP("rule", "r", "", "指定同步的规则（逗号分割）")
	deleteCmd.Flags().Bool("dry-run", false, "预览模式，不实际执行")
	deleteCmd.Flags().Bool("online", false, "使用在线标签检查冗余")
	rootCmd.AddCommand(deleteCmd)
}

func runDelete(cmd *cobra.Command, args []string) {
	configFile, _ := cmd.Flags().GetString("config")
	ruleNames, _ := cmd.Flags().GetString("rule")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	online, _ := cmd.Flags().GetBool("online")

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
		tagSet, err := syncer.PrepareDeleteTags(ctx, &rule, online)
		if err != nil {
			log.Errorf("Prepare delete error: %+v", err)
			continue
		}
		PrintTaskStats(tagSet)

		if dryRun {
			continue
		}

		ruleSum, err := syncer.ExecuteDelete(ctx, &rule, tagSet)
		if err != nil {
			log.Errorf("Execute delete error: %+v", err)
			continue
		}
		PrintTaskSummary(ruleSum)
	}
}
