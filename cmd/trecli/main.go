package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/davidmahbubi/trecli/internal/storage"
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

	model := tui.NewMainModel(store, cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
