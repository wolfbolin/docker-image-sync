package logger

import (
	"fmt"
	"strings"

	"github.com/wolfbolin/bolbox/pkg/log"
)

const (
	StyleLength = 9
)

var (
	MinCardWidth = float64(50)
)

type Pair struct {
	Key string
	Val string
}

func PrintInfoCard(title string, kvs []Pair) {
	maxKeyLen := float64(0)
	maxValLen := float64(0)
	for _, kv := range kvs {
		maxKeyLen = max(maxKeyLen, float64(len(kv.Key)))
		maxValLen = max(maxValLen, float64(len(kv.Val)))
	}
	cardWidth := int(max(MinCardWidth, StyleLength+maxValLen+maxKeyLen))

	// Title
	titleLine := "+"
	if len(title) != 0 {
		titleLine += fmt.Sprintf("-------- [%s] ", title)
	}
	titleLine += strings.Repeat("-", cardWidth-len(titleLine)-1)
	titleLine += "+"

	// Context
	contextLines := make([]string, 0)
	for _, kv := range kvs {
		keyStr := kv.Key + strings.Repeat(" ", int(maxKeyLen)-len(kv.Key))
		valStr := kv.Val + strings.Repeat(" ", int(maxValLen)-len(kv.Val))
		line := fmt.Sprintf("|  %s = %s", keyStr, valStr)
		line += strings.Repeat(" ", cardWidth-len(line)-3)
		line += "  |"
		contextLines = append(contextLines, strings.Clone(line))
	}

	// Bottom
	bottomLine := "+" + strings.Repeat("-", cardWidth-2) + "+"

	log.Info(titleLine)
	for _, line := range contextLines {
		log.Info(line)
	}
	log.Info(bottomLine)
}
