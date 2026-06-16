package knowledge_test

import (
	"strings"
	"testing"

	"github.com/zjutjh/jxh-go/internal/knowledge"
)

func TestParseRowsBuildsMenuPathAndIgnoresThirdColumn(t *testing.T) {
	rows := [][]string{
		{"%000", "朝晖校区交通\n\n%001 火车站", "维护备注"},
		{"%001", "请选择\n\n%0011 杭州东站", ""},
		{"%0011", "从朝晖去杭州东站路线", ""},
	}
	entries, report := knowledge.ParseRows(rows)
	if report.IgnoredNoteRows != 1 {
		t.Fatalf("ignored note rows = %d", report.IgnoredNoteRows)
	}
	leaf := findEntry(t, entries, "%0011")
	if leaf.Path != "朝晖校区交通 / 火车站 / 杭州东站" {
		t.Fatalf("path = %q", leaf.Path)
	}
	if !strings.Contains(leaf.Content, "杭州东站") || !strings.Contains(leaf.Content, "从朝晖去杭州东站路线") {
		t.Fatalf("content did not include path and answer: %q", leaf.Content)
	}
	if strings.Contains(leaf.Content, "维护备注") {
		t.Fatalf("third column leaked into content: %q", leaf.Content)
	}
}

func TestParseRowsMarksShortChitchatExactOnly(t *testing.T) {
	entries, _ := knowledge.ParseRows([][]string{{"晚安", "晚安哟"}})
	got := findEntry(t, entries, "晚安")
	if got.EntryType != knowledge.EntryTypeChitchat {
		t.Fatalf("entry type = %q", got.EntryType)
	}
	if !got.ExactReply || got.AIEnabled {
		t.Fatalf("exact=%v ai=%v", got.ExactReply, got.AIEnabled)
	}
}

func findEntry(t *testing.T, entries []knowledge.Entry, keyword string) knowledge.Entry {
	t.Helper()
	for _, entry := range entries {
		if entry.Keyword == keyword {
			return entry
		}
	}
	t.Fatalf("entry %q not found in %#v", keyword, entries)
	return knowledge.Entry{}
}
