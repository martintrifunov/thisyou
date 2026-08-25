package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/maco/thisyou/internal/reddit"
	"github.com/maco/thisyou/internal/ui"
)

func main() {
	program := tea.NewProgram(
		ui.New(reddit.NewClient()),
		tea.WithAltScreen(),
	)
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "thisyou:", err)
		os.Exit(1)
	}
}
