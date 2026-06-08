package search

import "testing"

func TestEscapeQuery(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"  ", ""},
		{"hello", `"hello"`},
		{"hello world", `"hello" AND "world"`},
		{`say "hi"`, `"say" AND """hi"""`},
	}
	for _, tc := range tests {
		got := EscapeQuery(tc.in)
		if got != tc.want {
			t.Errorf("EscapeQuery(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
