package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/martintrifunov/thisyou/internal/reddit"
)

type profileSummary struct {
	Posts               int
	Comments            int
	TotalScore          int
	PostScore           int
	CommentScore        int
	AverageScore        float64
	ActiveDays          int
	BusiestDay          string
	BusiestCount        int
	BusiestHour         int
	BusiestHourCount    int
	BusiestWeekday      time.Weekday
	BusiestWeekdayCount int
	UniqueSubreddits    int
	NSFWRecords         int
	Earliest            time.Time
	Latest              time.Time
	HasDateRange        bool
	TopSubreddits       []subredditSummary
}

type subredditSummary struct {
	Name         string
	Posts        int
	Comments     int
	Score        int
	Share        float64
	AverageScore float64
}

func (s profileSummary) totalRecords() int {
	return s.Posts + s.Comments
}

func (s subredditSummary) totalRecords() int {
	return s.Posts + s.Comments
}

func summarizeProfile(posts, comments []reddit.Item) profileSummary {
	summary := profileSummary{Posts: len(posts), Comments: len(comments)}
	days := make(map[string]int)
	hours := make(map[int]int)
	weekdays := make(map[time.Weekday]int)
	subreddits := make(map[string]subredditSummary)

	consume := func(items []reddit.Item, isPost bool) {
		for _, item := range items {
			summary.TotalScore += item.Score
			if isPost {
				summary.PostScore += item.Score
			} else {
				summary.CommentScore += item.Score
			}

			if item.CreatedUTC > 0 {
				created := item.Time().Local()
				day := created.Format("2006-01-02")
				days[day]++
				hours[created.Hour()]++
				weekdays[created.Weekday()]++
				if !summary.HasDateRange || created.Before(summary.Earliest) {
					summary.Earliest = created
				}
				if !summary.HasDateRange || created.After(summary.Latest) {
					summary.Latest = created
				}
				summary.HasDateRange = true
			}
			if item.Over18 {
				summary.NSFWRecords++
			}

			displayName := strings.TrimSpace(item.Subreddit)
			if displayName == "" {
				continue
			}
			key := strings.ToLower(displayName)
			entry := subreddits[key]
			if entry.Name == "" {
				entry.Name = displayName
			}
			entry.Score += item.Score
			if isPost {
				entry.Posts++
			} else {
				entry.Comments++
			}
			subreddits[key] = entry
		}
	}

	consume(posts, true)
	consume(comments, false)
	summary.UniqueSubreddits = len(subreddits)
	if total := summary.totalRecords(); total > 0 {
		summary.AverageScore = float64(summary.TotalScore) / float64(total)
	}

	orderedDays := make([]string, 0, len(days))
	for day := range days {
		orderedDays = append(orderedDays, day)
	}
	sort.Strings(orderedDays)
	summary.ActiveDays = len(orderedDays)
	for _, day := range orderedDays {
		if days[day] > summary.BusiestCount {
			summary.BusiestDay = day
			summary.BusiestCount = days[day]
		}
	}
	for hour := 0; hour < 24; hour++ {
		if hours[hour] > summary.BusiestHourCount {
			summary.BusiestHour = hour
			summary.BusiestHourCount = hours[hour]
		}
	}
	for weekday := time.Sunday; weekday <= time.Saturday; weekday++ {
		if weekdays[weekday] > summary.BusiestWeekdayCount {
			summary.BusiestWeekday = weekday
			summary.BusiestWeekdayCount = weekdays[weekday]
		}
	}

	summary.TopSubreddits = make([]subredditSummary, 0, len(subreddits))
	for _, entry := range subreddits {
		entry.Share = float64(entry.totalRecords()) / float64(max(1, summary.totalRecords())) * 100
		entry.AverageScore = float64(entry.Score) / float64(max(1, entry.totalRecords()))
		summary.TopSubreddits = append(summary.TopSubreddits, entry)
	}
	sort.Slice(summary.TopSubreddits, func(i, j int) bool {
		left, right := summary.TopSubreddits[i], summary.TopSubreddits[j]
		if left.totalRecords() != right.totalRecords() {
			return left.totalRecords() > right.totalRecords()
		}
		if left.Score != right.Score {
			return left.Score > right.Score
		}
		return strings.ToLower(left.Name) < strings.ToLower(right.Name)
	})
	if len(summary.TopSubreddits) > 10 {
		summary.TopSubreddits = summary.TopSubreddits[:10]
	}
	return summary
}

func renderSummary(username string, summary profileSummary, unavailable, skipped, width int) string {
	width = max(1, width)
	var b strings.Builder
	b.WriteString(titleStyle.Render(truncate("u/"+fallback(username, "unknown"), width)))
	if unavailable > 0 {
		message := fmt.Sprintf("Partial summary: %d archive stream(s) unavailable", unavailable)
		b.WriteString("\n" + errorStyle.Render(wrapText(message, width)))
	}
	if skipped > 0 {
		message := fmt.Sprintf("Data warning: %d archived record(s) skipped", skipped)
		b.WriteString("\n" + errorStyle.Render(wrapText(message, width)))
	}
	b.WriteString("\n\n" + logoStyle.Render(truncate("ARCHIVED ACTIVITY", width)) + "\n")
	b.WriteString(wrapText(fmt.Sprintf("%d records  •  %d posts  •  %d comments", summary.totalRecords(), summary.Posts, summary.Comments), width))
	if summary.HasDateRange {
		coverage := fmt.Sprintf("Coverage: %s — %s", summary.Earliest.Format("Jan 2, 2006"), summary.Latest.Format("Jan 2, 2006"))
		b.WriteString("\n" + wrapText(coverage, width))
		activity := fmt.Sprintf("%d active days", summary.ActiveDays)
		if summary.BusiestDay != "" {
			activity += fmt.Sprintf("  •  busiest %s (%d records)", summary.BusiestDay, summary.BusiestCount)
		}
		b.WriteString("\n" + wrapText(activity, width))
	} else {
		b.WriteString("\n" + mutedStyle.Render(wrapText("No activity dates available.", width)))
	}

	b.WriteString("\n\n" + logoStyle.Render(truncate("SCORE SNAPSHOT", width)) + "\n")
	b.WriteString(wrapText(fmt.Sprintf("%d combined  •  %.1f average per record", summary.TotalScore, summary.AverageScore), width))
	b.WriteString("\n" + wrapText(fmt.Sprintf("%d from posts  •  %d from comments", summary.PostScore, summary.CommentScore), width))
	b.WriteString("\n" + mutedStyle.Render(wrapText("Archive scores are snapshots, not current Reddit profile karma.", width)))

	b.WriteString("\n\n" + logoStyle.Render(truncate("ACTIVITY PATTERNS", width)) + "\n")
	postShare := float64(summary.Posts) / float64(max(1, summary.totalRecords())) * 100
	commentShare := float64(summary.Comments) / float64(max(1, summary.totalRecords())) * 100
	b.WriteString(wrapText(fmt.Sprintf("%.0f%% posts  •  %.0f%% comments  •  %d communities", postShare, commentShare, summary.UniqueSubreddits), width))
	if summary.BusiestHourCount > 0 {
		peak := fmt.Sprintf("Peak hour: %02d:00–%02d:59 (%d records)", summary.BusiestHour, summary.BusiestHour, summary.BusiestHourCount)
		if summary.BusiestWeekdayCount > 0 {
			peak += fmt.Sprintf("  •  peak weekday: %s (%d)", summary.BusiestWeekday, summary.BusiestWeekdayCount)
		}
		b.WriteString("\n" + wrapText(peak, width))
	}
	if summary.NSFWRecords > 0 {
		b.WriteString("\n" + mutedStyle.Render(wrapText(fmt.Sprintf("%d record(s) marked NSFW", summary.NSFWRecords), width)))
	}

	b.WriteString("\n\n" + logoStyle.Render(truncate("TOP SUBREDDITS", width)))
	if len(summary.TopSubreddits) == 0 {
		b.WriteString("\n" + mutedStyle.Render(wrapText("No subreddit data available.", width)))
		return b.String()
	}
	for i, entry := range summary.TopSubreddits {
		name := fmt.Sprintf("%d. r/%s", i+1, entry.Name)
		counts := fmt.Sprintf("%d records (%.1f%%)  •  %d posts  •  %d comments  •  %d score  •  %.1f average", entry.totalRecords(), entry.Share, entry.Posts, entry.Comments, entry.Score, entry.AverageScore)
		b.WriteString("\n\n" + titleStyle.Render(wrapText(name, width)))
		b.WriteString("\n" + mutedStyle.Render(wrapText(counts, width)))
	}
	return b.String()
}
