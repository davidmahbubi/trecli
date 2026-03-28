package tui

import (
	"fmt"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/davidmahbubi/trecli/internal/trello"
)

type detailState int

const (
	detailStateView detailState = iota
	detailStateMove
)

type CardDetailModel struct {
	client   *trello.Client
	card     trello.Card
	currList trello.List
	allLists []trello.List

	state    detailState
	moveList list.Model
	err      error
}

type moveListItem struct {
	list trello.List
}

func (i moveListItem) Title() string       { return i.list.Name }
func (i moveListItem) Description() string { return "" }
func (i moveListItem) FilterValue() string { return i.list.Name }

func NewCardDetailModel(client *trello.Client, card trello.Card, currList trello.List, allLists []trello.List, w, h int) CardDetailModel {
	ml := list.New([]list.Item{}, list.NewDefaultDelegate(), w, h)
	ml.Title = "Select List to Move Card To"

	var items []list.Item
	for _, l := range allLists {
		if l.ID != currList.ID {
			items = append(items, moveListItem{list: l})
		}
	}
	ml.SetItems(items)

	return CardDetailModel{
		client:   client,
		card:     card,
		currList: currList,
		allLists: allLists,
		state:    detailStateView,
		moveList: ml,
	}
}

func (m CardDetailModel) Init() tea.Cmd {
	return nil
}

func (m CardDetailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.moveList.SetSize(msg.Width, msg.Height)
	case tea.KeyMsg:
		if m.state == detailStateMove && m.moveList.FilterState() == list.Filtering {
			break
		}
		
		switch msg.String() {
		case "esc", "q":
			if m.state == detailStateMove {
				m.state = detailStateView
				return m, nil
			}
			return m, func() tea.Msg {
				return BackToKanbanMsg{}
			}
		
		case "m":
			if m.state == detailStateView {
				m.state = detailStateMove
				return m, nil
			}
			
		case "a":
			if m.state == detailStateView {
				// Archive
				err := m.client.ArchiveCard(m.card.ID)
				if err != nil {
					m.err = err
					return m, nil
				}
				// Go back to kanban immediately after archive
				return m, func() tea.Msg {
					return BackToKanbanMsg{Refresh: true}
				}
			}
			
		case "enter":
			if m.state == detailStateMove {
				if i, ok := m.moveList.SelectedItem().(moveListItem); ok {
					err := m.client.UpdateCardList(m.card.ID, i.list.ID)
					if err != nil {
						m.err = err
						return m, nil
					}
					return m, func() tea.Msg {
						return BackToKanbanMsg{Refresh: true}
					}
				}
			}
		}
	}

	if m.state == detailStateMove {
		var cmd tea.Cmd
		m.moveList, cmd = m.moveList.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m CardDetailModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress esc to return", m.err)
	}

	if m.state == detailStateMove {
		return m.moveList.View()
	}

	s := fmt.Sprintf("=== Card Details ===\n\n")
	s += fmt.Sprintf("Title: %s\n", m.card.Name)
	s += fmt.Sprintf("List: %s\n", m.currList.Name)
	s += fmt.Sprintf("Description: \n%s\n\n", m.card.Desc)
	
	s += "\n--- Actions ---\n"
	s += "[m] Move to another list\n"
	s += "[a] Archive card\n"
	s += "[esc/q] Back to Kanban\n"
	
	return s
}

type BackToKanbanMsg struct {
	Refresh bool
}
