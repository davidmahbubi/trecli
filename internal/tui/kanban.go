package tui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/davidmahbubi/trecli/internal/trello"
)

type KanbanModel struct {
	client  *trello.Client
	boardID string
	err     error
	loaded  bool
	
	lists []trello.List
	cards map[string][]trello.Card // listID -> cards
}

func NewKanbanModel(client *trello.Client, boardID string) KanbanModel {
	return KanbanModel{
		client:  client,
		boardID: boardID,
		cards:   make(map[string][]trello.Card),
	}
}

type kanbanLoadedMsg struct {
	lists []trello.List
	cards map[string][]trello.Card
}

func (m KanbanModel) loadKanban() tea.Msg {
	lists, err := m.client.GetLists(m.boardID)
	if err != nil {
		return errMsg{err}
	}
	
	cardsMap := make(map[string][]trello.Card)
	for _, l := range lists {
		cards, err := m.client.GetCardsInList(l.ID)
		if err != nil {
			return errMsg{err}
		}
		cardsMap[l.ID] = cards
	}
	
	return kanbanLoadedMsg{lists: lists, cards: cardsMap}
}

func (m KanbanModel) Init() tea.Cmd {
	return m.loadKanban
}

func (m KanbanModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case kanbanLoadedMsg:
		m.loaded = true
		m.lists = msg.lists
		m.cards = msg.cards
		return m, nil
	case errMsg:
		m.err = msg.err
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "esc" {
			return m, func() tea.Msg {
				return BackToBoardsMsg{}
			}
		}
	}
	return m, nil
}

func (m KanbanModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\nPress esc to go back", m.err)
	}
	if !m.loaded {
		return "Loading kanban board...\n"
	}
	
	s := "Board Kanban\n"
	s += "(Not interactive yet. Press 'esc' to go back)\n\n"
	
	for _, l := range m.lists {
		s += fmt.Sprintf("=== %s ===\n", l.Name)
		cards := m.cards[l.ID]
		for _, c := range cards {
			s += fmt.Sprintf(" - %s\n", c.Name)
		}
		s += "\n"
	}
	return s
}
