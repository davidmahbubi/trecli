package tui

import (
	"fmt"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/davidmahbubi/trecli/internal/trello"
)

type BoardsModel struct {
	client *trello.Client
	list   list.Model
	err    error
	loaded bool
}

type boardItem struct {
	board trello.Board
}

func (i boardItem) Title() string       { return i.board.Name }
func (i boardItem) Description() string { return i.board.Desc }
func (i boardItem) FilterValue() string { return i.board.Name }

func NewBoardsModel(client *trello.Client) BoardsModel {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 80, 20)
	l.Title = "Trello Boards"
	return BoardsModel{
		client: client,
		list:   l,
	}
}

type boardsLoadedMsg []trello.Board
type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

func (m BoardsModel) loadBoards() tea.Msg {
	boards, err := m.client.GetBoards()
	if err != nil {
		return errMsg{err}
	}
	return boardsLoadedMsg(boards)
}

func (m BoardsModel) Init() tea.Cmd {
	return m.loadBoards
}

func (m BoardsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
		return m, nil
	case boardsLoadedMsg:
		m.loaded = true
		items := make([]list.Item, len(msg))
		for i, b := range msg {
			items[i] = boardItem{board: b}
		}
		m.list.SetItems(items)
		return m, nil
	case errMsg:
		m.err = msg.err
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "enter" {
			i, ok := m.list.SelectedItem().(boardItem)
			if ok {
				return m, func() tea.Msg {
					return BoardSelectedMsg{BoardID: i.board.ID}
				}
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m BoardsModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error loading boards: %v\n", m.err)
	}
	if !m.loaded {
		return "Loading boards...\n"
	}
	return m.list.View()
}
