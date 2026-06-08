package ui

import (
	"io/fs"
	"testing"
)

func TestEmbedIncludesNextStatic(t *testing.T) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(sub, "_next"); err != nil {
		t.Skip("dist/_next missing — run make build-ui before this test")
	}
	matches, err := fs.Glob(sub, "_next/static/chunks/*.js")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("expected _next/static/chunks/*.js in embedded dist")
	}
}
