package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModel_InitialState(t *testing.T) {
	m := newModel("feat: initial message", func() (string, error) { return "", nil })
	if m.state != stateShowing {
		t.Errorf("initial state = %v, want stateShowing", m.state)
	}
	if m.message != "feat: initial message" {
		t.Errorf("initial message = %q, want %q", m.message, "feat: initial message")
	}
}

func TestUpdate_EnterAccepts(t *testing.T) {
	m := newModel("feat: a message", func() (string, error) { return "", nil })
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(model)
	if !m2.accepted {
		t.Errorf("accepted = false, want true after enter in stateShowing")
	}
}

func TestUpdate_QCancels(t *testing.T) {
	m := newModel("feat: a message", func() (string, error) { return "", nil })
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	m2 := updated.(model)
	if m2.accepted {
		t.Errorf("accepted = true, want false after q")
	}
	if cmd == nil {
		t.Fatal("expected a quit command after q, got nil")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("cmd() = %T, want tea.QuitMsg", cmd())
	}
}

func TestUpdate_CtrlCCancelsFromAnyState(t *testing.T) {
	m := newModel("feat: a message", func() (string, error) { return "", nil })
	m.state = stateRegenerating
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m2 := updated.(model)
	if m2.accepted {
		t.Errorf("accepted = true, want false after ctrl+c")
	}
	if cmd == nil {
		t.Fatal("expected a quit command after ctrl+c, got nil")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("cmd() = %T, want tea.QuitMsg", cmd())
	}
}

func TestUpdate_EEntersEditingWithCurrentMessage(t *testing.T) {
	m := newModel("feat: a message", func() (string, error) { return "", nil })
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	m2 := updated.(model)
	if m2.state != stateEditing {
		t.Errorf("state = %v, want stateEditing", m2.state)
	}
	if m2.textarea.Value() != "feat: a message" {
		t.Errorf("textarea value = %q, want %q", m2.textarea.Value(), "feat: a message")
	}
}

func TestUpdate_CtrlSInEditingSavesText(t *testing.T) {
	m := newModel("feat: original", func() (string, error) { return "", nil })
	m.state = stateEditing
	m.textarea.SetValue("feat: edited by hand")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m2 := updated.(model)
	if m2.state != stateShowing {
		t.Errorf("state = %v, want stateShowing after ctrl+s in editing", m2.state)
	}
	if m2.message != "feat: edited by hand" {
		t.Errorf("message = %q, want %q", m2.message, "feat: edited by hand")
	}
}

func TestUpdate_EnterInEditingInsertsNewline(t *testing.T) {
	m := newModel("feat: original", func() (string, error) { return "", nil })
	m.state = stateEditing
	m.textarea.SetValue("feat: original")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(model)
	if m2.state != stateEditing {
		t.Errorf("state = %v, want to stay stateEditing after enter (should insert newline, not save)", m2.state)
	}
	if !strings.Contains(m2.textarea.Value(), "\n") {
		t.Errorf("textarea value = %q, want it to contain a newline after enter", m2.textarea.Value())
	}
}

func TestUpdate_EscInEditingDiscards(t *testing.T) {
	m := newModel("feat: original", func() (string, error) { return "", nil })
	m.state = stateEditing
	m.textarea.SetValue("feat: a discarded edit")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m2 := updated.(model)
	if m2.state != stateShowing {
		t.Errorf("state = %v, want stateShowing after esc in editing", m2.state)
	}
	if m2.message != "feat: original" {
		t.Errorf("message = %q, want original message preserved, got edit discarded incorrectly", m2.message)
	}
}

func TestUpdate_EditingPassesOtherKeysToTextarea(t *testing.T) {
	m := newModel("feat: ", func() (string, error) { return "", nil })
	m.state = stateEditing
	m.textarea.SetValue("feat: ")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m2 := updated.(model)
	if m2.state != stateEditing {
		t.Errorf("state = %v, want to stay stateEditing", m2.state)
	}
	if m2.textarea.Value() == "feat: " {
		t.Errorf("textarea value unchanged, want the 'x' keystroke to have been applied")
	}
}

func TestUpdate_RInShowingStartsRegenerating(t *testing.T) {
	m := newModel("feat: a message", func() (string, error) { return "feat: regenerated", nil })
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m2 := updated.(model)
	if m2.state != stateRegenerating {
		t.Errorf("state = %v, want stateRegenerating", m2.state)
	}
	if cmd == nil {
		t.Fatal("expected a non-nil cmd to kick off regeneration")
	}
}

func TestUpdate_RegenResultSuccessReturnsToShowing(t *testing.T) {
	m := newModel("feat: a message", func() (string, error) { return "", nil })
	m.state = stateRegenerating

	updated, _ := m.Update(regenResultMsg{text: "feat: freshly regenerated", err: nil})
	m2 := updated.(model)
	if m2.state != stateShowing {
		t.Errorf("state = %v, want stateShowing", m2.state)
	}
	if m2.message != "feat: freshly regenerated" {
		t.Errorf("message = %q, want %q", m2.message, "feat: freshly regenerated")
	}
	if m2.err != nil {
		t.Errorf("err = %v, want nil", m2.err)
	}
}

func TestUpdate_RegenResultErrorShowsFallbackWithNote(t *testing.T) {
	m := newModel("feat: a message", func() (string, error) { return "", nil })
	m.state = stateRegenerating

	regenErr := errRegenTestFailure
	updated, _ := m.Update(regenResultMsg{text: "chore: update 2 file(s) (+5/-1)", err: regenErr})
	m2 := updated.(model)
	if m2.state != stateShowing {
		t.Errorf("state = %v, want stateShowing", m2.state)
	}
	if m2.message != "chore: update 2 file(s) (+5/-1)" {
		t.Errorf("message = %q, want the fallback text passed in the msg", m2.message)
	}
	if m2.err == nil {
		t.Errorf("err = nil, want the regen error to be recorded")
	}
	if view := m2.View(); view == "" {
		t.Error("View() returned empty string")
	}
}

var errRegenTestFailure = context.DeadlineExceeded
