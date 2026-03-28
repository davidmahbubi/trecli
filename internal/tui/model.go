package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/davidmahbubi/trecli/internal/storage"
	"github.com/davidmahbubi/trecli/internal/trello"
)

type state int

const (
	stateAuth state = iota
	stateBoards
	stateKanban
	stateCardDetail
)

type MainModel struct {
	state   state
	storage *storage.Storage
	config  *storage.Config
	client  *trello.Client

	auth   AuthModel
	boards BoardsModel
	kanban KanbanModel
	detail CardDetailModel
	
	width  int
	height int
}

func NewMainModel(store *storage.Storage, cfg *storage.Config) MainModel {
	m := MainModel{
		storage: store,
		config:  cfg,
		auth:    NewAuthModel(),
	}

	if cfg == nil {
		m.state = stateAuth
	} else {
		m.client = trello.NewClient(cfg.APIKey, cfg.APIToken)
		m.boards = NewBoardsModel(m.client)
		m.state = stateBoards
	}

	return m
}

func (m MainModel) Init() tea.Cmd {
	if m.state == stateAuth {
		return m.auth.Init()
	}
	return m.boards.Init()
}

func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case AuthSuccessMsg:
		cfg := storage.Config{
			APIKey:   msg.APIKey,
			APIToken: msg.APIToken,
		}
		m.storage.SaveConfig(&cfg)
		m.config = &cfg
		m.client = trello.NewClient(cfg.APIKey, cfg.APIToken)
		m.boards = NewBoardsModel(m.client)
		m.state = stateBoards
		return m, m.boards.Init()
	case BoardSelectedMsg:
		m.kanban = NewKanbanModel(m.client, msg.BoardID, m.width, m.height)
		m.state = stateKanban
		return m, m.kanban.Init()
	case BackToBoardsMsg:
		m.state = stateBoards
		return m, nil
	case CardSelectedMsg:
		m.detail = NewCardDetailModel(m.client, msg.BoardID, msg.Card, msg.List, msg.AllLists, m.width, m.height)
		m.state = stateCardDetail
		return m, m.detail.Init()
	case BackToKanbanMsg:
		m.state = stateKanban
		if msg.Refresh {
			return m, m.kanban.Init()
		}
		return m, nil
	case UpdateCardMsg:
		m.state = stateKanban
		newKanban, newCmd := m.kanban.Update(msg)
		m.kanban = newKanban.(KanbanModel)
		return m, newCmd
	case ArchiveCardMsg:
		m.state = stateKanban
		newKanban, newCmd := m.kanban.Update(msg)
		m.kanban = newKanban.(KanbanModel)
		return m, newCmd
	}

	switch m.state {
	case stateAuth:
		newAuth, newCmd := m.auth.Update(msg)
		m.auth = newAuth.(AuthModel)
		cmd = newCmd
	case stateBoards:
		newBoards, newCmd := m.boards.Update(msg)
		m.boards = newBoards.(BoardsModel)
		cmd = newCmd
	case stateKanban:
		newKanban, newCmd := m.kanban.Update(msg)
		m.kanban = newKanban.(KanbanModel)
		cmd = newCmd
	case stateCardDetail:
		newDetail, newCmd := m.detail.Update(msg)
		m.detail = newDetail.(CardDetailModel)
		cmd = newCmd
	}

	return m, cmd
}

func (m MainModel) View() string {
	switch m.state {
	case stateAuth:
		return m.auth.View()
	case stateBoards:
		return m.boards.View()
	case stateKanban:
		return m.kanban.View()
	case stateCardDetail:
		return m.detail.View()
	default:
		return "Unknown state"
	}
}

// Messages
type AuthSuccessMsg struct {
	APIKey   string
	APIToken string
}

type BoardSelectedMsg struct {
	BoardID string
}

type BackToBoardsMsg struct{}

type CardSelectedMsg struct {
	BoardID  string
	Card     trello.Card
	List     trello.List
	AllLists []trello.List
}

type UpdateCardMsg struct {
	Opts trello.UpdateCardOptions
}

type ArchiveCardMsg struct {
	CardID string
}
