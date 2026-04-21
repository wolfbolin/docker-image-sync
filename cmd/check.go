package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/wolfbolin/sync-docker/internal/config"
	"github.com/wolfbolin/sync-docker/internal/logger"
	"github.com/wolfbolin/sync-docker/internal/registry"
	"github.com/wolfbolin/sync-docker/internal/syncer"
)

var checkRuleNames string

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check rule matching results (dry run)",
	Run:   runCheck,
}

func init() {
	checkCmd.Flags().StringVarP(&checkRuleNames, "rule", "r", "", "Rule names to check (comma-separated)")
	rootCmd.AddCommand(checkCmd)
}

func runCheck(cmd *cobra.Command, args []string) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		logger.Fatal("Failed to load config: %v", err)
	}

	rules, err := cfg.FilterRules(checkRuleNames)
	if err != nil {
		logger.Fatal("Failed to filter rules: %v", err)
	}

	sourceClient := registry.NewSourceClient()

	s := syncer.NewSyncer(sourceClient, nil, &cfg.Sync, cfg)
	defer s.Close()

	total := len(rules)

	for i, rule := range rules {
		destPr := config.ParseRef(rule.Dest)
		harbor := registry.NewHarborClient(destPr.Registry)
		s.SetHarborClient(harbor)

		result, err := s.CheckRule(rule)
		if err != nil {
			label := rule.Name
			if label == "" {
				label = rule.Source
			}
			logger.Error("[Rule %d/%d] %s => %s - Error: %v", i+1, total, label, rule.Dest, err)
			fmt.Println()
			continue
		}

		printCheckHeader(i+1, total, rule, result)
		printCheckBody(result)
		fmt.Println()
	}
}

func printCheckHeader(idx, total int, rule config.Rule, result *syncer.CheckResult) {
	title := fmt.Sprintf("RULE %d/%d", idx, total)
	kvs := make(map[string]string)

	if rule.Name != "" {
		kvs["Name"] = rule.Name
	}
	kvs["Source"] = rule.Source
	kvs["Destination"] = rule.Dest

	if result.TagMode == "tags" {
		kvs["Mode"] = "tags (exact match)"
	} else if result.TagMode == "tag_regex" {
		kvs["Mode"] = "tag_regex (exact match)"
		kvs["Pattern"] = result.TagRegex
		kvs["Total tags"] = fmt.Sprintf("%d", result.TotalTags)
	}

	logger.PrintInfoCard(title, kvs)
}

func printCheckBody(result *syncer.CheckResult) {
	logger.PrintTagGroup(logger.ColorGreen+"[+] Will sync"+logger.ColorReset, result.ToSync)
	logger.PrintTagGroup(logger.ColorMagenta+"[~] Need update"+logger.ColorReset, result.Updated)
	logger.PrintTagGroup(logger.ColorYellow+"[=] Already exist"+logger.ColorReset, result.Exist)
}
