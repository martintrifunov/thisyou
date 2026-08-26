package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/maco/thisyou/internal/reddit"
)

func TestSummarizeProfile(t *testing.T) {
	t.Parallel()
	first := time.Date(2025, time.January, 2, 12, 0, 0, 0, time.Local)
	second := first.AddDate(0, 0, 1)
	posts := []reddit.Item{
		{Subreddit: "golang", Score: 10, CreatedUTC: first.Unix()},
		{Subreddit: "GoLang", Score: -2, CreatedUTC: second.Unix()},
	}
	comments := []reddit.Item{
		{Subreddit: "golang", Score: 3, CreatedUTC: second.Unix()},
		{Subreddit: "news", Score: 4, CreatedUTC: second.Unix()},
	}

	got := summarizeProfile(posts, comments)
	if got.Posts != 2 || got.Comments != 2 || got.totalRecords() != 4 {
		t.Fatalf("record totals = %d posts, %d comments, %d total", got.Posts, got.Comments, got.totalRecords())
	}
	if got.PostScore != 8 || got.CommentScore != 7 || got.TotalScore != 15 || got.AverageScore != 3.75 {
		t.Fatalf("score summary = posts %d, comments %d, total %d, average %v", got.PostScore, got.CommentScore, got.TotalScore, got.AverageScore)
	}
	if got.ActiveDays != 2 || got.BusiestDay != "2025-01-03" || got.BusiestCount != 3 {
		t.Fatalf("activity summary = %d days, busiest %q with %d", got.ActiveDays, got.BusiestDay, got.BusiestCount)
	}
	if got.BusiestHour != 12 || got.BusiestHourCount != 4 || got.UniqueSubreddits != 2 {
		t.Fatalf("activity patterns = hour %d (%d), communities %d", got.BusiestHour, got.BusiestHourCount, got.UniqueSubreddits)
	}
	if !got.Earliest.Equal(first) || !got.Latest.Equal(second) {
		t.Fatalf("date range = %v to %v", got.Earliest, got.Latest)
	}
	if len(got.TopSubreddits) != 2 {
		t.Fatalf("top subreddit count = %d", len(got.TopSubreddits))
	}
	top := got.TopSubreddits[0]
	if top.Name != "golang" || top.Posts != 2 || top.Comments != 1 || top.Score != 11 {
		t.Fatalf("top subreddit = %+v", top)
	}
	if top.Share != 75 || top.AverageScore != 11.0/3.0 {
		t.Fatalf("top subreddit analytics = %+v", top)
	}
}

func TestSummarizeProfileLimitsAndSortsSubreddits(t *testing.T) {
	t.Parallel()
	posts := make([]reddit.Item, 12)
	for i := range posts {
		posts[i] = reddit.Item{Subreddit: string(rune('a' + i)), Score: i}
	}
	posts = append(posts,
		reddit.Item{Subreddit: "z", Score: 1},
		reddit.Item{Subreddit: "z", Score: 1},
	)

	got := summarizeProfile(posts, nil)
	if len(got.TopSubreddits) != 10 {
		t.Fatalf("top subreddit count = %d, want 10", len(got.TopSubreddits))
	}
	if got.TopSubreddits[0].Name != "z" || got.TopSubreddits[0].totalRecords() != 2 {
		t.Fatalf("first subreddit = %+v", got.TopSubreddits[0])
	}
	if got.TopSubreddits[1].Name != "l" {
		t.Fatalf("score tie-breaker placed %q second, want l", got.TopSubreddits[1].Name)
	}
}

func TestRenderSummaryHandlesEmptyAndNarrowViews(t *testing.T) {
	t.Parallel()
	got := renderSummary("example", profileSummary{}, 2, 0, 18)
	plain := ansi.Strip(got)
	compact := strings.Join(strings.Fields(plain), " ")
	for _, want := range []string{"u/example", "Partial summary: 2 archive stream(s) unavailable", "0 records", "No activity dates available", "No subreddit data available"} {
		if !strings.Contains(compact, want) {
			t.Fatalf("summary does not contain %q: %q", want, plain)
		}
	}
	for _, line := range strings.Split(got, "\n") {
		if ansi.StringWidth(line) > 18 {
			t.Fatalf("summary line exceeds width: %q", line)
		}
	}
}

func TestSummaryViewOpensScrollsAndReturns(t *testing.T) {
	m := New(nil)
	m.screen = resultsScreen
	m.username = "example"
	for i := 0; i < 12; i++ {
		m.posts = append(m.posts, reddit.Item{
			Kind:       reddit.Post,
			ID:         string(rune('a' + i)),
			Subreddit:  "community" + string(rune('a'+i)),
			Score:      i,
			CreatedUTC: time.Date(2025, time.January, i+1, 12, 0, 0, 0, time.Local).Unix(),
		})
	}
	m.refreshVisible()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 36, Height: 12})
	_, _ = m.Update(runeKey("s"))
	if m.screen != summaryScreen {
		t.Fatalf("screen = %v, want summary", m.screen)
	}
	assertViewSize(t, m.View(), 36, 12)
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if m.summary.YOffset == 0 {
		t.Fatal("summary did not scroll")
	}
	_, _ = m.Update(tea.WindowSizeMsg{Width: 24, Height: 16})
	assertViewSize(t, m.View(), 24, 16)
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.screen != resultsScreen {
		t.Fatalf("screen = %v after escape, want results", m.screen)
	}
}
