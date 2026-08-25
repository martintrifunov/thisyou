package main

import (
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/maco/thisyou/internal/reddit"
	"github.com/maco/thisyou/internal/ui"
)

var version = "0.2.0"

func main() {
	if writeVersion(os.Args[1:], os.Stdout) {
		return
	}
	program := tea.NewProgram(
		ui.New(reddit.NewClient()),
		tea.WithAltScreen(),
	)
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "thisyou:", err)
		os.Exit(1)
	}
}

func writeVersion(args []string, output io.Writer) bool {
	if len(args) != 1 || (args[0] != "--version" && args[0] != "-v") {
		return false
	}
	fmt.Fprintf(output, "thisyou %s\n", version)
	return true
}
