package historyexport

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/maco/thisyou/internal/reddit"
)

func TestWriteExportsEveryFormatWithoutOverwriting(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	exportedAt := time.Date(2026, time.August, 25, 12, 34, 56, 0, time.Local)
	items := []reddit.Item{{
		Kind: reddit.Comment, ID: "one", Author: "alice", Subreddit: "AskReddit",
		CreatedUTC: exportedAt.Unix(), Score: 4, Body: "first,line\nsecond line",
		Permalink: "/r/AskReddit/comments/post/title/one", Sources: []string{"Arctic Shift", "PullPush"},
	}}
	for _, format := range []Format{CSV, JSON, Markdown, SQLite} {
		first, err := Write(directory, "alice", items, format, exportedAt)
		if err != nil {
			t.Fatal(err)
		}
		second, err := Write(directory, "alice", items, format, exportedAt)
		if err != nil {
			t.Fatal(err)
		}
		if first == second || filepath.Ext(first) != "."+string(format) {
			t.Fatalf("export paths = %q and %q", first, second)
		}
		info, err := os.Stat(first)
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
			t.Fatalf("export permissions = %o, want 600", info.Mode().Perm())
		}
	}
}

func TestSQLiteExportCreatesQueryableTables(t *testing.T) {
	directory := t.TempDir()
	path, err := Write(directory, "alice", []reddit.Item{{
		Kind: reddit.Comment, ID: "one", Author: "alice", Subreddit: "golang", Score: 7,
		Sources: []string{"Arctic Shift", "PullPush"},
	}}, SQLite, time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	database, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var username string
	var count int
	if err := database.QueryRow(`SELECT username, record_count FROM metadata`).Scan(&username, &count); err != nil {
		t.Fatal(err)
	}
	if username != "alice" || count != 1 {
		t.Fatalf("metadata = %q, %d", username, count)
	}
	var sources int
	if err := database.QueryRow(`SELECT COUNT(*) FROM record_sources`).Scan(&sources); err != nil {
		t.Fatal(err)
	}
	if sources != 2 {
		t.Fatalf("source rows = %d, want 2", sources)
	}
}

func TestCSVExportPreservesMultilineText(t *testing.T) {
	t.Parallel()
	data, err := encodeCSV([]reddit.Item{{Kind: reddit.Comment, ID: "one", Body: "first,line\nsecond"}})
	if err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(strings.NewReader(string(data))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[1][8] != "first,line\nsecond" {
		t.Fatalf("CSV records = %#v", records)
	}
}

func TestJSONExportIncludesMetadataAndRecords(t *testing.T) {
	t.Parallel()
	exportedAt := time.Date(2026, time.August, 25, 12, 34, 56, 0, time.UTC)
	data, err := encodeJSON("alice", []reddit.Item{{Kind: reddit.Post, ID: "one"}}, exportedAt)
	if err != nil {
		t.Fatal(err)
	}
	var decoded document
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Username != "alice" || decoded.Count != 1 || len(decoded.Records) != 1 || decoded.Records[0].ID != "one" {
		t.Fatalf("JSON document = %+v", decoded)
	}
}

func TestMarkdownExportContainsReadableRecord(t *testing.T) {
	t.Parallel()
	data := encodeMarkdown("alice", []reddit.Item{{
		Kind: reddit.Post, Author: "alice", Subreddit: "AskReddit", Title: "Example", Body: "first\nsecond", Score: 5,
	}}, time.Date(2026, time.August, 25, 12, 0, 0, 0, time.Local))
	for _, want := range []string{"# Reddit archive history for u/alice", "## 1. POST · r/AskReddit", "**Example**", "> first", "> second"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("Markdown export missing %q: %s", want, data)
		}
	}
}
