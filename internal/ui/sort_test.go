package ui

import (
	"testing"

	"github.com/martintrifunov/thisyou/internal/reddit"
)

func TestSortItemsModes(t *testing.T) {
	t.Parallel()
	items := []reddit.Item{
		{Kind: reddit.Post, ID: "middle", Subreddit: "zebra", CreatedUTC: 20, Score: 5},
		{Kind: reddit.Post, ID: "newest", Subreddit: "alpha", CreatedUTC: 30, Score: 1},
		{Kind: reddit.Post, ID: "oldest", Subreddit: "Beta", CreatedUTC: 10, Score: 20},
	}
	tests := []struct {
		mode sortMode
		want []string
	}{
		{newestFirst, []string{"newest", "middle", "oldest"}},
		{oldestFirst, []string{"oldest", "middle", "newest"}},
		{highestScore, []string{"oldest", "middle", "newest"}},
		{subredditName, []string{"newest", "oldest", "middle"}},
	}
	for _, test := range tests {
		got := append([]reddit.Item(nil), items...)
		sortItems(got, test.mode)
		for i, want := range test.want {
			if got[i].ID != want {
				t.Fatalf("%s item %d = %q, want %q", test.mode.label(), i, got[i].ID, want)
			}
		}
	}
}

func TestSortModeCycles(t *testing.T) {
	t.Parallel()
	mode := newestFirst
	for _, want := range []sortMode{oldestFirst, highestScore, subredditName, newestFirst} {
		mode = mode.next()
		if mode != want {
			t.Fatalf("next mode = %v, want %v", mode, want)
		}
	}
}
