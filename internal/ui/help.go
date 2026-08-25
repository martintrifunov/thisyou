package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type helpEntry struct {
	keys   string
	action string
}

func (m *Model) openHelp() {
	m.helpReturn = m.screen
	m.screen = helpScreen
	m.input.Blur()
	m.filter.Blur()
	m.compareInput.Blur()
	m.resizeHelp()
	m.help.GotoTop()
}

func (m *Model) updateHelp(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc", "backspace", "q", "?":
			m.screen = m.helpReturn
			switch m.screen {
			case searchScreen:
				m.input.Focus()
				return m, textinput.Blink
			case compareInputScreen:
				m.compareInput.Focus()
				return m, textinput.Blink
			case resultsScreen:
				if m.filtering {
					m.filter.Focus()
					return m, textinput.Blink
				}
			}
			return m, nil
		case "g", "home":
			m.help.GotoTop()
			return m, nil
		case "G", "end":
			m.help.GotoBottom()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.help, cmd = m.help.Update(msg)
	return m, cmd
}

func (m *Model) resizeHelp() {
	width := max(1, m.width-4)
	height := max(1, m.height-4)
	offset := m.help.YOffset
	m.help.Width = width
	m.help.Height = height
	m.help.SetContent(renderHelp(width))
	m.help.SetYOffset(offset)
}

func renderHelp(width int) string {
	width = max(1, width)
	sections := []struct {
		title   string
		entries []helpEntry
	}{
		{"SEARCH", []helpEntry{
			{"enter", "Search the entered Reddit username"},
			{"ctrl+o", "Open that user's latest cached history offline"},
			{"ctrl+b", "Open saved bookmarks"},
			{"esc", "Clear the username or cancel an active crawl"},
		}},
		{"BROWSE RESULTS", []helpEntry{
			{"j / k, arrows", "Move through records"},
			{"page up/down", "Move by one visible page"},
			{"ctrl+d / ctrl+u", "Move down or up by one visible page"},
			{"g / G", "Jump to the first or last record"},
			{"tab / shift+tab", "Switch All, Posts, and Comments"},
			{"enter", "Inspect the complete selected record"},
		}},
		{"RESULT ACTIONS", []helpEntry{
			{"b", "Save or remove the selected bookmark"},
			{"B", "Open all saved bookmarks"},
			{"o", "Open the selected record on Reddit"},
			{"y", "Copy a URL, text, author, subreddit, or linked URL"},
			{"s", "Open the current profile summary"},
			{"S", "Cycle newest, oldest, score, and subreddit sorting"},
			{"/", "Filter the current result view"},
			{"R", "Fetch and merge records newer than the current history"},
			{"d", "Open archive stream and coverage diagnostics"},
			{"C", "Compare with another Reddit profile"},
			{"r", "Return to username search"},
		}},
		{"FILTER TOKENS", []helpEntry{
			{"plain text", "Require every text term to match"},
			{"sub:golang", "Match one subreddit"},
			{"type:post", "Match posts or comments"},
			{"after:YYYY-MM-DD", "Match that date and later"},
			{"before:YYYY-MM-DD", "Match that date and earlier"},
			{"min:10 / max:100", "Match the archived score range"},
			{"enter / esc", "Apply the filter or clear it"},
		}},
		{"SCROLLABLE VIEWS", []helpEntry{
			{"j / k, arrows", "Scroll records, summaries, diagnostics, comparisons, or help"},
			{"g / G", "Jump to the top or bottom"},
			{"esc / backspace", "Return to the previous screen"},
		}},
		{"GLOBAL", []helpEntry{
			{"?", "Open or close this command reference"},
			{"ctrl+c", "Quit immediately from any screen"},
			{"q", "Quit from results or close the current secondary view"},
		}},
	}
	var b strings.Builder
	for sectionIndex, section := range sections {
		if sectionIndex > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(logoStyle.Render(truncate(section.title, width)))
		for _, entry := range section.entries {
			b.WriteByte('\n')
			b.WriteString(renderHelpEntry(entry, width))
		}
	}
	return b.String()
}

func renderHelpEntry(entry helpEntry, width int) string {
	if width < 42 {
		return titleStyle.Render(truncate(entry.keys, width)) + "\n" + mutedStyle.Render(wrapText(entry.action, width))
	}
	keyWidth := min(22, max(16, width/4))
	keys := lipgloss.PlaceHorizontal(keyWidth, lipgloss.Left, truncate(entry.keys, keyWidth))
	actionWidth := max(1, width-keyWidth-2)
	actionLines := strings.Split(wrapText(entry.action, actionWidth), "\n")
	var b strings.Builder
	for index, line := range actionLines {
		if index > 0 {
			b.WriteByte('\n')
		}
		keysForLine := strings.Repeat(" ", keyWidth)
		if index == 0 {
			keysForLine = keys
		}
		b.WriteString(fmt.Sprintf("%s  %s", keysForLine, mutedStyle.Render(line)))
	}
	return b.String()
}

func (m *Model) viewHelp() string {
	width := max(1, m.width-4)
	header := logoStyle.Render(truncate("THIS YOU?  /  COMMANDS", width))
	footer := mutedStyle.Render(truncate("j/k scroll  •  g/G jump  •  ?/esc return", width))
	content := lipgloss.JoinVertical(lipgloss.Left, header, m.help.View(), footer)
	framed := lipgloss.NewStyle().Padding(1, 2).Render(content)
	return fillScreen(m.width, m.height, framed, lipgloss.Left, lipgloss.Top)
}
