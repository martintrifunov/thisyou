package ui

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/martintrifunov/thisyou/internal/bookmarks"
	"github.com/martintrifunov/thisyou/internal/reddit"
)

func TestToggleSelectedBookmark(t *testing.T) {
	m := newBookmarkTestModel(t)
	item := reddit.Item{Kind: reddit.Post, ID: "one", Title: "saved post", Subreddit: "golang"}
	m.screen = resultsScreen
	m.posts = []reddit.Item{item}
	m.refreshVisible()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})

	_, command := m.Update(runeKey("b"))
	if command == nil {
		t.Fatal("bookmark action returned no command")
	}
	_, _ = m.Update(command())
	if !m.isBookmarked(item) || len(m.saved) != 1 || m.notice != "bookmark saved" || m.noticeError {
		t.Fatalf("saved state = %v, count = %d, notice = %q, error = %v", m.isBookmarked(item), len(m.saved), m.notice, m.noticeError)
	}
	if !strings.Contains(ansi.Strip(m.renderRow(item, true)), "★") {
		t.Fatal("bookmarked row does not contain a star")
	}

	_, command = m.Update(runeKey("b"))
	if command == nil {
		t.Fatal("remove bookmark action returned no command")
	}
	_, _ = m.Update(command())
	if m.isBookmarked(item) || len(m.saved) != 0 || m.notice != "bookmark removed" || m.noticeError {
		t.Fatalf("saved state = %v, count = %d, notice = %q, error = %v after removal", m.isBookmarked(item), len(m.saved), m.notice, m.noticeError)
	}
}

func TestBookmarksOpenFromSearchInspectAndReturn(t *testing.T) {
	m := newBookmarkTestModel(t)
	item := reddit.Item{Kind: reddit.Comment, ID: "comment", Author: "alice", Body: "saved comment"}
	if _, _, err := m.store.Toggle(item); err != nil {
		t.Fatal(err)
	}
	_, _ = m.Update(tea.WindowSizeMsg{Width: 50, Height: 16})

	_, command := m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	if m.screen != bookmarksScreen || command == nil {
		t.Fatalf("screen = %v, command nil = %v", m.screen, command == nil)
	}
	_, _ = m.Update(command())
	if len(m.saved) != 1 || m.saved[0].Item.ID != item.ID {
		t.Fatalf("loaded bookmarks = %+v", m.saved)
	}
	view := m.View()
	assertViewSize(t, view, 50, 16)
	if !strings.Contains(ansi.Strip(view), "u/alice") {
		t.Fatalf("bookmark view does not contain author: %q", ansi.Strip(view))
	}

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != detailScreen || m.inspected == nil || m.inspected.ID != item.ID {
		t.Fatalf("bookmark detail state = screen %v, item %+v", m.screen, m.inspected)
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != bookmarksScreen {
		t.Fatalf("screen after closing detail = %v", m.screen)
	}
	_, command = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != searchScreen || command == nil {
		t.Fatalf("screen after closing bookmarks = %v, command nil = %v", m.screen, command == nil)
	}
}

func TestBookmarksNavigateAndRemoveSelected(t *testing.T) {
	m := newBookmarkTestModel(t)
	for i := 0; i < 12; i++ {
		item := reddit.Item{Kind: reddit.Post, ID: string(rune('a' + i)), Title: "saved", Subreddit: "community"}
		if _, _, err := m.store.Toggle(item); err != nil {
			t.Fatal(err)
		}
	}
	m.screen = resultsScreen
	_, _ = m.Update(tea.WindowSizeMsg{Width: 36, Height: 12})
	_, command := m.Update(runeKey("B"))
	_, _ = m.Update(command())
	assertViewSize(t, m.View(), 36, 12)

	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if m.bookmarkSelected != len(m.saved)-1 || m.bookmarkOffset == 0 {
		t.Fatalf("selection = %d, offset = %d", m.bookmarkSelected, m.bookmarkOffset)
	}
	removedID := m.saved[m.bookmarkSelected].Item.ID
	_, command = m.Update(runeKey("b"))
	_, _ = m.Update(command())
	if len(m.saved) != 11 {
		t.Fatalf("bookmark count = %d after removal", len(m.saved))
	}
	for _, entry := range m.saved {
		if entry.Item.ID == removedID {
			t.Fatalf("removed bookmark %q is still present", removedID)
		}
	}
	assertViewSize(t, m.View(), 36, 12)
}

func TestStaleBookmarkLoadIsIgnored(t *testing.T) {
	m := newBookmarkTestModel(t)
	m.bookmarkRequest = 2
	entry := bookmarks.Entry{Item: reddit.Item{Kind: reddit.Post, ID: "stale"}, SavedAt: time.Now()}
	_, _ = m.Update(bookmarkMsg{id: 1, entries: []bookmarks.Entry{entry}})
	if len(m.saved) != 0 {
		t.Fatalf("stale bookmarks changed state: %+v", m.saved)
	}
}

func TestBookmarkFailureIsDisplayed(t *testing.T) {
	m := New(nil)
	m.store = failingBookmarkStore{}
	m.screen = resultsScreen
	m.posts = []reddit.Item{{Kind: reddit.Post, ID: "one"}}
	m.refreshVisible()
	_, command := m.Update(runeKey("b"))
	_, _ = m.Update(command())
	if !m.noticeError || m.notice != "action failed: disk unavailable" {
		t.Fatalf("notice = %q, error = %v", m.notice, m.noticeError)
	}
}

func newBookmarkTestModel(t *testing.T) *Model {
	t.Helper()
	m := New(nil)
	m.store = bookmarks.NewStoreAt(filepath.Join(t.TempDir(), "bookmarks.json"))
	return m
}

type failingBookmarkStore struct{}

func (failingBookmarkStore) Load() ([]bookmarks.Entry, error) {
	return nil, errors.New("disk unavailable")
}

func (failingBookmarkStore) Toggle(reddit.Item) (bool, []bookmarks.Entry, error) {
	return false, nil, errors.New("disk unavailable")
}
