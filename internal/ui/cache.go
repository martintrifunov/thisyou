package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/maco/thisyou/internal/historycache"
)

type historyStore interface {
	Load(string) (historycache.Snapshot, bool, error)
	Save(historycache.Snapshot) error
}

func (m *Model) beginCachedSearch() tea.Cmd {
	m.prepareSearch()
	m.loadingCache = true
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
			id:        searchID,
			result:    &searchResult{Posts: snapshot.Posts, Comments: snapshot.Comments},
			cachedAt:  snapshot.SavedAt,
			fromCache: true,
		}
	}
}

func cacheSnapshot(store historyStore, username string, result searchResult) error {
	return store.Save(historycache.Snapshot{
		Username: username,
		SavedAt:  time.Now().UTC(),
		Posts:    result.Posts,
		Comments: result.Comments,
	})
}
