package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/maco/thisyou/internal/historycache"
	"github.com/maco/thisyou/internal/localdata"
)

type fakeLocalHistoryStore struct {
	entries []historycache.Entry
	deleted string
}

func (s *fakeLocalHistoryStore) List() ([]historycache.Entry, error) {
	return append([]historycache.Entry(nil), s.entries...), nil
}

func (s *fakeLocalHistoryStore) Delete(username string) error {
	s.deleted = username
	s.entries = nil
	return nil
}

type fakeExportStore struct {
	entries []localdata.Export
	deleted string
}

func (s *fakeExportStore) List() ([]localdata.Export, error) {
	return append([]localdata.Export(nil), s.entries...), nil
}

func (s *fakeExportStore) Delete(name string) error {
	s.deleted = name
	s.entries = nil
	return nil
}

func TestLocalDataRequiresConfirmationBeforeDelete(t *testing.T) {
	m := New(nil)
	history := &fakeLocalHistoryStore{entries: []historycache.Entry{{Username: "alice", Posts: 1}}}
	m.historyData = history
	m.exports = &fakeExportStore{}
	_, command := m.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	if m.screen != dataScreen || command == nil {
		t.Fatalf("open data = screen %v, command %v", m.screen, command)
	}
	_, _ = m.Update(command())
	_, _ = m.Update(runeKey("x"))
	if !m.dataConfirm || history.deleted != "" {
		t.Fatalf("confirmation = %v, deleted = %q", m.dataConfirm, history.deleted)
	}
	_, command = m.Update(runeKey("y"))
	if command == nil {
		t.Fatal("confirmed deletion returned no command")
	}
	_, _ = m.Update(command())
	if history.deleted != "alice" || len(m.dataEntries) != 0 {
		t.Fatalf("deleted = %q, entries = %#v", history.deleted, m.dataEntries)
	}
}
