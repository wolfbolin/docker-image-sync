package logger

import (
	"fmt"
	"os"
)

// 日志级别颜色代码
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[0;31m"
	colorGreen  = "\033[0;32m"
	colorYellow = "\033[0;33m"
	colorBlue   = "\033[0;34m"
	colorWhite  = "\033[0;37m"
)

func Info(format string, args ...any) {
	fmt.Fprintf(os.Stdout, colorWhite+"[INFO] "+format+colorReset+"\n", args...)
}

func Done(format string, args ...any) {
	fmt.Fprintf(os.Stdout, colorGreen+"[DONE] "+format+colorReset+"\n", args...)
}

func Warn(format string, args ...any) {
	fmt.Fprintf(os.Stdout, colorYellow+"[WARN] "+format+colorReset+"\n", args...)
}

func Error(format string, args ...any) {
	fmt.Fprintf(os.Stderr, colorRed+"[ERROR] "+format+colorReset+"\n", args...)
}

func Fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, colorRed+"[FATAL] "+format+colorReset+"\n", args...)
	os.Exit(1)
}

func Debug(format string, args ...any) {
	fmt.Fprintf(os.Stdout, colorBlue+"[DEBUG] "+format+colorReset+"\n", args...)
}
