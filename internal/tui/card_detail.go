package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
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
)

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
}

type moveListItem struct {
	list trello.List
}

func (i moveListItem) Title() string       { return i.list.Name }
func (i moveListItem) Description() string { return "" }
func (i moveListItem) FilterValue() string { return i.list.Name }

func NewCardDetailModel(client *trello.Client, card trello.Card, currList trello.List, allLists []trello.List, w, h int) CardDetailModel {
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
		client:       client,
		card:         card,
		currList:     currList,
		allLists:     allLists,
		width:        w,
		height:       h,
		state:        detailStateView,
		moveList:     ml,
		help:         help.New(),
		ti:           ti,
		ta:           ta,
		tiDue:        tiDue,
		tiURL:        tiURL,
		formDestIdx:  destIdx,
		formPosIdx:   0,
		descRendered: descStr,
	}
}

func (m CardDetailModel) Init() tea.Cmd {
	return nil
}

func (m CardDetailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case errMsg:
		m.err = msg.err
		if m.state == detailStateDownloadLoad {
			m.state = detailStateView
		}
		return m, nil

	case attachmentDownloadedMsg:
		m.statusMsg = fmt.Sprintf("Successfully downloaded: %s", msg.filename)
		m.state = detailStateView
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width

		// Re-render markdown on window size change
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

					opts := trello.UpdateCardOptions{
						CardID:    m.card.ID,
						ListID:    m.allLists[m.formDestIdx].ID,
						Name:      title,
						Desc:      m.ta.Value(),
						Pos:       posStr,
						Due:       m.tiDue.Value(),
						URLSource: m.tiURL.Value(),
					}

					m.state = detailStateView
					m.ti.Placeholder = "Card Title (required)"
					return m, func() tea.Msg {
						return UpdateCardMsg{Opts: opts}
					}
				} else {
					// Give visual feedback when title is empty
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
				if m.formIdx == 2 && m.formDestIdx > 0 {
					m.formDestIdx--
					return m, nil
				}
				if m.formIdx == 3 {
					m.formPosIdx = 0
					return m, nil
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

		if m.state == detailStateMove && m.moveList.FilterState() == list.Filtering {
			break
		}

		switch msg.String() {
		case "esc", "q":
			if m.state == detailStateMove {
				m.state = detailStateView
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

		case "e":
			if m.state == detailStateView {
				m.state = detailStateEdit
				m.formIdx = 0
				m.ti.Focus()
				m.ta.Blur()
				m.tiDue.Blur()
				m.tiURL.Blur()
				return m, nil
			}

		case "m":
			if m.state == detailStateView {
				m.state = detailStateMove
				return m, nil
			}

		case "a":
			if m.state == detailStateView {
				return m, func() tea.Msg {
					return ArchiveCardMsg{CardID: m.card.ID}
				}
			}

		case "d":
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

	if m.state == detailStateDownloadLoad {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			"Downloading attachments...\n\nPress esc to cancel")
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

	// Default View
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
		"",
		h2.Render("Description:"),
		m.descRendered,
	)

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

	styledBox := box.Render(content)
	helpView := "\n" + m.help.View(detailKeys)

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
