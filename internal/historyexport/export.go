package historyexport

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/martintrifunov/thisyou/internal/reddit"
)

type Format string

const (
	CSV      Format = "csv"
	JSON     Format = "json"
	Markdown Format = "md"
	SQLite   Format = "sqlite"
)

type document struct {
	Username   string         `json:"username"`
	ExportedAt time.Time      `json:"exported_at"`
	Count      int            `json:"count"`
	Records    []exportRecord `json:"records"`
}

type exportRecord struct {
	Kind        reddit.Kind `json:"kind"`
	ID          string      `json:"id"`
	Author      string      `json:"author"`
	Subreddit   string      `json:"subreddit"`
	CreatedUTC  int64       `json:"created_utc"`
	CreatedAt   time.Time   `json:"created_at"`
	Score       int         `json:"score"`
	Title       string      `json:"title,omitempty"`
	Body        string      `json:"body,omitempty"`
	Permalink   string      `json:"permalink,omitempty"`
	RedditURL   string      `json:"reddit_url,omitempty"`
	LinkedURL   string      `json:"linked_url,omitempty"`
	NumComments int         `json:"num_comments,omitempty"`
	NSFW        bool        `json:"nsfw"`
	Sources     []string    `json:"sources"`
}

func Write(directory, username string, items []reddit.Item, format Format, exportedAt time.Time) (string, error) {
	if directory == "" {
		directory = "."
	}
	if exportedAt.IsZero() {
		exportedAt = time.Now()
	}
	if format == SQLite {
		return writeSQLite(directory, username, items, exportedAt)
	}
	data, err := encode(username, items, format, exportedAt)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", err
	}
	temporary, err := os.CreateTemp(directory, ".thisyou-export-*")
	if err != nil {
		return "", err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return "", err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return "", err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return "", err
	}
	if err := temporary.Close(); err != nil {
		return "", err
	}
	target, err := availablePath(directory, filename(username, format, exportedAt))
	if err != nil {
		return "", err
	}
	if err := os.Rename(temporaryPath, target); err != nil {
		return "", err
	}
	return target, nil
}

func writeSQLite(directory, username string, items []reddit.Item, exportedAt time.Time) (string, error) {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", err
	}
	temporary, err := os.CreateTemp(directory, ".thisyou-export-*.sqlite")
	if err != nil {
		return "", err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return "", err
	}
	if err := temporary.Close(); err != nil {
		return "", err
	}
	database, err := sql.Open("sqlite", temporaryPath)
	if err != nil {
		return "", err
	}
	database.SetMaxOpenConns(1)
	if err := populateSQLite(database, username, items, exportedAt); err != nil {
		_ = database.Close()
		return "", err
	}
	if err := database.Close(); err != nil {
		return "", err
	}
	target, err := availablePath(directory, filename(username, SQLite, exportedAt))
	if err != nil {
		return "", err
	}
	if err := os.Rename(temporaryPath, target); err != nil {
		return "", err
	}
	return target, nil
}

func populateSQLite(database *sql.DB, username string, items []reddit.Item, exportedAt time.Time) error {
	transaction, err := database.Begin()
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	statements := []string{
		`CREATE TABLE metadata (username TEXT NOT NULL, exported_at TEXT NOT NULL, record_count INTEGER NOT NULL)`,
		`CREATE TABLE records (
			kind TEXT NOT NULL, id TEXT NOT NULL, author TEXT, subreddit TEXT,
			created_utc INTEGER, created_at TEXT, score INTEGER, title TEXT, body TEXT,
			permalink TEXT, reddit_url TEXT, linked_url TEXT, num_comments INTEGER, nsfw INTEGER NOT NULL,
			PRIMARY KEY (kind, id)
		)`,
		`CREATE TABLE record_sources (
			kind TEXT NOT NULL, record_id TEXT NOT NULL, source TEXT NOT NULL,
			PRIMARY KEY (kind, record_id, source),
			FOREIGN KEY (kind, record_id) REFERENCES records(kind, id)
		)`,
		`CREATE INDEX records_subreddit_idx ON records(subreddit)`,
		`CREATE INDEX records_created_utc_idx ON records(created_utc)`,
	}
	for _, statement := range statements {
		if _, err := transaction.Exec(statement); err != nil {
			return err
		}
	}
	if _, err := transaction.Exec(
		`INSERT INTO metadata (username, exported_at, record_count) VALUES (?, ?, ?)`,
		username, exportedAt.UTC().Format(time.RFC3339), len(items),
	); err != nil {
		return err
	}
	recordStatement, err := transaction.Prepare(`INSERT INTO records (
		kind, id, author, subreddit, created_utc, created_at, score, title, body,
		permalink, reddit_url, linked_url, num_comments, nsfw
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer recordStatement.Close()
	sourceStatement, err := transaction.Prepare(`INSERT OR IGNORE INTO record_sources (kind, record_id, source) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer sourceStatement.Close()
	for _, item := range items {
		if _, err := recordStatement.Exec(
			item.Kind, item.ID, item.Author, item.Subreddit, item.CreatedUTC, item.Time().UTC().Format(time.RFC3339),
			item.Score, item.Title, item.Body, item.Permalink, item.RedditURL(), item.URL, item.NumComments, item.Over18,
		); err != nil {
			return err
		}
		for _, source := range item.Sources {
			if _, err := sourceStatement.Exec(item.Kind, item.ID, source); err != nil {
				return err
			}
		}
	}
	return transaction.Commit()
}

func encode(username string, items []reddit.Item, format Format, exportedAt time.Time) ([]byte, error) {
	switch format {
	case CSV:
		return encodeCSV(items)
	case JSON:
		return encodeJSON(username, items, exportedAt)
	case Markdown:
		return encodeMarkdown(username, items, exportedAt), nil
	default:
		return nil, fmt.Errorf("unsupported export format %q", format)
	}
}

func encodeCSV(items []reddit.Item) ([]byte, error) {
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	header := []string{
		"kind", "id", "author", "subreddit", "created_utc", "created_at", "score",
		"title", "body", "permalink", "reddit_url", "linked_url", "num_comments", "nsfw", "sources",
	}
	if err := writer.Write(header); err != nil {
		return nil, err
	}
	for _, item := range items {
		record := []string{
			string(item.Kind), item.ID, item.Author, item.Subreddit,
			strconv.FormatInt(item.CreatedUTC, 10), item.Time().UTC().Format(time.RFC3339), strconv.Itoa(item.Score),
			item.Title, item.Body, item.Permalink, item.RedditURL(), item.URL,
			strconv.Itoa(item.NumComments), strconv.FormatBool(item.Over18), strings.Join(item.Sources, ";"),
		}
		if err := writer.Write(record); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func encodeJSON(username string, items []reddit.Item, exportedAt time.Time) ([]byte, error) {
	records := make([]exportRecord, len(items))
	for index, item := range items {
		records[index] = toExportRecord(item)
	}
	data, err := json.MarshalIndent(document{
		Username: username, ExportedAt: exportedAt.UTC(), Count: len(records), Records: records,
	}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func encodeMarkdown(username string, items []reddit.Item, exportedAt time.Time) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# Reddit archive history for u/%s\n\n", username)
	fmt.Fprintf(&b, "Exported %s · %d records\n", exportedAt.Local().Format("2006-01-02 15:04 MST"), len(items))
	for index, item := range items {
		kind := strings.ToUpper(string(item.Kind))
		fmt.Fprintf(&b, "\n## %d. %s · r/%s\n\n", index+1, kind, fallback(item.Subreddit, "unknown"))
		if title := strings.TrimSpace(item.Title); title != "" {
			fmt.Fprintf(&b, "**%s**\n\n", title)
		}
		fmt.Fprintf(&b, "- Author: u/%s\n", fallback(item.Author, "unknown"))
		fmt.Fprintf(&b, "- Date: %s\n", item.Time().Local().Format(time.RFC1123))
		fmt.Fprintf(&b, "- Score: %d\n", item.Score)
		fmt.Fprintf(&b, "- Archive sources: %s\n", fallback(strings.Join(item.Sources, ", "), "unknown"))
		if item.Over18 {
			b.WriteString("- NSFW: yes\n")
		}
		if body := strings.TrimSpace(item.Body); body != "" {
			b.WriteString("\n")
			for _, line := range strings.Split(body, "\n") {
				fmt.Fprintf(&b, "> %s\n", line)
			}
		}
		if target := item.RedditURL(); target != "" {
			fmt.Fprintf(&b, "\nReddit: <%s>\n", markdownURL(target))
		}
		if item.URL != "" && item.URL != item.RedditURL() {
			fmt.Fprintf(&b, "\nLinked URL: <%s>\n", markdownURL(item.URL))
		}
	}
	return []byte(b.String())
}

func toExportRecord(item reddit.Item) exportRecord {
	return exportRecord{
		Kind: item.Kind, ID: item.ID, Author: item.Author, Subreddit: item.Subreddit,
		CreatedUTC: item.CreatedUTC, CreatedAt: item.Time().UTC(), Score: item.Score,
		Title: item.Title, Body: item.Body, Permalink: item.Permalink, RedditURL: item.RedditURL(),
		LinkedURL: item.URL, NumComments: item.NumComments, NSFW: item.Over18,
		Sources: append([]string(nil), item.Sources...),
	}
}

func filename(username string, format Format, exportedAt time.Time) string {
	return fmt.Sprintf("thisyou-%s-%s.%s", username, exportedAt.Local().Format("20060102-150405"), format)
}

func availablePath(directory, name string) (string, error) {
	target := filepath.Join(directory, name)
	extension := filepath.Ext(name)
	stem := strings.TrimSuffix(name, extension)
	for suffix := 2; ; suffix++ {
		if _, err := os.Stat(target); err != nil {
			if os.IsNotExist(err) {
				return target, nil
			}
			return "", err
		}
		target = filepath.Join(directory, fmt.Sprintf("%s-%d%s", stem, suffix, extension))
	}
}

func fallback(value, replacement string) string {
	if strings.TrimSpace(value) == "" {
		return replacement
	}
	return value
}

func markdownURL(value string) string {
	return strings.ReplaceAll(value, ">", "%3E")
}
