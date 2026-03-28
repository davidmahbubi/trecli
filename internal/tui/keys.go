package tui

import "github.com/charmbracelet/bubbles/key"

type KanbanKeyMap struct {
	Left  key.Binding
	Right key.Binding
	Up    key.Binding
	Down  key.Binding
	Enter key.Binding
	Quit  key.Binding
}

func (k KanbanKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Left, k.Right, k.Up, k.Down, k.Enter, k.Quit}
}

func (k KanbanKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Left, k.Right, k.Up, k.Down},
		{k.Enter, k.Quit},
	}
}

var kanbanKeys = KanbanKeyMap{
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "move left"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "move right"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "view card"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "esc"),
		key.WithHelp("q/esc", "back"),
	),
}

type DetailKeyMap struct {
	Move    key.Binding
	Archive key.Binding
	Back    key.Binding
}

func (k DetailKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Move, k.Archive, k.Back}
}

func (k DetailKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Move, k.Archive, k.Back},
	}
}

var detailKeys = DetailKeyMap{
	Move: key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "move list"),
	),
	Archive: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "archive"),
	),
	Back: key.NewBinding(
		key.WithKeys("q", "esc"),
		key.WithHelp("q/esc", "back"),
	),
}
