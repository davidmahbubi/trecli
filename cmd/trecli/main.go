package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/davidmahbubi/trecli/internal/cli"
	"github.com/davidmahbubi/trecli/internal/storage"
	"github.com/davidmahbubi/trecli/internal/trello"
	"github.com/davidmahbubi/trecli/internal/tui"
)

func main() {
	store, err := storage.New()
	if err != nil {
		fmt.Printf("Error initializing storage: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	cfg, err := store.GetConfig()
	if err != nil {
		fmt.Printf("Error checking DB config: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) > 1 {
		if cfg == nil {
			fmt.Println("Error: You are not logged in. Please run `trecli` without arguments first to authenticate.")
			os.Exit(1)
		}
		client := trello.NewClient(cfg.APIKey, cfg.APIToken)
		if cli.Run(client, os.Args) {
			return
		}
		// If cli.Run returns false, it means args were provided but they weren't enough for a command
		// However, in cli.Run we currently exit on unknown command, so it only returns false if len(args) < 2
	}

	model := tui.NewMainModel(store, cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
