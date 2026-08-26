package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/martintrifunov/thisyou/internal/historycache"
	"github.com/martintrifunov/thisyou/internal/localdata"
)

type localHistoryStore interface {
	List() ([]historycache.Entry, error)
	Delete(string) error
}

type exportStore interface {
	List() ([]localdata.Export, error)
	Delete(string) error
}

type dataEntry struct {
	kind     string
	name     string
	savedAt  time.Time
	posts    int
	comments int
	bytes    int64
}

type dataMsg struct {
	id      uint64
	entries []dataEntry
	notice  string
	err     error
}

func (m *Model) openDataManager() tea.Cmd {
	m.dataReturn = m.screen
	m.screen = dataScreen
	m.dataSelected, m.dataOffset = 0, 0
	m.dataLoading = true
	m.dataConfirm = false
	m.notice = ""
	m.noticeError = false
	return m.loadLocalData("")
}

func (m *Model) loadLocalData(notice string) tea.Cmd {
	m.dataRequest++
	id := m.dataRequest
	history := m.historyData
	exports := m.exports
	return func() tea.Msg {
		cached, err := history.List()
		if err != nil {
			return dataMsg{id: id, err: fmt.Errorf("list cache: %w", err)}
		}
		exported, err := exports.List()
		if err != nil {
			return dataMsg{id: id, err: fmt.Errorf("list exports: %w", err)}
		}
		entries := make([]dataEntry, 0, len(cached)+len(exported))
		for _, entry := range cached {
			entries = append(entries, dataEntry{
				kind: "CACHE", name: entry.Username, savedAt: entry.SavedAt,
				posts: entry.Posts, comments: entry.Comments, bytes: entry.Bytes,
			})
		}
		for _, entry := range exported {
			entries = append(entries, dataEntry{
				kind: "EXPORT", name: entry.Name, savedAt: entry.Modified, bytes: entry.Bytes,
			})
		}
		return dataMsg{id: id, entries: entries, notice: notice}
	}
}

func (m *Model) deleteLocalData() tea.Cmd {
	if len(m.dataEntries) == 0 {
		return nil
	}
	entry := m.dataEntries[m.dataSelected]
	m.dataLoading = true
	m.dataConfirm = false
	m.dataRequest++
	id := m.dataRequest
	history := m.historyData
	exports := m.exports
	return func() tea.Msg {
		var err error
		if entry.kind == "CACHE" {
			err = history.Delete(entry.name)
		} else {
			err = exports.Delete(entry.name)
		}
		if err != nil {
			return dataMsg{id: id, err: fmt.Errorf("delete %s: %w", strings.ToLower(entry.kind), err)}
		}
		cached, err := history.List()
		if err != nil {
			return dataMsg{id: id, err: err}
		}
		exported, err := exports.List()
		if err != nil {
			return dataMsg{id: id, err: err}
		}
		entries := make([]dataEntry, 0, len(cached)+len(exported))
		for _, value := range cached {
			entries = append(entries, dataEntry{kind: "CACHE", name: value.Username, savedAt: value.SavedAt, posts: value.Posts, comments: value.Comments, bytes: value.Bytes})
		}
		for _, value := range exported {
			entries = append(entries, dataEntry{kind: "EXPORT", name: value.Name, savedAt: value.Modified, bytes: value.Bytes})
		}
		return dataMsg{id: id, entries: entries, notice: strings.ToLower(entry.kind) + " deleted"}
	}
}

func (m *Model) updateDataManager(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.dataConfirm {
		switch key.String() {
		case "y", "Y":
			return m, m.deleteLocalData()
		case "esc", "n", "N":
			m.dataConfirm = false
			m.notice = ""
			return m, nil
		}
		return m, nil
	}
	switch key.String() {
	case "esc", "backspace", "q":
		m.screen = m.dataReturn
		if m.screen == searchScreen {
			m.input.Focus()
			return m, textinput.Blink
		}
	case "j", "down":
		m.dataSelected = min(len(m.dataEntries)-1, m.dataSelected+1)
		m.ensureDataVisible()
	case "k", "up":
		m.dataSelected = max(0, m.dataSelected-1)
		m.ensureDataVisible()
	case "g", "home":
		m.dataSelected, m.dataOffset = 0, 0
	case "G", "end":
		m.dataSelected = max(0, len(m.dataEntries)-1)
		m.ensureDataVisible()
	case "enter":
		if len(m.dataEntries) > 0 && m.dataEntries[m.dataSelected].kind == "CACHE" {
			m.username = m.dataEntries[m.dataSelected].name
			m.input.SetValue(m.username)
			return m, m.beginCachedSearch()
		}
	case "b":
		return m, m.openBookmarks()
	case "p":
		return m, m.openProfiles()
	case "x":
		if len(m.dataEntries) > 0 {
			m.dataConfirm = true
			m.notice = "delete " + strings.ToLower(m.dataEntries[m.dataSelected].kind) + "? y confirm  •  n cancel"
			m.noticeError = true
		}
	}
	return m, nil
}

func (m *Model) ensureDataVisible() {
	if len(m.dataEntries) == 0 {
		m.dataSelected, m.dataOffset = 0, 0
		return
	}
	m.dataSelected = min(len(m.dataEntries)-1, max(0, m.dataSelected))
	rows := max(1, m.height-8)
	if m.dataSelected < m.dataOffset {
		m.dataOffset = m.dataSelected
	}
	if m.dataSelected >= m.dataOffset+rows {
		m.dataOffset = m.dataSelected - rows + 1
	}
	m.dataOffset = min(max(0, len(m.dataEntries)-rows), max(0, m.dataOffset))
}

func (m *Model) viewDataManager() string {
	contentWidth := max(4, m.width-4)
	var b strings.Builder
	b.WriteString(logoStyle.Render(truncate("THIS YOU?  /  LOCAL DATA", contentWidth)))
	b.WriteString("\n" + mutedStyle.Render(truncate(fmt.Sprintf("%d saved profiles  •  %d bookmarks  •  exports: %s", len(m.savedProfiles), len(m.saved), m.config.ExportDirectory), contentWidth)))
	if m.dataLoading {
		b.WriteString("\n\n" + mutedStyle.Render("Loading local data..."))
	} else if len(m.dataEntries) == 0 {
		b.WriteString("\n\n" + mutedStyle.Render("No cached histories or managed exports found."))
	} else {
		rows := max(1, m.height-8)
		end := min(len(m.dataEntries), m.dataOffset+rows)
		for index := m.dataOffset; index < end; index++ {
			entry := m.dataEntries[index]
			line := ""
			if entry.kind == "CACHE" {
				line = fmt.Sprintf("%-6s  u/%s  •  %d posts  •  %d comments  •  %s  •  %s", entry.kind, entry.name, entry.posts, entry.comments, formatBytes(entry.bytes), entry.savedAt.Local().Format("2006-01-02"))
			} else {
				line = fmt.Sprintf("%-6s  %s  •  %s  •  %s", entry.kind, entry.name, formatBytes(entry.bytes), entry.savedAt.Local().Format("2006-01-02"))
			}
			style := lipgloss.NewStyle()
			marker := "  "
			if index == m.dataSelected {
				style = selectedStyle
				marker = logoStyle.Render("› ")
			}
			b.WriteString("\n" + marker + style.Render(truncate(line, max(1, contentWidth-2))))
		}
	}
	body := b.String()
	footer := m.actionFooter("enter open cache  •  b bookmarks  •  p profiles  •  x delete  •  esc return")
	innerHeight := max(1, m.height-2)
	spacer := max(0, innerHeight-lipgloss.Height(body)-1)
	framed := lipgloss.NewStyle().Padding(1, 2).Render(body + strings.Repeat("\n", spacer) + "\n" + truncate(footer, contentWidth))
	return fillScreen(m.width, m.height, framed, lipgloss.Left, lipgloss.Top)
}

func formatBytes(value int64) string {
	if value < 1024 {
		return fmt.Sprintf("%d B", value)
	}
	if value < 1024*1024 {
		return fmt.Sprintf("%.1f KiB", float64(value)/1024)
	}
	return fmt.Sprintf("%.1f MiB", float64(value)/(1024*1024))
}
