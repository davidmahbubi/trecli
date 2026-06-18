package tui

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/davidmahbubi/trecli/internal/trello"
)

type boardsUIState int

const (
	boardsStateList boardsUIState = iota
	boardsStateAddBoard
)

type BoardsModel struct {
	client      *trello.Client
	list        list.Model
	spin        spinner.Model
	err         error
	loaded      bool
	uiState     boardsUIState
	tiBoard     textinput.Model
	loadingText string
	width       int
	height      int
}

type boardItem struct {
	board trello.Board
}

func (i boardItem) Title() string       { return i.board.Name }
func (i boardItem) Description() string { return i.board.Desc }
func (i boardItem) FilterValue() string { return i.board.Name }

func NewBoardsModel(client *trello.Client) BoardsModel {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 80, 20)
	l.Title = "Select Trello Board (Press 'n' to create new)"

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	ti := textinput.New()
	ti.Placeholder = "Board Name"
	ti.CharLimit = 100

	return BoardsModel{
		client:  client,
		list:    l,
		spin:    s,
		tiBoard: ti,
	}
}

type boardsLoadedMsg []trello.Board
type errMsg struct{ err error }
type boardCreatedMsg struct{}

func (e errMsg) Error() string { return e.err.Error() }

func (m BoardsModel) loadBoards() tea.Msg {
	boards, err := m.client.GetBoards()
	if err != nil {
		return errMsg{err}
	}
	return boardsLoadedMsg(boards)
}

func (m BoardsModel) Init() tea.Cmd {
	return tea.Batch(m.loadBoards, m.spin.Tick)
}

func (m BoardsModel) createBoardReq(name string) tea.Cmd {
	return func() tea.Msg {
		_, err := m.client.CreateBoard(name, "")
		if err != nil {
			return errMsg{err}
		}
		return boardCreatedMsg{}
	}
}

func (m BoardsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if !m.loaded || m.loadingText != "" {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height)
		m.tiBoard.Width = (msg.Width / 2) - 4
		if m.tiBoard.Width < 20 {
			m.tiBoard.Width = 20
		}
		return m, nil
	case boardCreatedMsg:
		m.loadingText = ""
		return m, m.loadBoards
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
		m.loadingText = ""
		return m, nil
	case tea.KeyMsg:
		if m.uiState == boardsStateAddBoard {
			switch msg.String() {
			case "esc":
				m.uiState = boardsStateList
				return m, nil
			case "enter":
				name := m.tiBoard.Value()
				if name != "" {
					m.uiState = boardsStateList
					m.loadingText = "Creating board..."
					m.tiBoard.SetValue("")
					return m, tea.Batch(m.createBoardReq(name), m.spin.Tick)
				}
			}
			var cmd tea.Cmd
			m.tiBoard, cmd = m.tiBoard.Update(msg)
			return m, cmd
		}

		if m.list.FilterState() == list.Filtering {
			break
		}

		switch msg.String() {
		case "n", "c":
			m.uiState = boardsStateAddBoard
			m.tiBoard.Focus()
			return m, nil
		case "enter":
			i, ok := m.list.SelectedItem().(boardItem)
			if ok {
				return m, func() tea.Msg {
					return BoardSelectedMsg{BoardID: i.board.ID, BoardName: i.board.Name, BoardURL: i.board.URL}
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
		return "Error: " + m.err.Error()
	}
	if (!m.loaded && m.loadingText == "") || m.loadingText != "" {
		txt := "Loading boards..."
		if m.loadingText != "" {
			txt = m.loadingText
		}
		return "\n  " + m.spin.View() + " " + txt + "\n"
	}
	
	if m.uiState == boardsStateAddBoard {
		formStr := lipgloss.JoinVertical(lipgloss.Left,
			"Add New Board",
			"",
			m.tiBoard.View(),
			"",
			"(Enter to save • Esc to cancel)",
		)

		formBox := lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			Padding(1, 2).
			BorderForeground(lipgloss.Color("62")).
			Render(formStr)

		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			formBox,
		)
	}

	return m.list.View()
}
