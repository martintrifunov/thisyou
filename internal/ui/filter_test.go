package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/maco/thisyou/internal/reddit"
)

func TestResultFilterMatchesAdvancedFields(t *testing.T) {
	t.Parallel()
	filter, err := parseResultFilter("release notes sub:r/golang type:post after:2025-01-01 before:2025-01-31 min:10 max:20")
	if err != nil {
		t.Fatal(err)
	}
	item := reddit.Item{
		Kind:       reddit.Post,
		Title:      "Release notes",
		Body:       "Detailed release information",
		Subreddit:  "GoLang",
		Score:      15,
		CreatedUTC: time.Date(2025, time.January, 31, 12, 0, 0, 0, time.Local).Unix(),
	}
	if !filter.matches(item) {
		t.Fatal("matching record was rejected")
	}
	item.Score = 9
	if filter.matches(item) {
		t.Fatal("record below minimum score matched")
	}
	item.Score = 15
	item.CreatedUTC = time.Date(2025, time.February, 1, 0, 0, 0, 0, time.Local).Unix()
	if filter.matches(item) {
		t.Fatal("record after inclusive before date matched")
	}
}

func TestResultFilterRejectsInvalidQueries(t *testing.T) {
	t.Parallel()
	queries := []string{
		"after:today",
		"before:2025/01/01",
		"type:user",
		"min:nope",
		"min:10 max:2",
		"after:2025-02-01 before:2025-01-01",
		"unknown:value",
		"sub:",
	}
	for _, query := range queries {
		if _, err := parseResultFilter(query); err == nil {
			t.Fatalf("parseResultFilter(%q) returned no error", query)
		}
	}
}

func TestResultFilterUsesAllTextTerms(t *testing.T) {
	t.Parallel()
	filter, err := parseResultFilter("golang release")
	if err != nil {
		t.Fatal(err)
	}
	if !filter.matches(reddit.Item{Title: "GoLang", Body: "new release"}) {
		t.Fatal("record containing all terms was rejected")
	}
	if filter.matches(reddit.Item{Title: "GoLang only"}) {
		t.Fatal("record missing one term matched")
	}
}

func TestInvalidFilterStaysOpenAndFitsWindow(t *testing.T) {
	m := New(nil)
	m.screen = resultsScreen
	m.posts = []reddit.Item{{Kind: reddit.Post, ID: "one", Title: "example"}}
	m.refreshVisible()
	_, _ = m.Update(tea.WindowSizeMsg{Width: 32, Height: 16})
	_, _ = m.Update(runeKey("/"))
	m.filter.SetValue("after:today")
	m.refreshVisible()
	if m.filterErr == "" {
		t.Fatal("invalid filter did not set an error")
	}
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.filtering {
		t.Fatal("invalid filter closed on enter")
	}
	assertViewSize(t, m.View(), 32, 16)
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.filtering || m.filterErr != "" || m.filter.Value() != "" {
		t.Fatalf("filter state after escape = filtering %v, error %q, value %q", m.filtering, m.filterErr, m.filter.Value())
	}
}
