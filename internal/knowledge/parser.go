package knowledge

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	codePattern  = regexp.MustCompile(`^%\d+$`)
	childPattern = regexp.MustCompile(`(?m)(%\d+)\s+([^\n\r%]+)`)
)

type rawRow struct {
	keyword  string
	answer   string
	note     string
	aliases  []string
	category string
	usage    string
	status   string
	sourceID string
}

func ParseRows(rows [][]string) ([]Entry, ImportReport) {
	report := ImportReport{TotalRows: len(rows)}
	raws := make([]rawRow, 0, len(rows))
	for _, row := range rows {
		r := rowToRaw(row)
		if r.note != "" {
			report.IgnoredNoteRows++
		}
		if r.keyword == "" || r.answer == "" {
			report.SkippedRows++
			continue
		}
		raws = append(raws, r)
	}

	titleByCode := collectCodeTitles(raws)
	children := collectChildren(raws)
	pathByCode := buildPaths(titleByCode, children)
	seen := map[string]Entry{}
	order := make([]string, 0, len(raws))

	for _, raw := range raws {
		entry := enrich(raw, titleByCode, children, pathByCode)
		if existing, ok := seen[entry.SourceKey]; ok {
			if normalizeText(existing.Answer) == normalizeText(entry.Answer) {
				report.DuplicateRows++
				continue
			}
			report.ConflictingRows++
			report.ConflictMessages = append(report.ConflictMessages, fmt.Sprintf("source_key %s has conflicting answers", entry.SourceKey))
			existing.AIEnabled = false
			seen[entry.SourceKey] = existing
			continue
		}
		seen[entry.SourceKey] = entry
		order = append(order, entry.SourceKey)
	}

	out := make([]Entry, 0, len(order))
	for _, key := range order {
		out = append(out, seen[key])
	}
	report.ImportedRows = len(out)
	return out, report
}

func rowToRaw(row []string) rawRow {
	get := func(idx int) string {
		if idx >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[idx])
	}
	return rawRow{
		keyword:  get(0),
		answer:   get(1),
		note:     get(2),
		aliases:  splitList(get(3)),
		category: get(4),
		usage:    strings.ToLower(get(5)),
		status:   strings.ToLower(get(6)),
		sourceID: get(7),
	}
}

func collectCodeTitles(raws []rawRow) map[string]string {
	titles := make(map[string]string)
	for _, raw := range raws {
		if !codePattern.MatchString(raw.keyword) {
			continue
		}
		trimmed := strings.TrimSpace(strings.Split(raw.answer, "\n")[0])
		if trimmed != "" && !strings.Contains(trimmed, "%") {
			titles[raw.keyword] = compactTitle(trimmed)
		}
	}
	for _, raw := range raws {
		for _, match := range childPattern.FindAllStringSubmatch(raw.answer, -1) {
			titles[match[1]] = compactTitle(match[2])
		}
	}
	return titles
}

func collectChildren(raws []rawRow) map[string][]string {
	children := make(map[string][]string)
	for _, raw := range raws {
		if !codePattern.MatchString(raw.keyword) {
			continue
		}
		for _, match := range childPattern.FindAllStringSubmatch(raw.answer, -1) {
			children[raw.keyword] = append(children[raw.keyword], match[1])
		}
	}
	return children
}

func buildPaths(titles map[string]string, children map[string][]string) map[string]string {
	parent := make(map[string]string)
	for p, kids := range children {
		for _, child := range kids {
			if _, exists := parent[child]; !exists {
				parent[child] = p
			}
		}
	}
	paths := make(map[string]string)
	for code := range titles {
		var parts []string
		for cur := code; cur != ""; cur = parent[cur] {
			if title := titles[cur]; title != "" {
				parts = append([]string{title}, parts...)
			}
			if _, ok := parent[cur]; !ok {
				break
			}
		}
		if len(parts) > 0 {
			paths[code] = strings.Join(parts, " / ")
		}
	}
	return paths
}

func enrich(raw rawRow, titles map[string]string, children map[string][]string, paths map[string]string) Entry {
	entryType := classify(raw, children)
	enabled := raw.status == "" || raw.status == "enabled"
	usage := raw.usage
	if usage == "" {
		switch entryType {
		case EntryTypeChitchat:
			usage = "exact"
		case EntryTypeMenuNode:
			usage = "exact"
			if hasFacts(raw.answer) {
				usage = "both"
			}
		default:
			usage = "both"
		}
	}
	aliases := uniqueStrings(append([]string{}, raw.aliases...))
	if title := titles[raw.keyword]; title != "" {
		aliases = uniqueStrings(append(aliases, title))
	}
	if path := paths[raw.keyword]; path != "" {
		parts := strings.Split(path, " / ")
		aliases = uniqueStrings(append(aliases, parts[len(parts)-1]))
	}
	sourceKey := raw.sourceID
	if sourceKey == "" {
		sourceKey = normalizeKey(raw.keyword)
	}
	category := raw.category
	if category == "" {
		category = inferCategory(paths[raw.keyword], raw.keyword, raw.answer)
	}
	return Entry{
		SourceKey:  sourceKey,
		Keyword:    raw.keyword,
		EntryType:  entryType,
		Path:       paths[raw.keyword],
		Aliases:    aliases,
		Category:   category,
		Answer:     raw.answer,
		Content:    buildContent(paths[raw.keyword], raw.keyword, aliases, raw.answer),
		Enabled:    enabled,
		ExactReply: enabled && (usage == "both" || usage == "exact"),
		AIEnabled:  enabled && (usage == "both" || usage == "ai") && entryType != EntryTypeChitchat,
	}
}

func classify(raw rawRow, children map[string][]string) string {
	if len(children[raw.keyword]) > 0 {
		return EntryTypeMenuNode
	}
	if isChitchat(raw.keyword, raw.answer) {
		return EntryTypeChitchat
	}
	return EntryTypeKnowledge
}

func isChitchat(keyword, answer string) bool {
	if codePattern.MatchString(keyword) {
		return false
	}
	keyLen := utf8.RuneCountInString(keyword)
	ansLen := utf8.RuneCountInString(answer)
	if keyLen <= 6 && ansLen <= 30 && !strings.Contains(answer, "\n") {
		return true
	}
	return false
}

func hasFacts(answer string) bool {
	return strings.Contains(answer, "。") || strings.Contains(answer, "：") || strings.Contains(answer, "【")
}

func buildContent(path, keyword string, aliases []string, answer string) string {
	var b strings.Builder
	if path != "" {
		b.WriteString(path)
		b.WriteString("\n")
	}
	b.WriteString("关键词：")
	b.WriteString(keyword)
	if len(aliases) > 0 {
		b.WriteString("，")
		b.WriteString(strings.Join(aliases, "，"))
	}
	b.WriteString("\n知识正文：\n")
	b.WriteString(answer)
	return b.String()
}

func splitList(value string) []string {
	if value == "" {
		return nil
	}
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ';' || r == '；' || r == ',' || r == '，'
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if trimmed := strings.TrimSpace(field); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func normalizeKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeText(value string) string {
	return strings.TrimSpace(strings.ReplaceAll(value, "\r\n", "\n"))
}

func compactTitle(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t-—:：")
	return value
}

func inferCategory(path, keyword, answer string) string {
	text := path + keyword + answer
	switch {
	case strings.Contains(text, "交通") || strings.Contains(text, "火车") || strings.Contains(text, "机场") || strings.Contains(text, "公交"):
		return "交通"
	case strings.Contains(text, "选课") || strings.Contains(text, "学分") || strings.Contains(text, "绩点"):
		return "学习"
	case strings.Contains(text, "寝室") || strings.Contains(text, "宿舍"):
		return "宿舍"
	case strings.Contains(text, "报到") || strings.Contains(text, "开学"):
		return "报到"
	default:
		sum := sha1.Sum([]byte(keyword))
		_ = hex.EncodeToString(sum[:])
		return ""
	}
}
