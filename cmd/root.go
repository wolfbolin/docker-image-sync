package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wolfbolin/bolbox/pkg/log"
	"github.com/wolfbolin/sync-docker/internal/cfg"
	"github.com/wolfbolin/sync-docker/internal/logger"
	"github.com/wolfbolin/sync-docker/internal/sync"
)

var rootCmd = &cobra.Command{
	Use:   "sync-docker",
	Short: "Docker image sync tool",
	Long:  "docker-image-sync - Sync public container images to a private registry",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Runtime error: %+v", err)
	}
}

func PrintRuleInfo(idx, total int, rule *cfg.Rule) {
	log.Infof("")
	title := fmt.Sprintf("RULE %d/%d", idx, total)
	kvs := []logger.Pair{
		{"Name", rule.Name},
		{"Source", rule.Source},
		{"Target", rule.Target},
		{"Proxy", fmt.Sprintf("%t", rule.Proxy)},
		{"Regex", rule.TagRegex},
		{"Tags", ""},
	}
	if len(rule.Tags) > 5 {
		kvs[5].Val = "[" + strings.Join(rule.Tags[:5], " ") + "]"
	} else {
		kvs[5].Val = "[" + strings.Join(rule.Tags, " ") + "]"
	}
	logger.PrintInfoCard(title, kvs)
}

func PrintTaskStats(tagSet *sync.TagSet) {
	log.Infof("")
	kvs := []logger.Pair{
		{"Total tags", fmt.Sprintf("%d", len(tagSet.Sync)+len(tagSet.Same))},
		{"Sync tags", fmt.Sprintf("%d", len(tagSet.Sync))},
		{"Over tags", fmt.Sprintf("%d", len(tagSet.Over))},
		{"Diff tags", fmt.Sprintf("%d", len(tagSet.Diff))},
		{"Same tags", fmt.Sprintf("%d", len(tagSet.Same))},
	}
	logger.PrintInfoCard("STATS", kvs)
	logger.PrintTagGroup("[+] Sync", logger.ColorBlue, tagSet.Sync)
	logger.PrintTagGroup("[%] Over", logger.ColorYellow, tagSet.Over)
	logger.PrintTagGroup("[~] Diff", logger.ColorCyan, tagSet.Diff)
	logger.PrintTagGroup("[=] Same", logger.ColorGreen, tagSet.Same)
}

func PrintTaskSummary(sum *sync.RuleSum) {
	log.Infof("")
	kvs := []logger.Pair{
		{"Add", fmt.Sprintf("%d", len(sum.Add))},
		{"Del", fmt.Sprintf("%d", len(sum.Del))},
		{"Put", fmt.Sprintf("%d", len(sum.Put))},
		{"Err", fmt.Sprintf("%d", len(sum.Err))},
	}
	logger.PrintInfoCard("RESULT", kvs)
	logger.PrintTagGroup("[+] Add", logger.ColorBlue, sum.Add)
	logger.PrintTagGroup("[-] Del", logger.ColorYellow, sum.Del)
	logger.PrintTagGroup("[~] Put", logger.ColorCyan, sum.Put)
	logger.PrintTagGroup("[x] Err", logger.ColorRed, sum.Err)
}

func PrintHubTagStats(sourceTags, targetTags []string) {
	log.Infof("")
	kvs := []logger.Pair{
		{Key: "Source tags", Val: fmt.Sprintf("%d", len(sourceTags))},
	}
	if len(targetTags) != 0 {
		kvs = append(kvs, logger.Pair{Key: "Target tags", Val: fmt.Sprintf("%d", len(targetTags))})
	}
	logger.PrintInfoCard("TAGS", kvs)
	logger.PrintTagGroup("[S] Source", logger.ColorBlue, sourceTags)
	if len(targetTags) != 0 {
		logger.PrintTagGroup("[T] Target", logger.ColorCyan, targetTags)
	}
}
