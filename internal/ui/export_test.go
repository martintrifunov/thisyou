package ui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/maco/thisyou/internal/historyexport"
	"github.com/maco/thisyou/internal/reddit"
)

func TestExportCurrentVisibleView(t *testing.T) {
	m := New(nil)
	m.screen = resultsScreen
	m.username = "alice"
	m.posts = []reddit.Item{
		{Kind: reddit.Post, ID: "match", Title: "keep this"},
		{Kind: reddit.Post, ID: "other", Title: "exclude this"},
	}
	m.filter.SetValue("keep")
	m.refreshVisible()
	var gotUsername string
	var gotItems []reddit.Item
	var gotFormat historyexport.Format
	m.exportHistory = func(username string, items []reddit.Item, format historyexport.Format) (string, error) {
		gotUsername = username
		gotItems = append([]reddit.Item(nil), items...)
		gotFormat = format
		return "/tmp/thisyou-alice.csv", nil
	}
	_, _ = m.Update(runeKey("e"))
	if !m.exporting || !strings.Contains(m.actionFooter(""), "export current view") {
		t.Fatal("export menu did not open")
	}
	_, command := m.Update(runeKey("c"))
	if command == nil {
		t.Fatal("CSV export returned no command")
	}
	_, _ = m.Update(command())
	if gotUsername != "alice" || gotFormat != historyexport.CSV || len(gotItems) != 1 || gotItems[0].ID != "match" {
		t.Fatalf("export = username %q, format %q, items %#v", gotUsername, gotFormat, gotItems)
	}
	if m.notice != "exported 1 records to thisyou-alice.csv" || m.noticeError {
		t.Fatalf("export notice = %q, error = %v", m.notice, m.noticeError)
	}
}

func TestExportRejectsEmptyViewAndReportsWriteFailure(t *testing.T) {
	m := New(nil)
	m.screen = resultsScreen
	_, _ = m.Update(runeKey("e"))
	if m.exporting || !m.noticeError || m.notice != "no records in the current view" {
		t.Fatalf("empty export state = exporting %v, notice %q", m.exporting, m.notice)
	}
	m.posts = []reddit.Item{{Kind: reddit.Post, ID: "one"}}
	m.refreshVisible()
	m.exportHistory = func(string, []reddit.Item, historyexport.Format) (string, error) {
		return "", errors.New("disk full")
	}
	_, _ = m.Update(runeKey("e"))
	_, command := m.Update(runeKey("j"))
	_, _ = m.Update(command())
	if !m.noticeError || m.notice != "action failed: disk full" {
		t.Fatalf("failed export notice = %q, error = %v", m.notice, m.noticeError)
	}
}

func TestExportMenuCancels(t *testing.T) {
	m := New(nil)
	m.screen = resultsScreen
	m.posts = []reddit.Item{{Kind: reddit.Post, ID: "one"}}
	m.refreshVisible()
	_, _ = m.Update(runeKey("e"))
	_, command := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.exporting || command != nil {
		t.Fatalf("cancelled export state = exporting %v, command %v", m.exporting, command)
	}
}
