package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martintrifunov/thisyou/internal/config"
	"github.com/martintrifunov/thisyou/internal/reddit"
	"github.com/martintrifunov/thisyou/internal/ui"
)

var version = "0.3.1"

func main() {
	if writeVersion(os.Args[1:], os.Stdout) {
		return
	}
	settings, path, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "thisyou: load %s: %v\n", path, err)
		os.Exit(1)
	}
	client := reddit.NewClient()
	client.HTTP = &http.Client{Timeout: time.Duration(settings.RequestTimeoutSeconds) * time.Second}
	ui.ApplyTheme(settings.Theme)
	program := tea.NewProgram(
		ui.NewWithConfig(client, settings),
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
