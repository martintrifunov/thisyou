package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/maco/thisyou/internal/profiles"
)

type fakeProfileStore struct {
	entries []profiles.Entry
}

func (s *fakeProfileStore) Load() ([]profiles.Entry, error) {
	return append([]profiles.Entry(nil), s.entries...), nil
}

func (s *fakeProfileStore) Toggle(username string) (bool, []profiles.Entry, error) {
	for index, entry := range s.entries {
		if entry.Username == username {
			s.entries = append(s.entries[:index], s.entries[index+1:]...)
			return false, append([]profiles.Entry(nil), s.entries...), nil
		}
	}
	s.entries = append(s.entries, profiles.Entry{Username: username})
	return true, append([]profiles.Entry(nil), s.entries...), nil
}

func TestSavedProfilesOpenAndToggle(t *testing.T) {
	m := New(nil)
	store := &fakeProfileStore{entries: []profiles.Entry{{Username: "alice"}}}
	m.profiles = store
	_, command := m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	if m.screen != profilesScreen || command == nil {
		t.Fatalf("open profiles = screen %v, command %v", m.screen, command)
	}
	_, _ = m.Update(command())
	if len(m.savedProfiles) != 1 || m.savedProfiles[0].Username != "alice" {
		t.Fatalf("saved profiles = %#v", m.savedProfiles)
	}
	_, command = m.Update(runeKey("x"))
	if command == nil {
		t.Fatal("remove profile returned no command")
	}
	_, _ = m.Update(command())
	if len(m.savedProfiles) != 0 || m.notice != "saved profile removed" {
		t.Fatalf("profiles = %#v, notice = %q", m.savedProfiles, m.notice)
	}
}
