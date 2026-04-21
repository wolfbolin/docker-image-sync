package logger

import (
	"fmt"
	"os"
	"strings"
)

const (
	ColorReset   = "\033[0m"
	ColorRed     = "\033[0;31m"
	ColorGreen   = "\033[0;32m"
	ColorYellow  = "\033[0;33m"
	ColorBlue    = "\033[0;34m"
	ColorCyan    = "\033[0;36m"
	ColorMagenta = "\033[0;35m"
	ColorWhite   = "\033[0;37m"
	ColorBold    = "\033[1m"
	ColorDim     = "\033[2m"

	MaxDisplayTags = 30
)

func Info(format string, args ...any) {
	fmt.Fprintf(os.Stdout, ColorWhite+"[INFO] "+format+ColorReset+"\n", args...)
}

func Done(format string, args ...any) {
	fmt.Fprintf(os.Stdout, ColorGreen+"[DONE] "+format+ColorReset+"\n", args...)
}

func Warn(format string, args ...any) {
	fmt.Fprintf(os.Stdout, ColorYellow+"[WARN] "+format+ColorReset+"\n", args...)
}

func Error(format string, args ...any) {
	fmt.Fprintf(os.Stderr, ColorRed+"[ERROR] "+format+ColorReset+"\n", args...)
}

func Fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, ColorRed+"[FATAL] "+format+ColorReset+"\n", args...)
	os.Exit(1)
}

func Debug(format string, args ...any) {
	fmt.Fprintf(os.Stdout, ColorBlue+"[DEBUG] "+format+ColorReset+"\n", args...)
}

func PrintTagGroup(label string, tags []string) {
	count := len(tags)
	if count == 0 {
		fmt.Printf("  %s (%d): %s-%s\n", label, count, ColorDim, ColorReset)
		return
	}

	display := tags
	truncated := false
	if count > MaxDisplayTags {
		display = tags[:MaxDisplayTags]
		truncated = true
	}

	var parts []string
	for _, t := range display {
		parts = append(parts, t)
	}
	tagStr := strings.Join(parts, "  ")

	suffix := ""
	if truncated {
		suffix = fmt.Sprintf(" %s... +%d more%s", ColorDim, count-MaxDisplayTags, ColorReset)
	}

	fmt.Printf("  %s (%d): %s%s\n", label, count, tagStr, suffix)
}

func FormatTagList(tags []string) string {
	if len(tags) > 10 {
		return strings.Join(tags[:10], ", ") + fmt.Sprintf(" ... (+%d more)", len(tags)-10)
	}
	return strings.Join(tags, ", ")
}
