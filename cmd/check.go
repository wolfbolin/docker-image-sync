package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/wolfbolin/sync-docker/internal/config"
	"github.com/wolfbolin/sync-docker/internal/logger"
	"github.com/wolfbolin/sync-docker/internal/registry"
	"github.com/wolfbolin/sync-docker/internal/syncer"
)

const (
	cReset    = "\033[0m"
	cGreen    = "\033[0;32m"
	cYellow   = "\033[0;33m"
	cCyan     = "\033[0;36m"
	cMagenta  = "\033[0;35m"
	cGray     = "\033[0;90m"
	cBold     = "\033[1m"
	cDim      = "\033[2m"
	maxTags   = 30
	termWidth = 60
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

	proxy := cfg.GetProxy()

	dockerHub := registry.NewDockerHubClient(proxy)
	s := syncer.NewSyncer(dockerHub, nil, &cfg.Sync, cfg)
	defer s.Close()

	total := len(rules)

	for i, rule := range rules {
		destPr := config.ParseRef(rule.Dest)
		harbor := registry.NewHarborClient(destPr.Registry, proxy)
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

		printRuleHeader(i+1, total, rule, result)
		printRuleBody(result)
		fmt.Println()
	}
}

func printRuleHeader(idx, total int, rule config.Rule, result *syncer.CheckResult) {
	title := fmt.Sprintf(" Rule %d/%d ", idx, total)
	line := strings.Repeat("─", termWidth-len(title))
	fmt.Printf("\n╭%s%s╮\n", title, line)

	if rule.Name != "" {
		fmt.Printf("│ %sName:%s        %s%s%s\n", cBold, cReset, cCyan, rule.Name, cReset)
	}
	fmt.Printf("│ %sSource:%s      %s\n", cBold, cReset, rule.Source)
	fmt.Printf("│ %sDestination:%s %s\n", cBold, cReset, rule.Dest)

	if result.TagMode == "tags" {
		fmt.Printf("│ %sMode:%s        %stags%s (exact match)\n", cBold, cReset, cCyan, cReset)
	} else if result.TagMode == "tag_regex" {
		fmt.Printf("│ %sMode:%s        %stag_regex%s\n", cBold, cReset, cCyan, cReset)
		fmt.Printf("│ %sPattern:%s     %s%s%s\n", cBold, cReset, cYellow, result.TagRegex, cReset)
		fmt.Printf("│ %sTotal tags:%s  %d\n", cBold, cReset, result.TotalTags)
	}

	fmt.Printf("╰%s╯\n", strings.Repeat("─", termWidth))
}

func printRuleBody(result *syncer.CheckResult) {
	printTagGroup(cGreen+"✓ Will sync"+cReset, result.ToSync, cGreen)
	printTagGroup(cMagenta+"↻ Need update"+cReset, result.Updated, cMagenta)
	printTagGroup(cYellow+"● Already exist"+cReset, result.Exist, cYellow)
	printTagGroup(cGray+"○ Skipped V1"+cReset, result.SkippedV1, cDim)
}

func printTagGroup(label string, tags []string, color string) {
	count := len(tags)
	if count == 0 {
		fmt.Printf("  %s (%d): %s-%s\n", label, count, cDim, cReset)
		return
	}

	display := tags
	truncated := false
	if count > maxTags {
		display = tags[:maxTags]
		truncated = true
	}

	tagStr := formatTagsHorizontal(display, color)
	suffix := ""
	if truncated {
		suffix = fmt.Sprintf(" %s... +%d more%s", cDim, count-maxTags, cReset)
	}

	fmt.Printf("  %s (%d): %s%s\n", label, count, tagStr, suffix)
}

func formatTagsHorizontal(tags []string, color string) string {
	var parts []string
	for _, t := range tags {
		parts = append(parts, color+t+cReset)
	}
	return strings.Join(parts, "  ")
}
