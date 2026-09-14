package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

var colorEnabled = detectColorSupport()

func detectColorSupport() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiDim    = "\x1b[2m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiCyan   = "\x1b[36m"
)

func style(code, s string) string {
	if !colorEnabled {
		return s
	}
	return code + s + ansiReset
}

func bold(s string) string   { return style(ansiBold, s) }
func dim(s string) string    { return style(ansiDim, s) }
func red(s string) string    { return style(ansiRed, s) }
func green(s string) string  { return style(ansiGreen, s) }
func yellow(s string) string { return style(ansiYellow, s) }
func cyan(s string) string   { return style(ansiCyan, s) }

func success(format string, args ...interface{}) {
	fmt.Printf("%s %s\n", green("✓"), fmt.Sprintf(format, args...))
}

func warn(format string, args ...interface{}) {
	fmt.Printf("%s %s\n", yellow("!"), fmt.Sprintf(format, args...))
}

func step(format string, args ...interface{}) {
	fmt.Printf("%s %s\n", cyan("→"), fmt.Sprintf(format, args...))
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "%s %s\n", red("✗"), err)
	os.Exit(1)
}

func confirm(format string, args ...interface{}) bool {
	fmt.Printf("%s %s %s", yellow("?"), fmt.Sprintf(format, args...), dim("[y/N]: "))
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}
