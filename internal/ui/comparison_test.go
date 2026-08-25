package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/maco/thisyou/internal/reddit"
)

func TestRenderComparisonWideAndNarrow(t *testing.T) {
	t.Parallel()
	left := summarizeProfile(
		[]reddit.Item{{Subreddit: "golang", Score: 10, CreatedUTC: time.Date(2025, 1, 1, 0, 0, 0, 0, time.Local).Unix()}},
		[]reddit.Item{{Subreddit: "golang", Score: 2, CreatedUTC: time.Date(2025, 1, 2, 0, 0, 0, 0, time.Local).Unix()}},
	)
	right := summarizeProfile([]reddit.Item{{Subreddit: "rust", Score: 5, CreatedUTC: time.Date(2024, 1, 1, 0, 0, 0, 0, time.Local).Unix()}}, nil)
	for _, width := range []int{80, 28} {
		got := renderComparison("alice", left, 0, "bob", right, 1, width)
		plain := ansi.Strip(got)
		for _, want := range []string{"u/alice", "u/bob", "Records", "Top subreddit", "r/golang", "r/rust", "Partial data"} {
			if !strings.Contains(strings.ToLower(plain), strings.ToLower(want)) {
				t.Fatalf("width %d comparison missing %q: %q", width, want, plain)
			}
		}
		for _, line := range strings.Split(got, "\n") {
			if ansi.StringWidth(line) > width {
				t.Fatalf("width %d line exceeds viewport: %q", width, line)
			}
		}
	}
}

func TestComparisonInputValidatesUsername(t *testing.T) {
	m := New(nil)
	m.screen = resultsScreen
	m.username = "alice"
	_, _ = m.Update(runeKey("C"))
	if m.screen != compareInputScreen {
		t.Fatalf("screen = %d, want comparison input", m.screen)
	}
	m.compareInput.SetValue("alice")
	_, command := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil || m.compareErr != "enter a different username" {
		t.Fatalf("same-user comparison error = %q, command = %v", m.compareErr, command)
	}
}

func TestComparisonResultOpensAndIgnoresStaleEvents(t *testing.T) {
	m := New(nil)
	m.screen = compareLoadingScreen
	m.username = "alice"
	m.compareUsername = "bob"
	m.compareID = 3
	stale := reddit.Result{Posts: []reddit.Item{{ID: "stale"}}}
	_, _ = m.Update(compareEvent{id: 2, result: &stale})
	if m.screen != compareLoadingScreen || len(m.compareResult.Posts) != 0 {
		t.Fatal("stale comparison event changed state")
	}
	result := reddit.Result{Comments: []reddit.Item{{Kind: reddit.Comment, ID: "fresh"}}}
	_, _ = m.Update(compareEvent{id: 3, result: &result})
	if m.screen != comparisonScreen || len(m.compareResult.Comments) != 1 {
		t.Fatalf("comparison state = screen %d, comments %d", m.screen, len(m.compareResult.Comments))
	}
}

func TestComparisonScreensFillAndReflow(t *testing.T) {
	for _, size := range []tea.WindowSizeMsg{{Width: 24, Height: 12}, {Width: 40, Height: 16}, {Width: 90, Height: 28}} {
		m := New(nil)
		m.username = "alice"
		m.compareUsername = "bob"
		m.compareResult.Posts = []reddit.Item{{Kind: reddit.Post, Subreddit: "golang", CreatedUTC: 1}}
		_, _ = m.Update(size)

		m.screen = compareInputScreen
		assertViewSize(t, m.View(), size.Width, size.Height)

		m.screen = compareLoadingScreen
		for _, kind := range []reddit.Kind{reddit.Post, reddit.Comment} {
			for _, source := range []string{"Arctic Shift", "PullPush"} {
				key := string(kind) + ":" + source
				m.compareStatus[key] = reddit.Progress{Kind: kind, Source: source, Page: 2, Total: 100, Done: true}
			}
		}
		assertViewSize(t, m.View(), size.Width, size.Height)

		m.screen = comparisonScreen
		m.resizeComparison()
		assertViewSize(t, m.View(), size.Width, size.Height)
	}
}
