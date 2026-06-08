package search

import (
	"strings"
	"unicode"
)

// EscapeQuery converts free-text input into a safe FTS5 MATCH expression.
// Each token becomes a quoted term joined with AND — O(n) over runes, no allocations beyond builder.
func EscapeQuery(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var terms []string
	for _, tok := range strings.FieldsFunc(raw, func(r rune) bool {
		return unicode.IsSpace(r)
	}) {
		tok = strings.ReplaceAll(tok, `"`, `""`)
		if tok == "" {
			continue
		}
		terms = append(terms, `"`+tok+`"`)
	}
	if len(terms) == 0 {
		return ""
	}
	return strings.Join(terms, " AND ")
}
