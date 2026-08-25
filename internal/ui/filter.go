package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/maco/thisyou/internal/reddit"
)

type resultFilter struct {
	terms     []string
	subreddit string
	kind      reddit.Kind
	after     time.Time
	before    time.Time
	minScore  *int
	maxScore  *int
}

func parseResultFilter(query string) (resultFilter, error) {
	var filter resultFilter
	for _, token := range strings.Fields(query) {
		key, value, hasPrefix := strings.Cut(token, ":")
		if !hasPrefix {
			filter.terms = append(filter.terms, strings.ToLower(token))
			continue
		}
		if value == "" {
			return resultFilter{}, fmt.Errorf("%s filter needs a value", key)
		}
		switch strings.ToLower(key) {
		case "sub", "subreddit":
			filter.subreddit = strings.TrimPrefix(strings.ToLower(value), "r/")
		case "type", "kind":
			switch strings.ToLower(value) {
			case "post", "posts":
				filter.kind = reddit.Post
			case "comment", "comments":
				filter.kind = reddit.Comment
			default:
				return resultFilter{}, fmt.Errorf("type must be post or comment")
			}
		case "after":
			parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
			if err != nil {
				return resultFilter{}, fmt.Errorf("after date must be YYYY-MM-DD")
			}
			filter.after = parsed
		case "before":
			parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
			if err != nil {
				return resultFilter{}, fmt.Errorf("before date must be YYYY-MM-DD")
			}
			filter.before = parsed.AddDate(0, 0, 1)
		case "min", "minscore":
			score, err := strconv.Atoi(value)
			if err != nil {
				return resultFilter{}, fmt.Errorf("minimum score must be a number")
			}
			filter.minScore = &score
		case "max", "maxscore":
			score, err := strconv.Atoi(value)
			if err != nil {
				return resultFilter{}, fmt.Errorf("maximum score must be a number")
			}
			filter.maxScore = &score
		default:
			return resultFilter{}, fmt.Errorf("unknown filter %q", key)
		}
	}
	if !filter.after.IsZero() && !filter.before.IsZero() && !filter.after.Before(filter.before) {
		return resultFilter{}, fmt.Errorf("after date must not be later than before date")
	}
	if filter.minScore != nil && filter.maxScore != nil && *filter.minScore > *filter.maxScore {
		return resultFilter{}, fmt.Errorf("minimum score must not exceed maximum score")
	}
	return filter, nil
}

func (f resultFilter) matches(item reddit.Item) bool {
	if f.kind != "" && item.Kind != f.kind {
		return false
	}
	if f.subreddit != "" && strings.ToLower(item.Subreddit) != f.subreddit {
		return false
	}
	if f.minScore != nil && item.Score < *f.minScore {
		return false
	}
	if f.maxScore != nil && item.Score > *f.maxScore {
		return false
	}
	if !f.after.IsZero() || !f.before.IsZero() {
		if item.CreatedUTC <= 0 {
			return false
		}
		created := item.Time().Local()
		if !f.after.IsZero() && created.Before(f.after) {
			return false
		}
		if !f.before.IsZero() && !created.Before(f.before) {
			return false
		}
	}
	if len(f.terms) == 0 {
		return true
	}
	haystack := strings.ToLower(strings.Join([]string{item.Title, item.Body, item.Subreddit, item.Author}, "\n"))
	for _, term := range f.terms {
		if !strings.Contains(haystack, term) {
			return false
		}
	}
	return true
}
