package tui

import (
	"fmt"
	"github.com/charmbracelet/bubbles/list"
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
	
	list list.Model
}

type kanbanItem struct {
	card trello.Card
	list trello.List
}

func (i kanbanItem) Title() string       { return fmt.Sprintf("[%s] %s", i.list.Name, i.card.Name) }
func (i kanbanItem) Description() string { return i.card.Desc }
func (i kanbanItem) FilterValue() string { return i.card.Name }

func NewKanbanModel(client *trello.Client, boardID string, w, h int) KanbanModel {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), w, h)
	l.Title = "Kanban Board Cards"
	
	return KanbanModel{
		client:  client,
		boardID: boardID,
		cards:   make(map[string][]trello.Card),
		list:    l,
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
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
	case kanbanLoadedMsg:
		m.loaded = true
		m.lists = msg.lists
		m.cards = msg.cards
		
		var items []list.Item
		for _, l := range m.lists {
			for _, c := range m.cards[l.ID] {
				items = append(items, kanbanItem{card: c, list: l})
			}
		}
		m.list.SetItems(items)
		
		return m, nil
	case errMsg:
		m.err = msg.err
		return m, nil
	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}
		if msg.String() == "esc" || msg.String() == "q" {
			return m, func() tea.Msg {
				return BackToBoardsMsg{}
			}
		}
		if msg.String() == "enter" {
			if i, ok := m.list.SelectedItem().(kanbanItem); ok {
				return m, func() tea.Msg {
					return CardSelectedMsg{Card: i.card, List: i.list, AllLists: m.lists}
				}
			}
		}
	}
	
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	
	return m, cmd
}

func (m KanbanModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\nPress esc to go back", m.err)
	}
	if !m.loaded {
		return "Loading kanban board...\n"
	}
	
	return m.list.View()
}
