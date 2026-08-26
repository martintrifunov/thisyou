package ui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/aymanbagabas/go-osc52/v2"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/martintrifunov/thisyou/internal/reddit"
)

type actionMsg struct {
	success string
	err     error
}

func copyToClipboard(value string) error {
	if err := clipboard.WriteAll(value); err == nil {
		return nil
	}
	if len(value) > 100_000 {
		return fmt.Errorf("system clipboard unavailable and text is too large for terminal copy")
	}
	_, err := osc52.New(value).WriteTo(os.Stderr)
	return err
}

func openInBrowser(value string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", value)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", value)
	default:
		command = exec.Command("xdg-open", value)
	}
	return command.Run()
}

func (m *Model) openItem(item reddit.Item) tea.Cmd {
	value := item.RedditURL()
	if value == "" {
		m.setActionError("no safe Reddit URL available")
		return nil
	}
	m.copying = false
	m.notice = "opening Reddit URL..."
	m.noticeError = false
	openURL := m.openURL
	return func() tea.Msg {
		return actionMsg{success: "opened Reddit URL", err: openURL(value)}
	}
}

func (m *Model) copyItem(item reddit.Item, choice string) tea.Cmd {
	label, value := copyValue(item, choice)
	if value == "" {
		m.copying = false
		m.setActionError("no " + label + " available")
		return nil
	}
	m.copying = false
	m.notice = "copying " + label + "..."
	m.noticeError = false
	copyText := m.copyText
	return func() tea.Msg {
		return actionMsg{success: "copied " + label, err: copyText(value)}
	}
}

func copyValue(item reddit.Item, choice string) (string, string) {
	switch choice {
	case "u":
		return "Reddit URL", item.RedditURL()
	case "t":
		return "text", recordText(item)
	case "a":
		if item.Author == "" {
			return "author", ""
		}
		return "author", "u/" + item.Author
	case "s":
		if item.Subreddit == "" {
			return "subreddit", ""
		}
		return "subreddit", "r/" + item.Subreddit
	case "l":
		return "linked URL", item.URL
	default:
		return "value", ""
	}
}

func recordText(item reddit.Item) string {
	parts := make([]string, 0, 2)
	if title := strings.TrimSpace(item.Title); title != "" {
		parts = append(parts, title)
	}
	if body := strings.TrimSpace(item.Body); body != "" {
		parts = append(parts, body)
	}
	return strings.Join(parts, "\n\n")
}

func (m *Model) updateCopyMenu(key string, item reddit.Item) tea.Cmd {
	if key == "esc" {
		m.copying = false
		m.notice = ""
		m.noticeError = false
		return nil
	}
	if strings.Contains("utasl", key) && len(key) == 1 {
		return m.copyItem(item, key)
	}
	return nil
}

func (m *Model) setActionError(message string) {
	m.notice = message
	m.noticeError = true
}

func (m *Model) actionFooter(defaultText string) string {
	text := defaultText
	style := mutedStyle
	if m.copying {
		text = "copy: u Reddit URL  t text  a author  s subreddit  l linked URL  esc cancel"
		style = titleStyle
	} else if m.exporting {
		text = "export current view: c CSV  j JSON  m Markdown  s SQLite  esc cancel"
		style = titleStyle
	} else if m.notice != "" {
		text = m.notice
		if m.noticeError {
			style = errorStyle
		} else {
			style = titleStyle
		}
	}
	return style.Render(text)
}

func actionError(err error) string {
	return fmt.Sprintf("action failed: %v", err)
}
