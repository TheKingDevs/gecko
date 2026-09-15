// Package ui is the interactive layer of gpm: raw-terminal prompts, ANSI
// output, a TUI wizard for gpm init and a plain fallback for non-TTY runs so
// automation and tests never get stuck on fancy terminals.
package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// Terminal wraps stdin/stdout with the terminal-specific behaviours gpm
// needs: detect TTY, clear the screen and prompt without echo.
type Terminal struct {
	In      *os.File
	Out     io.Writer
	raw     bool
	restore func() error // reinstates the previous terminal state
}

// labelW is the width of the wizard label column. Short field labels are
// padded to this width so every prompt starts on the same column.
const labelW = 30

// NewTerminal builds a Terminal bound to the process stdin/stdout.
func NewTerminal() *Terminal {
	return &Terminal{In: os.Stdin, Out: os.Stdout}
}

// IsTTY reports whether the terminal is interactive.
func (t *Terminal) IsTTY() bool {
	return term.IsTerminal(int(t.In.Fd()))
}

// enterRaw switches the terminal to raw mode so prompts can read without
// echo and intercept Ctrl+C.
func (t *Terminal) enterRaw() {
	if t.raw || !t.IsTTY() {
		return
	}
	s, err := term.MakeRaw(int(t.In.Fd()))
	if err != nil {
		return // stay in cooked mode; prompts still work
	}
	t.raw = true
	t.restore = func() error { return term.Restore(int(t.In.Fd()), s) }
}

// leaveRaw returns the terminal to its previous state.
func (t *Terminal) leaveRaw() {
	if t.raw && t.restore != nil {
		_ = t.restore()
	}
	t.raw = false
}

// ClearScreen erases the terminal and homes the cursor.
func (t *Terminal) ClearScreen() {
	if t.IsTTY() {
		fmt.Fprint(t.Out, "\x1b[2J\x1b[H")
	}
}

// Print writes to the terminal output.
func (t *Terminal) Print(a ...any) { fmt.Fprint(t.Out, a...) }

// Println writes a line to the terminal output.
func (t *Terminal) Println(a ...any) { fmt.Fprintln(t.Out, a...) }

// Prompt asks a line-input question, offering a default. Returns the chosen
// value (default when the user just presses Enter).
func (t *Terminal) Prompt(label, def string) (string, error) {
	return t.PromptP(label, def, "")
}

// PromptP asks a line-input question with an optional default and an optional
// placeholder hint. The placeholder is shown in gray next to empty inputs and
// is never returned. An empty input returns def (or "" when there is no
// default).
func (t *Terminal) PromptP(label, def, placeholder string) (string, error) {
	t.writePromptLabel(label, def, placeholder)
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		if err == io.EOF {
			return def, nil
		}
		return def, err
	}
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return def, nil
	}
	return line, nil
}

// writePromptLabel writes an aligned prompt label into the label column,
// followed by a gray default or placeholder hint (or a bare colon when neither
// is present, e.g. yes/no questions whose hint is part of the label).
func (t *Terminal) writePromptLabel(label, def, placeholder string) {
	var b strings.Builder
	if len(label) < labelW {
		fmt.Fprintf(&b, "%-*s", labelW, label)
	} else {
		b.WriteString(label)
	}
	switch {
	case def != "":
		fmt.Fprintf(&b, " [%s] ", Dim(def))
	case placeholder != "":
		fmt.Fprintf(&b, " %s ", Dim(placeholder))
	default:
		b.WriteString(": ")
	}
	fmt.Fprint(t.Out, b.String())
}

// Errf prints a validation error line in red (muted when colors are off).
func (t *Terminal) Errf(format string, a ...any) {
	fmt.Fprintln(t.Out, Red("✘ "+fmt.Sprintf(format, a...)))
}

// Confirm asks a yes/no question and returns the boolean result.
func (t *Terminal) Confirm(label string, def bool) (bool, error) {
	defTxt := "y/N"
	if def {
		defTxt = "Y/n"
	}
	ans, err := t.Prompt(label+" ("+defTxt+")", "")
	if err != nil {
		return def, err
	}
	switch strings.ToLower(strings.TrimSpace(ans)) {
	case "y", "yes":
		return true, nil
	case "n", "no":
		return false, nil
	case "":
		return def, nil
	}
	return def, nil
}

// Select lets the user pick one option with the arrow keys (or Enter to
// accept the highlighted one). The index of the chosen option is returned.
func (t *Terminal) Select(label string, options []string, def int) (int, error) {
	if len(options) == 0 {
		return 0, nil
	}
	if !t.IsTTY() {
		if len(label) < labelW {
			fmt.Fprintf(t.Out, "%-*s\n", labelW, label)
		} else {
			fmt.Fprintln(t.Out, label)
		}
		for i, o := range options {
			mark := "  "
			prefix := "    "
			if i == def {
				mark = Green(">")
				prefix = "   "
			}
			fmt.Fprintf(t.Out, "%s %s %d. %s\n", prefix, mark, i+1, o)
		}
		r := bufio.NewReader(os.Stdin)
		var idx int
		for {
			line, err := r.ReadString('\n')
			if err != nil && line == "" {
				return def, nil
			}
			line = strings.TrimSpace(line)
			if line == "" {
				return def, nil
			}
			n := 0
			fmt.Sscanf(line, "%d", &n)
			if n >= 1 && n <= len(options) {
				idx = n - 1
				break
			}
			fmt.Fprint(t.Out, "pick a number: ")
		}
		return idx, nil
	}

	t.enterRaw()
	defer t.leaveRaw()
	sel := def
	draw := func() {
		fmt.Fprint(t.Out, "\r\x1b[K")
		var b strings.Builder
		if len(label) < labelW {
			fmt.Fprintf(&b, "%-*s\n", labelW, label)
		} else {
			fmt.Fprintf(&b, "%s\n", label)
		}
		for i, o := range options {
			prefix := "    "
			if i == sel {
				prefix = Green("> ") + "  "
			}
			fmt.Fprintf(&b, "%s%s\n", prefix, o)
		}
		fmt.Fprint(t.Out, b.String())
	}
	draw()
	var b [1]byte
	for {
		n, err := os.Stdin.Read(b[:])
		if n == 0 && err != nil {
			return def, fmt.Errorf("reading selection: %w", err)
		}
		switch b[0] {
		case 0x03: // Ctrl+C
			fmt.Fprint(t.Out, "\n")
			return 0, fmt.Errorf("aborted by user (Ctrl+C)")
		case 0x0d, 0x0a, ' ': // Enter or Space
			fmt.Fprintf(t.Out, "\r\x1b[%dA\x1b[0J", len(options)+1)
			return sel, nil
		case 0x1b: // escape sequence
			if _, err := os.Stdin.Read(b[:]); err != nil {
				continue
			}
			if b[0] != '[' {
				continue
			}
			if _, err := os.Stdin.Read(b[:]); err != nil {
				continue
			}
			switch b[0] {
			case 'A': // up
				if sel > 0 {
					sel--
					redraw(t.Out, label, options, sel)
				}
			case 'B': // down
				if sel < len(options)-1 {
					sel++
					redraw(t.Out, label, options, sel)
				}
			}
		}
	}
}

// Checkbox lets the user toggle one or more options. selection maps each
// option to its on/off state. Returns the map plus whether the user
// confirmed or aborted.
func (t *Terminal) Checkbox(label string, options []string, selected []bool) ([]bool, error) {
	if len(options) == 0 {
		return selected, nil
	}
	if !t.IsTTY() {
		if len(label) < labelW {
			fmt.Fprintf(t.Out, "%-*s\n", labelW, label)
		} else {
			fmt.Fprintln(t.Out, label)
		}
		for i, o := range options {
			mark := "[ ]"
			if i < len(selected) && selected[i] {
				mark = Green("[x]")
			}
			prefix := "    "
			fmt.Fprintf(t.Out, "%s %s %d. %s\n", prefix, mark, i+1, o)
		}
		fmt.Fprintln(t.Out, "confirm? [Y/n]")
		r := bufio.NewReader(os.Stdin)
		for {
			line, err := r.ReadString('\n')
			line = strings.ToLower(strings.TrimSpace(line))
			if err != nil && line == "" {
				break
			}
			if line == "" || line == "y" || line == "yes" {
				break
			}
			if line == "n" || line == "no" {
				sel := make([]bool, len(options))
				return sel, nil
			}
		}
		return selected, nil
	}

	t.enterRaw()
	defer t.leaveRaw()
	sel := 0
	to := make([]bool, len(options))
	copy(to, selected)
	drawCheck := func() {
		fmt.Fprintf(t.Out, "\r\x1b[%dA\x1b[0J", len(options)+1)
		var b strings.Builder
		if len(label) < labelW {
			fmt.Fprintf(&b, "%-*s\n", labelW, label)
		} else {
			fmt.Fprintf(&b, "%s\n", label)
		}
		for i, o := range options {
			mark := "[ ]"
			if i < len(to) && to[i] {
				mark = Green("[x]")
			}
			prefix := "    "
			if i == sel {
				prefix = Green("> ") + "  "
			}
			fmt.Fprintf(&b, "%s%s %s\n", prefix, mark, o)
		}
		fmt.Fprint(t.Out, b.String())
	}
	drawCheck()
	var b [1]byte
	for {
		n, err := os.Stdin.Read(b[:])
		if n == 0 && err != nil {
			return nil, fmt.Errorf("reading selection: %w", err)
		}
		switch b[0] {
		case 0x03:
			return nil, fmt.Errorf("aborted by user (Ctrl+C)")
		case 0x1b:
			if _, err := os.Stdin.Read(b[:]); err != nil {
				continue
			}
			if b[0] != '[' {
				continue
			}
			if _, err := os.Stdin.Read(b[:]); err != nil {
				continue
			}
			switch b[0] {
			case 'A':
				if sel > 0 {
					sel--
					drawCheck()
				}
			case 'B':
				if sel < len(options)-1 {
					sel++
					drawCheck()
				}
			}
		case ' ':
			if sel < len(to) {
				to[sel] = !to[sel]
				drawCheck()
			}
		case 0x0d, 0x0a:
			fmt.Fprintf(t.Out, "\r\x1b[%dA\x1b[0J", len(options)+1)
			return to, nil
		}
	}
}

// WaitChar asks for a single keypress and returns it.
func (t *Terminal) WaitChar() (byte, error) {
	t.enterRaw()
	defer t.leaveRaw()
	var b [1]byte
	n, err := os.Stdin.Read(b[:])
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, io.EOF
	}
	return b[0], nil
}

func redraw(out io.Writer, label string, options []string, sel int) {
	var b strings.Builder
	if len(label) < labelW {
		fmt.Fprintf(&b, "%-*s\n", labelW, label)
	} else {
		fmt.Fprintf(&b, "%s\n", label)
	}
	for i, o := range options {
		prefix := "    "
		if i == sel {
			prefix = Green("> ") + "  "
		}
		fmt.Fprintf(&b, "%s%s\n", prefix, o)
	}
	fmt.Fprintf(out, "\r\x1b[%dA\x1b[0J", len(options)+1)
	fmt.Fprint(out, b.String())
}

// TrimKeywords converts " a, b ,c " into ["a","b","c"].
func TrimKeywords(raw string) []string {
	var out []string
	for _, k := range strings.Split(raw, ",") {
		if k = strings.TrimSpace(k); k != "" {
			out = append(out, k)
		}
	}
	return out
}
