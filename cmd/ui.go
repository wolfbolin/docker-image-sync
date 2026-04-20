package cmd

import (
	"fmt"
	"strings"

	"github.com/wolfbolin/sync-docker/internal/config"
)

const (
	cReset    = "\033[0m"
	cRed      = "\033[0;31m"
	cGreen    = "\033[0;32m"
	cYellow   = "\033[0;33m"
	cCyan     = "\033[0;36m"
	cMagenta  = "\033[0;35m"
	cBold     = "\033[1m"
	cDim      = "\033[2m"
	maxTags   = 30
	termWidth = 60
)

type headerOptions struct {
	ShowSource bool
	TagMode    string
	ModeSuffix string
	TagRegex   string
	TotalTags  int
	ExtraLines []string
}

func printRuleBoxHeader(idx, total int, rule config.Rule, opts headerOptions) {
	title := fmt.Sprintf(" Rule %d/%d ", idx, total)
	line := strings.Repeat("─", termWidth-len(title))
	fmt.Printf("\n╭%s%s╮\n", title, line)

	if rule.Name != "" {
		fmt.Printf("│ %sName:%s        %s%s%s\n", cBold, cReset, cCyan, rule.Name, cReset)
	}
	if opts.ShowSource {
		fmt.Printf("│ %sSource:%s      %s\n", cBold, cReset, rule.Source)
	}
	fmt.Printf("│ %sDestination:%s %s\n", cBold, cReset, rule.Dest)

	if opts.TagMode == "tags" {
		fmt.Printf("│ %sMode:%s        %stags%s %s\n", cBold, cReset, cCyan, cReset, opts.ModeSuffix)
	} else if opts.TagMode == "tag_regex" {
		suffix := opts.ModeSuffix
		if suffix != "" {
			suffix = " " + suffix
		}
		fmt.Printf("│ %sMode:%s        %stag_regex%s%s\n", cBold, cReset, cCyan, cReset, suffix)
		fmt.Printf("│ %sPattern:%s     %s%s%s\n", cBold, cReset, cYellow, opts.TagRegex, cReset)
		fmt.Printf("│ %sTotal tags:%s  %d\n", cBold, cReset, opts.TotalTags)
	}

	for _, line := range opts.ExtraLines {
		fmt.Println(line)
	}

	fmt.Printf("╰%s╯\n", strings.Repeat("─", termWidth))
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

func formatTagList(tags []string) string {
	if len(tags) > 10 {
		return strings.Join(tags[:10], ", ") + fmt.Sprintf(" ... (+%d more)", len(tags)-10)
	}
	return strings.Join(tags, ", ")
}
