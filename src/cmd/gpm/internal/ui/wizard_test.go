package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	initpkg "cmd/gpm/internal/init"
)

// --- synthetic key helpers -------------------------------------------------

func keyRune(r rune) tea.Msg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }
func keyEnter() tea.Msg      { return tea.KeyMsg{Type: tea.KeyEnter} }
func keyEsc() tea.Msg        { return tea.KeyMsg{Type: tea.KeyEsc} }
func keyUp() tea.Msg         { return tea.KeyMsg{Type: tea.KeyUp} }
func keyDown() tea.Msg       { return tea.KeyMsg{Type: tea.KeyDown} }
func keyCtrlU() tea.Msg      { return tea.KeyMsg{Type: tea.KeyCtrlU} }

// typeRaw feeds a string as individual keystrokes.
func typeRaw(s string) []tea.Msg {
	var msgs []tea.Msg
	for _, r := range s {
		msgs = append(msgs, keyRune(r))
	}
	return msgs
}

// drive pushes messages through Update, threading the returned model back
// into the value under test (like the tea program loop would).
func drive(m *wizardModel, msgs ...tea.Msg) {
	var cur tea.Model = *m
	for _, msg := range msgs {
		next, _ := cur.Update(msg)
		if stopped, ok := next.(wizardModel); ok && stopped.done || stopped.aborted {
			cur = next
			break
		}
		cur = next
	}
	*m = cur.(wizardModel)
}

// walkthrough returns the key stream that answers the eight questions.
func walkthrough(name, version, desc, mainFile, scripts, author, keywords string) []tea.Msg {
	var msgs []tea.Msg
	for _, part := range []string{name, version, desc, mainFile, scripts, author, keywords} {
		msgs = append(msgs, typeRaw(part)...)
		msgs = append(msgs, keyEnter())
	}
	msgs = append(msgs, keyEnter()) // license: confirm the selection (default MIT)
	return msgs
}

// newTestModel builds a wizard model fitted to a fixed 80x24 canvas.
func newTestModel() wizardModel {
	o := &initpkg.Options{Dir: "demo", Name: ""}
	o.ApplyDefaults()
	m := newWizardModel(buildSteps(o))
	m.seenResize = true
	m.width = 80
	m.height = 24
	return m
}

// --- model-level behavior --------------------------------------------------

func TestWizardFullRun(t *testing.T) {
	m := newTestModel()
	// Typed answers overwrite the pre-filled defaults with ctrl+u first.
	msgs := append([]tea.Msg{keyCtrlU()}, typeRaw("myproj")...)
	msgs = append(msgs, keyEnter())
	msgs = append(msgs, keyCtrlU(), keyRune('1'), keyRune('.'), keyRune('2'), keyRune('.'), keyRune('3'), keyEnter())
	msgs = append(msgs, typeRaw("a neat geo module")...)
	msgs = append(msgs, keyEnter())
	msgs = append(msgs, keyCtrlU())
	msgs = append(msgs, typeRaw("main.gk")...)
	msgs = append(msgs, keyEnter())
	msgs = append(msgs, keyCtrlU())
	msgs = append(msgs, typeRaw("start:gecko run main.gk")...)
	msgs = append(msgs, keyEnter())
	msgs = append(msgs, typeRaw("jane")...)
	msgs = append(msgs, keyEnter())
	msgs = append(msgs, typeRaw("cli, terminal")...)
	msgs = append(msgs, keyEnter())
	msgs = append(msgs, keyEnter()) // license: confirm the selection
	drive(&m, msgs...)

	if !m.done {
		t.Fatalf("wizard did not finish")
	}
	if m.aborted {
		t.Fatalf("wizard aborted")
	}
	got := []string{
		m.steps[0].value, m.steps[1].value, m.steps[2].value,
		m.steps[3].value, m.steps[4].value, m.steps[5].value,
		m.steps[6].value, m.steps[7].value,
	}
	want := []string{"myproj", "1.2.3", "a neat geo module", "main.gk", "start:gecko run main.gk", "jane", "cli, terminal", "MIT"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("step %d value = %q, want %q", i, got[i], want[i])
		}
	}
	view := m.View()
	for _, want := range []string{"Create Project", "100%"} {
		if !strings.Contains(view, want) {
			t.Errorf("done view missing %q", want)
		}
	}
}

func TestWizardNameRequired(t *testing.T) {
	m := newTestModel()
	// Clear the default name and submit empty: the step must refuse to move on.
	drive(&m, keyCtrlU(), keyEnter())
	if m.idx != 0 || m.validateErr != "a project name is required" {
		t.Fatalf("after empty submit: idx=%d err=%q", m.idx, m.validateErr)
	}
	if view := m.View(); !strings.Contains(view, "a project name is required") {
		t.Errorf("error not rendered:\n%s", view)
	}
	// A valid name then passes through to the end.
	msgs := append(typeRaw("proj"), keyEnter())
	for i := 1; i < 8; i++ { // version through license
		msgs = append(msgs, keyEnter())
	}
	drive(&m, msgs...)
	if !m.done {
		t.Fatalf("wizard did not finish")
	}
	if m.steps[0].value != "proj" {
		t.Errorf("name = %q, want proj", m.steps[0].value)
	}
}

func TestWizardDefaultsOnEnter(t *testing.T) {
	m := newTestModel()
	// Straight Enters keep every pre-filled default.
	var msgs []tea.Msg
	for i := 0; i < 8; i++ {
		msgs = append(msgs, keyEnter())
	}
	drive(&m, msgs...)
	if !m.done {
		t.Fatalf("wizard did not finish")
	}
	if m.steps[0].value != "demo" {
		t.Errorf("name = %q, want default demo (pre-filled)", m.steps[0].value)
	}
	if m.steps[1].value != "1.0.0" {
		t.Errorf("version = %q, want default 1.0.0 (pre-filled)", m.steps[1].value)
	}
	if m.steps[3].value != "main.gk" {
		t.Errorf("main = %q, want default main.gk (pre-filled)", m.steps[3].value)
	}
	if m.steps[5].value != "" {
		t.Errorf("author = %q, want empty", m.steps[5].value)
	}
	if m.steps[6].value != "" {
		t.Errorf("keywords = %q, want empty", m.steps[6].value)
	}
}

func TestWizardValidationRejection(t *testing.T) {
	m := newTestModel()
	drive(&m, append(typeRaw("proj"), keyEnter())...)

	// An invalid version must be rejected and keep us on the version step.
	drive(&m, typeRaw("bad version")...)
	drive(&m, keyEnter())
	if m.idx != 1 || m.steps[1].value != "" {
		t.Fatalf("expected to stay on the version step after a bad input (idx=%d)", m.idx)
	}
	if !strings.Contains(m.validateErr, "invalid version") {
		t.Errorf("validateErr = %q, want invalid-version message", m.validateErr)
	}

	// Clear the field, type a valid version, and continue to the end.
	drive(&m, keyCtrlU())
	drive(&m, append(typeRaw("1.0.0"), keyEnter())...)
	if m.idx != 2 {
		t.Fatalf("version was not accepted (idx=%d)", m.idx)
	}
	for i := 2; i < 8; i++ { // description through license (last enter confirms)
		drive(&m, keyEnter())
	}
	if !m.done {
		t.Fatalf("wizard did not finish")
	}
	if m.steps[1].value != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0", m.steps[1].value)
	}
}

func TestWizardSelectKeys(t *testing.T) {
	m := newTestModel()
	// Keep defaults for all inputs, then move down in the license radio list.
	msgs := append([]tea.Msg{keyCtrlU()}, typeRaw("p")...)
	msgs = append(msgs, keyEnter())
	for i := 1; i < 7; i++ {
		msgs = append(msgs, keyEnter())
	}
	msgs = append(msgs, keyDown(), keyEnter())
	drive(&m, msgs...)
	if !m.done {
		t.Fatalf("wizard did not finish")
	}
	if m.steps[7].value != "Apache-2.0" {
		t.Errorf("license = %q, want Apache-2.0", m.steps[7].value)
	}
}

func TestWizardAbortEsc(t *testing.T) {
	m := newTestModel()
	drive(&m, keyEsc())
	if !m.aborted {
		t.Errorf("Esc should abort the wizard")
	}
}

func TestWizardAbortCtrlC(t *testing.T) {
	m := newTestModel()
	drive(&m, tea.KeyMsg{Type: tea.KeyCtrlC})
	if !m.aborted {
		t.Errorf("ctrl+c should abort the wizard")
	}
}

func TestWizardLicenseStaysUntilConfirm(t *testing.T) {
	m := newTestModel()
	m.idx = 7
	m.steps[7].cursor = 0
	drive(&m, keyDown())
	if m.done || m.steps[7].value != "" {
		t.Fatalf("license advanced without Enter")
	}
	if m.steps[7].cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.steps[7].cursor)
	}
	drive(&m, keyEnter())
	if !m.done || m.steps[7].value != "Apache-2.0" {
		t.Errorf("enter should select and finish: done=%v value=%q", m.done, m.steps[7].value)
	}
}

func TestWizardSelectBounds(t *testing.T) {
	m := newTestModel()
	m.idx = 7
	m.steps[7].cursor = 0
	drive(&m, keyUp())
	if m.steps[7].cursor != 0 {
		t.Errorf("cursor went above the first option")
	}
	m.steps[7].cursor = len(licenses) - 1
	drive(&m, keyDown())
	if m.steps[7].cursor != len(licenses)-1 {
		t.Errorf("cursor went past the last option")
	}
}

func TestWizardResizeKeepsValue(t *testing.T) {
	o := &initpkg.Options{Dir: "demo", Name: ""}
	o.ApplyDefaults()
	m := newWizardModel(buildSteps(o))
	m.seenResize = true
	m.width = 80
	drive(&m, keyCtrlU())
	drive(&m, typeRaw("myproj")...)
	// "myproj" stays typed after a resize rebuilds the field.
	m.width = 40
	drive(&m, tea.WindowSizeMsg{Width: 40, Height: 20})
	if got := m.field.Value(); got != "myproj" {
		t.Errorf("field value after resize = %q, want myproj", got)
	}
}

// --- rendering -------------------------------------------------------------

func TestWizardRenderInputStep(t *testing.T) {
	m := newTestModel()
	view := m.View()
	for _, want := range []string{"Create Project", "0%", "Project name", "> ", "enter to continue"} {
		if !strings.Contains(view, want) {
			t.Errorf("input step missing %q", want)
		}
	}
}

func TestWizardRenderSelectStep(t *testing.T) {
	m := newTestModel()
	m.idx = 7
	m.steps[7].cursor = 0
	view := m.View()
	for _, want := range []string{"License", "○", "● MIT", "↑↓ navigate", "87%"} {
		if !strings.Contains(view, want) {
			t.Errorf("select step missing %q", want)
		}
	}
}

func TestWizardRenderDoneFrame(t *testing.T) {
	m := newTestModel()
	m.done = true
	view := m.View()
	for _, want := range []string{"✓ project ready", "100%"} {
		if !strings.Contains(view, want) {
			t.Errorf("done frame missing %q", want)
		}
	}
}

func TestWizardPercent(t *testing.T) {
	m := newTestModel()
	want := []int{0, 12, 25, 37, 50, 62, 75, 87}
	for i := range want {
		m.idx = i
		if got := m.percent(); got != want[i] {
			t.Errorf("percent at step %d = %d, want %d", i, got, want[i])
		}
	}
	m.done = true
	if got := m.percent(); got != 100 {
		t.Errorf("done percent = %d, want 100", got)
	}
}

// --- helpers ---------------------------------------------------------------

func TestParseScripts(t *testing.T) {
	got := parseScripts("start:gecko run main.gk, test:gecko test, build: gk build")
	if got["start"] != "gecko run main.gk" {
		t.Errorf("start = %q", got["start"])
	}
	if got["test"] != "gecko test" {
		t.Errorf("test = %q", got["test"])
	}
	if got["build"] != "gk build" {
		t.Errorf("build = %q", got["build"])
	}
	if got["missing"] != "" {
		t.Errorf("unexpected key")
	}
}

// --- end-to-end run off a real reader -------------------------------------

func TestWizardProgramSmoke(t *testing.T) {
	o := &initpkg.Options{Dir: "demo", Name: ""}
	o.ApplyDefaults()
	m := newWizardModel(buildSteps(o))

	keys := "\x15myproj\r\r\r\r\r\r\r\r" // name typed, every other answer defaulted
	var out strings.Builder
	p := tea.NewProgram(m, tea.WithInput(strings.NewReader(keys)), tea.WithOutput(&out))
	final, err := p.Run()
	if err != nil {
		t.Fatalf("tui run: %v", err)
	}
	wm, ok := final.(wizardModel)
	if !ok {
		t.Fatalf("unexpected final model %T", final)
	}
	if !wm.done {
		t.Fatalf("wizard did not finish")
	}
	if wm.steps[0].value != "myproj" {
		t.Errorf("name = %q, want myproj", wm.steps[0].value)
	}
}
