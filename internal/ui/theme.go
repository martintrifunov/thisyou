package ui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/martintrifunov/thisyou/internal/config"
)

func ApplyTheme(theme config.Theme) {
	orange = lipgloss.Color(theme.Accent)
	muted = lipgloss.Color(theme.Muted)
	foreground = lipgloss.Color(theme.Foreground)
	border = lipgloss.Color(theme.Border)
	logoStyle = lipgloss.NewStyle().Foreground(orange).Bold(true)
	titleStyle = lipgloss.NewStyle().Foreground(foreground).Bold(true)
	mutedStyle = lipgloss.NewStyle().Foreground(muted)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Background(lipgloss.Color("#272729"))
	markerStyle = lipgloss.NewStyle().Background(orange)
}
