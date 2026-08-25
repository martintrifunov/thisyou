package ui

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/maco/thisyou/internal/bookmarks"
	"github.com/maco/thisyou/internal/historycache"
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
	summaryScreen
	bookmarksScreen
	diagnosticsScreen
	compareInputScreen
	compareLoadingScreen
	comparisonScreen
	helpScreen
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
	border     = lipgloss.Color("#343536")

	logoStyle     = lipgloss.NewStyle().Foreground(orange).Bold(true)
	titleStyle    = lipgloss.NewStyle().Foreground(foreground).Bold(true)
	mutedStyle    = lipgloss.NewStyle().Foreground(muted)
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff6b6b"))
	linkStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#4da6ff")).Underline(true)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Background(lipgloss.Color("#272729"))
	selectedBody  = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Bold(true)
	markerStyle   = lipgloss.NewStyle().Background(orange)
)

type searchEvent struct {
	id        uint64
	progress  *reddit.Progress
	result    *searchResult
	err       error
	cacheErr  error
	cachedAt  time.Time
	fromCache bool
	refreshed bool
	added     int
}

type searchResult = reddit.Result

type Model struct {
	client   *reddit.Client
	openURL  func(string) error
	copyText func(string) error
	store    bookmarkStore
	history  historyStore

	screen       screen
	width        int
	height       int
	username     string
	input        textinput.Model
	filter       textinput.Model
	compareInput textinput.Model
	spinner      spinner.Model
	spinning     bool
	loadingCache bool
	refreshing   bool
	detail       viewport.Model
	summary      viewport.Model
	diagnostics  viewport.Model
	comparison   viewport.Model
	help         viewport.Model
	helpReturn   screen

	posts    []reddit.Item
	comments []reddit.Item
	visible  []reddit.Item
	tab      tab
	ordering sortMode
	selected int
	offset   int

	saved            []bookmarks.Entry
	bookmarkIDs      map[string]struct{}
	bookmarkSelected int
	bookmarkOffset   int
	bookmarkReturn   screen
	detailReturn     screen
	bookmarksLoading bool
	bookmarkRequest  uint64

	filtering       bool
	filterErr       string
	status          map[string]reddit.Progress
	errors          []error
	warnings        []error
	searchErr       string
	events          chan searchEvent
	cancel          context.CancelFunc
	searchID        uint64
	inspected       *reddit.Item
	copying         bool
	notice          string
	noticeError     bool
	fromCache       bool
	cachedAt        time.Time
	refreshBase     searchResult
	diagnosticRun   string
	refreshStatus   map[string]reddit.Progress
	refreshRun      string
	refreshCached   bool
	refreshAt       time.Time
	compareUsername string
	compareResult   reddit.Result
	compareErr      string
	compareStatus   map[string]reddit.Progress
	compareEvents   chan compareEvent
	compareCancel   context.CancelFunc
	compareID       uint64
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
	filter.Placeholder = "text or sub:golang after:2025-01-01 min:10"
	filter.Prompt = "/ "
	filter.CharLimit = 100
	filter.PromptStyle = lipgloss.NewStyle().Foreground(orange)
	filter.TextStyle = lipgloss.NewStyle().Foreground(foreground)

	compareInput := textinput.New()
	compareInput.Placeholder = "second redditor username"
	compareInput.Prompt = "u/"
	compareInput.CharLimit = 80
	compareInput.Width = 42
	compareInput.PromptStyle = lipgloss.NewStyle().Foreground(orange).Bold(true)
	compareInput.TextStyle = lipgloss.NewStyle().Foreground(foreground)

	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = lipgloss.NewStyle().Foreground(orange)

	return &Model{
		client:        client,
		openURL:       openInBrowser,
		copyText:      copyToClipboard,
		store:         bookmarks.NewStore(),
		history:       historycache.NewStore(),
		screen:        searchScreen,
		input:         in,
		filter:        filter,
		compareInput:  compareInput,
		spinner:       spin,
		detail:        viewport.New(80, 20),
		summary:       viewport.New(80, 20),
		diagnostics:   viewport.New(80, 20),
		comparison:    viewport.New(80, 20),
		help:          viewport.New(80, 20),
		status:        make(map[string]reddit.Progress),
		bookmarkIDs:   make(map[string]struct{}),
		compareStatus: make(map[string]reddit.Progress),
		detailReturn:  resultsScreen,
	}
}

func (m *Model) Init() tea.Cmd { return tea.Batch(textinput.Blink, m.loadBookmarks(true)) }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.Width = max(4, min(42, msg.Width-12))
		m.filter.Width = max(4, msg.Width-10)
		m.compareInput.Width = max(4, min(42, msg.Width-12))
		m.resizeDetail()
		m.resizeSummary()
		m.resizeDiagnostics()
		m.resizeComparison()
		m.resizeHelp()
		m.ensureVisible()
		return m, nil
	case searchEvent:
		if msg.id != m.searchID || m.screen != loadingScreen {
			return m, nil
		}
		if msg.progress != nil {
			key := string(msg.progress.Kind) + ":" + msg.progress.Source
			m.status[key] = *msg.progress
			return m, waitForEvent(m.events)
		}
		if msg.err != nil {
			m.screen = searchScreen
			m.loadingCache = false
			m.refreshing = false
			m.events = nil
			m.cancel = nil
			m.searchErr = msg.err.Error()
			m.input.Focus()
			return m, textinput.Blink
		}
		if msg.result != nil {
			m.posts = msg.result.Posts
			m.comments = msg.result.Comments
			m.errors = msg.result.Errors
			m.warnings = msg.result.Warnings
			m.screen = resultsScreen
			m.loadingCache = false
			m.refreshing = false
			m.fromCache = msg.fromCache
			m.cachedAt = msg.cachedAt
			m.events = nil
			if m.cancel != nil {
				m.cancel()
			}
			m.cancel = nil
			m.refreshVisible()
			if msg.refreshed {
				m.notice = fmt.Sprintf("refresh complete: %d new record(s)", msg.added)
				m.noticeError = false
			}
			if msg.cacheErr != nil {
				m.setActionError("cache failed: " + msg.cacheErr.Error())
			}
			return m, nil
		}
	case compareEvent:
		return m, m.handleCompareEvent(msg)
	case actionMsg:
		m.copying = false
		if msg.err != nil {
			m.setActionError(actionError(msg.err))
		} else {
			m.notice = msg.success
			m.noticeError = false
		}
		return m, nil
	case bookmarkMsg:
		if msg.id != m.bookmarkRequest {
			return m, nil
		}
		m.bookmarksLoading = false
		if msg.err != nil {
			if !msg.silent {
				m.setActionError(actionError(msg.err))
			}
		} else {
			m.replaceBookmarks(msg.entries)
			if msg.notice != "" {
				m.notice = msg.notice
				m.noticeError = false
			}
		}
		return m, nil
	case spinner.TickMsg:
		if m.screen == loadingScreen || m.screen == compareLoadingScreen {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		m.spinning = false
	}

	key, ok := msg.(tea.KeyMsg)
	if ok && key.String() == "ctrl+c" {
		if m.cancel != nil {
			m.cancel()
		}
		if m.compareCancel != nil {
			m.compareCancel()
		}
		return m, tea.Quit
	}
	if ok && key.String() == "ctrl+b" && (m.screen == searchScreen || m.screen == resultsScreen) {
		return m, m.openBookmarks()
	}
	if ok && key.String() == "?" && m.screen != loadingScreen && m.screen != compareLoadingScreen && m.screen != helpScreen {
		m.openHelp()
		return m, nil
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
	case summaryScreen:
		return m.updateSummary(msg)
	case bookmarksScreen:
		return m.updateBookmarks(msg)
	case diagnosticsScreen:
		return m.updateDiagnosticsScreen(msg)
	case compareInputScreen:
		return m.updateCompareInput(msg)
	case compareLoadingScreen:
		return m.updateCompareLoading(msg)
	case comparisonScreen:
		return m.updateComparison(msg)
	case helpScreen:
		return m.updateHelp(msg)
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
		case "ctrl+o":
			username, err := reddit.NormalizeUsername(m.input.Value())
			if err != nil {
				m.searchErr = err.Error()
				return m, nil
			}
			m.username = username
			m.searchErr = ""
			return m, m.beginCachedSearch()
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
		m.searchID++
		m.events = nil
		m.cancel = nil
		if m.refreshing {
			m.posts = m.refreshBase.Posts
			m.comments = m.refreshBase.Comments
			m.errors = m.refreshBase.Errors
			m.warnings = m.refreshBase.Warnings
			m.status = m.refreshStatus
			m.diagnosticRun = m.refreshRun
			m.fromCache = m.refreshCached
			m.cachedAt = m.refreshAt
			m.refreshing = false
			m.screen = resultsScreen
			m.refreshVisible()
			return m, nil
		}
		m.loadingCache = false
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
				if m.filterErr != "" {
					return m, nil
				}
				m.filtering = false
				m.filter.Blur()
				return m, nil
			case "esc":
				m.filtering = false
				m.filter.SetValue("")
				m.filter.Blur()
				m.filterErr = ""
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
	if m.copying {
		if len(m.visible) == 0 {
			m.copying = false
			m.setActionError("no record selected")
			return m, nil
		}
		return m, m.updateCopyMenu(key.String(), m.visible[m.selected])
	}
	if key.String() != "o" && key.String() != "y" {
		m.notice = ""
		m.noticeError = false
	}
	switch key.String() {
	case "q":
		return m, tea.Quit
	case "r":
		m.screen = searchScreen
		m.inspected = nil
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
		if len(m.visible) > 0 {
			m.selected = min(len(m.visible)-1, m.selected+m.pageRows())
			m.ensureVisible()
		}
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
	case "o":
		if len(m.visible) > 0 {
			return m, m.openItem(m.visible[m.selected])
		}
		m.setActionError("no record selected")
	case "y":
		if len(m.visible) > 0 {
			m.copying = true
			m.notice = ""
			m.noticeError = false
		} else {
			m.setActionError("no record selected")
		}
	case "s":
		m.openSummary()
	case "S":
		m.ordering = m.ordering.next()
		m.refreshVisible()
		m.notice = "sort: " + strings.ToLower(m.ordering.label())
		m.noticeError = false
	case "R":
		return m, m.beginRefresh()
	case "d":
		m.openDiagnostics()
	case "C":
		return m, m.openComparisonInput()
	case "B":
		return m, m.openBookmarks()
	case "b":
		if len(m.visible) > 0 {
			return m, m.toggleBookmark(m.visible[m.selected])
		}
		m.setActionError("no record selected")
	}
	return m, nil
}

func (m *Model) updateDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if m.copying && m.inspected != nil {
			return m, m.updateCopyMenu(key.String(), *m.inspected)
		}
		if key.String() != "o" && key.String() != "y" {
			m.notice = ""
			m.noticeError = false
		}
		switch key.String() {
		case "esc", "backspace", "q":
			m.screen = m.detailReturn
			m.inspected = nil
			m.copying = false
			return m, nil
		case "o":
			if m.inspected != nil {
				return m, m.openItem(*m.inspected)
			}
		case "y":
			if m.inspected != nil {
				m.copying = true
				m.notice = ""
				m.noticeError = false
				return m, nil
			}
		case "b":
			if m.inspected != nil {
				return m, m.toggleBookmark(*m.inspected)
			}
		}
	}
	var cmd tea.Cmd
	m.detail, cmd = m.detail.Update(msg)
	return m, cmd
}

func (m *Model) updateSummary(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc", "backspace", "q":
			m.screen = resultsScreen
			return m, nil
		case "g", "home":
			m.summary.GotoTop()
			return m, nil
		case "G", "end":
			m.summary.GotoBottom()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.summary, cmd = m.summary.Update(msg)
	return m, cmd
}

func (m *Model) updateDiagnosticsScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc", "backspace", "q", "d":
			m.screen = resultsScreen
			return m, nil
		case "g", "home":
			m.diagnostics.GotoTop()
			return m, nil
		case "G", "end":
			m.diagnostics.GotoBottom()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.diagnostics, cmd = m.diagnostics.Update(msg)
	return m, cmd
}

func (m *Model) beginSearch() tea.Cmd {
	m.prepareSearch()
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.loadingCache = false
	m.diagnosticRun = "full crawl"
	searchID := m.searchID
	m.events = make(chan searchEvent, 128)
	ch := m.events
	username := m.username
	client := m.client
	store := m.history
	go func() {
		defer close(ch)
		result := client.SearchAll(ctx, username, func(p reddit.Progress) {
			copy := p
			select {
			case ch <- searchEvent{id: searchID, progress: &copy}:
			case <-ctx.Done():
			}
		})
		var cacheErr error
		if ctx.Err() == nil {
			cacheErr = cacheSnapshot(store, username, result)
		}
		select {
		case ch <- searchEvent{id: searchID, result: &result, cacheErr: cacheErr}:
		case <-ctx.Done():
		}
	}()
	commands := []tea.Cmd{waitForEvent(ch)}
	if !m.spinning {
		m.spinning = true
		commands = append(commands, m.spinner.Tick)
	}
	return tea.Batch(commands...)
}

func (m *Model) beginRefresh() tea.Cmd {
	base := searchResult{
		Posts:    append([]reddit.Item(nil), m.posts...),
		Comments: append([]reddit.Item(nil), m.comments...),
		Errors:   append([]error(nil), m.errors...),
		Warnings: append([]error(nil), m.warnings...),
	}
	previousStatus := cloneStatus(m.status)
	previousRun := m.diagnosticRun
	previousCached := m.fromCache
	previousAt := m.cachedAt
	postsAfter := latestTimestamp(base.Posts)
	commentsAfter := latestTimestamp(base.Comments)
	m.prepareSearch()
	m.refreshing = true
	m.refreshBase = base
	m.refreshStatus = previousStatus
	m.refreshRun = previousRun
	m.refreshCached = previousCached
	m.refreshAt = previousAt
	m.diagnosticRun = "incremental refresh"
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	searchID := m.searchID
	m.events = make(chan searchEvent, 128)
	ch := m.events
	username := m.username
	client := m.client
	store := m.history
	go func() {
		defer close(ch)
		fresh := client.SearchAfter(ctx, username, postsAfter, commentsAfter, func(p reddit.Progress) {
			copy := p
			select {
			case ch <- searchEvent{id: searchID, progress: &copy}:
			case <-ctx.Done():
			}
		})
		merged := reddit.MergeResults(base, fresh)
		added := len(merged.Posts) + len(merged.Comments) - len(base.Posts) - len(base.Comments)
		var cacheErr error
		if ctx.Err() == nil {
			cacheErr = cacheSnapshot(store, username, merged)
		}
		select {
		case ch <- searchEvent{id: searchID, result: &merged, cacheErr: cacheErr, refreshed: true, added: added}:
		case <-ctx.Done():
		}
	}()
	commands := []tea.Cmd{waitForEvent(ch)}
	if !m.spinning {
		m.spinning = true
		commands = append(commands, m.spinner.Tick)
	}
	return tea.Batch(commands...)
}

func latestTimestamp(items []reddit.Item) int64 {
	var latest int64
	for _, item := range items {
		if item.CreatedUTC > latest {
			latest = item.CreatedUTC
		}
	}
	return latest
}

func cloneStatus(status map[string]reddit.Progress) map[string]reddit.Progress {
	cloned := make(map[string]reddit.Progress, len(status))
	for key, value := range status {
		cloned[key] = value
	}
	return cloned
}

func (m *Model) prepareSearch() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.searchID++
	m.screen = loadingScreen
	m.input.Blur()
	m.status = make(map[string]reddit.Progress)
	m.errors = nil
	m.warnings = nil
	m.posts, m.comments, m.visible = nil, nil, nil
	m.inspected = nil
	m.copying = false
	m.notice = ""
	m.noticeError = false
	m.filtering = false
	m.filterErr = ""
	m.filter.SetValue("")
	m.filter.Blur()
	m.tab = allTab
	m.ordering = newestFirst
	m.selected, m.offset = 0, 0
	m.fromCache = false
	m.cachedAt = time.Time{}
	m.refreshing = false
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
		source = append([]reddit.Item(nil), m.posts...)
	case commentsTab:
		source = append([]reddit.Item(nil), m.comments...)
	default:
		source = make([]reddit.Item, 0, len(m.posts)+len(m.comments))
		source = append(source, m.posts...)
		source = append(source, m.comments...)
	}
	filter, err := parseResultFilter(m.filter.Value())
	if err != nil {
		m.filterErr = err.Error()
		m.visible = nil
	} else if strings.TrimSpace(m.filter.Value()) == "" {
		m.filterErr = ""
		m.visible = source
	} else {
		m.filterErr = ""
		m.visible = make([]reddit.Item, 0, len(source))
		for _, item := range source {
			if filter.matches(item) {
				m.visible = append(m.visible, item)
			}
		}
	}
	sortItems(m.visible, m.ordering)
	m.selected, m.offset = 0, 0
}

func (m *Model) ensureVisible() {
	if len(m.visible) == 0 {
		m.selected, m.offset = 0, 0
		return
	}
	m.selected = min(len(m.visible)-1, max(0, m.selected))
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

func (m *Model) pageRows() int {
	chrome := 5
	if m.filtering || m.filter.Value() != "" {
		chrome++
	}
	if m.filterErr != "" {
		chrome++
	}
	if len(m.errors) > 0 {
		chrome++
	}
	if len(m.warnings) > 0 {
		chrome++
	}
	return max(1, (m.height-chrome)/2)
}

func (m *Model) openDetail(item reddit.Item) {
	m.detailReturn = m.screen
	m.screen = detailScreen
	m.inspected = &item
	m.copying = false
	m.notice = ""
	m.noticeError = false
	m.resizeDetail()
	m.detail.GotoTop()
}

func (m *Model) openSummary() {
	m.screen = summaryScreen
	m.resizeSummary()
	m.summary.GotoTop()
}

func (m *Model) resizeDetail() {
	width := max(1, m.width-4)
	height := max(1, m.height-4)
	offset := m.detail.YOffset
	m.detail.Width = width
	m.detail.Height = height
	if m.inspected != nil {
		m.detail.SetContent(renderDetail(*m.inspected, width))
		m.detail.SetYOffset(offset)
	}
}

func (m *Model) resizeSummary() {
	width := max(1, m.width-4)
	height := max(1, m.height-4)
	offset := m.summary.YOffset
	m.summary.Width = width
	m.summary.Height = height
	m.summary.SetContent(renderSummary(m.username, summarizeProfile(m.posts, m.comments), len(m.errors), len(m.warnings), width))
	m.summary.SetYOffset(offset)
}

func (m *Model) resizeDiagnostics() {
	width := max(1, m.width-4)
	height := max(1, m.height-4)
	offset := m.diagnostics.YOffset
	m.diagnostics.Width = width
	m.diagnostics.Height = height
	m.diagnostics.SetContent(renderDiagnostics(m.username, m.diagnosticRun, m.posts, m.comments, m.status, m.errors, m.warnings, width))
	m.diagnostics.SetYOffset(offset)
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
	case summaryScreen:
		return m.viewSummary()
	case bookmarksScreen:
		return m.viewBookmarks()
	case diagnosticsScreen:
		return m.viewDiagnostics()
	case compareInputScreen:
		return m.viewCompareInput()
	case compareLoadingScreen:
		return m.viewCompareLoading()
	case comparisonScreen:
		return m.viewComparison()
	case helpScreen:
		return m.viewHelp()
	default:
		return ""
	}
}

func (m *Model) viewSearch() string {
	var b strings.Builder
	if m.width >= 76 && m.height >= 20 {
		b.WriteString(logoStyle.Render(strings.TrimPrefix(logo, "\n")))
	} else {
		b.WriteString(logoStyle.Render("THIS YOU?"))
	}
	gap := "\n\n"
	if m.height < 14 {
		gap = "\n"
	}
	b.WriteString(gap)
	contentWidth := max(4, min(58, m.width-4))
	b.WriteString(titleStyle.Render(wrapText("A Reddit archive history viewer", contentWidth)))
	if m.height >= 15 {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render(wrapText("Search public Arctic Shift + PullPush archive records", contentWidth)))
	}
	b.WriteString(gap)
	b.WriteString(m.renderUsernameInput())
	if m.searchErr != "" {
		b.WriteString("\n" + errorStyle.Render(wrapText(m.searchErr, contentWidth)))
	}
	help := "enter search  •  ctrl+o offline  •  ? help  •  ctrl+c quit"
	if m.width < 30 {
		help = "enter  •  ^o cache  •  ? help  •  ^c quit"
	}
	b.WriteString(gap + mutedStyle.Render(wrapText(help, contentWidth)))
	return center(m.width, m.height, b.String())
}

func (m *Model) viewLoading() string {
	var b strings.Builder
	b.WriteString(logoStyle.Render("THIS YOU?"))
	gap := "\n\n"
	if m.height < 16 {
		gap = "\n"
	}
	b.WriteString(gap)
	contentWidth := max(4, min(72, m.width-4))
	activity := "Crawling u/" + m.username
	if m.loadingCache {
		activity = "Loading cached history for u/" + m.username
	} else if m.refreshing {
		activity = "Checking for new history from u/" + m.username
	}
	b.WriteString(m.spinner.View() + " " + titleStyle.Render(wrapText(activity, max(3, contentWidth-3))))
	if m.height >= 14 {
		description := "Fetching all available history from both archives."
		if m.loadingCache {
			description = "Reading the most recently saved crawl without network access."
		} else if m.refreshing {
			description = "Fetching only records newer than the current history."
		}
		if m.width >= 48 && m.height >= 18 {
			description = "Fetching the complete available history; each archive has an independent cursor."
			if m.loadingCache {
				description = "Reading the most recently saved crawl without contacting either archive."
			} else if m.refreshing {
				description = "Fetching only newer posts and comments with independent timestamp boundaries."
			}
		}
		b.WriteString("\n" + mutedStyle.Render(wrapText(description, contentWidth)))
	}
	b.WriteString(gap)
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
		label := p.Source + " " + string(p.Kind) + "s"
		line := fmt.Sprintf("%-28s %s", label, state)
		if m.width < 58 {
			line = label + "  " + state
		}
		b.WriteString(truncate(line, contentWidth) + "\n")
	}
	b.WriteString("\n" + mutedStyle.Render(truncate("esc cancel  •  ctrl+c quit", contentWidth)))
	return center(m.width, m.height, b.String())
}

func (m *Model) viewResults() string {
	var b strings.Builder
	contentWidth := max(4, m.width-4)
	headerText := fmt.Sprintf("THIS YOU?  /  u/%s  /  %s", m.username, m.ordering.label())
	if m.fromCache {
		headerText += "  /  CACHED " + m.cachedAt.Local().Format("2006-01-02 15:04")
	}
	header := truncate(headerText, contentWidth)
	b.WriteString(logoStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(m.renderTabs())
	b.WriteString("\n")
	if m.filtering || m.filter.Value() != "" {
		b.WriteString(m.renderFilterInput() + "\n")
	}
	if m.filterErr != "" {
		b.WriteString(errorStyle.Render(truncate(m.filterErr, contentWidth)) + "\n")
	}
	if len(m.visible) == 0 {
		b.WriteString("\n" + mutedStyle.Render(wrapText("No archived records found for this view.", contentWidth)) + "\n")
	} else {
		rows := m.pageRows()
		end := min(len(m.visible), m.offset+rows)
		for i := m.offset; i < end; i++ {
			b.WriteString(m.renderRow(m.visible[i], i == m.selected))
			b.WriteByte('\n')
		}
	}
	if len(m.errors) > 0 {
		message := fmt.Sprintf("Partial results: %d archive stream(s) unavailable", len(m.errors))
		b.WriteString(errorStyle.Render(truncate(message, contentWidth)) + "\n")
	}
	if len(m.warnings) > 0 {
		message := fmt.Sprintf("Data warning: %d archived record(s) skipped", len(m.warnings))
		b.WriteString(errorStyle.Render(truncate(message, contentWidth)) + "\n")
	}
	position := "0/0"
	if len(m.visible) > 0 {
		position = fmt.Sprintf("%d/%d", m.selected+1, len(m.visible))
	}
	footerText := fmt.Sprintf("%s  •  j/k move  •  enter inspect  •  / filter  •  ? help  •  q quit", position)
	if m.filtering {
		footerText = "enter apply filter  •  esc clear  •  ? help"
	}
	footer := truncate(m.actionFooter(footerText), max(10, m.width-4))
	body := strings.TrimRight(b.String(), "\n")
	innerHeight := max(1, m.height-2)
	spacer := max(0, innerHeight-lipgloss.Height(body)-lipgloss.Height(footer))
	content := body + strings.Repeat("\n", spacer) + "\n" + footer
	framed := lipgloss.NewStyle().Padding(1, 2).Render(content)
	return fillScreen(m.width, m.height, framed, lipgloss.Left, lipgloss.Top)
}

func (m *Model) renderTabs() string {
	var labels []string
	if m.width < 30 {
		labels = []string{"A", "P", "C"}
	} else if m.width < 45 {
		labels = []string{
			fmt.Sprintf("A %d", len(m.posts)+len(m.comments)),
			fmt.Sprintf("P %d", len(m.posts)),
			fmt.Sprintf("C %d", len(m.comments)),
		}
	} else {
		labels = []string{
			fmt.Sprintf("ALL %d", len(m.posts)+len(m.comments)),
			fmt.Sprintf("POSTS %d", len(m.posts)),
			fmt.Sprintf("COMMENTS %d", len(m.comments)),
		}
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

func (m *Model) renderUsernameInput() string {
	width := max(4, min(46, m.width-8))
	content := ansi.Truncate(m.input.View(), width, "")
	content = lipgloss.PlaceHorizontal(width, lipgloss.Left, content)
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).Padding(0, 1).Render(content)
}

func (m *Model) renderFilterInput() string {
	width := max(4, m.width-4)
	content := ansi.Truncate(m.filter.View(), width, "")
	return lipgloss.PlaceHorizontal(width, lipgloss.Left, content)
}

func (m *Model) renderRow(item reddit.Item, selected bool) string {
	return m.renderRecordRow(item, selected, false)
}

func (m *Model) renderBookmarkRow(item reddit.Item, selected bool) string {
	return m.renderRecordRow(item, selected, true)
}

func (m *Model) renderRecordRow(item reddit.Item, selected, showAuthor bool) string {
	rowWidth := max(4, m.width-4)
	contentWidth := max(1, rowWidth-1)
	kind := "COMMENT"
	headline := clean(item.Body)
	if item.Kind == reddit.Post {
		kind = "POST"
		headline = clean(item.Title)
		if headline == "" {
			headline = clean(item.Body)
		}
	}
	context := fmt.Sprintf("%-7s  r/%s", kind, fallback(item.Subreddit, "unknown"))
	if showAuthor {
		context = "u/" + fallback(item.Author, "unknown") + "  •  " + context
	}
	meta := fmt.Sprintf("%s  •  %s  •  %d points", context, item.Time().Local().Format("2006-01-02 15:04"), item.Score)
	body := truncate(headline, contentWidth)
	if contentWidth > 2 {
		body = "  " + truncate(headline, contentWidth-2)
	}
	bookmarked := m.isBookmarked(item)
	if selected {
		marker := markerStyle.Render(" ")
		if bookmarked {
			marker = logoStyle.Render("★")
		}
		metadata := selectedStyle.Width(contentWidth).Render(truncate(meta, contentWidth))
		return marker + metadata + "\n" + marker + selectedBody.Render(body)
	}
	content := truncate(meta, contentWidth) + "\n" + body
	style := lipgloss.NewStyle().Width(contentWidth).Height(2).Foreground(foreground)
	marker := " \n "
	if bookmarked {
		marker = logoStyle.Render("★\n★")
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, marker, style.Render(content))
}

func (m *Model) viewDetail() string {
	width := max(1, m.width-4)
	header := logoStyle.Render(truncate("THIS YOU?  /  RECORD", width))
	help := "j/k scroll  •  b save  •  o open  •  ? help  •  esc return"
	if width < 48 {
		help = "j/k  •  ? help  •  esc"
	}
	footer := truncate(m.actionFooter(help), width)
	content := lipgloss.JoinVertical(lipgloss.Left, header, m.detail.View(), footer)
	framed := lipgloss.NewStyle().Padding(1, 2).Render(content)
	return fillScreen(m.width, m.height, framed, lipgloss.Left, lipgloss.Top)
}

func (m *Model) viewSummary() string {
	width := max(1, m.width-4)
	header := logoStyle.Render(truncate("THIS YOU?  /  PROFILE SUMMARY", width))
	help := "j/k scroll  •  ? help  •  esc return"
	if width < 48 {
		help = "j/k  •  ? help  •  esc"
	}
	footer := mutedStyle.Render(truncate(help, width))
	content := lipgloss.JoinVertical(lipgloss.Left, header, m.summary.View(), footer)
	framed := lipgloss.NewStyle().Padding(1, 2).Render(content)
	return fillScreen(m.width, m.height, framed, lipgloss.Left, lipgloss.Top)
}

func (m *Model) viewDiagnostics() string {
	width := max(1, m.width-4)
	header := logoStyle.Render(truncate("THIS YOU?  /  ARCHIVE DIAGNOSTICS", width))
	help := "j/k scroll  •  ? help  •  esc return"
	if width < 48 {
		help = "j/k  •  ? help  •  esc"
	}
	footer := mutedStyle.Render(truncate(help, width))
	content := lipgloss.JoinVertical(lipgloss.Left, header, m.diagnostics.View(), footer)
	framed := lipgloss.NewStyle().Padding(1, 2).Render(content)
	return fillScreen(m.width, m.height, framed, lipgloss.Left, lipgloss.Top)
}

func renderDetail(item reddit.Item, width int) string {
	var b strings.Builder
	textWidth := max(1, width)
	typeName := strings.ToUpper(string(item.Kind))
	typeLabel := logoStyle.Render(truncate(typeName, textWidth))
	b.WriteString(typeLabel)
	if item.Over18 {
		if remaining := textWidth - lipgloss.Width(typeLabel) - 2; remaining >= 4 {
			b.WriteString("  " + errorStyle.Render("NSFW"))
		} else {
			b.WriteString("\n" + errorStyle.Render(truncate("NSFW", textWidth)))
		}
	}
	b.WriteString("\n")
	if item.Title != "" {
		b.WriteString(titleStyle.Render(wrapText(item.Title, textWidth)) + "\n\n")
	}
	metadata := fmt.Sprintf("u/%s in r/%s", fallback(item.Author, "unknown"), fallback(item.Subreddit, "unknown"))
	b.WriteString(mutedStyle.Render(wrapText(metadata, textWidth)))
	b.WriteString("\n" + mutedStyle.Render(wrapText(item.Time().Local().Format(time.RFC1123), textWidth)))
	activity := fmt.Sprintf("%d points", item.Score)
	if item.Kind == reddit.Post {
		activity += fmt.Sprintf("  •  %d comments", item.NumComments)
	}
	b.WriteString("\n" + wrapText(activity, textWidth))
	b.WriteString("\n" + wrapText("archive: "+strings.Join(item.Sources, " + "), textWidth))
	b.WriteString("\n\n")
	if item.Body == "" {
		b.WriteString(mutedStyle.Render("(no text content)"))
	} else {
		b.WriteString(wrapText(item.Body, textWidth))
	}
	if link := item.RedditURL(); link != "" {
		b.WriteString("\n\n" + mutedStyle.Render("Reddit URL:") + "\n" + renderLink(link, textWidth))
	}
	if item.URL != "" && item.URL != item.RedditURL() {
		b.WriteString("\n\n" + mutedStyle.Render("Linked URL:") + "\n" + renderLink(item.URL, textWidth))
	}
	return b.String()
}

func renderLink(value string, width int) string {
	target, ok := safeWebURL(value)
	if !ok {
		value = strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return -1
			}
			return r
		}, value)
		return wrapText(value, width)
	}
	link := ansi.SetHyperlink(target) + linkStyle.Render(target) + ansi.ResetHyperlink()
	return wrapText(link, width)
}

func safeWebURL(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" {
		return "", false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return parsed.String(), true
	default:
		return "", false
	}
}

func center(width, height int, content string) string {
	width, height = max(1, width), max(1, height)
	vertical := lipgloss.Center
	if lipgloss.Height(content) >= height {
		vertical = lipgloss.Top
	}
	return fillScreen(width, height, content, lipgloss.Center, vertical)
}

func fillScreen(width, height int, content string, horizontal, vertical lipgloss.Position) string {
	return lipgloss.Place(max(1, width), max(1, height), horizontal, vertical, content)
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
	return ansi.Truncate(value, width, "…")
}

func wrapText(value string, width int) string {
	if width <= 0 || value == "" {
		return value
	}
	return ansi.Wrap(strings.ReplaceAll(value, "\r\n", "\n"), width, " \t")
}

func fallback(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}
