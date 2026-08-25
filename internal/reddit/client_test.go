package reddit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestSearchAllMergesSourcesAndKinds(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var records []map[string]any
		switch r.URL.Path {
		case "/api/posts/search":
			records = []map[string]any{{"id": "shared", "author": "alice", "created_utc": 30, "title": "rich title", "subreddit": "golang"}}
		case "/reddit/search/submission/":
			records = []map[string]any{
				{"id": "shared", "author": "alice", "created_utc": 30, "selftext": "second source body"},
				{"id": "pull-only", "author": "alice", "created_utc": "20", "title": "older"},
			}
		case "/api/comments/search":
			records = []map[string]any{{"id": "comment-1", "author": "alice", "created_utc": 25.0, "body": "hello"}}
		case "/reddit/search/comment/":
			records = nil
		default:
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": records})
	}))
	defer server.Close()

	client := NewClient()
	client.ArcticShift = server.URL
	client.PullPush = server.URL
	client.RetryDelay = 0
	result := client.SearchAll(context.Background(), "alice", nil)

	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if len(result.Posts) != 2 || len(result.Comments) != 1 {
		t.Fatalf("got %d posts and %d comments", len(result.Posts), len(result.Comments))
	}
	if got := result.Posts[0]; got.ID != "shared" || got.Title != "rich title" || got.Body != "second source body" || len(got.Sources) != 2 {
		t.Fatalf("merged post = %#v", got)
	}
	if result.Posts[1].CreatedUTC != 20 {
		t.Fatalf("timestamp string not decoded: %#v", result.Posts[1])
	}
}

func TestCrawlUsesIndependentTimestampCursor(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	seenBefore := map[string][]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		kind := r.URL.Path
		before := r.URL.Query().Get("before")
		mu.Lock()
		seenBefore[kind] = append(seenBefore[kind], before)
		mu.Unlock()

		var records []map[string]any
		// Exactly 100 records forces another page. The sources end at different cursors.
		if before == "" {
			start := int64(1000)
			prefix := "a"
			if kind == "/reddit/search/submission/" {
				start, prefix = 500, "p"
			}
			for i := 0; i < pageLimit; i++ {
				records = append(records, map[string]any{"id": fmt.Sprintf("%s%d", prefix, i), "created_utc": start - int64(i)})
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": records})
	}))
	defer server.Close()

	client := NewClient()
	client.ArcticShift = server.URL
	client.PullPush = server.URL
	client.RetryDelay = 0
	result := client.SearchAll(context.Background(), "alice", nil)
	if len(result.Posts) != 200 {
		t.Fatalf("got %d posts, want 200", len(result.Posts))
	}

	mu.Lock()
	defer mu.Unlock()
	if got := seenBefore["/api/posts/search"]; len(got) != 2 || got[1] != strconv.FormatInt(901, 10) {
		t.Fatalf("Arctic cursors = %v", got)
	}
	if got := seenBefore["/reddit/search/submission/"]; len(got) != 2 || got[1] != strconv.FormatInt(401, 10) {
		t.Fatalf("PullPush cursors = %v", got)
	}
}

func TestSearchAllRetainsPartialResultsOnSourceError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/posts/search" {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "ok", "created_utc": 1}}})
			return
		}
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient()
	client.ArcticShift = server.URL
	client.PullPush = server.URL
	client.RetryDelay = time.Millisecond
	result := client.SearchAll(context.Background(), "alice", nil)
	if len(result.Posts) != 1 || result.Posts[0].ID != "ok" {
		t.Fatalf("partial posts = %#v", result.Posts)
	}
	if len(result.Errors) != 3 {
		t.Fatalf("got %d errors, want 3: %v", len(result.Errors), result.Errors)
	}
}

func TestCancellationInterruptsRetryDelay(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "retry later", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient()
	client.RetryDelay = time.Second
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(10*time.Millisecond, cancel)
	_, err := client.get(ctx, server.URL, true)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("get error = %v, want context.Canceled", err)
	}
}

func TestRateLimitErrorHidesProviderMessage(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "long misleading scraping message", http.StatusTooManyRequests)
	}))
	defer server.Close()
	client := NewClient()
	client.RetryDelay = 0
	_, err := client.get(context.Background(), server.URL, true)
	if err == nil || err.Error() != "HTTP 429: rate limited this connection" {
		t.Fatalf("rate-limit error = %q", err)
	}
}

func TestRateLimitDelayUsesProviderHeaders(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		key   string
		value string
		want  time.Duration
	}{
		{"retry seconds", "Retry-After", "7", 7 * time.Second},
		{"retry date", "Retry-After", now.Add(9 * time.Second).Format(http.TimeFormat), 9 * time.Second},
		{"reset seconds", "X-RateLimit-Reset", "11.5", 11500 * time.Millisecond},
		{"reset timestamp", "X-RateLimit-Reset-At", "1787659213", 13 * time.Second},
		{"fallback", "", "", 1200 * time.Millisecond},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			header := make(http.Header)
			if test.key != "" {
				header.Set(test.key, test.value)
			}
			if got := rateLimitDelay(header, now, 1200*time.Millisecond); got != test.want {
				t.Fatalf("delay = %v, want %v", got, test.want)
			}
		})
	}
}

func TestSearchAfterUsesIndependentKindBoundaries(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	seenAfter := map[string][]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seenAfter[r.URL.Path] = append(seenAfter[r.URL.Path], r.URL.Query().Get("after"))
		mu.Unlock()
		created := int64(101)
		if r.URL.Path == "/api/comments/search" || r.URL.Path == "/reddit/search/comment/" {
			created = 201
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": r.URL.Path, "created_utc": created}}})
	}))
	defer server.Close()

	client := NewClient()
	client.ArcticShift = server.URL
	client.PullPush = server.URL
	client.RetryDelay = 0
	result := client.SearchAfter(context.Background(), "alice", 100, 200, nil)
	if len(result.Posts) != 2 || len(result.Comments) != 2 {
		t.Fatalf("got %d posts and %d comments", len(result.Posts), len(result.Comments))
	}
	mu.Lock()
	defer mu.Unlock()
	for path, values := range seenAfter {
		want := "100"
		if path == "/api/comments/search" || path == "/reddit/search/comment/" {
			want = "200"
		}
		for _, got := range values {
			if got != want {
				t.Fatalf("%s after = %q, want %q", path, got, want)
			}
		}
	}
}

func TestSearchAfterExcludesBoundaryRecords(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
			{"id": "new", "created_utc": 11},
			{"id": "boundary", "created_utc": 10},
		}})
	}))
	defer server.Close()
	client := NewClient()
	client.ArcticShift = server.URL
	client.PullPush = server.URL
	client.RetryDelay = 0
	result := client.SearchAfter(context.Background(), "alice", 10, 10, nil)
	if len(result.Posts) != 1 || result.Posts[0].ID != "new" || len(result.Comments) != 1 || result.Comments[0].ID != "new" {
		t.Fatalf("boundary filtering returned posts %#v, comments %#v", result.Posts, result.Comments)
	}
}

func TestMergeResultsDeduplicatesFreshRecords(t *testing.T) {
	t.Parallel()
	base := Result{Posts: []Item{{Kind: Post, ID: "same", CreatedUTC: 10, Sources: []string{"PullPush"}}}}
	fresh := Result{
		Posts:  []Item{{Kind: Post, ID: "same", CreatedUTC: 10, Title: "richer", Sources: []string{"Arctic Shift"}}, {Kind: Post, ID: "new", CreatedUTC: 20}},
		Errors: []error{errors.New("partial")},
	}
	merged := MergeResults(base, fresh)
	if len(merged.Posts) != 2 || merged.Posts[0].ID != "new" || merged.Posts[1].Title != "richer" || len(merged.Errors) != 1 {
		t.Fatalf("merged result = %#v", merged)
	}
}
