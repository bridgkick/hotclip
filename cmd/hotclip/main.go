package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/bridgkick/hotclip/internal/store"
	"github.com/bridgkick/hotclip/internal/ui"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hotclip: %v\n", err)
		os.Exit(1)
	}
	dataPath := filepath.Join(home, ".hotclip", "links.json")

	s, err := store.New(dataPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hotclip: failed to open store: %v\n", err)
		os.Exit(1)
	}

	m := ui.New(s)
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "hotclip: %v\n", err)
		os.Exit(1)
	}
}
