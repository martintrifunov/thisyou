package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestHelpOpensAndReturnsToSearch(t *testing.T) {
	m := New(nil)
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	_, _ = m.Update(runeKey("?"))
	if m.screen != helpScreen || m.helpReturn != searchScreen || m.input.Focused() {
		t.Fatalf("help state = screen %d, return %d, input focused %v", m.screen, m.helpReturn, m.input.Focused())
	}
	_, command := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != searchScreen || !m.input.Focused() || command == nil {
		t.Fatalf("return state = screen %d, input focused %v, command %v", m.screen, m.input.Focused(), command)
	}
}

func TestHelpRestoresActiveFilter(t *testing.T) {
	m := New(nil)
	m.screen = resultsScreen
	m.filtering = true
	m.filter.Focus()
	_, _ = m.Update(runeKey("?"))
	_, command := m.Update(runeKey("?"))
	if m.screen != resultsScreen || !m.filter.Focused() || command == nil {
		t.Fatalf("filter return state = screen %d, focused %v, command %v", m.screen, m.filter.Focused(), command)
	}
}

func TestHelpIsUnavailableDuringCrawls(t *testing.T) {
	for _, current := range []screen{loadingScreen, compareLoadingScreen} {
		m := New(nil)
		m.screen = current
		_, _ = m.Update(runeKey("?"))
		if m.screen != current {
			t.Fatalf("screen %d changed to %d", current, m.screen)
		}
	}
}

func TestHelpViewFitsCommonTerminalSizes(t *testing.T) {
	for _, size := range []tea.WindowSizeMsg{
		{Width: 20, Height: 12},
		{Width: 32, Height: 16},
		{Width: 60, Height: 20},
		{Width: 100, Height: 30},
	} {
		m := New(nil)
		_, _ = m.Update(size)
		m.openHelp()
		assertViewSize(t, m.View(), size.Width, size.Height)
		for _, line := range strings.Split(renderHelp(max(1, size.Width-4)), "\n") {
			if ansi.StringWidth(line) > max(1, size.Width-4) {
				t.Fatalf("size %dx%d help line exceeds width: %q", size.Width, size.Height, line)
			}
		}
	}
}

func TestSearchFooterKeepsEssentialCommandsOnOneLine(t *testing.T) {
	m := New(nil)
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	plain := ansi.Strip(m.View())
	want := "enter search  •  ctrl+o offline  •  ? help  •  ctrl+c quit"
	if !strings.Contains(plain, want) {
		t.Fatalf("search footer missing compact help: %q", plain)
	}
}
