package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/maco/thisyou/internal/reddit"
)

const logo = `
 _______  __   __  ___   _______    __   __  _______  __   __  ______
|       ||  | |  ||   | |       |  |  | |  ||       ||  | |  ||      |
|_     _||  |_|  ||   | |  _____|  |  |_|  ||   _   ||  | |  ||___   |
  |   |  |       ||   | | |_____   |       ||  | |  ||  |_|  |  __|  |
  |   |  |       ||   | |_____  |  |_     _||  |_|  ||       | |_____|
  |   |  |   _   ||   |  _____| |    |   |  |       ||       |   __
  |___|  |__| |__||___| |_______|    |___|  |_______||_______|  |__|`

type screen int

const (
	searchScreen screen = iota
	loadingScreen
	resultsScreen
	detailScreen
)

type tab int

const (
	allTab tab = iota
	postsTab
	commentsTab
)

var (
	orange     = lipgloss.Color("#ff4500")
	muted      = lipgloss.Color("#818384")
	foreground = lipgloss.Color("#d7dadc")
	panel      = lipgloss.Color("#1a1a1b")
	border     = lipgloss.Color("#343536")

	logoStyle     = lipgloss.NewStyle().Foreground(orange).Bold(true)
	titleStyle    = lipgloss.NewStyle().Foreground(foreground).Bold(true)
	mutedStyle    = lipgloss.NewStyle().Foreground(muted)
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff6b6b"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Background(lipgloss.Color("#272729"))
)

type searchEvent struct {
	progress *reddit.Progress
	result   *reddit.Result
}

type Model struct {
	client *reddit.Client

	screen   screen
	width    int
	height   int
	username string
	input    textinput.Model
	filter   textinput.Model
	spinner  spinner.Model
	detail   viewport.Model

	posts    []reddit.Item
	comments []reddit.Item
	visible  []reddit.Item
	tab      tab
	selected int
	offset   int

	filtering bool
	status    map[string]reddit.Progress
	errors    []error
	searchErr string
	events    chan searchEvent
	cancel    context.CancelFunc
}

func New(client *reddit.Client) *Model {
	if client == nil {
		client = reddit.NewClient()
	}
	in := textinput.New()
	in.Placeholder = "redditor username"
	in.Prompt = "u/"
	in.CharLimit = 80
	in.Width = 42
	in.Focus()
	in.PromptStyle = lipgloss.NewStyle().Foreground(orange).Bold(true)
	in.TextStyle = lipgloss.NewStyle().Foreground(foreground)

	filter := textinput.New()
	filter.Placeholder = "filter current results"
	filter.Prompt = "/ "
	filter.CharLimit = 100
	filter.PromptStyle = lipgloss.NewStyle().Foreground(orange)
	filter.TextStyle = lipgloss.NewStyle().Foreground(foreground)

	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = lipgloss.NewStyle().Foreground(orange)

	return &Model{
		client:  client,
		screen:  searchScreen,
		input:   in,
		filter:  filter,
		spinner: spin,
		detail:  viewport.New(80, 20),
		status:  make(map[string]reddit.Progress),
	}
}

func (m *Model) Init() tea.Cmd { return textinput.Blink }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.detail.Width = max(20, msg.Width-6)
		m.detail.Height = max(5, msg.Height-8)
		m.ensureVisible()
		return m, nil
	case searchEvent:
		if msg.progress != nil {
			key := string(msg.progress.Kind) + ":" + msg.progress.Source
			m.status[key] = *msg.progress
			return m, waitForEvent(m.events)
		}
		if msg.result != nil {
			m.posts = msg.result.Posts
			m.comments = msg.result.Comments
			m.errors = msg.result.Errors
			m.screen = resultsScreen
			m.events = nil
			m.cancel = nil
			m.refreshVisible()
			return m, nil
		}
	case spinner.TickMsg:
		if m.screen == loadingScreen {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	key, ok := msg.(tea.KeyMsg)
	if ok && key.String() == "ctrl+c" {
		if m.cancel != nil {
			m.cancel()
		}
		return m, tea.Quit
	}

	switch m.screen {
	case searchScreen:
		return m.updateSearch(msg)
	case loadingScreen:
		return m.updateLoading(msg)
	case resultsScreen:
		return m.updateResults(msg)
	case detailScreen:
		return m.updateDetail(msg)
	default:
		return m, nil
	}
}

func (m *Model) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			username, err := reddit.NormalizeUsername(m.input.Value())
			if err != nil {
				m.searchErr = err.Error()
				return m, nil
			}
			m.username = username
			m.searchErr = ""
			return m, m.beginSearch()
		case "esc":
			m.input.SetValue("")
			m.searchErr = ""
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *Model) updateLoading(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		if m.cancel != nil {
			m.cancel()
		}
		m.events = nil
		m.cancel = nil
		m.screen = searchScreen
		m.input.Focus()
		return m, textinput.Blink
	}
	return m, nil
}

func (m *Model) updateResults(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.filtering {
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "enter":
				m.filtering = false
				m.filter.Blur()
				return m, nil
			case "esc":
				m.filtering = false
				m.filter.SetValue("")
				m.filter.Blur()
				m.refreshVisible()
				return m, nil
			}
		}
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		m.refreshVisible()
		return m, cmd
	}

	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "q":
		return m, tea.Quit
	case "r":
		m.screen = searchScreen
		m.input.SetValue("")
		m.input.Focus()
		m.searchErr = ""
		return m, textinput.Blink
	case "tab", "right", "l":
		m.tab = (m.tab + 1) % 3
		m.refreshVisible()
	case "shift+tab", "left", "h":
		m.tab = (m.tab + 2) % 3
		m.refreshVisible()
	case "j", "down":
		if m.selected < len(m.visible)-1 {
			m.selected++
			m.ensureVisible()
		}
	case "k", "up":
		if m.selected > 0 {
			m.selected--
			m.ensureVisible()
		}
	case "pgdown", "ctrl+d":
		m.selected = min(len(m.visible)-1, m.selected+m.pageRows())
		m.ensureVisible()
	case "pgup", "ctrl+u":
		m.selected = max(0, m.selected-m.pageRows())
		m.ensureVisible()
	case "g", "home":
		m.selected, m.offset = 0, 0
	case "G", "end":
		m.selected = max(0, len(m.visible)-1)
		m.ensureVisible()
	case "/":
		m.filtering = true
		m.filter.Focus()
		return m, textinput.Blink
	case "enter":
		if len(m.visible) > 0 {
			m.openDetail(m.visible[m.selected])
		}
	}
	return m, nil
}

func (m *Model) updateDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc", "backspace", "q":
			m.screen = resultsScreen
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.detail, cmd = m.detail.Update(msg)
	return m, cmd
}

func (m *Model) beginSearch() tea.Cmd {
	if m.cancel != nil {
		m.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.screen = loadingScreen
	m.status = make(map[string]reddit.Progress)
	m.errors = nil
	m.posts, m.comments, m.visible = nil, nil, nil
	m.events = make(chan searchEvent, 128)
	ch := m.events
	username := m.username
	client := m.client
	go func() {
		result := client.SearchAll(ctx, username, func(p reddit.Progress) {
			copy := p
			select {
			case ch <- searchEvent{progress: &copy}:
			case <-ctx.Done():
			}
		})
		select {
		case ch <- searchEvent{result: &result}:
		case <-ctx.Done():
		}
	}()
	return tea.Batch(m.spinner.Tick, waitForEvent(ch))
}

func waitForEvent(ch <-chan searchEvent) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

func (m *Model) refreshVisible() {
	var source []reddit.Item
	switch m.tab {
	case postsTab:
		source = m.posts
	case commentsTab:
		source = m.comments
	default:
		source = make([]reddit.Item, 0, len(m.posts)+len(m.comments))
		source = append(source, m.posts...)
		source = append(source, m.comments...)
		sort.SliceStable(source, func(i, j int) bool { return source[i].CreatedUTC > source[j].CreatedUTC })
	}
	query := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	if query == "" {
		m.visible = source
	} else {
		m.visible = make([]reddit.Item, 0, len(source))
		for _, item := range source {
			haystack := strings.ToLower(strings.Join([]string{item.Title, item.Body, item.Subreddit, item.Author}, "\n"))
			if strings.Contains(haystack, query) {
				m.visible = append(m.visible, item)
			}
		}
	}
	m.selected, m.offset = 0, 0
}

func (m *Model) ensureVisible() {
	rows := m.pageRows()
	if m.selected < m.offset {
		m.offset = m.selected
	}
	if m.selected >= m.offset+rows {
		m.offset = m.selected - rows + 1
	}
	maxOffset := max(0, len(m.visible)-rows)
	m.offset = min(maxOffset, max(0, m.offset))
}

func (m *Model) pageRows() int { return max(1, (m.height-9)/3) }

func (m *Model) openDetail(item reddit.Item) {
	m.screen = detailScreen
	m.detail.Width = max(20, m.width-6)
	m.detail.Height = max(5, m.height-8)
	m.detail.SetContent(renderDetail(item, m.detail.Width))
	m.detail.GotoTop()
}

func (m *Model) View() string {
	switch m.screen {
	case searchScreen:
		return m.viewSearch()
	case loadingScreen:
		return m.viewLoading()
	case resultsScreen:
		return m.viewResults()
	case detailScreen:
		return m.viewDetail()
	default:
		return ""
	}
}

func (m *Model) viewSearch() string {
	var b strings.Builder
	if m.width >= 76 {
		b.WriteString(logoStyle.Render(logo))
	} else {
		b.WriteString(logoStyle.Render("THIS YOU?"))
	}
	b.WriteString("\n\n")
	b.WriteString(titleStyle.Render("A Reddit archive history viewer"))
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Search public Arctic Shift + PullPush archive records"))
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).Padding(0, 1).Render(m.input.View()))
	if m.searchErr != "" {
		b.WriteString("\n" + errorStyle.Render(m.searchErr))
	}
	b.WriteString("\n\n" + mutedStyle.Render("enter search  •  ctrl+c quit"))
	return center(m.width, m.height, b.String())
}

func (m *Model) viewLoading() string {
	var b strings.Builder
	b.WriteString(logoStyle.Render("THIS YOU?"))
	b.WriteString("\n\n")
	b.WriteString(m.spinner.View() + " " + titleStyle.Render("Crawling u/"+m.username))
	b.WriteString("\n" + mutedStyle.Render("Fetching the complete available history; each archive has an independent cursor."))
	b.WriteString("\n\n")
	keys := make([]string, 0, len(m.status))
	for key := range m.status {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		p := m.status[key]
		state := fmt.Sprintf("page %d  •  %d records", p.Page, p.Total)
		if p.Done {
			state = fmt.Sprintf("done  •  %d records", p.Total)
		}
		if p.Err != nil {
			state = fmt.Sprintf("unavailable  •  %d records retained", p.Total)
		}
		b.WriteString(fmt.Sprintf("%-28s %s\n", p.Source+" "+string(p.Kind)+"s", state))
	}
	b.WriteString("\n" + mutedStyle.Render("esc cancel  •  ctrl+c quit"))
	return center(m.width, m.height, b.String())
}

func (m *Model) viewResults() string {
	var b strings.Builder
	header := fmt.Sprintf("THIS YOU?  /  u/%s", m.username)
	b.WriteString(logoStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(m.renderTabs())
	b.WriteString("\n")
	if m.filtering || m.filter.Value() != "" {
		b.WriteString(m.filter.View() + "\n")
	}
	if len(m.visible) == 0 {
		b.WriteString("\n" + mutedStyle.Render("No archived records found for this view.") + "\n")
	} else {
		rows := m.pageRows()
		end := min(len(m.visible), m.offset+rows)
		for i := m.offset; i < end; i++ {
			b.WriteString(m.renderRow(m.visible[i], i == m.selected))
			b.WriteByte('\n')
		}
	}
	if len(m.errors) > 0 {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Partial results: %d archive stream(s) unavailable", len(m.errors))) + "\n")
	}
	position := "0/0"
	if len(m.visible) > 0 {
		position = fmt.Sprintf("%d/%d", m.selected+1, len(m.visible))
	}
	footer := fmt.Sprintf("%s  •  j/k move  enter inspect  tab view  / filter  r new search  q quit", position)
	b.WriteString(mutedStyle.Render(truncate(footer, max(10, m.width))))
	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m *Model) renderTabs() string {
	labels := []string{
		fmt.Sprintf("ALL %d", len(m.posts)+len(m.comments)),
		fmt.Sprintf("POSTS %d", len(m.posts)),
		fmt.Sprintf("COMMENTS %d", len(m.comments)),
	}
	parts := make([]string, len(labels))
	for i, label := range labels {
		style := lipgloss.NewStyle().Padding(0, 1).Foreground(muted)
		if tab(i) == m.tab {
			style = style.Foreground(lipgloss.Color("#ffffff")).Background(orange).Bold(true)
		}
		parts[i] = style.Render(label)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (m *Model) renderRow(item reddit.Item, selected bool) string {
	width := max(30, m.width-6)
	kind := "COMMENT"
	headline := clean(item.Body)
	if item.Kind == reddit.Post {
		kind = "POST"
		headline = clean(item.Title)
		if headline == "" {
			headline = clean(item.Body)
		}
	}
	meta := fmt.Sprintf("%-7s  r/%s  •  %s  •  %d points", kind, fallback(item.Subreddit, "unknown"), item.Time().Local().Format("2006-01-02 15:04"), item.Score)
	body := "  " + truncate(headline, width-2)
	content := truncate(meta, width) + "\n" + body
	style := lipgloss.NewStyle().Width(width).Height(2).Foreground(foreground).PaddingLeft(1)
	if selected {
		style = style.Inherit(selectedStyle).BorderLeft(true).BorderStyle(lipgloss.ThickBorder()).BorderForeground(orange).PaddingLeft(0)
	}
	return style.Render(content)
}

func (m *Model) viewDetail() string {
	var b strings.Builder
	b.WriteString(logoStyle.Render("THIS YOU?  /  RECORD"))
	b.WriteString("\n")
	b.WriteString(m.detail.View())
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("j/k or arrows scroll  •  esc/backspace return"))
	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func renderDetail(item reddit.Item, width int) string {
	var b strings.Builder
	typeName := strings.ToUpper(string(item.Kind))
	b.WriteString(logoStyle.Render(typeName))
	if item.Over18 {
		b.WriteString("  " + errorStyle.Render("NSFW"))
	}
	b.WriteString("\n")
	if item.Title != "" {
		b.WriteString(titleStyle.Render(wrapText(item.Title, max(10, width-2))) + "\n\n")
	}
	b.WriteString(mutedStyle.Render(fmt.Sprintf("u/%s in r/%s", fallback(item.Author, "unknown"), fallback(item.Subreddit, "unknown"))))
	b.WriteString("\n" + mutedStyle.Render(item.Time().Local().Format(time.RFC1123)))
	b.WriteString(fmt.Sprintf("\n%d points", item.Score))
	if item.Kind == reddit.Post {
		b.WriteString(fmt.Sprintf("  •  %d comments", item.NumComments))
	}
	b.WriteString("\narchive: " + strings.Join(item.Sources, " + "))
	b.WriteString("\n\n")
	if item.Body == "" {
		b.WriteString(mutedStyle.Render("(no text content)"))
	} else {
		b.WriteString(wrapText(item.Body, max(10, width-2)))
	}
	if link := item.RedditURL(); link != "" {
		b.WriteString("\n\n" + mutedStyle.Render("Reddit URL:") + "\n" + wrapText(link, max(10, width-2)))
	}
	if item.URL != "" && item.URL != item.RedditURL() {
		b.WriteString("\n\n" + mutedStyle.Render("Linked URL:") + "\n" + wrapText(item.URL, max(10, width-2)))
	}
	return b.String()
}

func center(width, height int, content string) string {
	return lipgloss.Place(max(1, width), max(1, height), lipgloss.Center, lipgloss.Center, content)
}

func clean(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.Join(strings.Fields(value), " ")
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if utf8.RuneCountInString(value) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}
	runes := []rune(value)
	return string(runes[:width-1]) + "…"
}

func wrapText(value string, width int) string {
	if width <= 0 || value == "" {
		return value
	}
	lines := strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n")
	wrapped := make([]string, 0, len(lines))
	for _, line := range lines {
		runes := []rune(line)
		for len(runes) > width {
			cut := width
			for i := width; i > 0; i-- {
				if runes[i-1] == ' ' || runes[i-1] == '\t' {
					cut = i - 1
					break
				}
			}
			if cut == 0 {
				cut = width
			}
			wrapped = append(wrapped, strings.TrimRight(string(runes[:cut]), " \t"))
			runes = runes[cut:]
			for len(runes) > 0 && (runes[0] == ' ' || runes[0] == '\t') {
				runes = runes[1:]
			}
		}
		wrapped = append(wrapped, string(runes))
	}
	return strings.Join(wrapped, "\n")
}

func fallback(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}
