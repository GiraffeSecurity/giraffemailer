package main

import (
	"context"
	"database/sql"
	"testing"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+t.TempDir()+"/t.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			full_name TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user',
			is_active INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func passwordHash(t *testing.T, db *sql.DB) string {
	t.Helper()
	var h string
	if err := db.QueryRow(`SELECT password_hash FROM users WHERE email = 'admin@localhost'`).Scan(&h); err != nil {
		t.Fatal(err)
	}
	return h
}

func TestSeedAdminCreatesThenSkips(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := seedAdmin(ctx, db, false); err != nil {
		t.Fatal(err)
	}
	first := passwordHash(t, db)
	if err := bcrypt.CompareHashAndPassword([]byte(first), []byte("admin123")); err != nil {
		t.Fatalf("expected admin123: %v", err)
	}
	if err := seedAdmin(ctx, db, false); err != nil {
		t.Fatal(err)
	}
	if passwordHash(t, db) != first {
		t.Fatal("second seed without force must not change password hash")
	}
}

func TestSeedAdminForceResetsPassword(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	if err := seedAdmin(ctx, db, false); err != nil {
		t.Fatal(err)
	}
	custom, err := bcrypt.GenerateFromPassword([]byte("changed-pass"), 4)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE users SET password_hash = ? WHERE email = 'admin@localhost'`, string(custom)); err != nil {
		t.Fatal(err)
	}
	if err := seedAdmin(ctx, db, true); err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash(t, db)), []byte("admin123")); err != nil {
		t.Fatalf("force should reset to admin123: %v", err)
	}
}
