package main

import "strings"

type injectionPattern struct {
	text     string
	category string
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// normalize strips zero-width/invisible Unicode control characters and
// collapses fullwidth ASCII variants to standard ASCII before lowercasing.
// These are documented bypass vectors against substring-matching guards.
func normalize(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\u200b' || r == '\u200c' || r == '\u200d' ||
			r == '\u202e' || r == '\ufeff' || r == '\u00ad':
			continue
		case r >= 0xff01 && r <= 0xff5e:
			b.WriteRune(r - 0xfee0)
		default:
			b.WriteRune(r)
		}
	}
	return strings.ToLower(b.String())
}

// scan iterates messages matching the role filter and returns
// (matched, patternText, category) on the first hit, or (false, "", "").
// Pure function — no PDK dependency — enables host-side unit testing.
func scan(messages []chatMessage, roleSet map[string]bool, scanAll bool) (bool, string, string) {
	for _, msg := range messages {
		role := strings.ToLower(msg.Role)
		if !scanAll && !roleSet[role] {
			continue
		}
		normalized := normalize(msg.Content)
		for _, p := range defaultPatterns {
			if strings.Contains(normalized, p.text) {
				return true, p.text, p.category
			}
		}
	}
	return false, "", ""
}
