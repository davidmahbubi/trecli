package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/davidmahbubi/trecli/internal/trello"
	"github.com/skratchdot/open-golang/open"
)

type detailState int

const (
	detailStateView detailState = iota
	detailStateMove
	detailStateEdit
	detailStateDownloadLoad
	detailStateChecklist
	detailStateAddChecklist
	detailStateAddCheckItem
	detailStateChecklistLoading
)

type flatCheckItem struct {
	listIdx int
	itemIdx int
	id      string
	state   string
}

type CardDetailModel struct {
	client   *trello.Client
	card     trello.Card
	currList trello.List
	allLists []trello.List

	width  int
	height int

	state    detailState
	moveList list.Model
	err      error
	help     help.Model

	ti          textinput.Model
	ta          textarea.Model
	formIdx     int
	formDestIdx int
	formPosIdx  int
	tiDue       textinput.Model
	tiURL       textinput.Model

	statusMsg    string
	descRendered string

	boardID          string
	boardLabels      []trello.Label
	selectedLabelIDs map[string]bool
	labelCursor      int

	checklists       []trello.Checklist
	flatCheckItems   []flatCheckItem
	checklistCursor  int
	checklistsLoaded bool
	prevState        detailState

	tiChecklist textinput.Model
	spin        spinner.Model
	loadingText string
}

type moveListItem struct {
	list trello.List
}

func (i moveListItem) Title() string       { return i.list.Name }
func (i moveListItem) Description() string { return "" }
func (i moveListItem) FilterValue() string { return i.list.Name }

func NewCardDetailModel(client *trello.Client, boardID string, card trello.Card, currList trello.List, allLists []trello.List, w, h int) CardDetailModel {
	ml := list.New([]list.Item{}, list.NewDefaultDelegate(), w, h-6)
	ml.Title = "Select List to Move Card To"
	ml.SetShowHelp(false)

	var items []list.Item
	destIdx := 0
	for i, l := range allLists {
		if l.ID != currList.ID {
			items = append(items, moveListItem{list: l})
		}
		if l.ID == card.IDList {
			destIdx = i
		}
	}
	ml.SetItems(items)

	ti := textinput.New()
	ti.Placeholder = "Card Title (required)"
	ti.SetValue(card.Name)
	ti.CharLimit = 156

	ta := textarea.New()
	ta.Placeholder = "Card Description (optional)"
	ta.SetValue(card.Desc)
	ta.SetHeight(5)

	tiDue := textinput.New()
	tiDue.Placeholder = "YYYY-MM-DD (optional)"
	tiDue.SetValue(card.Due)
	tiDue.CharLimit = 10

	tiURL := textinput.New()
	tiURL.Placeholder = "https://... (optional)"
	tiURL.SetValue(card.URLSource)

	inputWidth := (w / 2) - 4
	if inputWidth < 20 {
		inputWidth = 20
	}
	ti.Width = inputWidth
	ta.SetWidth(inputWidth)
	tiDue.Width = inputWidth
	tiURL.Width = inputWidth

	var descStr string
	if card.Desc == "" {
		descStr = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("(No description provided)")
	} else {
		wrapW := w - 8
		if wrapW < 20 {
			wrapW = 20
		}
		r, err := glamour.NewTermRenderer(
			glamour.WithStandardStyle("dark"),
			glamour.WithWordWrap(wrapW),
		)
		if err == nil {
			rendered, err := r.Render(card.Desc)
			if err == nil {
				descStr = strings.TrimSpace(rendered)
			} else {
				descStr = card.Desc
			}
		} else {
			descStr = card.Desc
		}
	}

	return CardDetailModel{
		client:           client,
		boardID:          boardID,
		card:             card,
		currList:         currList,
		allLists:         allLists,
		width:            w,
		height:           h,
		state:            detailStateView,
		moveList:         ml,
		help:             help.New(),
		ti:               ti,
		ta:               ta,
		tiDue:            tiDue,
		tiURL:            tiURL,
		formDestIdx:      destIdx,
		formPosIdx:       0,
		descRendered:     descStr,
		selectedLabelIDs: initSelectedLabels(card.Labels),
		checklistsLoaded: false,
		tiChecklist:      textinput.New(),
		spin:             spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("205")))),
	}
}

func (m CardDetailModel) Init() tea.Cmd {
	return tea.Batch(m.fetchChecklists(), m.spin.Tick)
}

func (m CardDetailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		if m.state == detailStateChecklistLoading {
			var cmd tea.Cmd
			m.spin, cmd = m.spin.Update(msg)
			return m, cmd
		}

	case errMsg:
		m.err = msg.err
		if m.state == detailStateDownloadLoad || m.state == detailStateChecklistLoading {
			if m.state == detailStateChecklistLoading {
				m.state = m.prevState
			} else {
				m.state = detailStateView
			}
		}
		return m, nil

	case attachmentDownloadedMsg:
		m.statusMsg = fmt.Sprintf("Successfully downloaded: %s", msg.filename)
		m.state = detailStateView
		return m, nil

	case boardLabelsLoadedMsg:
		m.boardLabels = msg.labels
		return m, nil

	case checklistsLoadedMsg:
		m.checklists = msg.checklists
		m.checklistsLoaded = true
		m.buildFlatCheckItems()
		if m.state == detailStateChecklistLoading {
			m.state = m.prevState
		} else if m.state == detailStateView {
			// Stay in view if we are not actively in checklist mode
			m.state = detailStateView
		}
		m.loadingText = ""
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width

		if m.card.Desc != "" {
			wrapW := msg.Width - 8
			if wrapW < 20 {
				wrapW = 20
			}
			r, err := glamour.NewTermRenderer(
				glamour.WithStandardStyle("dark"),
				glamour.WithWordWrap(wrapW),
			)
			if err == nil {
				rendered, err := r.Render(m.card.Desc)
				if err == nil {
					m.descRendered = strings.TrimSpace(rendered)
				}
			}
		}
		m.moveList.SetSize(msg.Width, msg.Height-6)

		inputWidth := (m.width / 2) - 4
		if inputWidth < 20 {
			inputWidth = 20
		}
		m.ti.Width = inputWidth
		m.ta.SetWidth(inputWidth)
		m.tiDue.Width = inputWidth
		m.tiURL.Width = inputWidth

	case tea.KeyMsg:
		if m.state == detailStateAddChecklist || m.state == detailStateAddCheckItem {
			switch msg.String() {
			case "enter":
				name := m.tiChecklist.Value()
				if name == "" {
					return m, nil
				}
				if m.state == detailStateAddChecklist {
					m.loadingText = "Creating checklist..."
					m.prevState = detailStateView // Go back to card view after new checklist
					m.state = detailStateChecklistLoading
					return m, tea.Batch(m.createChecklistCmd(name), m.spin.Tick)
				} else {
					var id string
					if len(m.flatCheckItems) > 0 {
						item := m.flatCheckItems[m.checklistCursor]
						id = m.checklists[item.listIdx].ID
					} else if len(m.checklists) > 0 {
						id = m.checklists[0].ID
					}

					if id == "" {
						m.state = detailStateView
						return m, nil
					}

					m.loadingText = "Adding checklist item..."
					m.prevState = detailStateChecklist
					m.state = detailStateChecklistLoading
					return m, tea.Batch(m.createCheckItemCmd(id, name), m.spin.Tick)
				}
			case "esc":
				if m.state == detailStateAddChecklist {
					m.state = detailStateView
				} else {
					m.state = detailStateChecklist
				}
				return m, nil
			}
			var cmd tea.Cmd
			m.tiChecklist, cmd = m.tiChecklist.Update(msg)
			return m, cmd
		}

		if m.state == detailStateEdit {
			switch msg.String() {
			case "esc":
				m.state = detailStateView
				return m, nil
			case "ctrl+s":
				title := m.ti.Value()
				if title != "" {
					posStr := "bottom"
					if m.formPosIdx == 1 {
						posStr = "top"
					}

					var labelIDs []string
					for id, selected := range m.selectedLabelIDs {
						if selected {
							labelIDs = append(labelIDs, id)
						}
					}

					opts := trello.UpdateCardOptions{
						CardID:    m.card.ID,
						ListID:    m.allLists[m.formDestIdx].ID,
						Name:      title,
						Desc:      m.ta.Value(),
						Pos:       posStr,
						Due:       m.tiDue.Value(),
						URLSource: m.tiURL.Value(),
						LabelIDs:  labelIDs,
					}

					m.state = detailStateView
					m.ti.Placeholder = "Card Title (required)"
					return m, func() tea.Msg {
						return UpdateCardMsg{Opts: opts}
					}
				} else {
					m.ti.Placeholder = "[TITLE IS REQUIRED]"
					m.formIdx = 0
					m.ti.Focus()
					m.ta.Blur()
					m.tiDue.Blur()
					m.tiURL.Blur()
				}
			case "tab", "shift+tab":
				m.formIdx = (m.formIdx + 1) % 7
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
				if m.formIdx == 5 {
					m.tiDue.Focus()
				}
				if m.formIdx == 6 {
					m.tiURL.Focus()
				}
				return m, nil
			case "left":
				if m.formIdx == 2 && m.formDestIdx > 0 {
					m.formDestIdx--
					return m, nil
				}
				if m.formIdx == 3 {
					m.formPosIdx = 0
					return m, nil
				}
				if m.formIdx == 4 && len(m.boardLabels) > 0 {
					// cycle label cursor left — handled via labelCursor field if added, skip for now
				}
			case "right":
				if m.formIdx == 2 && m.formDestIdx < len(m.allLists)-1 {
					m.formDestIdx++
					return m, nil
				}
				if m.formIdx == 3 {
					m.formPosIdx = 1
					return m, nil
				}
			case " ":
				if m.formIdx == 4 && m.labelCursor < len(m.boardLabels) {
					lid := m.boardLabels[m.labelCursor].ID
					m.selectedLabelIDs[lid] = !m.selectedLabelIDs[lid]
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
				switch msg.String() {
				case "up", "k":
					if m.labelCursor > 0 {
						m.labelCursor--
					}
				case "down", "j":
					if m.labelCursor < len(m.boardLabels)-1 {
						m.labelCursor++
					}
				}
			}
			if m.formIdx == 5 {
				m.tiDue, cmd = m.tiDue.Update(msg)
			}
			if m.formIdx == 6 {
				m.tiURL, cmd = m.tiURL.Update(msg)
			}
			return m, cmd
		}

		if m.state == detailStateMove && m.moveList.FilterState() == list.Filtering {
			break
		}

		switch msg.String() {
		case "esc", "q":
			if m.state == detailStateMove || m.state == detailStateChecklist || m.state == detailStateAddChecklist || m.state == detailStateAddCheckItem {
				m.state = detailStateView
				if m.state == detailStateAddChecklist || m.state == detailStateAddCheckItem {
					m.state = detailStateChecklist
				}
				return m, nil
			}
			if m.state == detailStateDownloadLoad {
				m.state = detailStateView
				return m, nil
			}
			m.statusMsg = ""
			m.err = nil
			return m, func() tea.Msg {
				return BackToKanbanMsg{}
			}

		case "c":
			if m.state == detailStateView && len(m.checklists) > 0 {
				m.state = detailStateChecklist
				if m.checklistCursor >= len(m.flatCheckItems) {
					m.checklistCursor = 0
				}
				return m, nil
			}

		case "n":
			if m.state == detailStateChecklist {
				m.state = detailStateAddChecklist
				m.tiChecklist.Placeholder = "Checklist Name"
				m.tiChecklist.Focus()
				m.tiChecklist.SetValue("")
				return m, nil
			}

		case "x":
			if m.state == detailStateChecklist && len(m.flatCheckItems) > 0 {
				item := m.flatCheckItems[m.checklistCursor]
				m.loadingText = "Deleting checklist..."
				m.prevState = detailStateChecklist
				m.state = detailStateChecklistLoading
				return m, tea.Batch(m.deleteChecklistCmd(m.checklists[item.listIdx].ID), m.spin.Tick)
			}

		case "up", "k":
			if m.state == detailStateChecklist {
				if m.checklistCursor > 0 {
					m.checklistCursor--
				}
				return m, nil
			}

		case "down", "j":
			if m.state == detailStateChecklist {
				if m.checklistCursor < len(m.flatCheckItems)-1 {
					m.checklistCursor++
				}
				return m, nil
			}

		case " ":
			if m.state == detailStateChecklist {
				if len(m.flatCheckItems) > 0 {
					item := m.flatCheckItems[m.checklistCursor]
					newState := "complete"
					if item.state == "complete" {
						newState = "incomplete"
					}

					m.flatCheckItems[m.checklistCursor].state = newState
					m.checklists[item.listIdx].CheckItems[item.itemIdx].State = newState

					return m, m.updateCheckItemCmd(item.id, newState)
				}
			}

		case "e":
			if m.state == detailStateView {
				m.state = detailStateEdit
				m.formIdx = 0
				m.labelCursor = 0
				m.ti.Focus()
				m.ta.Blur()
				m.tiDue.Blur()
				m.tiURL.Blur()
				if len(m.boardLabels) == 0 {
					return m, m.fetchBoardLabels()
				}
				return m, nil
			}

		case "m":
			if m.state == detailStateView {
				m.state = detailStateMove
				return m, nil
			}

		case "a":
			if m.state == detailStateChecklist && len(m.checklists) > 0 {
				m.state = detailStateAddCheckItem
				m.tiChecklist.Placeholder = "Item Name"
				m.tiChecklist.Focus()
				m.tiChecklist.SetValue("")
				return m, nil
			}
			if m.state == detailStateView {
				return m, func() tea.Msg {
					return ArchiveCardMsg{CardID: m.card.ID}
				}
			}

		case "d":
			if m.state == detailStateChecklist && len(m.flatCheckItems) > 0 {
				item := m.flatCheckItems[m.checklistCursor]
				m.loadingText = "Deleting checklist item..."
				m.prevState = detailStateChecklist
				m.state = detailStateChecklistLoading
				return m, tea.Batch(m.deleteCheckItemCmd(m.checklists[item.listIdx].ID, item.id), m.spin.Tick)
			}
			if m.state == detailStateView {
				m.state = detailStateDownloadLoad
				m.statusMsg = ""
				m.err = nil
				return m, m.downloadAttachments()
			}

		case "o":
			if m.state == detailStateView {
				if m.card.ShortUrl != "" {
					_ = open.Run(m.card.ShortUrl)
					m.statusMsg = "Opening card in default browser..."
				} else {
					m.err = fmt.Errorf("no URL available for this card")
				}
				return m, nil
			}

		case "enter":
			if m.state == detailStateMove {
				if i, ok := m.moveList.SelectedItem().(moveListItem); ok {
					opts := trello.UpdateCardOptions{
						CardID: m.card.ID,
						ListID: i.list.ID,
						Pos:    "top",
					}
					return m, func() tea.Msg {
						return UpdateCardMsg{Opts: opts}
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

	if m.state == detailStateDownloadLoad || m.state == detailStateChecklistLoading {
		txt := "Downloading attachments..."
		if m.state == detailStateChecklistLoading {
			txt = m.loadingText
		}
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			fmt.Sprintf("%s %s\n\nPress esc to cancel", m.spin.View(), txt))
	}

	if m.state == detailStateEdit {
		style := func(idx int) lipgloss.Style {
			s := lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
			if m.formIdx == idx {
				return s.Foreground(lipgloss.Color("62")).Bold(true)
			}
			return s
		}

		destView := fmt.Sprintf("[ ←  %s  → ]", m.allLists[m.formDestIdx].Name)
		posView := "[ ← Bottom → ]"
		if m.formPosIdx == 1 {
			posView = "[ ← Top → ]"
		}

		formStr := lipgloss.JoinVertical(lipgloss.Left,
			"Edit Card Details",
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
			style(4).Render("Labels (↑↓ navigate • Space to toggle):"),
			m.renderLabelPicker(),
			"",
			style(5).Render("Due Date (Optional):"),
			m.tiDue.View(),
			style(6).Render("URL Source (Optional):"),
			m.tiURL.View(),
			"",
			"(Ctrl+S to save • Tab to cycle • Esc to cancel)",
		)

		formBox := lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			Padding(1, 2).
			BorderForeground(lipgloss.Color("62")).
			Render(formStr)

		helpStr := "\n" + m.help.View(formKeys)
		editUI := lipgloss.JoinVertical(lipgloss.Left, formBox, helpStr)

		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center,
			editUI,
		)
	}

	if m.state == detailStateMove {
		helpView := "\n\n" + m.help.View(detailKeys)
		return lipgloss.JoinVertical(lipgloss.Left, m.moveList.View(), helpView)
	}

	h1 := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62")).MarginBottom(1)
	h2 := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 3).
		Width(m.width - 4)

	content := lipgloss.JoinVertical(lipgloss.Left,
		h1.Render(m.card.Name),
		h2.Render("List: ")+m.currList.Name,
	)

	if len(m.card.Labels) > 0 {
		content = lipgloss.JoinVertical(lipgloss.Left, content, "", h2.Render("Labels: ")+renderLabelBadges(m.card.Labels))
	}

	content = lipgloss.JoinVertical(lipgloss.Left, content,
		"",
		h2.Render("Description:"),
		m.descRendered,
	)

	if len(m.checklists) > 0 {
		var checklistView strings.Builder
		flatIdx := 0
		for _, list := range m.checklists {
			checklistView.WriteString(fmt.Sprintf("\n%s\n", h2.Render(list.Name)))

			total := len(list.CheckItems)
			completed := 0
			for _, item := range list.CheckItems {
				if item.State == "complete" {
					completed++
				}
			}
			if total > 0 {
				pct := (completed * 100) / total
				barLen := 20
				filled := (pct * barLen) / 100
				bar := strings.Repeat("█", filled) + strings.Repeat("░", barLen-filled)
				checklistView.WriteString(fmt.Sprintf("%d%% [%s]\n", pct, bar))
			}

			for _, item := range list.CheckItems {
				cursor := "  "
				if m.state == detailStateChecklist && flatIdx == m.checklistCursor {
					cursor = "▶ "
				}
				box := "[ ]"
				if item.State == "complete" {
					box = "[✓]"
				}
				style := lipgloss.NewStyle()
				if m.state == detailStateChecklist && flatIdx == m.checklistCursor {
					style = style.Foreground(lipgloss.Color("62"))
				}
				checklistView.WriteString(style.Render(fmt.Sprintf("%s%s %s", cursor, box, item.Name)) + "\n")
				flatIdx++
			}
		}
		content = lipgloss.JoinVertical(lipgloss.Left, content, checklistView.String())
	}

	if m.card.Due != "" {
		content = lipgloss.JoinVertical(lipgloss.Left, content, "", h2.Render("Due Date: ")+m.card.Due)
	}
	if m.card.URLSource != "" {
		content = lipgloss.JoinVertical(lipgloss.Left, content, "", h2.Render("URL Source: ")+m.card.URLSource)
	}

	if m.statusMsg != "" {
		statusStr := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(m.statusMsg)
		content = lipgloss.JoinVertical(lipgloss.Left, content, "", statusStr)
	}

	if m.state == detailStateAddChecklist || m.state == detailStateAddCheckItem {
		m.tiChecklist.Width = m.width - 10
		title := "Add New Checklist"
		if m.state == detailStateAddCheckItem {
			title = "Add Item to Checklist"
		}

		formStr := lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62")).Render(title),
			"",
			m.tiChecklist.View(),
			"",
			lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("(Enter to save • Esc to cancel)"),
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

	styledBox := box.Render(content)

	var keysToUse help.KeyMap = detailKeys
	if m.state == detailStateChecklist {
		keysToUse = checklistKeys
	}
	helpView := "\n" + m.help.View(keysToUse)

	return lipgloss.JoinVertical(lipgloss.Left, styledBox, helpView)
}

type BackToKanbanMsg struct {
	Refresh bool
}

type attachmentDownloadedMsg struct {
	filename string
}

func (m CardDetailModel) downloadAttachments() tea.Cmd {
	return func() tea.Msg {
		atts, err := m.client.GetAttachments(m.card.ID)
		if err != nil {
			return errMsg{fmt.Errorf("failed to fetch attachments: %w", err)}
		}

		if len(atts) == 0 {
			return errMsg{fmt.Errorf("no attachments found on this card")}
		}

		var files []string
		for _, target := range atts {
			b, err := m.client.DownloadAttachment(target.URL)
			if err != nil {
				return errMsg{fmt.Errorf("failed to download attachment %s: %w", target.Name, err)}
			}

			safeCardName := strings.ReplaceAll(m.card.Name, "/", "-")
			safeCardName = strings.ReplaceAll(safeCardName, "\\", "-")

			ext := filepath.Ext(target.Name)
			originalBase := strings.TrimSuffix(target.Name, ext)

			baseName := fmt.Sprintf("%s - %s", safeCardName, originalBase)

			filename := baseName + ext
			counter := 1
			for {
				if _, err := os.Stat(filename); os.IsNotExist(err) {
					break
				}
				filename = fmt.Sprintf("%s (%d)%s", baseName, counter, ext)
				counter++
			}

			err = os.WriteFile(filename, b, 0644)
			if err != nil {
				return errMsg{fmt.Errorf("failed to save file %s locally: %w", filename, err)}
			}
			files = append(files, filename)
		}

		msgStr := strings.Join(files, ", ")
		return attachmentDownloadedMsg{filename: msgStr}
	}
}

type checklistsLoadedMsg struct {
	checklists []trello.Checklist
}

func (m CardDetailModel) fetchChecklists() tea.Cmd {
	return func() tea.Msg {
		lists, err := m.client.GetChecklists(m.card.ID)
		if err != nil {
			return errMsg{fmt.Errorf("failed to load checklists: %w", err)}
		}
		return checklistsLoadedMsg{checklists: lists}
	}
}

func (m *CardDetailModel) buildFlatCheckItems() {
	var flat []flatCheckItem
	for i, list := range m.checklists {
		for j, item := range list.CheckItems {
			flat = append(flat, flatCheckItem{
				listIdx: i,
				itemIdx: j,
				id:      item.ID,
				state:   item.State,
			})
		}
	}
	m.flatCheckItems = flat
	if m.checklistCursor >= len(m.flatCheckItems) {
		m.checklistCursor = len(m.flatCheckItems) - 1
	}
	if m.checklistCursor < 0 && len(m.flatCheckItems) > 0 {
		m.checklistCursor = 0
	}
}

func (m CardDetailModel) updateCheckItemCmd(idCheckItem, state string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.UpdateCheckItemState(m.card.ID, idCheckItem, state)
		if err != nil {
			return errMsg{fmt.Errorf("failed to update checklist item: %w", err)}
		}
		return m.fetchChecklists()()
	}
}

func (m CardDetailModel) createChecklistCmd(name string) tea.Cmd {
	return func() tea.Msg {
		_, err := m.client.CreateChecklist(m.card.ID, name)
		if err != nil {
			return errMsg{fmt.Errorf("failed to create checklist: %w", err)}
		}
		return m.fetchChecklists()()
	}
}

func (m CardDetailModel) deleteChecklistCmd(checklistID string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.DeleteChecklist(checklistID)
		if err != nil {
			return errMsg{fmt.Errorf("failed to delete checklist: %w", err)}
		}
		return m.fetchChecklists()()
	}
}

func (m CardDetailModel) createCheckItemCmd(checklistID, name string) tea.Cmd {
	return func() tea.Msg {
		_, err := m.client.CreateCheckItem(checklistID, name)
		if err != nil {
			return errMsg{fmt.Errorf("failed to add checklist item: %w", err)}
		}
		return m.fetchChecklists()()
	}
}

func (m CardDetailModel) deleteCheckItemCmd(checklistID, checkItemID string) tea.Cmd {
	return func() tea.Msg {
		err := m.client.DeleteCheckItem(checklistID, checkItemID)
		if err != nil {
			return errMsg{fmt.Errorf("failed to delete checklist item: %w", err)}
		}
		return m.fetchChecklists()()
	}
}

type boardLabelsLoadedMsg struct {
	labels []trello.Label
}

func initSelectedLabels(existing []trello.Label) map[string]bool {
	m := make(map[string]bool)
	for _, l := range existing {
		m[l.ID] = true
	}
	return m
}

func (m CardDetailModel) fetchBoardLabels() tea.Cmd {
	return func() tea.Msg {
		labels, err := m.client.GetBoardLabels(m.boardID)
		if err != nil {
			return errMsg{fmt.Errorf("failed to load board labels: %w", err)}
		}
		return boardLabelsLoadedMsg{labels: labels}
	}
}

func (m CardDetailModel) renderLabelPicker() string {
	if len(m.boardLabels) == 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("  (Loading labels...)")
	}

	var sb strings.Builder
	focused := m.formIdx == 4
	for i, l := range m.boardLabels {
		name := l.Name
		if name == "" {
			name = l.Color
		}
		checked := "○"
		if m.selectedLabelIDs[l.ID] {
			checked = "◉"
		}
		cursor := "  "
		if focused && i == m.labelCursor {
			cursor = "▶ "
		}
		color := trelloColorToANSI(l.Color)
		badge := lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(fmt.Sprintf("%s%s %s", cursor, checked, name))
		sb.WriteString(badge + "\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func renderLabelBadges(labels []trello.Label) string {
	var parts []string
	for _, l := range labels {
		name := l.Name
		if name == "" {
			name = l.Color
		}
		color := trelloColorToANSI(l.Color)
		badge := lipgloss.NewStyle().
			Foreground(lipgloss.Color("0")).
			Background(lipgloss.Color(color)).
			Padding(0, 1).
			Render(name)
		parts = append(parts, badge)
	}
	return strings.Join(parts, " ")
}

func trelloColorToANSI(color string) string {
	switch color {
	case "green":
		return "green"
	case "yellow":
		return "yellow"
	case "orange":
		return "214"
	case "red":
		return "red"
	case "purple":
		return "135"
	case "blue":
		return "69"
	case "sky":
		return "81"
	case "lime":
		return "154"
	case "pink":
		return "213"
	case "black":
		return "240"
	default:
		return "252"
	}
}
