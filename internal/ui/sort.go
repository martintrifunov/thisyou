package ui

import (
	"sort"
	"strings"

	"github.com/maco/thisyou/internal/reddit"
)

type sortMode int

const (
	newestFirst sortMode = iota
	oldestFirst
	highestScore
	subredditName
)

func (s sortMode) next() sortMode {
	return (s + 1) % 4
}

func (s sortMode) label() string {
	switch s {
	case oldestFirst:
		return "OLDEST"
	case highestScore:
		return "TOP SCORE"
	case subredditName:
		return "SUBREDDIT"
	default:
		return "NEWEST"
	}
}

func sortItems(items []reddit.Item, mode sortMode) {
	sort.SliceStable(items, func(i, j int) bool {
		left, right := items[i], items[j]
		switch mode {
		case oldestFirst:
			if left.CreatedUTC != right.CreatedUTC {
				return left.CreatedUTC < right.CreatedUTC
			}
		case highestScore:
			if left.Score != right.Score {
				return left.Score > right.Score
			}
		case subredditName:
			leftSubreddit := strings.ToLower(left.Subreddit)
			rightSubreddit := strings.ToLower(right.Subreddit)
			if leftSubreddit != rightSubreddit {
				return leftSubreddit < rightSubreddit
			}
		default:
			if left.CreatedUTC != right.CreatedUTC {
				return left.CreatedUTC > right.CreatedUTC
			}
		}
		if left.CreatedUTC != right.CreatedUTC {
			return left.CreatedUTC > right.CreatedUTC
		}
		return bookmarkKey(left) < bookmarkKey(right)
	})
}
