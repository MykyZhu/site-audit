package main

import (
	"fmt"
	"os"
	"site-audit/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	var model tui.Model

	// If a log file path is passed as an argument, open it directly in the viewer
	if len(os.Args) > 1 {
		path := os.Args[1]
		m, err := tui.NewWithFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
			os.Exit(1)
		}
		model = m
	} else {
		model = tui.New()
	}

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
