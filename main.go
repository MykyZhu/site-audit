package main

import (
	"fmt"
	"os"
	"site-audit/tui"

	tea "github.com/charmbracelet/bubbletea"
)

// Version is set at build time by GoReleaser.
var Version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Println("site-audit", Version)
			return
		default:
			// Treat as a log file path to open directly
			m, err := tui.NewWithFile(os.Args[1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
				os.Exit(1)
			}
			run(m)
			return
		}
	}
	run(tui.New())
}

func run(m tui.Model) {
	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
