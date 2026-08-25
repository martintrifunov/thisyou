package reddit

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Kind identifies the Reddit object represented by an Item.
type Kind string

const (
	Post    Kind = "post"
	Comment Kind = "comment"
)

// Item is the common subset of an archived Reddit post or comment used by the UI.
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
	if strings.HasPrefix(i.Permalink, "http://") || strings.HasPrefix(i.Permalink, "https://") {
		return i.Permalink
	}
	if i.Permalink != "" {
		return "https://www.reddit.com" + "/" + strings.TrimPrefix(i.Permalink, "/")
	}
	return i.URL
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
		Author:      record.Author,
		Subreddit:   record.Subreddit,
		CreatedUTC:  created,
		Score:       record.Score,
		Title:       record.Title,
		Body:        body,
		Permalink:   record.Permalink,
		URL:         record.URL,
		NumComments: record.NumComments,
		Over18:      record.Over18,
		Sources:     []string{source},
	}, nil
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
