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
)

type MainModel struct {
	state   state
	storage *storage.Storage
	config  *storage.Config
	client  *trello.Client

	auth   AuthModel
	boards BoardsModel
	kanban KanbanModel
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
		m.kanban = NewKanbanModel(m.client, msg.BoardID)
		m.state = stateKanban
		return m, m.kanban.Init()
	case BackToBoardsMsg:
		m.state = stateBoards
		return m, m.boards.Init()
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
