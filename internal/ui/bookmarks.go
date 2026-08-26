package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/martintrifunov/thisyou/internal/bookmarks"
	"github.com/martintrifunov/thisyou/internal/reddit"
)

type bookmarkStore interface {
	Load() ([]bookmarks.Entry, error)
	Toggle(reddit.Item) (bool, []bookmarks.Entry, error)
}

type bookmarkMsg struct {
	id      uint64
	entries []bookmarks.Entry
	notice  string
	err     error
	silent  bool
}

func (m *Model) loadBookmarks(silent bool) tea.Cmd {
	m.bookmarkRequest++
	id := m.bookmarkRequest
	store := m.store
	return func() tea.Msg {
		entries, err := store.Load()
		return bookmarkMsg{id: id, entries: entries, err: err, silent: silent}
	}
}

func (m *Model) toggleBookmark(item reddit.Item) tea.Cmd {
	if item.ID == "" {
		m.setActionError("record cannot be bookmarked")
		return nil
	}
	if m.isBookmarked(item) {
		m.notice = "removing bookmark..."
	} else {
		m.notice = "saving bookmark..."
	}
	m.noticeError = false
	m.bookmarkRequest++
	id := m.bookmarkRequest
	store := m.store
	return func() tea.Msg {
		saved, entries, err := store.Toggle(item)
		notice := "bookmark removed"
		if saved {
			notice = "bookmark saved"
		}
		return bookmarkMsg{id: id, entries: entries, notice: notice, err: err}
	}
}

func (m *Model) replaceBookmarks(entries []bookmarks.Entry) {
	m.saved = entries
	m.bookmarkIDs = make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		m.bookmarkIDs[bookmarkKey(entry.Item)] = struct{}{}
	}
	m.ensureBookmarkVisible()
}

func (m *Model) isBookmarked(item reddit.Item) bool {
	_, ok := m.bookmarkIDs[bookmarkKey(item)]
	return ok
}

func bookmarkKey(item reddit.Item) string {
	return string(item.Kind) + ":" + item.ID
}

func (m *Model) openBookmarks() tea.Cmd {
	m.bookmarkReturn = m.screen
	m.screen = bookmarksScreen
	m.bookmarksLoading = true
	m.copying = false
	m.notice = ""
	m.noticeError = false
	m.bookmarkSelected, m.bookmarkOffset = 0, 0
	return m.loadBookmarks(false)
}

func (m *Model) updateBookmarks(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.copying {
		if len(m.saved) == 0 {
			m.copying = false
			m.setActionError("no bookmark selected")
			return m, nil
		}
		if key, ok := msg.(tea.KeyMsg); ok {
			return m, m.updateCopyMenu(key.String(), m.saved[m.bookmarkSelected].Item)
		}
		return m, nil
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if key.String() != "b" && key.String() != "o" && key.String() != "y" {
		m.notice = ""
		m.noticeError = false
	}
	switch key.String() {
	case "esc", "backspace", "q", "B":
		m.screen = m.bookmarkReturn
		if m.screen == searchScreen {
			m.input.Focus()
			return m, textinput.Blink
		}
	case "j", "down":
		if m.bookmarkSelected < len(m.saved)-1 {
			m.bookmarkSelected++
			m.ensureBookmarkVisible()
		}
	case "k", "up":
		if m.bookmarkSelected > 0 {
			m.bookmarkSelected--
			m.ensureBookmarkVisible()
		}
	case "pgdown", "ctrl+d":
		if len(m.saved) > 0 {
			m.bookmarkSelected = min(len(m.saved)-1, m.bookmarkSelected+m.bookmarkPageRows())
			m.ensureBookmarkVisible()
		}
	case "pgup", "ctrl+u":
		m.bookmarkSelected = max(0, m.bookmarkSelected-m.bookmarkPageRows())
		m.ensureBookmarkVisible()
	case "g", "home":
		m.bookmarkSelected, m.bookmarkOffset = 0, 0
	case "G", "end":
		m.bookmarkSelected = max(0, len(m.saved)-1)
		m.ensureBookmarkVisible()
	case "enter":
		if len(m.saved) > 0 {
			m.openDetail(m.saved[m.bookmarkSelected].Item)
		}
	case "o":
		if len(m.saved) > 0 {
			return m, m.openItem(m.saved[m.bookmarkSelected].Item)
		}
		m.setActionError("no bookmark selected")
	case "y":
		if len(m.saved) > 0 {
			m.copying = true
			m.notice = ""
			m.noticeError = false
		} else {
			m.setActionError("no bookmark selected")
		}
	case "b":
		if len(m.saved) > 0 {
			return m, m.toggleBookmark(m.saved[m.bookmarkSelected].Item)
		}
		m.setActionError("no bookmark selected")
	}
	return m, nil
}

func (m *Model) viewBookmarks() string {
	var b strings.Builder
	contentWidth := max(4, m.width-4)
	b.WriteString(logoStyle.Render(truncate("THIS YOU?  /  BOOKMARKS", contentWidth)))
	b.WriteByte('\n')
	if m.bookmarksLoading && len(m.saved) == 0 {
		b.WriteString("\n" + mutedStyle.Render("Loading bookmarks...") + "\n")
	} else if len(m.saved) == 0 {
		b.WriteString("\n" + mutedStyle.Render(wrapText("No bookmarks saved. Search a user and press b on a record.", contentWidth)) + "\n")
	} else {
		rows := m.bookmarkPageRows()
		end := min(len(m.saved), m.bookmarkOffset+rows)
		for i := m.bookmarkOffset; i < end; i++ {
			b.WriteString(m.renderBookmarkRow(m.saved[i].Item, i == m.bookmarkSelected))
			b.WriteByte('\n')
		}
	}
	position := "0/0"
	if len(m.saved) > 0 {
		position = fmt.Sprintf("%d/%d", m.bookmarkSelected+1, len(m.saved))
	}
	footerText := fmt.Sprintf("%s  •  j/k move  •  enter inspect  •  b remove  •  ? help  •  esc return", position)
	footer := truncate(m.actionFooter(footerText), max(10, m.width-4))
	body := strings.TrimRight(b.String(), "\n")
	innerHeight := max(1, m.height-2)
	spacer := max(0, innerHeight-lipgloss.Height(body)-lipgloss.Height(footer))
	content := body + strings.Repeat("\n", spacer) + "\n" + footer
	framed := lipgloss.NewStyle().Padding(1, 2).Render(content)
	return fillScreen(m.width, m.height, framed, lipgloss.Left, lipgloss.Top)
}

func (m *Model) ensureBookmarkVisible() {
	if len(m.saved) == 0 {
		m.bookmarkSelected, m.bookmarkOffset = 0, 0
		return
	}
	m.bookmarkSelected = min(len(m.saved)-1, max(0, m.bookmarkSelected))
	rows := m.bookmarkPageRows()
	if m.bookmarkSelected < m.bookmarkOffset {
		m.bookmarkOffset = m.bookmarkSelected
	}
	if m.bookmarkSelected >= m.bookmarkOffset+rows {
		m.bookmarkOffset = m.bookmarkSelected - rows + 1
	}
	maximum := max(0, len(m.saved)-rows)
	m.bookmarkOffset = min(maximum, max(0, m.bookmarkOffset))
}

func (m *Model) bookmarkPageRows() int {
	return max(1, (m.height-4)/2)
}
