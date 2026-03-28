package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type AuthModel struct {
	inputs []textinput.Model
	focus  int
	err    error
}

func NewAuthModel() AuthModel {
	m := AuthModel{
		inputs: make([]textinput.Model, 2),
	}

	var t textinput.Model
	for i := range m.inputs {
		t = textinput.New()
		t.CharLimit = 100

		switch i {
		case 0:
			t.Placeholder = "Trello API Key"
			t.Focus()
		case 1:
			t.Placeholder = "Trello API Token"
			t.EchoMode = textinput.EchoPassword
			t.EchoCharacter = '•'
		}
		m.inputs[i] = t
	}
	return m
}

func (m AuthModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m AuthModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.focus == len(m.inputs)-1 {
				return m, func() tea.Msg {
					return AuthSuccessMsg{
						APIKey:   m.inputs[0].Value(),
						APIToken: m.inputs[1].Value(),
					}
				}
			}
			m.focus++
		case "up", "shift+tab":
			m.focus--
			if m.focus < 0 {
				m.focus = 0
			}
		case "down", "tab":
			m.focus++
			if m.focus >= len(m.inputs) {
				m.focus = len(m.inputs) - 1
			}
		}
	}

	for i := range m.inputs {
		if i == m.focus {
			cmds = append(cmds, m.inputs[i].Focus())
		} else {
			m.inputs[i].Blur()
		}
		newInput, cmd := m.inputs[i].Update(msg)
		m.inputs[i] = newInput
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func (m AuthModel) View() string {
	s := "Please authenticate with Trello API\n\n"
	for i := range m.inputs {
		s += m.inputs[i].View() + "\n"
	}
	s += "\n[up/down]: Move Focus • [enter]: Submit\n"
	return s
}
