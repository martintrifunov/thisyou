package reddit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultArcticShift = "https://arctic-shift.photon-reddit.com"
	defaultPullPush    = "https://api.pullpush.io"
	pageLimit          = 100
	maxResponseBytes   = 32 << 20
)

type source struct {
	name string
	base string
}

// Progress describes a completed archive page, or a source that has finished.
type Progress struct {
	Kind   Kind
	Source string
	Page   int
	Items  int
	Total  int
	Done   bool
	Err    error
}

// Result contains all successfully retrieved, merged records and any source errors.
type Result struct {
	Posts    []Item
	Comments []Item
	Errors   []error
}

// Client retrieves public Reddit archive records from Arctic Shift and PullPush.
type Client struct {
	HTTP        *http.Client
	ArcticShift string
	PullPush    string
	RetryDelay  time.Duration
}

func NewClient() *Client {
	return &Client{
		HTTP:        &http.Client{Timeout: 25 * time.Second},
		ArcticShift: defaultArcticShift,
		PullPush:    defaultPullPush,
		RetryDelay:  1200 * time.Millisecond,
	}
}

// SearchAll crawls posts and comments from both sources concurrently. Progress may be nil.
func (c *Client) SearchAll(ctx context.Context, username string, progress func(Progress)) Result {
	type crawlResult struct {
		kind   Kind
		source string
		items  []Item
		err    error
	}

	sources := []source{{"Arctic Shift", c.ArcticShift}, {"PullPush", c.PullPush}}
	kinds := []Kind{Post, Comment}
	results := make(chan crawlResult, len(sources)*len(kinds))
	var wg sync.WaitGroup
	for _, kind := range kinds {
		for _, src := range sources {
			wg.Add(1)
			go func(kind Kind, src source) {
				defer wg.Done()
				items, err := c.crawl(ctx, username, kind, src, progress)
				results <- crawlResult{kind: kind, source: src.name, items: items, err: err}
			}(kind, src)
		}
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	var out Result
	byKind := map[Kind][]Item{}
	for result := range results {
		byKind[result.kind] = append(byKind[result.kind], result.items...)
		if result.err != nil && !errors.Is(result.err, context.Canceled) {
			out.Errors = append(out.Errors, fmt.Errorf("%s %ss: %w", result.source, result.kind, result.err))
		}
	}
	out.Posts = mergeAndSort(byKind[Post])
	out.Comments = mergeAndSort(byKind[Comment])
	return out
}

func (c *Client) crawl(ctx context.Context, username string, kind Kind, src source, progress func(Progress)) ([]Item, error) {
	var all []Item
	var before int64
	page := 0
	for {
		page++
		items, err := c.fetchPage(ctx, username, kind, src, before)
		if err != nil {
			report(progress, Progress{Kind: kind, Source: src.name, Page: page, Total: len(all), Done: true, Err: err})
			return all, err
		}
		if len(items) == 0 {
			report(progress, Progress{Kind: kind, Source: src.name, Page: page, Total: len(all), Done: true})
			return all, nil
		}

		all = append(all, items...)
		report(progress, Progress{Kind: kind, Source: src.name, Page: page, Items: len(items), Total: len(all)})
		oldest := items[0].CreatedUTC
		for _, item := range items[1:] {
			if item.CreatedUTC < oldest {
				oldest = item.CreatedUTC
			}
		}
		// Both archives treat before as a timestamp boundary. Keeping the exact
		// oldest timestamp matches their cursor contract and avoids skipping a
		// record when several items were created in the same second.
		next := oldest
		// A source ignoring the cursor would otherwise make the crawl infinite.
		if oldest <= 0 || (before != 0 && next >= before) {
			report(progress, Progress{Kind: kind, Source: src.name, Page: page, Total: len(all), Done: true})
			return all, nil
		}
		before = next
	}
}

func (c *Client) fetchPage(ctx context.Context, username string, kind Kind, src source, before int64) ([]Item, error) {
	u, err := url.Parse(strings.TrimRight(src.base, "/"))
	if err != nil {
		return nil, err
	}
	if src.name == "Arctic Shift" {
		if kind == Post {
			u.Path = "/api/posts/search"
		} else {
			u.Path = "/api/comments/search"
		}
	} else if kind == Post {
		u.Path = "/reddit/search/submission/"
	} else {
		u.Path = "/reddit/search/comment/"
	}
	q := u.Query()
	q.Set("author", username)
	q.Set("limit", fmt.Sprint(pageLimit))
	q.Set("sort", "desc")
	if src.name == "PullPush" {
		q.Set("test", "")
	}
	if before > 0 {
		q.Set("before", fmt.Sprint(before))
	}
	u.RawQuery = q.Encode()

	body, err := c.get(ctx, u.String(), true)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Data  []json.RawMessage `json:"data"`
		Error any               `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if envelope.Error != nil && len(envelope.Data) == 0 {
		return nil, fmt.Errorf("archive error: %v", envelope.Error)
	}
	items := make([]Item, 0, len(envelope.Data))
	for _, raw := range envelope.Data {
		item, err := decodeItem(raw, kind, src.name)
		if err == nil {
			items = append(items, item)
		}
	}
	return items, nil
}

func (c *Client) get(ctx context.Context, endpoint string, retry bool) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "thisyou/1.0 (public archive TUI)")
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		if retry && ctx.Err() == nil {
			if err := wait(ctx, c.RetryDelay); err == nil {
				return c.get(ctx, endpoint, false)
			}
		}
		return nil, err
	}
	defer res.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes))
	if readErr != nil {
		return nil, readErr
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		if retry && (res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500) {
			if err := wait(ctx, c.RetryDelay); err == nil {
				return c.get(ctx, endpoint, false)
			}
		}
		message := strings.TrimSpace(string(body))
		if len(message) > 180 {
			message = message[:180] + "..."
		}
		return nil, fmt.Errorf("HTTP %d: %s", res.StatusCode, message)
	}
	return body, nil
}

func wait(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func report(fn func(Progress), update Progress) {
	if fn != nil {
		fn(update)
	}
}

func mergeAndSort(items []Item) []Item {
	merged := make(map[string]Item, len(items))
	for _, item := range items {
		if existing, ok := merged[item.ID]; ok {
			// Arctic Shift generally has richer records, so retain it as the base.
			if len(existing.Sources) > 0 && existing.Sources[0] == "PullPush" && len(item.Sources) > 0 && item.Sources[0] == "Arctic Shift" {
				merged[item.ID] = mergeItem(item, existing)
			} else {
				merged[item.ID] = mergeItem(existing, item)
			}
		} else {
			merged[item.ID] = item
		}
	}
	out := make([]Item, 0, len(merged))
	for _, item := range merged {
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CreatedUTC == out[j].CreatedUTC {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedUTC > out[j].CreatedUTC
	})
	return out
}
