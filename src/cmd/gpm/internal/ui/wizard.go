package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	initpkg "cmd/gpm/internal/init"
	"cmd/gpm/internal/semver"
)

// WizardConfig controls how the init wizard runs.
type WizardConfig struct {
	Dir            string // project directory (".", a name or a path)
	Name           string // fixed name (already given via CLI); "" to ask
	AskName        bool   // whether the name question is asked
	NonInteractive bool   // -y: skip all questions, return defaults
	Type           string // pre-selected type checking mode (dynamic|typed)
}

// licenses is the set of known, valid license identifiers the wizard offers.
var licenses = []string{"MIT", "Apache-2.0", "BSD-3-Clause", "BSD-2-Clause", "GPL-3.0", "ISC", "0BSD", "MPL-2.0", "Unlicense"}

// RunWizard collects the answers for gpm init as a single-question-at-a-time
// TUI. Optional fields (description, author, keywords) stay empty when
// skipped, required fields keep their default when the user just presses
// Enter, and the license is chosen with radio buttons. Non-interactive runs
// (or non-TTY terminals) return fully defaulted Options without touching the
// terminal, so automation never gets stuck.
func RunWizard(t *Terminal, cfg WizardConfig) (*initpkg.Options, error) {
	if cfg.NonInteractive || !t.IsTTY() {
		o := &initpkg.Options{Dir: cfg.Dir, Name: cfg.Name, Type: cfg.Type}
		o.ApplyDefaults()
		return o, nil
	}

	o := &initpkg.Options{Dir: cfg.Dir, Name: cfg.Name, Type: cfg.Type}
	o.ApplyDefaults() // the defaults become the pre-filled field values

	steps := buildSteps(o)
	model := newWizardModel(steps)
	prog := tea.NewProgram(model, tea.WithAltScreen(), tea.WithOutput(t.Out))
	final, err := prog.Run()
	if err != nil {
		return nil, err
	}
	wm, ok := final.(wizardModel)
	if !ok {
		return nil, errors.New("wizard exited unexpectedly")
	}
	if wm.aborted {
		return nil, errors.New("aborted")
	}

	o.Name = wm.steps[0].value
	o.Version = strings.TrimSpace(wm.steps[1].value)
	o.Description = wm.steps[2].value
	o.MainFile = wm.steps[3].value
	for key, cmd := range parseScripts(wm.steps[4].value) {
		switch key {
		case "start":
			o.StartScript = cmd
		case "test":
			o.TestScript = cmd
		}
	}
	o.Author = wm.steps[5].value
	o.Keywords = TrimKeywords(wm.steps[6].value)
	o.License = wm.steps[7].value
	o.Type = wm.steps[8].value
	o.ApplyDefaults()

	return o, nil
}

// parseScripts parses "start:gecko run main.gk, test:gecko test" key:command
// pairs. A bare command without a colon becomes the start script.
func parseScripts(raw string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(raw, ",") {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		if i := strings.Index(p, ":"); i > 0 {
			out[strings.TrimSpace(p[:i])] = strings.TrimSpace(p[i+1:])
			continue
		}
		out["start"] = p
	}
	return out
}

// initDirName derives the default project name from a directory argument.
func initDirName(dir string) string {
	if dir == "" || dir == "." {
		if cwd, err := os.Getwd(); err == nil {
			dir = cwd
		} else {
			return "helloo"
		}
	}
	base := filepath.Base(dir)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return "helloo"
	}
	return sanitize(base)
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case b.Len() > 0:
			b.WriteRune('-')
		}
	}
	if b.Len() == 0 {
		return "helloo"
	}
	return strings.ToLower(b.String())
}

// ---------------------------------------------------------------------------
// The TUI
// ---------------------------------------------------------------------------

type stepKind int

const (
	stepInput stepKind = iota
	stepSelect
)

// wizardStep is a single question of the wizard. Exactly one step is on screen
// at a time; the UI redraws the same terminal area as the user moves between
// steps.
type wizardStep struct {
	kind        stepKind
	title       string
	subtitle    string
	placeholder string
	def         string
	options     []string
	validate    func(string) error
	value       string
	cursor      int
}

// wizardModel is the bubbletea model driving the step-by-step TUI.
type wizardModel struct {
	width  int
	height int

	steps []wizardStep
	idx   int

	field       *textinput.Model
	validateErr string
	seenResize  bool
	done        bool
	aborted     bool
}

// newWizardModel builds the model around the prepared steps.
func newWizardModel(steps []wizardStep) wizardModel {
	m := wizardModel{steps: steps}
	m.field = newField(&steps[0], 70)
	return m
}

// buildSteps lists the wizard questions in the documented order: name,
// version, description, main file, scripts, author, keywords, license, type
// mode.
func buildSteps(o *initpkg.Options) []wizardStep {
	return []wizardStep{
		{
			kind:        stepInput,
			title:       "Project name",
			subtitle:    "short module name, used as the import root",
			placeholder: "e.g. my-awesome-project",
			def:         o.Name,
			validate: func(s string) error {
				if strings.TrimSpace(s) == "" {
					return errors.New("a project name is required")
				}
				return nil
			},
		},
		{
			kind:        stepInput,
			title:       "Version",
			subtitle:    "semantic version for the initial release",
			placeholder: "e.g. 1.0.0",
			def:         o.Version,
			validate: func(s string) error {
				if v := strings.TrimSpace(s); v != "" && !semver.Valid(v) {
					return fmt.Errorf("invalid version %q (want semver)", v)
				}
				return nil
			},
		},
		{
			kind:        stepInput,
			title:       "Description",
			subtitle:    "one line about what the module does",
			placeholder: "short description (optional)",
		},
		{
			kind:        stepInput,
			title:       "Main file",
			subtitle:    "entry point run by gpm run start",
			placeholder: "main.gk",
			def:         o.MainFile,
		},
		{
			kind:        stepInput,
			title:       "Scripts",
			subtitle:    "key:command pairs — e.g. start:gecko run main.gk, test:gecko test",
			placeholder: "start:gecko run main.gk",
			def:         "start:" + o.StartScript,
		},
		{
			kind:        stepInput,
			title:       "Author",
			subtitle:    "name or GitHub handle",
			placeholder: "e.g. Jane Doe (optional)",
		},
		{
			kind:        stepInput,
			title:       "Keywords",
			subtitle:    "comma separated tags",
			placeholder: "cli, terminal, gecko",
		},
		{
			kind:     stepSelect,
			title:    "License",
			subtitle: "choose a known, valid license",
			options:  licenses,
			cursor:   licenseIndex(o.License),
		},
		{
			kind:     stepSelect,
			title:    "Type mode",
			subtitle: "dynamic infers types like gecko today; typed requires explicit annotations",
			options:  []string{"dynamic", "typed"},
			cursor:   typeIndex(o.Type),
		},
	}
}

// typeIndex locates the current type mode among the offered options.
func typeIndex(want string) int {
	if want == "" {
		want = "dynamic"
	}
	for i, opt := range []string{"dynamic", "typed"} {
		if opt == want {
			return i
		}
	}
	return 0
}

// licenseIndex locates the current license among the offered options.
func licenseIndex(want string) int {
	if want == "" {
		want = "MIT"
	}
	for i, l := range licenses {
		if l == want {
			return i
		}
	}
	return 0
}

// newField builds a text input for the current step and focuses it.
func newField(s *wizardStep, w int) *textinput.Model {
	f := textinput.New()
	f.Prompt = "> "
	f.Placeholder = s.placeholder
	f.CharLimit = 256
	f.SetValue(s.def)
	f.CursorEnd()
	fw := w - 6
	if fw < 20 {
		fw = 20
	}
	f.Width = fw
	f.PromptStyle = lipgloss.NewStyle().Foreground(green)
	f.TextStyle = lipgloss.NewStyle()
	f.PlaceholderStyle = lipgloss.NewStyle().Foreground(muted)
	_ = f.Focus()
	return &f
}

func (m wizardModel) Init() tea.Cmd {
	if m.field != nil {
		return m.field.Focus()
	}
	return textinput.Blink
}

func (m wizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.seenResize = true
		if m.steps[m.idx].kind == stepInput {
			curVal := m.field.Value()
			m.field = newField(&m.steps[m.idx], m.viewWidth())
			m.field.SetValue(curVal)
			m.field.CursorEnd()
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.aborted = true
			return m, tea.Quit
		}
	}

	step := &m.steps[m.idx]
	switch step.kind {
	case stepSelect:
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "up", "k":
				if step.cursor > 0 {
					step.cursor--
				}
			case "down", "j":
				if step.cursor < len(step.options)-1 {
					step.cursor++
				}
			case "enter", " ":
				step.value = step.options[step.cursor]
				m.advance()
			}
			if m.done {
				return m, tea.Quit
			}
		}
		return m, nil
	case stepInput:
		var cmd tea.Cmd
		*m.field, cmd = m.field.Update(msg)
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "enter":
				v := m.field.Value()
				if step.validate != nil {
					if err := step.validate(v); err != nil {
						m.validateErr = err.Error()
						return m, nil
					}
				}
				m.validateErr = ""
				step.value = v
				m.advance()
				if m.done {
					return m, tea.Quit
				}
			}
		}
		return m, cmd
	}
	return m, nil
}

// advance moves to the next step (or marks the wizard finished).
func (m *wizardModel) advance() {
	if m.idx < len(m.steps)-1 {
		m.idx++
		m.validateErr = ""
		if m.steps[m.idx].kind == stepInput {
			m.field = newField(&m.steps[m.idx], m.viewWidth())
		}
	} else {
		m.done = true
	}
}

// viewWidth is the usable width for the text input inside the render.
func (m wizardModel) viewWidth() int {
	w := m.width - 4
	if w < 20 {
		w = 20
	}
	return w
}

// contentWidth is the full terminal width the layout spans.
func (m wizardModel) contentWidth() int {
	if m.width > 0 {
		return m.width
	}
	return 80
}

// percent is the wizard progress, 0..89 across the nine steps and 100 when
// the last step is confirmed.
func (m wizardModel) percent() int {
	if m.done {
		return 100
	}
	return m.idx * 100 / len(m.steps)
}

func (m wizardModel) View() string {
	if !m.seenResize {
		return ""
	}
	if m.done {
		return m.renderDone()
	}
	return m.renderStep()
}

var (
	green      = lipgloss.Color("2")
	muted      = lipgloss.Color("8")
	styleTitle = lipgloss.NewStyle().Bold(true).Foreground(green)
	stylePct   = lipgloss.NewStyle().Bold(true).Foreground(green)
	styleDim   = lipgloss.NewStyle().Foreground(muted)
	styleHead  = lipgloss.NewStyle().Foreground(green)
	styleBar   = lipgloss.NewStyle().Foreground(green)
	styleErr   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleSel   = lipgloss.NewStyle().Bold(true).Foreground(green)
)

// progressBar renders a full-width filled track.
func (m wizardModel) progressBar(pct int) string {
	full := m.contentWidth()
	if full < 1 {
		full = 1
	}
	filled := full * pct / 100
	return styleBar.Render(strings.Repeat("━", filled)) +
		styleDim.Render(strings.Repeat("─", full-filled))
}

func (m wizardModel) header(pct int) string {
	title := "Create Project"
	pctTxt := fmt.Sprintf("%3d%%", pct)
	gap := m.contentWidth() - len(title) - len(pctTxt)
	if gap < 1 {
		gap = 1
	}
	return styleTitle.Render(title) + strings.Repeat(" ", gap) + stylePct.Render(pctTxt)
}

// hr is a full-width separator line that runs above the footer hints.
func (m wizardModel) hr() string {
	return styleDim.Render(strings.Repeat("─", m.contentWidth()))
}

// compose stacks the body with the footer pinned to the bottom edge of the
// terminal: a spacer fills the gap and a full-width line separates the
// question area from the footer hints.
func (m wizardModel) compose(body []string, hint string) string {
	if m.height < 1 {
		m.height = 24
	}
	spacer := m.height - len(body) - 2
	if spacer < 1 {
		spacer = 1
	}
	all := make([]string, 0, len(body)+spacer+2)
	all = append(all, body...)
	for i := 0; i < spacer; i++ {
		all = append(all, "")
	}
	all = append(all, m.hr())
	if hint != "" {
		all = append(all, "  "+styleDim.Render(hint))
	}
	return strings.Join(all, "\n")
}

func (m wizardModel) renderStep() string {
	s := &m.steps[m.idx]
	body := []string{
		m.header(m.percent()),
		m.progressBar(m.percent()),
		"",
		"",
	}

	switch s.kind {
	case stepInput:
		body = append(body, "  "+styleTitle.Render(s.title))
		body = append(body, "  "+styleDim.Render(s.subtitle))
		body = append(body, "")
		body = append(body, "  "+m.field.View())
		if m.validateErr != "" {
			body = append(body, "")
			body = append(body, "  "+styleErr.Render("✘ "+m.validateErr))
		}
		return m.compose(body, "enter to continue    esc to cancel    ctrl+u clears the field")
	case stepSelect:
		body = append(body, "  "+styleTitle.Render(s.title))
		body = append(body, "  "+styleDim.Render(s.subtitle))
		body = append(body, "")
		for i, opt := range s.options {
			mark, label := "○", styleDim.Render(opt)
			if i == s.cursor {
				mark, label = "●", styleSel.Render(opt)
			}
			body = append(body, "   "+styleHead.Render(mark)+" "+label)
		}
		return m.compose(body, "↑↓ navigate    enter selects    esc cancels")
	}
	return ""
}

// renderDone shows a short confirmation frame with a full progress bar.
func (m wizardModel) renderDone() string {
	body := []string{
		m.header(100),
		m.progressBar(100),
		"",
		"",
		"  " + styleTitle.Render("✓ project ready"),
		"  " + styleDim.Render("creating project…"),
	}
	return m.compose(body, "")
}
