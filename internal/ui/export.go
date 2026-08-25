package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/maco/thisyou/internal/historyexport"
	"github.com/maco/thisyou/internal/reddit"
)

type exportWriter func(string, []reddit.Item, historyexport.Format) (string, error)

func writeExport(username string, items []reddit.Item, format historyexport.Format) (string, error) {
	return historyexport.Write(".", username, items, format, time.Now())
}

func (m *Model) openExportMenu() {
	if len(m.visible) == 0 {
		m.setActionError("no records in the current view")
		return
	}
	m.exporting = true
	m.copying = false
	m.notice = ""
	m.noticeError = false
}

func (m *Model) updateExportMenu(key string) tea.Cmd {
	if key == "esc" {
		m.exporting = false
		m.notice = ""
		m.noticeError = false
		return nil
	}
	formats := map[string]historyexport.Format{
		"c": historyexport.CSV,
		"j": historyexport.JSON,
		"m": historyexport.Markdown,
	}
	format, ok := formats[strings.ToLower(key)]
	if !ok || len(key) != 1 {
		return nil
	}
	m.exporting = false
	m.notice = "exporting current view..."
	m.noticeError = false
	username := m.username
	items := append([]reddit.Item(nil), m.visible...)
	writer := m.exportHistory
	return func() tea.Msg {
		path, err := writer(username, items, format)
		return actionMsg{
			success: fmt.Sprintf("exported %d records to %s", len(items), filepath.Base(path)),
			err:     err,
		}
	}
}
