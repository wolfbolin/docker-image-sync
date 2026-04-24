package cmd

import (
	"context"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/wolfbolin/bolbox/pkg/log"
	"github.com/wolfbolin/sync-docker/internal/cfg"
	"github.com/wolfbolin/sync-docker/internal/hub"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List target registry image tags",
	Run:   runList,
}

func init() {
	listCmd.Flags().StringP("config", "c", "", "指定同步所需配置文件")
	listCmd.Flags().StringP("rule", "r", "", "指定同步的规则（逗号分割）")
	listCmd.Flags().Bool("online", false, "同时获取源端和目标端标签")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) {
	configFile, _ := cmd.Flags().GetString("config")
	ruleNames, _ := cmd.Flags().GetString("rule")
	online, _ := cmd.Flags().GetBool("online")

	config := cfg.LoadConfig(configFile)
	config.FilterRules(ruleNames)
	log.Info("Config loaded successfully")

	ctx := context.Background()
	for idx, rule := range config.Rules {
		PrintRuleInfo(idx+1, len(config.Rules), &rule)

		targetClient := hub.NewContainersClient()
		targetImage := hub.ParseImage(rule.Target)
		targetTags, err := targetClient.ImageTags(ctx, targetImage)
		if err != nil {
			log.Errorf("List tags error: %+v", err)
			continue
		}

		if !online {
			PrintHubTagStats(targetTags, nil)
			continue
		}

		sourceClient := hub.NewContainersClient()
		sourceImage := hub.ParseImage(rule.Source)
		if rule.Proxy {
			proxyUrl, _ := url.Parse(config.Proxy)
			sourceClient.SetProxy(proxyUrl)
		}

		sourceTags, err := sourceClient.ImageTags(ctx, sourceImage)
		if err != nil {
			log.Errorf("List source tags error: %+v", err)
			continue
		}

		PrintHubTagStats(sourceTags, targetTags)
	}
}
