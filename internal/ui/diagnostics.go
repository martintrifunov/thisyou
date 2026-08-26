package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/martintrifunov/thisyou/internal/reddit"
)

func (m *Model) openDiagnostics() {
	m.screen = diagnosticsScreen
	m.resizeDiagnostics()
	m.diagnostics.GotoTop()
}

func renderDiagnostics(username, run string, posts, comments []reddit.Item, status map[string]reddit.Progress, errors, warnings []error, width int) string {
	textWidth := max(1, width)
	var b strings.Builder
	b.WriteString(titleStyle.Render(truncate("u/"+username, textWidth)))
	b.WriteString("\n" + mutedStyle.Render(wrapText("Last operation: "+fallback(run, "no network crawl in this session"), textWidth)))
	b.WriteString("\n" + wrapText(fmt.Sprintf("Merged history: %d posts  •  %d comments  •  %d total", len(posts), len(comments), len(posts)+len(comments)), textWidth))

	b.WriteString("\n\n" + logoStyle.Render("RETAINED SOURCE COVERAGE"))
	coverage := sourceCoverage(posts, comments)
	for _, source := range []string{"Arctic Shift", "PullPush"} {
		counts := coverage[source]
		b.WriteString("\n" + wrapText(fmt.Sprintf("%-12s  %d posts  •  %d comments", source, counts[reddit.Post], counts[reddit.Comment]), textWidth))
	}

	b.WriteString("\n\n" + logoStyle.Render("ARCHIVE STREAMS"))
	keys := make([]string, 0, len(status))
	for key := range status {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		b.WriteString("\n" + mutedStyle.Render(wrapText("No stream activity is available. Cached history was loaded without contacting the archives.", textWidth)))
	}
	for _, key := range keys {
		progress := status[key]
		state := "in progress"
		if progress.Done {
			state = "complete"
		}
		if progress.Err != nil {
			state = "unavailable"
		}
		line := fmt.Sprintf("%s %ss  •  %s  •  %d page(s)  •  %d fetched", progress.Source, progress.Kind, state, progress.Page, progress.Total)
		if progress.Skipped > 0 {
			line += fmt.Sprintf("  •  %d skipped", progress.Skipped)
		}
		style := mutedStyle
		if progress.Err != nil {
			style = errorStyle
		}
		b.WriteString("\n" + style.Render(wrapText(line, textWidth)))
	}

	if len(errors) > 0 {
		b.WriteString("\n\n" + errorStyle.Render("PARTIAL FAILURES"))
		for _, err := range errors {
			b.WriteString("\n" + errorStyle.Render(wrapText("• "+err.Error(), textWidth)))
		}
	}
	if len(warnings) > 0 {
		b.WriteString("\n\n" + errorStyle.Render("DATA WARNINGS"))
		for _, warning := range warnings {
			b.WriteString("\n" + errorStyle.Render(wrapText("• "+warning.Error(), textWidth)))
		}
	}
	return b.String()
}

func sourceCoverage(posts, comments []reddit.Item) map[string]map[reddit.Kind]int {
	coverage := make(map[string]map[reddit.Kind]int)
	for _, group := range []struct {
		kind  reddit.Kind
		items []reddit.Item
	}{{reddit.Post, posts}, {reddit.Comment, comments}} {
		for _, item := range group.items {
			for _, source := range item.Sources {
				if coverage[source] == nil {
					coverage[source] = make(map[reddit.Kind]int)
				}
				coverage[source][group.kind]++
			}
		}
	}
	return coverage
}
