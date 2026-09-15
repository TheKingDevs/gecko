package ui

import (
	"os"
	"strings"

	"golang.org/x/term"
)

// ANSI styling for the gpm terminal output. Colors are only emitted when
// ui.EnableColors(true) has been called (gpm enables them for TTY terminals);
// otherwise every helper returns its text unchanged, so piped output and
// tests stay clean.
var colorsOn = os.Getenv("NO_COLOR") == ""

// EnableColors toggles ANSI coloring for all subsequent output.
func EnableColors(on bool) {
	colorsOn = on
}

// EnableColorsForTTY turns on colors only when the process stdout is attached
// to a terminal. Piped and redirected output stays plain so logs and tests
// never get escape codes.
func EnableColorsForTTY() {
	EnableColors(term.IsTerminal(int(os.Stdout.Fd())))
}

// ColorsOn reports whether ANSI coloring is currently enabled.
func ColorsOn() bool { return colorsOn }

const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiDim    = "\x1b[2m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiCyan   = "\x1b[36m"
)

// Bold renders text bold.
func Bold(s string) string {
	if !colorsOn {
		return s
	}
	return ansiBold + s + ansiReset
}

// Dim renders text in a muted gray.
func Dim(s string) string {
	if !colorsOn {
		return s
	}
	return ansiDim + s + ansiReset
}

// Red renders text in red.
func Red(s string) string {
	if !colorsOn {
		return s
	}
	return ansiRed + s + ansiReset
}

// Green renders text in green.
func Green(s string) string {
	if !colorsOn {
		return s
	}
	return ansiGreen + s + ansiReset
}

// Yellow renders text in yellow.
func Yellow(s string) string {
	if !colorsOn {
		return s
	}
	return ansiYellow + s + ansiReset
}

// Cyan renders text in cyan.
func Cyan(s string) string {
	if !colorsOn {
		return s
	}
	return ansiCyan + s + ansiReset
}

// stripCodes removes ANSI escape sequences so width math works on styled text.
func stripCodes(s string) string {
	var b strings.Builder
	in := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			in = true
			continue
		}
		if in {
			if s[i] >= 'a' && s[i] <= 'z' {
				in = false
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// Box frames a single title line in a rounded box. Multi-line titles become
// multiple boxed rows; the box width fits the longest styled line.
func Box(title string) string {
	rows := strings.Split(title, "\n")
	width := 0
	for _, r := range rows {
		if n := len(stripCodes(r)); n > width {
			width = n
		}
	}
	var b strings.Builder
	b.WriteString("╭─" + strings.Repeat("─", width) + "─╮\n")
	for _, r := range rows {
		b.WriteString("│ " + r + strings.Repeat(" ", width-len(stripCodes(r))) + " │\n")
	}
	b.WriteString("╰─" + strings.Repeat("─", width) + "─╯")
	return b.String()
}
