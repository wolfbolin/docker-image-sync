package logger

import (
	"fmt"
	"strings"

	"github.com/wolfbolin/bolbox/pkg/log"
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

func PrintTagGroup(label, color string, tags []string) {
	groupString := fmt.Sprintf("%s%s%s: ", color, label, ColorReset)
	var overflow string
	var tagsList string
	if len(tags) > MaxDisplayTags {
		tagsList = strings.Join(tags[:MaxDisplayTags], "  ")
		overflow = fmt.Sprintf(" %s... +%d more%s", ColorDim, len(tags)-MaxDisplayTags, ColorReset)
	} else {
		tagsList = strings.Join(tags, "  ")
	}
	groupString += fmt.Sprintf("%s%s", tagsList, overflow)
	log.Info(groupString)
}

func FormatTagList(tags []string) string {
	if len(tags) > 10 {
		return strings.Join(tags[:10], ", ") + fmt.Sprintf(" ... (+%d more)", len(tags)-10)
	}
	return strings.Join(tags, ", ")
}
