package reddit

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type Kind string

const (
	Post    Kind = "post"
	Comment Kind = "comment"
)

type Item struct {
	Kind        Kind
	ID          string
	Author      string
	Subreddit   string
	CreatedUTC  int64
	Score       int
	Title       string
	Body        string
	Permalink   string
	URL         string
	NumComments int
	Over18      bool
	Sources     []string
}

func (i Item) Time() time.Time { return time.Unix(i.CreatedUTC, 0) }

func (i Item) RedditURL() string {
	permalink := strings.TrimSpace(i.Permalink)
	if permalink == "" {
		return ""
	}
	parsed, err := url.Parse(permalink)
	if err != nil {
		return ""
	}
	if !parsed.IsAbs() {
		permalink = "https://www.reddit.com/" + strings.TrimPrefix(permalink, "/")
		parsed, err = url.Parse(permalink)
		if err != nil {
			return ""
		}
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "reddit.com" && !strings.HasSuffix(host, ".reddit.com") && host != "redd.it" && !strings.HasSuffix(host, ".redd.it") {
		return ""
	}
	parsed.Scheme = "https"
	return parsed.String()
}

type archiveItem struct {
	ID          string          `json:"id"`
	Author      string          `json:"author"`
	Subreddit   string          `json:"subreddit"`
	CreatedUTC  json.RawMessage `json:"created_utc"`
	Score       int             `json:"score"`
	Title       string          `json:"title"`
	Selftext    string          `json:"selftext"`
	Body        string          `json:"body"`
	Permalink   string          `json:"permalink"`
	URL         string          `json:"url"`
	NumComments int             `json:"num_comments"`
	Over18      bool            `json:"over_18"`
}

func decodeItem(raw json.RawMessage, kind Kind, source string) (Item, error) {
	var record archiveItem
	if err := json.Unmarshal(raw, &record); err != nil {
		return Item{}, err
	}
	record.ID = sanitizeText(record.ID, false)
	if record.ID == "" {
		return Item{}, fmt.Errorf("record has no id")
	}
	created, err := parseTimestamp(record.CreatedUTC)
	if err != nil {
		return Item{}, fmt.Errorf("record %s: %w", record.ID, err)
	}
	body := record.Body
	if kind == Post {
		body = record.Selftext
	}
	return Item{
		Kind:        kind,
		ID:          record.ID,
		Author:      sanitizeText(record.Author, false),
		Subreddit:   sanitizeText(record.Subreddit, false),
		CreatedUTC:  created,
		Score:       record.Score,
		Title:       sanitizeText(record.Title, false),
		Body:        sanitizeText(body, true),
		Permalink:   sanitizeText(record.Permalink, false),
		URL:         sanitizeText(record.URL, false),
		NumComments: record.NumComments,
		Over18:      record.Over18,
		Sources:     []string{source},
	}, nil
}

func sanitizeText(value string, multiline bool) string {
	return strings.Map(func(r rune) rune {
		if multiline && r == '\n' {
			return r
		}
		if unicode.IsControl(r) {
			if unicode.IsSpace(r) {
				return ' '
			}
			return -1
		}
		return r
	}, value)
}

func parseTimestamp(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, fmt.Errorf("missing created_utc")
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		if n, err := number.Int64(); err == nil {
			return n, nil
		}
		if f, err := strconv.ParseFloat(number.String(), 64); err == nil {
			return int64(f), nil
		}
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		f, err := strconv.ParseFloat(text, 64)
		return int64(f), err
	}
	return 0, fmt.Errorf("invalid created_utc %q", raw)
}

func mergeItem(preferred, other Item) Item {
	if preferred.Author == "" {
		preferred.Author = other.Author
	}
	if preferred.Subreddit == "" {
		preferred.Subreddit = other.Subreddit
	}
	if preferred.CreatedUTC == 0 {
		preferred.CreatedUTC = other.CreatedUTC
	}
	if preferred.Score == 0 {
		preferred.Score = other.Score
	}
	if preferred.Title == "" {
		preferred.Title = other.Title
	}
	if preferred.Body == "" {
		preferred.Body = other.Body
	}
	if preferred.Permalink == "" {
		preferred.Permalink = other.Permalink
	}
	if preferred.URL == "" {
		preferred.URL = other.URL
	}
	if preferred.NumComments == 0 {
		preferred.NumComments = other.NumComments
	}
	preferred.Over18 = preferred.Over18 || other.Over18
	for _, source := range other.Sources {
		found := false
		for _, existing := range preferred.Sources {
			if existing == source {
				found = true
				break
			}
		}
		if !found {
			preferred.Sources = append(preferred.Sources, source)
		}
	}
	return preferred
}
