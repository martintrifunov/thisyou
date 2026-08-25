package reddit

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRedditURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		item Item
		want string
	}{
		{
			name: "relative permalink",
			item: Item{Permalink: "/r/golang/comments/abc/example/"},
			want: "https://www.reddit.com/r/golang/comments/abc/example/",
		},
		{
			name: "absolute permalink",
			item: Item{Permalink: "https://old.reddit.com/r/golang/comments/abc"},
			want: "https://old.reddit.com/r/golang/comments/abc",
		},
		{
			name: "external URL is not a Reddit URL",
			item: Item{URL: "https://example.com/article"},
			want: "",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := test.item.RedditURL(); got != test.want {
				t.Fatalf("RedditURL() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestDecodeItemSanitizesTerminalControlCharacters(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{
		"id":"abc\u001b",
		"author":"user\u0007name",
		"subreddit":"go\nlang",
		"created_utc":1,
		"title":"\u001b[2Jtitle",
		"selftext":"first line\nsecond\tcolumn\u001b[2J"
	}`)
	item, err := decodeItem(raw, Post, "test")
	if err != nil {
		t.Fatalf("decodeItem: %v", err)
	}
	for name, value := range map[string]string{
		"id": item.ID, "author": item.Author, "subreddit": item.Subreddit,
		"title": item.Title, "body": item.Body,
	} {
		if strings.ContainsRune(value, '\x1b') || strings.ContainsRune(value, '\a') {
			t.Fatalf("%s still contains terminal control characters: %q", name, value)
		}
	}
	if item.Body != "first line\nsecond column[2J" {
		t.Fatalf("body = %q", item.Body)
	}
}
