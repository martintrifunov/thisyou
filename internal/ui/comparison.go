package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/martintrifunov/thisyou/internal/reddit"
)

type compareEvent struct {
	id       uint64
	progress *reddit.Progress
	result   *reddit.Result
	cacheErr error
}

type comparisonMetric struct {
	label string
	left  string
	right string
}

func (m *Model) openComparisonInput() tea.Cmd {
	m.screen = compareInputScreen
	m.compareInput.SetValue("")
	m.compareInput.Focus()
	m.compareErr = ""
	m.notice = ""
	m.noticeError = false
	return textinput.Blink
}

func (m *Model) updateCompareInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			username, err := reddit.NormalizeUsername(m.compareInput.Value())
			if err != nil {
				m.compareErr = err.Error()
				return m, nil
			}
			if username == m.username {
				m.compareErr = "enter a different username"
				return m, nil
			}
			m.compareUsername = username
			m.compareErr = ""
			return m, m.beginComparison()
		case "esc":
			m.screen = resultsScreen
			m.compareInput.Blur()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.compareInput, cmd = m.compareInput.Update(msg)
	return m, cmd
}

func (m *Model) beginComparison() tea.Cmd {
	if m.compareCancel != nil {
		m.compareCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.compareCancel = cancel
	m.compareID++
	id := m.compareID
	m.screen = compareLoadingScreen
	m.compareInput.Blur()
	m.compareStatus = make(map[string]reddit.Progress)
	m.compareEvents = make(chan compareEvent, 128)
	ch := m.compareEvents
	username := m.compareUsername
	client := m.client
	store := m.history
	go func() {
		defer close(ch)
		result := client.SearchAll(ctx, username, func(progress reddit.Progress) {
			copy := progress
			select {
			case ch <- compareEvent{id: id, progress: &copy}:
			case <-ctx.Done():
			}
		})
		var cacheErr error
		if ctx.Err() == nil {
			cacheErr = cacheSnapshot(store, username, result)
		}
		select {
		case ch <- compareEvent{id: id, result: &result, cacheErr: cacheErr}:
		case <-ctx.Done():
		}
	}()
	commands := []tea.Cmd{waitForCompareEvent(ch)}
	if !m.spinning {
		m.spinning = true
		commands = append(commands, m.spinner.Tick)
	}
	return tea.Batch(commands...)
}

func waitForCompareEvent(ch <-chan compareEvent) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

func (m *Model) handleCompareEvent(msg compareEvent) tea.Cmd {
	if msg.id != m.compareID || m.screen != compareLoadingScreen {
		return nil
	}
	if msg.progress != nil {
		key := string(msg.progress.Kind) + ":" + msg.progress.Source
		m.compareStatus[key] = *msg.progress
		return waitForCompareEvent(m.compareEvents)
	}
	if msg.result == nil {
		return nil
	}
	m.compareResult = *msg.result
	m.screen = comparisonScreen
	m.compareEvents = nil
	if m.compareCancel != nil {
		m.compareCancel()
	}
	m.compareCancel = nil
	if msg.cacheErr != nil {
		m.setActionError("cache failed: " + msg.cacheErr.Error())
	}
	m.resizeComparison()
	m.comparison.GotoTop()
	return nil
}

func (m *Model) updateCompareLoading(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		if m.compareCancel != nil {
			m.compareCancel()
		}
		m.compareID++
		m.compareEvents = nil
		m.compareCancel = nil
		m.screen = compareInputScreen
		m.compareInput.Focus()
		return m, textinput.Blink
	}
	return m, nil
}

func (m *Model) updateComparison(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc", "backspace", "q":
			m.screen = resultsScreen
			return m, nil
		case "C":
			return m, m.openComparisonInput()
		case "g", "home":
			m.comparison.GotoTop()
			return m, nil
		case "G", "end":
			m.comparison.GotoBottom()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.comparison, cmd = m.comparison.Update(msg)
	return m, cmd
}

func (m *Model) resizeComparison() {
	width := max(1, m.width-4)
	height := max(1, m.height-4)
	offset := m.comparison.YOffset
	m.comparison.Width = width
	m.comparison.Height = height
	left := summarizeProfile(m.posts, m.comments)
	right := summarizeProfile(m.compareResult.Posts, m.compareResult.Comments)
	m.comparison.SetContent(renderComparison(
		m.username, left, len(m.errors), len(m.warnings),
		m.compareUsername, right, len(m.compareResult.Errors), len(m.compareResult.Warnings), width,
	))
	m.comparison.SetYOffset(offset)
}

func renderComparison(leftName string, left profileSummary, leftErrors, leftWarnings int, rightName string, right profileSummary, rightErrors, rightWarnings, width int) string {
	width = max(1, width)
	metrics := comparisonMetrics(left, right)
	var b strings.Builder
	if leftErrors+leftWarnings+rightErrors+rightWarnings > 0 {
		message := fmt.Sprintf(
			"Partial data: u/%s has %d unavailable stream(s) and %d skipped record(s); u/%s has %d and %d",
			leftName, leftErrors, leftWarnings, rightName, rightErrors, rightWarnings,
		)
		b.WriteString(errorStyle.Render(wrapText(message, width)) + "\n\n")
	}
	if width >= 64 {
		labelWidth := min(18, max(12, width/4))
		valueWidth := max(8, (width-labelWidth-4)/2)
		row := func(label, leftValue, rightValue string) string {
			label = lipgloss.PlaceHorizontal(labelWidth, lipgloss.Left, truncate(label, labelWidth))
			leftValue = lipgloss.PlaceHorizontal(valueWidth, lipgloss.Left, truncate(leftValue, valueWidth))
			rightValue = lipgloss.PlaceHorizontal(valueWidth, lipgloss.Left, truncate(rightValue, valueWidth))
			return label + "  " + leftValue + "  " + rightValue
		}
		b.WriteString(titleStyle.Render(row("METRIC", "u/"+leftName, "u/"+rightName)))
		for _, metric := range metrics {
			b.WriteString("\n" + row(metric.label, metric.left, metric.right))
		}
		return b.String()
	}
	b.WriteString(titleStyle.Render(wrapText("u/"+leftName+"  VS  u/"+rightName, width)))
	for _, metric := range metrics {
		b.WriteString("\n\n" + logoStyle.Render(truncate(strings.ToUpper(metric.label), width)))
		b.WriteString("\n" + wrapText("u/"+leftName+": "+metric.left, width))
		b.WriteString("\n" + wrapText("u/"+rightName+": "+metric.right, width))
	}
	return b.String()
}

func comparisonMetrics(left, right profileSummary) []comparisonMetric {
	return []comparisonMetric{
		{"Records", fmt.Sprint(left.totalRecords()), fmt.Sprint(right.totalRecords())},
		{"Posts", fmt.Sprint(left.Posts), fmt.Sprint(right.Posts)},
		{"Comments", fmt.Sprint(left.Comments), fmt.Sprint(right.Comments)},
		{"Score snapshot", fmt.Sprint(left.TotalScore), fmt.Sprint(right.TotalScore)},
		{"Average score", fmt.Sprintf("%.1f", left.AverageScore), fmt.Sprintf("%.1f", right.AverageScore)},
		{"Active days", fmt.Sprint(left.ActiveDays), fmt.Sprint(right.ActiveDays)},
		{"Communities", fmt.Sprint(left.UniqueSubreddits), fmt.Sprint(right.UniqueSubreddits)},
		{"Peak hour", peakHour(left), peakHour(right)},
		{"Top subreddit", topSubreddit(left), topSubreddit(right)},
		{"Coverage", profileCoverage(left), profileCoverage(right)},
	}
}

func peakHour(summary profileSummary) string {
	if summary.BusiestHourCount == 0 {
		return "unknown"
	}
	return fmt.Sprintf("%02d:00 (%d)", summary.BusiestHour, summary.BusiestHourCount)
}

func topSubreddit(summary profileSummary) string {
	if len(summary.TopSubreddits) == 0 {
		return "none"
	}
	return "r/" + summary.TopSubreddits[0].Name
}

func profileCoverage(summary profileSummary) string {
	if !summary.HasDateRange {
		return "unknown"
	}
	return summary.Earliest.Format("2006-01-02") + " — " + summary.Latest.Format("2006-01-02")
}

func (m *Model) viewCompareInput() string {
	var b strings.Builder
	contentWidth := max(4, min(58, m.width-4))
	if m.width >= 76 && m.height >= 20 {
		b.WriteString(logoStyle.Render(strings.TrimPrefix(logo, "\n")))
	} else {
		b.WriteString(logoStyle.Render("THIS YOU?  /  COMPARE"))
	}
	gap := "\n\n"
	if m.height < 16 {
		gap = "\n"
	}
	b.WriteString(gap + titleStyle.Render(wrapText("Compare u/"+m.username+" with another Redditor", contentWidth)))
	b.WriteString(gap + m.renderCompareInput())
	if m.compareErr != "" {
		b.WriteString("\n" + errorStyle.Render(wrapText(m.compareErr, contentWidth)))
	}
	b.WriteString(gap + mutedStyle.Render(wrapText("enter compare  •  ? help  •  esc return", contentWidth)))
	return center(m.width, m.height, b.String())
}

func (m *Model) renderCompareInput() string {
	width := max(4, min(46, m.width-8))
	content := truncate(m.compareInput.View(), width)
	content = lipgloss.PlaceHorizontal(width, lipgloss.Left, content)
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).Padding(0, 1).Render(content)
}

func (m *Model) viewCompareLoading() string {
	var b strings.Builder
	contentWidth := max(4, min(72, m.width-4))
	b.WriteString(logoStyle.Render("THIS YOU?  /  COMPARE"))
	b.WriteString("\n\n" + m.spinner.View() + " " + titleStyle.Render(wrapText("Crawling u/"+m.compareUsername, max(3, contentWidth-3))))
	b.WriteString("\n" + mutedStyle.Render(wrapText("Building a complete archived profile for comparison.", contentWidth)))
	keys := make([]string, 0, len(m.compareStatus))
	for key := range m.compareStatus {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		progress := m.compareStatus[key]
		state := fmt.Sprintf("page %d  •  %d records", progress.Page, progress.Total)
		if progress.Done {
			state = fmt.Sprintf("done  •  %d records", progress.Total)
		}
		if progress.Err != nil {
			state = fmt.Sprintf("unavailable  •  %d retained", progress.Total)
		}
		if progress.Skipped > 0 {
			state += fmt.Sprintf("  •  %d skipped", progress.Skipped)
		}
		b.WriteString("\n" + truncate(progress.Source+" "+string(progress.Kind)+"s  "+state, contentWidth))
	}
	b.WriteString("\n\n" + mutedStyle.Render(truncate("esc cancel  •  ctrl+c quit", contentWidth)))
	return center(m.width, m.height, b.String())
}

func (m *Model) viewComparison() string {
	width := max(1, m.width-4)
	header := logoStyle.Render(truncate("THIS YOU?  /  PROFILE COMPARISON", width))
	help := "j/k scroll  •  C new comparison  •  ? help  •  esc return"
	if width < 48 {
		help = "j/k  •  C new  •  ? help  •  esc"
	}
	footer := truncate(m.actionFooter(help), width)
	content := lipgloss.JoinVertical(lipgloss.Left, header, m.comparison.View(), footer)
	framed := lipgloss.NewStyle().Padding(1, 2).Render(content)
	return fillScreen(m.width, m.height, framed, lipgloss.Left, lipgloss.Top)
}
