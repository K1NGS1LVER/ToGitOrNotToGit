package tui

import (
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

type state int

const (
	stateShowing state = iota
	stateEditing
	stateRegenerating
)

type regenResultMsg struct {
	text string
	err  error
}

type model struct {
	message  string
	state    state
	regen    func() (string, error)
	spinner  spinner.Model
	textarea textarea.Model
	accepted bool
	err      error
}

func newModel(initial string, regen func() (string, error)) model {
	ta := textarea.New()
	ta.SetValue(initial)

	return model{
		message:  initial,
		state:    stateShowing,
		regen:    regen,
		spinner:  spinner.New(),
		textarea: ta,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func regenerateCmd(regen func() (string, error)) tea.Cmd {
	return func() tea.Msg {
		text, err := regen()
		return regenResultMsg{text: text, err: err}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.Type == tea.KeyCtrlC {
		m.accepted = false
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.state {
		case stateShowing:
			switch msg.String() {
			case "enter":
				m.accepted = true
				return m, tea.Quit
			case "e":
				m.state = stateEditing
				m.textarea.SetValue(m.message)
				cmd := m.textarea.Focus()
				return m, cmd
			case "r":
				m.state = stateRegenerating
				return m, tea.Batch(m.spinner.Tick, regenerateCmd(m.regen))
			case "q":
				m.accepted = false
				return m, tea.Quit
			}
		case stateEditing:
			switch msg.Type {
			case tea.KeyCtrlS:
				m.message = m.textarea.Value()
				m.state = stateShowing
				return m, nil
			case tea.KeyEsc:
				m.state = stateShowing
				return m, nil
			}
			// bubbles textarea.Update is a no-op while unfocused; stateEditing
			// implies the textarea is the active input, so keep focus in sync
			// even if something set the state directly without calling Focus().
			if !m.textarea.Focused() {
				m.textarea.Focus()
			}
			var cmd tea.Cmd
			m.textarea, cmd = m.textarea.Update(msg)
			return m, cmd
		}
	case regenResultMsg:
		m.message = msg.text
		m.err = msg.err
		m.state = stateShowing
		return m, nil
	case spinner.TickMsg:
		if m.state == stateRegenerating {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m model) View() string {
	switch m.state {
	case stateEditing:
		return m.textarea.View() + "\n[ctrl+s] save  [esc] discard\n"
	case stateRegenerating:
		return m.spinner.View() + " regenerating...\n"
	default:
		note := ""
		if m.err != nil {
			note = "\n(regenerate failed, showing fallback message)\n"
		}
		return m.message + note + "\n[enter] accept  [e] edit  [r] regenerate  [q/ctrl+c] cancel\n"
	}
}

// Run launches the interactive preview screen and blocks until the user
// accepts or cancels. regen is called (possibly more than once) whenever
// the user asks to regenerate; it is expected to always return usable text
// even when err is non-nil (e.g. a fallback message), since that text is
// what gets displayed alongside the failure note.
func Run(initial string, regen func() (string, error)) (final string, accepted bool, err error) {
	p := tea.NewProgram(newModel(initial, regen))
	result, err := p.Run()
	if err != nil {
		return "", false, err
	}
	m := result.(model)
	return m.message, m.accepted, nil
}

// ponytail: no lipgloss styling - plain text View(). Add color/layout when someone actually asks for visual polish.
