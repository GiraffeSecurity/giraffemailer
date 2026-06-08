package sqlite

import (
	"encoding/base64"
	"encoding/json"
)

type pageCursor struct {
	D  string `json:"d"`
	ID string `json:"id"`
}

func encodeCursor(date *string, id string) string {
	d := ""
	if date != nil {
		d = *date
	}
	b, _ := json.Marshal(pageCursor{D: d, ID: id})
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(s string) (date *string, id string, ok bool) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, "", false
	}
	var c pageCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, "", false
	}
	if c.ID == "" {
		return nil, "", false
	}
	if c.D != "" {
		return &c.D, c.ID, true
	}
	return nil, c.ID, true
}

func cursorWhereDesc(where *[]string, args *[]any, cursorDate *string, cursorID string) {
	if cursorDate != nil {
		*where = append(*where, "(m.date < ? OR (m.date = ? AND m.id < ?) OR m.date IS NULL)")
		*args = append(*args, *cursorDate, *cursorDate, cursorID)
	} else {
		*where = append(*where, "m.date IS NULL AND m.id < ?")
		*args = append(*args, cursorID)
	}
}

func cursorWhereAsc(where *[]string, args *[]any, cursorDate *string, cursorID string) {
	if cursorDate != nil {
		*where = append(*where, "(m.date > ? OR (m.date = ? AND m.id > ?))")
		*args = append(*args, *cursorDate, *cursorDate, cursorID)
	} else {
		*where = append(*where, "m.date IS NULL AND m.id > ?")
		*args = append(*args, cursorID)
	}
}
