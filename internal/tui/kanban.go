package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/davidmahbubi/trecli/internal/trello"
	"github.com/skratchdot/open-golang/open"
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
	kanbanStateMoveCard
)

type KanbanModel struct {
	client      *trello.Client
	boardID     string
	err         error
	loaded      bool
	loadingText string

	width  int
	height int

	tLists []trello.List
	cards  map[string][]trello.Card

	models         []list.Model
	focusedListIdx int
	windowStartIdx int

	spin spinner.Model
	help help.Model

	uiState      kanbanUIState
	ti           textinput.Model
	ta           textarea.Model
	formIdx      int
	formDestIdx  int
	formPosIdx   int
	tiDue        textinput.Model
	tiURL        textinput.Model
	moveList     list.Model
	selectedCard trello.Card
	boardURL     string
	statusMsg    string
}

type kanbanItem struct {
	card  trello.Card
	tList trello.List
}

func (i kanbanItem) Title() string { return i.card.Name }
func (i kanbanItem) Description() string {
	var parts []string

	if len(i.card.Labels) > 0 {
		var labelStrs []string
		for _, l := range i.card.Labels {
			name := l.Name
			if name == "" {
				name = l.Color
			}
			if name != "" {
				labelStrs = append(labelStrs, "["+name+"]")
			}
		}
		if len(labelStrs) > 0 {
			parts = append(parts, strings.Join(labelStrs, " "))
		}
	}

	desc := i.card.Desc
	if desc == "" {
		desc = "No Description"
	}
	parts = append(parts, desc)

	return strings.Join(parts, " • ")
}
func (i kanbanItem) FilterValue() string { return i.card.Name }

func NewKanbanModel(client *trello.Client, boardID, boardURL string, w, h int) KanbanModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	ti := textinput.New()
	ti.Placeholder = "Card Title (required)"
	ti.CharLimit = 156

	ta := textarea.New()
	ta.Placeholder = "Card Description (optional)"
	ta.SetHeight(5)

	tiDue := textinput.New()
	tiDue.Placeholder = "YYYY-MM-DD (optional)"
	tiDue.CharLimit = 10

	tiURL := textinput.New()
	tiURL.Placeholder = "https://... (optional)"

	ml := list.New([]list.Item{}, list.NewDefaultDelegate(), w/2, h*2/3)
	ml.Title = "Select List to Move Card To"
	ml.SetShowHelp(false)

	return KanbanModel{
		client:   client,
		boardID:  boardID,
		boardURL: boardURL,
		cards:    make(map[string][]trello.Card),
		width:    w,
		height:   h,
		spin:     s,
		help:     help.New(),
		ti:       ti,
		ta:       ta,
		tiDue:    tiDue,
		tiURL:    tiURL,
		moveList: ml,
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

func (m KanbanModel) createCard(opts trello.CreateCardOptions) tea.Cmd {
	return func() tea.Msg {
		_, err := m.client.CreateCard(opts)
		if err != nil {
			return errMsg{err}
		}
		return m.loadKanban()
	}
}

func (m KanbanModel) updateCardReq(opts trello.UpdateCardOptions) tea.Cmd {
	return func() tea.Msg {
		_, err := m.client.UpdateCard(opts)
		if err != nil {
			return errMsg{err}
		}
		return m.loadKanban()
	}
}

func (m KanbanModel) archiveCardReq(cardID string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.ArchiveCard(cardID)
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
		if !m.loaded || m.loadingText != "" {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}

	case clearStatusMsg:
		m.statusMsg = ""
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width

		inputWidth := (m.width / 2) - 4
		if inputWidth < 20 {
			inputWidth = 20
		}
		m.ti.Width = inputWidth
		m.ta.SetWidth(inputWidth)
		m.tiDue.Width = inputWidth
		m.tiURL.Width = inputWidth
		m.moveList.SetSize(msg.Width/2, msg.Height*2/3)

		m.resizeModels()
		m.adjustWindow()
		return m, nil

	case kanbanLoadedMsg:
		m.loaded = true
		m.loadingText = ""
		m.tLists = msg.lists
		m.cards = msg.cards

		m.models = make([]list.Model, len(m.tLists))
		for i, l := range m.tLists {
			delegate := list.NewDefaultDelegate()
			delegate.ShowDescription = true

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
		m.loadingText = ""
		return m, nil

	case UpdateCardMsg:
		m.loadingText = "Updating card..."
		return m, tea.Batch(m.updateCardReq(msg.Opts), m.spin.Tick)

	case ArchiveCardMsg:
		m.loadingText = "Archiving card..."
		return m, tea.Batch(m.archiveCardReq(msg.CardID), m.spin.Tick)

	case tea.KeyMsg:
		if m.uiState == kanbanStateAddCard {
			switch msg.String() {
			case "esc":
				m.uiState = kanbanStateList
				m.ti.SetValue("")
				m.ta.SetValue("")
				m.tiDue.SetValue("")
				m.tiURL.SetValue("")
				return m, nil
			case "ctrl+s":
				title := m.ti.Value()
				if title != "" {
					m.loadingText = "Creating card..."
					m.uiState = kanbanStateList

					posStr := "bottom"
					if m.formPosIdx == 1 {
						posStr = "top"
					}

					opts := trello.CreateCardOptions{
						ListID:    m.tLists[m.formDestIdx].ID,
						Name:      title,
						Desc:      m.ta.Value(),
						Pos:       posStr,
						Due:       m.tiDue.Value(),
						URLSource: m.tiURL.Value(),
					}

					m.ti.SetValue("")
					m.ta.SetValue("")
					m.tiDue.SetValue("")
					m.tiURL.SetValue("")
					m.ti.Placeholder = "Card Title (required)"
					return m, tea.Batch(m.createCard(opts), m.spin.Tick)
				} else {
					m.ti.Placeholder = "[TITLE IS REQUIRED]"
					m.formIdx = 0
					m.ti.Focus()
					m.ta.Blur()
					m.tiDue.Blur()
					m.tiURL.Blur()
				}
			case "tab", "shift+tab":
				m.formIdx = (m.formIdx + 1) % 6
				m.ti.Blur()
				m.ta.Blur()
				m.tiDue.Blur()
				m.tiURL.Blur()

				if m.formIdx == 0 {
					m.ti.Focus()
				}
				if m.formIdx == 1 {
					m.ta.Focus()
				}
				if m.formIdx == 4 {
					m.tiDue.Focus()
				}
				if m.formIdx == 5 {
					m.tiURL.Focus()
				}
				return m, nil
			case "left":
				if m.formIdx == 2 {
					if m.formDestIdx > 0 {
						m.formDestIdx--
					}
					return m, nil
				}
				if m.formIdx == 3 {
					m.formPosIdx = 0
					return m, nil
				}
			case "right":
				if m.formIdx == 2 {
					if m.formDestIdx < len(m.tLists)-1 {
						m.formDestIdx++
					}
					return m, nil
				}
				if m.formIdx == 3 {
					m.formPosIdx = 1
					return m, nil
				}
			}

			var cmd tea.Cmd
			if m.formIdx == 0 {
				m.ti, cmd = m.ti.Update(msg)
			}
			if m.formIdx == 1 {
				m.ta, cmd = m.ta.Update(msg)
			}
			if m.formIdx == 4 {
				m.tiDue, cmd = m.tiDue.Update(msg)
			}
			if m.formIdx == 5 {
				m.tiURL, cmd = m.tiURL.Update(msg)
			}
			return m, cmd
		}

		if m.uiState == kanbanStateMoveCard {
			if m.moveList.FilterState() == list.Filtering {
				var cmd tea.Cmd
				m.moveList, cmd = m.moveList.Update(msg)
				return m, cmd
			}

			switch msg.String() {
			case "esc", "q":
				m.uiState = kanbanStateList
				return m, nil
			case "enter":
				if i, ok := m.moveList.SelectedItem().(moveListItem); ok {
					opts := trello.UpdateCardOptions{
						CardID: m.selectedCard.ID,
						ListID: i.list.ID,
						Pos:    "top",
					}
					m.uiState = kanbanStateList
					m.loadingText = "Moving card..."
					return m, tea.Batch(m.updateCardReq(opts), m.spin.Tick)
				}
			}

			var cmd tea.Cmd
			m.moveList, cmd = m.moveList.Update(msg)
			return m, cmd
		}

		if m.loaded && len(m.models) > 0 {
			if m.models[m.focusedListIdx].FilterState() != list.Filtering {
				switch msg.String() {
				case "esc", "q":
					return m, func() tea.Msg { return BackToBoardsMsg{} }
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
					m.formPosIdx = 0
					m.ti.Focus()
					m.ta.Blur()
					m.tiDue.Blur()
					m.tiURL.Blur()
					return m, nil
				case "m":
					if i, ok := m.models[m.focusedListIdx].SelectedItem().(kanbanItem); ok {
						m.uiState = kanbanStateMoveCard
						m.selectedCard = i.card

						var items []list.Item
						for _, l := range m.tLists {
							if l.ID != i.tList.ID {
								items = append(items, moveListItem{list: l})
							}
						}
						m.moveList.SetItems(items)
					}
					return m, nil
				case "enter":
					if i, ok := m.models[m.focusedListIdx].SelectedItem().(kanbanItem); ok {
						return m, func() tea.Msg {
							return CardSelectedMsg{BoardID: m.boardID, Card: i.card, List: i.tList, AllLists: m.tLists}
						}
					}
				case "o":
					if i, ok := m.models[m.focusedListIdx].SelectedItem().(kanbanItem); ok {
						if i.card.ShortUrl != "" {
							_ = open.Run(i.card.ShortUrl)
							m.statusMsg = "Opening card in default browser..."
							return m, m.clearStatusAfter(3 * time.Second)
						} else {
							m.err = fmt.Errorf("no URL available for this card")
						}
					}
					return m, nil
				case "ctrl+o":
					if m.boardURL != "" {
						_ = open.Run(m.boardURL)
						m.statusMsg = "Opening board in default browser..."
						return m, m.clearStatusAfter(3 * time.Second)
					} else {
						m.err = fmt.Errorf("no URL available for this board")
					}
					return m, nil
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

	listWidth := targetColWidth - 4
	listHeight := m.height - 8

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
	if !m.loaded && m.loadingText == "" {
		return "\n  " + m.spin.View() + " Loading kanban board...\n"
	}

	if len(m.models) == 0 && m.loadingText == "" {
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

	leftArrow := "  "
	if m.windowStartIdx > 0 {
		leftArrow = "← "
	}
	rightArrow := "  "
	if endIdx < len(m.models) {
		rightArrow = " →"
	}
	indicatorText := fmt.Sprintf("%s Showing lists %d-%d of %d %s", leftArrow, m.windowStartIdx+1, endIdx, len(m.models), rightArrow)
	indicatorView := lipgloss.NewStyle().
		Foreground(lipgloss.Color("62")).
		PaddingBottom(1).
		Render(indicatorText)
	boardView = lipgloss.JoinVertical(lipgloss.Left, indicatorView, boardView)


	if m.uiState == kanbanStateAddCard {
		style := func(idx int) lipgloss.Style {
			s := lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
			if m.formIdx == idx {
				return s.Foreground(lipgloss.Color("62")).Bold(true)
			}
			return s
		}

		destView := fmt.Sprintf("[ ←  %s  → ]", m.tLists[m.formDestIdx].Name)
		posView := "[ ← Bottom → ]"
		if m.formPosIdx == 1 {
			posView = "[ ← Top → ]"
		}

		formStr := lipgloss.JoinVertical(lipgloss.Left,
			"Add New Card",
			"",
			style(0).Render("Title:"),
			m.ti.View(),
			style(1).Render("Description:"),
			m.ta.View(),
			"",
			style(2).Render("Destination List:"),
			style(2).Render(destView),
			"",
			style(3).Render("Position:"),
			style(3).Render(posView),
			"",
			style(4).Render("Due Date (Optional):"),
			m.tiDue.View(),
			style(5).Render("URL Source (Optional):"),
			m.tiURL.View(),
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
	} else if m.uiState == kanbanStateMoveCard {
		moveListStr := m.moveList.View()
		moveBox := lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			Padding(1, 2).
			BorderForeground(lipgloss.Color("62")).
			Render(moveListStr)

		boardView = lipgloss.Place(m.width, m.height-3,
			lipgloss.Center, lipgloss.Center,
			moveBox,
		)
	} else if m.loadingText != "" {
		popupStr := lipgloss.JoinHorizontal(lipgloss.Center, m.spin.View(), " "+m.loadingText)
		popupBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 4).
			BorderForeground(lipgloss.Color("62")).
			Render(popupStr)

		boardView = lipgloss.Place(m.width, m.height-3,
			lipgloss.Center, lipgloss.Center,
			popupBox,
		)
	}

	if m.statusMsg != "" {
		statusStr := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(m.statusMsg)
		boardView = lipgloss.JoinVertical(lipgloss.Center, boardView, statusStr)
	}

	keysToUse := kanbanKeys
	helpView := "\n" + m.help.View(keysToUse)

	if m.uiState == kanbanStateAddCard {
		helpView = "\n" + m.help.View(formKeys)
	}

	return lipgloss.JoinVertical(lipgloss.Left, boardView, helpView)
}

type clearStatusMsg struct{}

func (m KanbanModel) clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}
