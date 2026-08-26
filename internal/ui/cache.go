package ui

import (
	"errors"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/martintrifunov/thisyou/internal/historycache"
	"github.com/martintrifunov/thisyou/internal/reddit"
)

type historyStore interface {
	Load(string) (historycache.Snapshot, bool, error)
	Save(historycache.Snapshot) error
}

func (m *Model) beginCachedSearch() tea.Cmd {
	m.prepareSearch()
	m.loadingCache = true
	m.diagnosticRun = "cached history"
	searchID := m.searchID
	username := m.username
	store := m.history
	return func() tea.Msg {
		snapshot, found, err := store.Load(username)
		if err == nil && !found {
			err = fmt.Errorf("no cached history for u/%s", username)
		}
		if err != nil {
			return searchEvent{id: searchID, err: err}
		}
		return searchEvent{
			id: searchID,
			result: &searchResult{
				Posts: snapshot.Posts, Comments: snapshot.Comments,
				Errors: stringsToErrors(snapshot.Errors), Warnings: stringsToErrors(snapshot.Warnings),
			},
			cachedAt:  snapshot.SavedAt,
			fromCache: true,
		}
	}
}

func cacheSnapshot(store historyStore, username string, result searchResult) error {
	if result.SuccessfulStreams == 0 && len(result.Errors) > 0 {
		return nil
	}
	if len(result.Errors) > 0 {
		cached, found, err := store.Load(username)
		if err == nil && found {
			result = reddit.MergeResults(searchResult{
				Posts: cached.Posts, Comments: cached.Comments,
				Warnings: stringsToErrors(cached.Warnings),
			}, result)
		}
	}
	return store.Save(historycache.Snapshot{
		Username: username,
		SavedAt:  time.Now().UTC(),
		Posts:    result.Posts,
		Comments: result.Comments,
		Errors:   errorsToStrings(result.Errors),
		Warnings: errorsToStrings(result.Warnings),
	})
}

func errorsToStrings(values []error) []string {
	converted := make([]string, 0, len(values))
	for _, value := range values {
		if value != nil {
			converted = append(converted, value.Error())
		}
	}
	return converted
}

func stringsToErrors(values []string) []error {
	converted := make([]error, 0, len(values))
	for _, value := range values {
		converted = append(converted, errors.New(value))
	}
	return converted
}
