package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/martintrifunov/thisyou/internal/profiles"
)

type profileStore interface {
	Load() ([]profiles.Entry, error)
	Toggle(string) (bool, []profiles.Entry, error)
}

type profileMsg struct {
	id      uint64
	entries []profiles.Entry
	notice  string
	err     error
	silent  bool
}

func (m *Model) loadProfiles(silent bool) tea.Cmd {
	m.profileRequest++
	id := m.profileRequest
	store := m.profiles
	return func() tea.Msg {
		entries, err := store.Load()
		return profileMsg{id: id, entries: entries, err: err, silent: silent}
	}
}

func (m *Model) toggleProfile(username string) tea.Cmd {
	m.notice = "saving profile..."
	if m.isProfileSaved(username) {
		m.notice = "removing saved profile..."
	}
	m.noticeError = false
	m.profileRequest++
	id := m.profileRequest
	store := m.profiles
	return func() tea.Msg {
		saved, entries, err := store.Toggle(username)
		notice := "saved profile removed"
		if saved {
			notice = "profile saved"
		}
		return profileMsg{id: id, entries: entries, notice: notice, err: err}
	}
}

func (m *Model) replaceProfiles(entries []profiles.Entry) {
	m.savedProfiles = entries
	m.savedProfileIDs = make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		m.savedProfileIDs[strings.ToLower(entry.Username)] = struct{}{}
	}
	m.ensureProfileVisible()
}

func (m *Model) isProfileSaved(username string) bool {
	_, ok := m.savedProfileIDs[strings.ToLower(username)]
	return ok
}

func (m *Model) openProfiles() tea.Cmd {
	m.profileReturn = m.screen
	m.screen = profilesScreen
	m.profilesLoading = true
	m.notice = ""
	m.noticeError = false
	m.profileSelected, m.profileOffset = 0, 0
	return m.loadProfiles(false)
}

func (m *Model) updateProfiles(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "esc", "backspace", "q":
		m.screen = m.profileReturn
		if m.screen == searchScreen {
			m.input.Focus()
			return m, textinput.Blink
		}
	case "j", "down":
		m.profileSelected = min(len(m.savedProfiles)-1, m.profileSelected+1)
		m.ensureProfileVisible()
	case "k", "up":
		m.profileSelected = max(0, m.profileSelected-1)
		m.ensureProfileVisible()
	case "g", "home":
		m.profileSelected, m.profileOffset = 0, 0
	case "G", "end":
		m.profileSelected = max(0, len(m.savedProfiles)-1)
		m.ensureProfileVisible()
	case "enter", "o":
		if len(m.savedProfiles) > 0 {
			m.username = m.savedProfiles[m.profileSelected].Username
			m.input.SetValue(m.username)
			return m, m.beginCachedSearch()
		}
	case "R":
		if len(m.savedProfiles) > 0 {
			m.username = m.savedProfiles[m.profileSelected].Username
			m.input.SetValue(m.username)
			return m, m.beginSearch()
		}
	case "x", "p":
		if len(m.savedProfiles) > 0 {
			return m, m.toggleProfile(m.savedProfiles[m.profileSelected].Username)
		}
	}
	return m, nil
}

func (m *Model) ensureProfileVisible() {
	if len(m.savedProfiles) == 0 {
		m.profileSelected, m.profileOffset = 0, 0
		return
	}
	m.profileSelected = min(len(m.savedProfiles)-1, max(0, m.profileSelected))
	rows := max(1, m.height-6)
	if m.profileSelected < m.profileOffset {
		m.profileOffset = m.profileSelected
	}
	if m.profileSelected >= m.profileOffset+rows {
		m.profileOffset = m.profileSelected - rows + 1
	}
	m.profileOffset = min(max(0, len(m.savedProfiles)-rows), max(0, m.profileOffset))
}

func (m *Model) viewProfiles() string {
	contentWidth := max(4, m.width-4)
	var b strings.Builder
	b.WriteString(logoStyle.Render(truncate("THIS YOU?  /  SAVED PROFILES", contentWidth)))
	b.WriteByte('\n')
	if m.profilesLoading {
		b.WriteString("\n" + mutedStyle.Render("Loading saved profiles..."))
	} else if len(m.savedProfiles) == 0 {
		b.WriteString("\n" + mutedStyle.Render(wrapText("No saved profiles. Press p while browsing results to save one.", contentWidth)))
	} else {
		rows := max(1, m.height-6)
		end := min(len(m.savedProfiles), m.profileOffset+rows)
		for index := m.profileOffset; index < end; index++ {
			entry := m.savedProfiles[index]
			line := fmt.Sprintf("u/%s  •  saved %s", entry.Username, entry.SavedAt.Local().Format("2006-01-02 15:04"))
			style := lipgloss.NewStyle()
			marker := "  "
			if index == m.profileSelected {
				style = selectedStyle
				marker = logoStyle.Render("› ")
			}
			b.WriteString(marker + style.Render(truncate(line, max(1, contentWidth-2))) + "\n")
		}
	}
	body := strings.TrimRight(b.String(), "\n")
	footer := m.actionFooter("enter cached  •  R refresh  •  x remove  •  esc return  •  ? help")
	innerHeight := max(1, m.height-2)
	spacer := max(0, innerHeight-lipgloss.Height(body)-1)
	framed := lipgloss.NewStyle().Padding(1, 2).Render(body + strings.Repeat("\n", spacer) + "\n" + truncate(footer, contentWidth))
	return fillScreen(m.width, m.height, framed, lipgloss.Left, lipgloss.Top)
}
