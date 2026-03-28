package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/davidmahbubi/trecli/internal/trello"
)

var (
	focusedBorderColor   = lipgloss.Color("62")  // Purple
	unfocusedBorderColor = lipgloss.Color("240") // Dark Gray

	focusedStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(focusedBorderColor).
			Padding(1, 1)

	unfocusedStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(unfocusedBorderColor).
			Padding(1, 1)
)

type kanbanUIState int

const (
	kanbanStateList kanbanUIState = iota
	kanbanStateAddCard
)

type KanbanModel struct {
	client  *trello.Client
	boardID string
	err     error
	loaded  bool

	width  int
	height int

	tLists []trello.List
	cards  map[string][]trello.Card // listID -> cards

	models         []list.Model
	focusedListIdx int
	windowStartIdx int

	spin spinner.Model
	help help.Model

	uiState     kanbanUIState
	ti          textinput.Model
	ta          textarea.Model
	formIdx     int
	formDestIdx int
}

type kanbanItem struct {
	card  trello.Card
	tList trello.List
}

func (i kanbanItem) Title() string       { return i.card.Name }
func (i kanbanItem) Description() string { return i.card.Desc }
func (i kanbanItem) FilterValue() string { return i.card.Name }

func NewKanbanModel(client *trello.Client, boardID string, w, h int) KanbanModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	ti := textinput.New()
	ti.Placeholder = "Card Title (required)"
	ti.CharLimit = 156

	ta := textarea.New()
	ta.Placeholder = "Card Description (optional)"
	ta.SetHeight(5)

	return KanbanModel{
		client:  client,
		boardID: boardID,
		cards:   make(map[string][]trello.Card),
		width:   w,
		height:  h,
		spin:    s,
		help:    help.New(),
		ti:      ti,
		ta:      ta,
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

func (m KanbanModel) createCard(listID, title, desc string) tea.Cmd {
	return func() tea.Msg {
		_, err := m.client.CreateCard(listID, title, desc)
		if err != nil {
			return errMsg{err}
		}
		return m.loadKanban()
	}
}

func (m KanbanModel) Init() tea.Cmd {
	return tea.Batch(m.loadKanban, m.spin.Tick)
}

func (m KanbanModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case spinner.TickMsg:
		if !m.loaded {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width

		m.ti.Width = (m.width / 2) - 4
		if m.ti.Width < 20 {
			m.ti.Width = 20
		}
		m.ta.SetWidth((m.width / 2) - 4)
		if m.ta.Width() < 20 {
			m.ta.SetWidth(20)
		}

		m.resizeModels()
		m.adjustWindow()
		return m, nil

	case kanbanLoadedMsg:
		m.loaded = true
		m.tLists = msg.lists
		m.cards = msg.cards

		m.models = make([]list.Model, len(m.tLists))
		for i, l := range m.tLists {
			delegate := list.NewDefaultDelegate()
			delegate.ShowDescription = false // Hide description to save vertical space

			lm := list.New([]list.Item{}, delegate, 0, 0)
			lm.Title = l.Name
			lm.SetShowHelp(false)

			var items []list.Item
			for _, c := range m.cards[l.ID] {
				items = append(items, kanbanItem{card: c, tList: l})
			}
			lm.SetItems(items)
			m.models[i] = lm
		}
		m.resizeModels()
		m.adjustWindow()
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		if m.uiState == kanbanStateAddCard {
			switch msg.String() {
			case "esc":
				m.uiState = kanbanStateList
				m.ti.SetValue("")
				m.ta.SetValue("")
				return m, nil
			case "ctrl+s":
				title := m.ti.Value()
				if title != "" {
					m.loaded = false
					m.uiState = kanbanStateList
					desc := m.ta.Value()
					listID := m.tLists[m.formDestIdx].ID

					m.ti.SetValue("")
					m.ta.SetValue("")
					return m, tea.Batch(m.createCard(listID, title, desc), m.spin.Tick)
				}
			case "tab", "shift+tab":
				m.formIdx = (m.formIdx + 1) % 3
				if m.formIdx == 0 {
					m.ta.Blur()
					m.ti.Focus()
				} else if m.formIdx == 1 {
					m.ti.Blur()
					m.ta.Focus()
				} else {
					m.ti.Blur()
					m.ta.Blur()
				}
				return m, nil
			case "left", "h":
				if m.formIdx == 2 && m.formDestIdx > 0 {
					m.formDestIdx--
					return m, nil
				}
			case "right", "l":
				if m.formIdx == 2 && m.formDestIdx < len(m.tLists)-1 {
					m.formDestIdx++
					return m, nil
				}
			}

			// pass updates to text inputs only
			var cmd tea.Cmd
			if m.formIdx == 0 {
				m.ti, cmd = m.ti.Update(msg)
			} else if m.formIdx == 1 {
				m.ta, cmd = m.ta.Update(msg)
			}
			return m, cmd
		}

		// List State Key Bindings
		if m.loaded && len(m.models) > 0 {
			if m.models[m.focusedListIdx].FilterState() != list.Filtering {
				switch msg.String() {
				case "esc", "q":
					return m, func() tea.Msg {
						return BackToBoardsMsg{}
					}
				case "left", "h":
					if m.focusedListIdx > 0 {
						m.focusedListIdx--
						m.adjustWindow()
					}
					return m, nil
				case "right", "l":
					if m.focusedListIdx < len(m.models)-1 {
						m.focusedListIdx++
						m.adjustWindow()
					}
					return m, nil
				case "n", "c":
					m.uiState = kanbanStateAddCard
					m.formIdx = 0
					m.formDestIdx = m.focusedListIdx
					m.ti.Focus()
					m.ta.Blur()
					return m, nil
				case "enter":
					if i, ok := m.models[m.focusedListIdx].SelectedItem().(kanbanItem); ok {
						return m, func() tea.Msg {
							return CardSelectedMsg{Card: i.card, List: i.tList, AllLists: m.tLists}
						}
					}
				}
			}
		}
	}

	if m.loaded && len(m.models) > 0 {
		var cmd tea.Cmd
		m.models[m.focusedListIdx], cmd = m.models[m.focusedListIdx].Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *KanbanModel) adjustWindow() {
	targetColWidth := 40
	if m.width < 40 {
		targetColWidth = m.width
	}
	visibleCols := m.width / targetColWidth
	if visibleCols < 1 {
		visibleCols = 1
	}

	if m.focusedListIdx < m.windowStartIdx {
		m.windowStartIdx = m.focusedListIdx
	} else if m.focusedListIdx >= m.windowStartIdx+visibleCols {
		m.windowStartIdx = m.focusedListIdx - visibleCols + 1
	}
}

func (m *KanbanModel) resizeModels() {
	if !m.loaded || len(m.models) == 0 {
		return
	}

	targetColWidth := 40
	if m.width < 40 {
		targetColWidth = m.width
	}

	// Subtract borders and padding (4) + 2 for help menu height
	listWidth := targetColWidth - 4
	listHeight := m.height - 6

	if listWidth < 10 {
		listWidth = 10
	}
	if listHeight < 5 {
		listHeight = 5
	}

	for i := range m.models {
		m.models[i].SetSize(listWidth, listHeight)
	}
}

func (m KanbanModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\nPress esc to go back", m.err)
	}
	if !m.loaded {
		return "\n  " + m.spin.View() + " Loading kanban board...\n"
	}

	if len(m.models) == 0 {
		return "Board is empty. Press esc to go back.\n"
	}

	targetColWidth := 40
	if m.width < 40 {
		targetColWidth = m.width
	}
	visibleCols := m.width / targetColWidth
	if visibleCols < 1 {
		visibleCols = 1
	}

	endIdx := m.windowStartIdx + visibleCols
	if endIdx > len(m.models) {
		endIdx = len(m.models)
	}

	var views []string
	for i := m.windowStartIdx; i < endIdx; i++ {
		mod := m.models[i]
		v := mod.View()
		if i == m.focusedListIdx {
			v = focusedStyle.Render(v)
		} else {
			v = unfocusedStyle.Render(v)
		}
		views = append(views, v)
	}

	boardView := lipgloss.JoinHorizontal(lipgloss.Top, views...)

	if m.uiState == kanbanStateAddCard {
		tStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
		if m.formIdx == 0 {
			tStyle = tStyle.Foreground(lipgloss.Color("62")).Bold(true)
		}

		dStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
		if m.formIdx == 1 {
			dStyle = dStyle.Foreground(lipgloss.Color("62")).Bold(true)
		}

		lStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
		if m.formIdx == 2 {
			lStyle = lStyle.Foreground(lipgloss.Color("62")).Bold(true)
		}

		destView := fmt.Sprintf("[ ←  %s  → ]", m.tLists[m.formDestIdx].Name)

		formStr := lipgloss.JoinVertical(lipgloss.Left,
			"Add New Card",
			"",
			tStyle.Render("Title:"),
			m.ti.View(),
			"",
			dStyle.Render("Description:"),
			m.ta.View(),
			"",
			lStyle.Render("Destination List:"),
			lStyle.Render(destView),
			"",
			"(Ctrl+S to save • Tab to cycle • Esc to cancel)",
		)

		formBox := lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			Padding(1, 2).
			BorderForeground(lipgloss.Color("62")).
			Render(formStr)

		boardView = lipgloss.Place(m.width, m.height-3,
			lipgloss.Center, lipgloss.Center,
			formBox,
		)
	}

	keysToUse := kanbanKeys
	helpView := "\n" + m.help.View(keysToUse)

	if m.uiState == kanbanStateAddCard {
		helpView = "\n" + m.help.View(formKeys)
	}

	return lipgloss.JoinVertical(lipgloss.Left, boardView, helpView)
}
