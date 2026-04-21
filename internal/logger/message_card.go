package logger

import (
	"fmt"
	"strings"
)

const (
	StyleLength = 9
)

var (
	MinCardWidth = float64(50)
)

func PrintInfoCard(title string, kvs map[string]string) {
	maxKeyLen := float64(0)
	maxValLen := float64(0)
	for key, val := range kvs {
		maxKeyLen = max(maxKeyLen, float64(len(key)))
		maxValLen = max(maxValLen, float64(len(val)))
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
	for key, val := range kvs {
		keyStr := key + strings.Repeat(" ", int(maxKeyLen)-len(key))
		valStr := val + strings.Repeat(" ", int(maxValLen)-len(val))
		line := fmt.Sprintf("|  %s = %s", keyStr, valStr)
		line += strings.Repeat(" ", cardWidth-len(line)-3)
		line += "  |"
		contextLines = append(contextLines, strings.Clone(line))
	}

	// Bottom
	bottomLine := "+" + strings.Repeat("-", cardWidth-2) + "+"

	fmt.Println(titleLine)
	for _, line := range contextLines {
		fmt.Println(line)
	}
	fmt.Println(bottomLine)
}
