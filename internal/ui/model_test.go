package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/maco/thisyou/internal/bookmarks"
	"github.com/maco/thisyou/internal/historycache"
	"github.com/maco/thisyou/internal/reddit"
)

type fakeHistoryStore struct {
	snapshot historycache.Snapshot
	found    bool
	err      error
	saved    historycache.Snapshot
}

func (s *fakeHistoryStore) Load(string) (historycache.Snapshot, bool, error) {
	return s.snapshot, s.found, s.err
}

func (s *fakeHistoryStore) Save(snapshot historycache.Snapshot) error {
	s.saved = snapshot
	return s.err
}

func TestWrapTextDoesNotExceedWidth(t *testing.T) {
	t.Parallel()
	got := wrapText("a short 🚀 sentence followed-by-a-very-long-unbroken-value", 12)
	for _, line := range strings.Split(got, "\n") {
		if ansi.StringWidth(line) > 12 {
			t.Fatalf("line %q exceeds width: %q", line, got)
		}
	}
}

func TestWrapTextPreservesParagraphs(t *testing.T) {
	t.Parallel()
	got := wrapText("first paragraph\n\nsecond paragraph", 80)
	if got != "first paragraph\n\nsecond paragraph" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderLinkCreatesWrappedHyperlink(t *testing.T) {
	t.Parallel()
	target := "https://example.com/a/long/path?with=query"
	got := renderLink(target, 16)
	if !strings.Contains(got, ansi.SetHyperlink(target)) {
		t.Fatalf("rendered link does not contain hyperlink target: %q", got)
	}
	if !strings.Contains(got, ansi.ResetHyperlink()) {
		t.Fatalf("rendered link does not reset hyperlink: %q", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if ansi.StringWidth(line) > 16 {
			t.Fatalf("line %q exceeds link width", line)
		}
	}
}

func TestRenderLinkRejectsUnsafeTargets(t *testing.T) {
	t.Parallel()
	for _, target := range []string{
		"javascript:alert(1)",
		"file:///etc/passwd",
		"https://example.com/\x1b]8;;https://attacker.example",
		"/relative/path",
	} {
		t.Run(target, func(t *testing.T) {
			got := renderLink(target, 80)
			if strings.Contains(got, "\x1b]8;") {
				t.Fatalf("unsafe target %q was rendered as a hyperlink", target)
			}
		})
	}
}

func TestDetailRendersClickableURLs(t *testing.T) {
	t.Parallel()
	item := reddit.Item{
		Kind:      reddit.Post,
		Permalink: "/r/golang/comments/example/post",
		URL:       "https://go.dev/doc/",
	}
	got := renderDetail(item, 80)
	for _, target := range []string{item.RedditURL(), item.URL} {
		if !strings.Contains(got, ansi.SetHyperlink(target)) {
			t.Fatalf("detail does not contain hyperlink for %q", target)
		}
	}
}

func TestSearchViewFitsAndFillsSmallWindow(t *testing.T) {
	m := New(nil)
	_, _ = m.Update(tea.WindowSizeMsg{Width: 36, Height: 16})
	assertViewSize(t, m.View(), 36, 16)
	m.searchErr = "use 1-30 letters, numbers, underscores, or hyphens"
	assertViewSize(t, m.View(), 36, 16)
	m.searchErr = ""
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 28})
	assertViewSize(t, m.View(), 100, 28)
}

func TestUsernameInputKeepsWidthWhenPlaceholderDisappears(t *testing.T) {
	m := New(nil)
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	emptyWidth := lipgloss.Width(m.renderUsernameInput())
	m.input.SetValue("spez")
	typedWidth := lipgloss.Width(m.renderUsernameInput())
	if typedWidth != emptyWidth {
		t.Fatalf("typed input width = %d, placeholder width = %d", typedWidth, emptyWidth)
	}
	assertViewSize(t, m.View(), 80, 24)
}

func TestFilterInputKeepsWidthWhenPlaceholderDisappears(t *testing.T) {
	m := New(nil)
	_, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	emptyWidth := lipgloss.Width(m.renderFilterInput())
	m.filter.SetValue("golang")
	typedWidth := lipgloss.Width(m.renderFilterInput())
	if typedWidth != emptyWidth {
		t.Fatalf("typed filter width = %d, placeholder width = %d", typedWidth, emptyWidth)
	}
}

func TestLoadingViewFitsAndFillsSmallWindow(t *testing.T) {
	m := New(nil)
	m.screen = loadingScreen
	m.username = "example_user"
	_, _ = m.Update(tea.WindowSizeMsg{Width: 40, Height: 24})
	for _, kind := range []reddit.Kind{reddit.Post, reddit.Comment} {
		for _, source := range []string{"Arctic Shift", "PullPush"} {
			key := string(kind) + ":" + source
			m.status[key] = reddit.Progress{Kind: kind, Source: source, Page: 2, Total: 125}
		}
	}
	assertViewSize(t, m.View(), 40, 24)
	_, _ = m.Update(tea.WindowSizeMsg{Width: 32, Height: 16})
	assertViewSize(t, m.View(), 32, 16)
}

func TestResultsViewFillsWindow(t *testing.T) {
	m := New(nil)
	m.screen = resultsScreen
	m.username = "example"
	m.posts = []reddit.Item{{Kind: reddit.Post, ID: "one", Title: "A result", Subreddit: "golang"}}
	m.refreshVisible()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	assertViewSize(t, m.View(), 60, 20)
}

func TestResultsViewUsesAvailableRows(t *testing.T) {
	m := New(nil)
	m.screen = resultsScreen
	m.posts = make([]reddit.Item, 20)
	for i := range m.posts {
		m.posts[i] = reddit.Item{Kind: reddit.Post, ID: string(rune('a' + i)), Title: "result"}
	}
	m.refreshVisible()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	if got, want := m.pageRows(), 7; got != want {
		t.Fatalf("pageRows() = %d, want %d", got, want)
	}
	assertViewSize(t, m.View(), 60, 20)
}

func TestLongSelectedRowsKeepFixedHeight(t *testing.T) {
	m := New(nil)
	m.screen = resultsScreen
	m.username = "example"
	longText := strings.Repeat("long Unicode 🚀 content that must remain on one preview line ", 20)
	for i := 0; i < 12; i++ {
		kind := reddit.Post
		item := reddit.Item{Kind: kind, ID: string(rune('a' + i)), Title: longText, Subreddit: "long_subreddit_name"}
		if i%2 == 1 {
			item.Kind = reddit.Comment
			item.Body = longText
		}
		m.posts = append(m.posts, item)
	}
	m.refreshVisible()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	for selected := 0; selected < m.pageRows(); selected++ {
		m.selected = selected
		m.ensureVisible()
		if got := lipgloss.Height(m.renderRow(m.visible[selected], true)); got != 2 {
			t.Fatalf("selected row %d height = %d, want 2", selected, got)
		}
		for row := 0; row < m.pageRows(); row++ {
			rendered := m.renderRow(m.visible[row], row == selected)
			if got := lipgloss.Height(rendered); got != 2 {
				t.Fatalf("row %d height = %d, want 2", row, got)
			}
			if got, want := lipgloss.Width(rendered), 56; got != want {
				t.Fatalf("row %d width = %d, want %d", row, got, want)
			}
		}
		assertViewSize(t, m.View(), 60, 20)
	}
}

func TestSelectedRowsKeepEmojiGraphemesIntact(t *testing.T) {
	m := New(nil)
	m.screen = resultsScreen
	m.username = "example"
	_, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	for _, emoji := range []string{"❤️", "❤", "⭐", "🏳️‍🌈", "👨‍👩‍👧‍👦"} {
		item := reddit.Item{Kind: reddit.Comment, ID: emoji, Body: "thanks " + emoji}
		rendered := m.renderRow(item, true)
		if !strings.Contains(rendered, emoji) {
			t.Fatalf("selected row split emoji %q: %q", emoji, rendered)
		}
		if got := lipgloss.Height(rendered); got != 2 {
			t.Fatalf("selected row height = %d, want 2", got)
		}
		if got, want := lipgloss.Width(rendered), 56; got != want {
			t.Fatalf("selected row width = %d, want %d", got, want)
		}
	}
}

func TestResultsViewFitsWithFilterAndError(t *testing.T) {
	m := New(nil)
	m.screen = resultsScreen
	m.username = "example"
	m.posts = []reddit.Item{{Kind: reddit.Post, ID: "one", Title: "matching result"}}
	m.errors = []error{assertionError("source unavailable")}
	m.filter.SetValue("matching")
	m.refreshVisible()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 32, Height: 16})
	assertViewSize(t, m.View(), 32, 16)
	_, _ = m.Update(tea.WindowSizeMsg{Width: 24, Height: 14})
	assertViewSize(t, m.View(), 24, 14)
}

func TestDetailViewFillsAndReflowsOnResize(t *testing.T) {
	m := New(nil)
	item := reddit.Item{
		Kind:       reddit.Post,
		Title:      "A long title that must adapt to the available terminal width",
		Body:       strings.Repeat("A body with Unicode 🚀 text that wraps correctly. ", 12),
		Author:     "example",
		Subreddit:  "golang",
		Permalink:  "/r/golang/comments/example",
		CreatedUTC: 1,
	}
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.openDetail(item)
	assertViewSize(t, m.View(), 80, 24)
	if got, want := m.detail.Height, 20; got != want {
		t.Fatalf("detail height = %d, want %d", got, want)
	}

	_, _ = m.Update(tea.WindowSizeMsg{Width: 36, Height: 16})
	assertViewSize(t, m.View(), 36, 16)
	if got, want := m.detail.Width, 32; got != want {
		t.Fatalf("detail width = %d, want %d", got, want)
	}
	if got, want := m.detail.Height, 12; got != want {
		t.Fatalf("detail height = %d, want %d", got, want)
	}
	for _, line := range strings.Split(renderDetail(item, m.detail.Width), "\n") {
		if got := ansi.StringWidth(line); got > m.detail.Width {
			t.Fatalf("detail line width = %d, viewport width = %d: %q", got, m.detail.Width, line)
		}
	}
}

func TestStaleSearchEventIsIgnored(t *testing.T) {
	m := New(nil)
	m.screen = searchScreen
	m.searchID = 2
	result := reddit.Result{Posts: []reddit.Item{{ID: "stale"}}}
	_, _ = m.Update(searchEvent{id: 1, result: &result})
	if m.screen != searchScreen || len(m.posts) != 0 {
		t.Fatalf("stale search event changed model state")
	}
}

func TestOpenCachedHistoryOffline(t *testing.T) {
	savedAt := time.Date(2026, time.August, 25, 12, 30, 0, 0, time.Local)
	store := &fakeHistoryStore{
		found: true,
		snapshot: historycache.Snapshot{
			Username: "alice",
			SavedAt:  savedAt,
			Posts:    []reddit.Item{{Kind: reddit.Post, ID: "cached", Title: "offline result"}},
		},
	}
	m := New(nil)
	m.history = store
	m.input.SetValue("alice")
	_, command := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	if command == nil || m.screen != loadingScreen || !m.loadingCache {
		t.Fatal("offline cache load did not start")
	}
	_, _ = m.Update(command())
	if m.screen != resultsScreen || !m.fromCache || len(m.posts) != 1 {
		t.Fatalf("cached result state = screen %d, cached %v, posts %d", m.screen, m.fromCache, len(m.posts))
	}
	m.width, m.height = 100, 20
	if got := ansi.Strip(m.View()); !strings.Contains(got, "CACHED 2026-08-25 12:30") {
		t.Fatalf("cached timestamp missing from view: %q", got)
	}
}

func TestOpenMissingCachedHistoryReturnsToSearch(t *testing.T) {
	m := New(nil)
	m.history = &fakeHistoryStore{}
	m.input.SetValue("alice")
	_, command := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	_, _ = m.Update(command())
	if m.screen != searchScreen || !strings.Contains(m.searchErr, "no cached history") {
		t.Fatalf("cache miss state = screen %d, error %q", m.screen, m.searchErr)
	}
}

func TestEmptyResultsNavigationKeepsValidSelection(t *testing.T) {
	m := New(nil)
	m.screen = resultsScreen
	_, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if m.selected != 0 || m.offset != 0 {
		t.Fatalf("empty results selection = %d, offset = %d", m.selected, m.offset)
	}
}

func TestOpenSelectedRecord(t *testing.T) {
	m := New(nil)
	m.screen = resultsScreen
	m.posts = []reddit.Item{{
		Kind: reddit.Post, ID: "one", Permalink: "/r/golang/comments/one/example",
	}}
	m.refreshVisible()
	var opened string
	m.openURL = func(value string) error {
		opened = value
		return nil
	}

	_, command := m.Update(runeKey("o"))
	if command == nil {
		t.Fatal("open action returned no command")
	}
	_, _ = m.Update(command())
	if got, want := opened, "https://www.reddit.com/r/golang/comments/one/example"; got != want {
		t.Fatalf("opened URL = %q, want %q", got, want)
	}
	if m.notice != "opened Reddit URL" || m.noticeError {
		t.Fatalf("open notice = %q, error = %v", m.notice, m.noticeError)
	}
}

func TestOpenRejectsUnsafePermalink(t *testing.T) {
	m := New(nil)
	m.screen = resultsScreen
	m.posts = []reddit.Item{{Kind: reddit.Post, ID: "one", Permalink: "https://example.com/phishing"}}
	m.refreshVisible()
	called := false
	m.openURL = func(string) error {
		called = true
		return nil
	}

	_, command := m.Update(runeKey("o"))
	if command != nil || called {
		t.Fatal("unsafe permalink was opened")
	}
	if !m.noticeError || m.notice != "no safe Reddit URL available" {
		t.Fatalf("open notice = %q, error = %v", m.notice, m.noticeError)
	}
}

func TestCopyMenuCopiesRecordFields(t *testing.T) {
	item := reddit.Item{
		Kind: reddit.Post, Title: "Example title", Body: "Example body", Author: "alice",
		Subreddit: "golang", Permalink: "/r/golang/comments/one/example", URL: "https://go.dev",
	}
	tests := map[string]string{
		"u": "https://www.reddit.com/r/golang/comments/one/example",
		"t": "Example title\n\nExample body",
		"a": "u/alice",
		"s": "r/golang",
		"l": "https://go.dev",
	}
	for choice, want := range tests {
		t.Run(choice, func(t *testing.T) {
			m := New(nil)
			m.screen = resultsScreen
			m.posts = []reddit.Item{item}
			m.refreshVisible()
			var copied string
			m.copyText = func(value string) error {
				copied = value
				return nil
			}

			_, _ = m.Update(runeKey("y"))
			if !m.copying {
				t.Fatal("copy menu did not open")
			}
			_, command := m.Update(runeKey(choice))
			if command == nil {
				t.Fatal("copy action returned no command")
			}
			_, _ = m.Update(command())
			if copied != want {
				t.Fatalf("copied value = %q, want %q", copied, want)
			}
			if m.noticeError || !strings.HasPrefix(m.notice, "copied ") {
				t.Fatalf("copy notice = %q, error = %v", m.notice, m.noticeError)
			}
		})
	}
}

func TestActionFailureIsDisplayed(t *testing.T) {
	m := New(nil)
	m.screen = detailScreen
	item := reddit.Item{Kind: reddit.Comment, Body: "text"}
	m.inspected = &item
	m.copyText = func(string) error { return errors.New("clipboard unavailable") }

	_, _ = m.Update(runeKey("y"))
	_, command := m.Update(runeKey("t"))
	if command == nil {
		t.Fatal("copy action returned no command")
	}
	_, _ = m.Update(command())
	if !m.noticeError || m.notice != "action failed: clipboard unavailable" {
		t.Fatalf("copy notice = %q, error = %v", m.notice, m.noticeError)
	}
}

func TestEveryScreenFitsCommonTerminalSizes(t *testing.T) {
	sizes := []tea.WindowSizeMsg{
		{Width: 20, Height: 12},
		{Width: 32, Height: 16},
		{Width: 60, Height: 20},
		{Width: 80, Height: 12},
		{Width: 100, Height: 30},
	}
	for _, size := range sizes {
		t.Run(fmt.Sprintf("%dx%d", size.Width, size.Height), func(t *testing.T) {
			search := New(nil)
			search.searchErr = "invalid username value"
			_, _ = search.Update(size)
			assertViewSize(t, search.View(), size.Width, size.Height)

			loading := New(nil)
			loading.screen = loadingScreen
			loading.username = "example_user"
			for _, kind := range []reddit.Kind{reddit.Post, reddit.Comment} {
				for _, source := range []string{"Arctic Shift", "PullPush"} {
					key := string(kind) + ":" + source
					loading.status[key] = reddit.Progress{Kind: kind, Source: source, Page: 2, Total: 125}
				}
			}
			_, _ = loading.Update(size)
			assertViewSize(t, loading.View(), size.Width, size.Height)

			results := New(nil)
			results.screen = resultsScreen
			results.username = "example"
			for i := 0; i < 20; i++ {
				results.posts = append(results.posts, reddit.Item{Kind: reddit.Post, ID: fmt.Sprint(i), Title: strings.Repeat("long result ", 20)})
			}
			results.refreshVisible()
			_, _ = results.Update(size)
			assertViewSize(t, results.View(), size.Width, size.Height)
			results.copying = true
			assertViewSize(t, results.View(), size.Width, size.Height)

			detail := New(nil)
			_, _ = detail.Update(size)
			detail.openDetail(reddit.Item{Kind: reddit.Comment, Body: strings.Repeat("long comment 🚀 ", 30), CreatedUTC: 1})
			assertViewSize(t, detail.View(), size.Width, size.Height)
			detail.copying = true
			assertViewSize(t, detail.View(), size.Width, size.Height)

			summary := New(nil)
			summary.username = "example"
			for i := 0; i < 12; i++ {
				summary.posts = append(summary.posts, reddit.Item{Kind: reddit.Post, Subreddit: fmt.Sprint("community", i), Score: i})
			}
			_, _ = summary.Update(size)
			summary.openSummary()
			assertViewSize(t, summary.View(), size.Width, size.Height)

			saved := New(nil)
			entries := make([]bookmarks.Entry, 12)
			for i := range entries {
				entries[i] = bookmarks.Entry{Item: reddit.Item{Kind: reddit.Comment, ID: fmt.Sprint(i), Body: strings.Repeat("saved comment ", 20)}}
			}
			saved.replaceBookmarks(entries)
			saved.screen = bookmarksScreen
			_, _ = saved.Update(size)
			assertViewSize(t, saved.View(), size.Width, size.Height)
		})
	}
}

type assertionError string

func (e assertionError) Error() string { return string(e) }

func assertViewSize(t *testing.T, view string, width, height int) {
	t.Helper()
	if got := lipgloss.Width(view); got != width {
		t.Fatalf("view width = %d, want %d", got, width)
	}
	if got := lipgloss.Height(view); got != height {
		t.Fatalf("view height = %d, want %d", got, height)
	}
}

func runeKey(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}
