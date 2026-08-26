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
	"strconv"
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

type Progress struct {
	Kind    Kind
	Source  string
	Page    int
	Items   int
	Total   int
	Done    bool
	Err     error
	Skipped int
}

type Result struct {
	Posts             []Item
	Comments          []Item
	Errors            []error
	Warnings          []error
	SuccessfulStreams int
}

type Health struct {
	Source     string
	Available  bool
	StatusCode int
	Latency    time.Duration
	Err        error
}

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

func (c *Client) CheckHealth(ctx context.Context) []Health {
	sources := []source{{"Arctic Shift", c.ArcticShift}, {"PullPush", c.PullPush}}
	results := make(chan Health, len(sources))
	for _, value := range sources {
		go func(value source) {
			started := time.Now()
			req, err := http.NewRequestWithContext(ctx, http.MethodHead, strings.TrimRight(value.base, "/"), nil)
			if err != nil {
				results <- Health{Source: value.name, Latency: time.Since(started), Err: err}
				return
			}
			req.Header.Set("User-Agent", "thisyou/1.0 (archive health check)")
			client := c.HTTP
			if client == nil {
				client = http.DefaultClient
			}
			response, err := client.Do(req)
			health := Health{Source: value.name, Latency: time.Since(started), Err: err}
			if response != nil {
				health.StatusCode = response.StatusCode
				health.Available = response.StatusCode >= 200 && response.StatusCode < 400
				_ = response.Body.Close()
			}
			results <- health
		}(value)
	}
	health := make([]Health, 0, len(sources))
	for range sources {
		health = append(health, <-results)
	}
	sort.Slice(health, func(i, j int) bool { return health[i].Source < health[j].Source })
	return health
}

func (c *Client) SearchAll(ctx context.Context, username string, progress func(Progress)) Result {
	return c.search(ctx, username, nil, progress)
}

func (c *Client) SearchAfter(ctx context.Context, username string, postsAfter, commentsAfter int64, progress func(Progress)) Result {
	return c.search(ctx, username, map[Kind]int64{Post: postsAfter, Comment: commentsAfter}, progress)
}

func (c *Client) search(ctx context.Context, username string, after map[Kind]int64, progress func(Progress)) Result {
	type crawlResult struct {
		kind     Kind
		source   string
		items    []Item
		err      error
		warnings []error
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
				items, warnings, err := c.crawl(ctx, username, kind, src, after[kind], progress)
				results <- crawlResult{kind: kind, source: src.name, items: items, warnings: warnings, err: err}
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
		} else if result.err == nil {
			out.SuccessfulStreams++
		}
		for _, warning := range result.warnings {
			out.Warnings = append(out.Warnings, fmt.Errorf("%s %ss: %w", result.source, result.kind, warning))
		}
	}
	out.Posts = mergeAndSort(byKind[Post])
	out.Comments = mergeAndSort(byKind[Comment])
	sort.Slice(out.Errors, func(i, j int) bool { return out.Errors[i].Error() < out.Errors[j].Error() })
	sort.Slice(out.Warnings, func(i, j int) bool { return out.Warnings[i].Error() < out.Warnings[j].Error() })
	return out
}

func (c *Client) crawl(ctx context.Context, username string, kind Kind, src source, after int64, progress func(Progress)) ([]Item, []error, error) {
	var all []Item
	var warnings []error
	var before int64
	page := 0
	for {
		page++
		items, pageWarnings, err := c.fetchPage(ctx, username, kind, src, before, after)
		warnings = append(warnings, pageWarnings...)
		if err != nil {
			report(progress, Progress{Kind: kind, Source: src.name, Page: page, Total: len(all), Done: true, Err: err, Skipped: len(warnings)})
			return all, warnings, err
		}
		if len(items) == 0 {
			report(progress, Progress{Kind: kind, Source: src.name, Page: page, Total: len(all), Done: true, Skipped: len(warnings)})
			return all, warnings, nil
		}

		for _, item := range items {
			if item.CreatedUTC > after {
				all = append(all, item)
			}
		}
		report(progress, Progress{Kind: kind, Source: src.name, Page: page, Items: len(items), Total: len(all), Skipped: len(warnings)})
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
		if oldest <= after || (before != 0 && next >= before) {
			report(progress, Progress{Kind: kind, Source: src.name, Page: page, Total: len(all), Done: true, Skipped: len(warnings)})
			return all, warnings, nil
		}
		before = next
	}
}

func (c *Client) fetchPage(ctx context.Context, username string, kind Kind, src source, before, after int64) ([]Item, []error, error) {
	u, err := url.Parse(strings.TrimRight(src.base, "/"))
	if err != nil {
		return nil, nil, err
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
	if after > 0 {
		q.Set("after", fmt.Sprint(after))
	}
	u.RawQuery = q.Encode()

	body, err := c.get(ctx, u.String(), true)
	if err != nil {
		return nil, nil, err
	}
	var envelope struct {
		Data  []json.RawMessage `json:"data"`
		Error any               `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, nil, fmt.Errorf("decode response: %w", err)
	}
	if envelope.Error != nil && len(envelope.Data) == 0 {
		return nil, nil, fmt.Errorf("archive error: %v", envelope.Error)
	}
	items := make([]Item, 0, len(envelope.Data))
	var warnings []error
	for index, raw := range envelope.Data {
		item, err := decodeItem(raw, kind, src.name)
		if err != nil {
			warnings = append(warnings, fmt.Errorf("record %d skipped: %w", index+1, err))
			continue
		}
		items = append(items, item)
	}
	return items, warnings, nil
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
			if waitErr := wait(ctx, c.RetryDelay); waitErr != nil {
				return nil, waitErr
			}
			return c.get(ctx, endpoint, false)
		}
		return nil, err
	}
	body, readErr := io.ReadAll(io.LimitReader(res.Body, maxResponseBytes))
	_ = res.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		if retry && (res.StatusCode == http.StatusTooManyRequests || res.StatusCode >= 500) {
			delay := c.RetryDelay
			if res.StatusCode == http.StatusTooManyRequests {
				delay = rateLimitDelay(res.Header, time.Now(), delay)
			}
			if waitErr := wait(ctx, delay); waitErr != nil {
				return nil, waitErr
			}
			return c.get(ctx, endpoint, false)
		}
		if res.StatusCode == http.StatusTooManyRequests {
			return nil, fmt.Errorf("HTTP 429: rate limited this connection")
		}
		message := strings.TrimSpace(string(body))
		if len(message) > 180 {
			message = message[:180] + "..."
		}
		return nil, fmt.Errorf("HTTP %d: %s", res.StatusCode, message)
	}
	return body, nil
}

func rateLimitDelay(header http.Header, now time.Time, fallback time.Duration) time.Duration {
	if value := strings.TrimSpace(header.Get("Retry-After")); value != "" {
		if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds >= 0 {
			return time.Duration(seconds * float64(time.Second))
		}
		if reset, err := http.ParseTime(value); err == nil && reset.After(now) {
			return reset.Sub(now)
		}
	}
	if value := strings.TrimSpace(header.Get("X-RateLimit-Reset")); value != "" {
		if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds >= 0 {
			return time.Duration(seconds * float64(time.Second))
		}
	}
	if value := strings.TrimSpace(header.Get("X-RateLimit-Reset-At")); value != "" {
		if timestamp, err := strconv.ParseFloat(value, 64); err == nil {
			seconds := int64(timestamp)
			reset := time.Unix(seconds, int64((timestamp-float64(seconds))*float64(time.Second)))
			if reset.After(now) {
				return reset.Sub(now)
			}
		}
	}
	return max(0, fallback)
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

func MergeResults(base, fresh Result) Result {
	return Result{
		Posts:             mergeAndSort(append(append([]Item(nil), base.Posts...), fresh.Posts...)),
		Comments:          mergeAndSort(append(append([]Item(nil), base.Comments...), fresh.Comments...)),
		Errors:            append([]error(nil), fresh.Errors...),
		Warnings:          mergeWarnings(base.Warnings, fresh.Warnings),
		SuccessfulStreams: fresh.SuccessfulStreams,
	}
}

func mergeWarnings(groups ...[]error) []error {
	seen := make(map[string]struct{})
	var merged []error
	for _, group := range groups {
		for _, warning := range group {
			if warning == nil {
				continue
			}
			message := warning.Error()
			if _, exists := seen[message]; exists {
				continue
			}
			seen[message] = struct{}{}
			merged = append(merged, warning)
		}
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].Error() < merged[j].Error() })
	return merged
}
