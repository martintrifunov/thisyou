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
