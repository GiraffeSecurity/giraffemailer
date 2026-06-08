package store

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestBlobStore_WriteReadVerify(t *testing.T) {
	tests := []struct {
		name    string
		raw     []byte
		encrypt bool
	}{
		{"small message plaintext", []byte("From: alice@example.com\r\nSubject: Hello\r\n\r\nBody text."), false},
		{"small message encrypted", []byte("From: alice@example.com\r\nSubject: Secret\r\n\r\nConfidential body."), true},
		{"binary attachment payload", makeRandom(t, 4096), false},
		{"large message 1MB plaintext", makeRandom(t, 1<<20), false},
		{"large message 1MB encrypted", makeRandom(t, 1<<20), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			var key *[32]byte
			if tt.encrypt {
				key = randomKey(t)
			}
			store := New(dir, key)

			hash, err := store.Write("account-1", tt.raw)
			if err != nil {
				t.Fatalf("Write: %v", err)
			}
			if len(hash) != 64 {
				t.Fatalf("hash length = %d, want 64", len(hash))
			}
			if got := sha256Hex(tt.raw); got != hash {
				t.Fatalf("hash mismatch: Write returned %s, direct hash = %s", hash, got)
			}

			got, err := store.Read("account-1", hash)
			if err != nil {
				t.Fatalf("Read: %v", err)
			}
			if string(got) != string(tt.raw) {
				t.Fatalf("Read returned different bytes (len %d vs %d)", len(got), len(tt.raw))
			}

			if err := store.Verify("account-1", hash); err != nil {
				t.Fatalf("Verify: %v", err)
			}
		})
	}
}

func TestBlobStore_Dedup(t *testing.T) {
	dir := t.TempDir()
	store := New(dir, nil)
	raw := []byte("From: alice@example.com\r\n\r\nDuplicate message body.")

	hash1, err := store.Write("account-1", raw)
	if err != nil {
		t.Fatalf("first Write: %v", err)
	}

	hash2, err := store.Write("account-1", raw)
	if err != nil {
		t.Fatalf("second Write: %v", err)
	}
	if hash1 != hash2 {
		t.Fatalf("dedup: got different hashes %s vs %s", hash1, hash2)
	}

	// Same content, different account → same blob in different account dir,
	// but that's a separate path — not cross-account dedup (by design).
	hash3, err := store.Write("account-2", raw)
	if err != nil {
		t.Fatalf("third Write (account-2): %v", err)
	}
	if hash3 != hash1 {
		t.Fatalf("hash should be identical regardless of account: %s vs %s", hash3, hash1)
	}

	// Confirm only one blob file per account dir.
	count := countBlobFiles(t, filepath.Join(dir, "blobs", "account-1"))
	if count != 1 {
		t.Fatalf("account-1 blob count = %d, want 1", count)
	}
}

func TestBlobStore_CorruptionDetected(t *testing.T) {
	dir := t.TempDir()
	store := New(dir, nil)
	raw := []byte("From: bob@example.com\r\n\r\nImportant email.")

	hash, err := store.Write("account-1", raw)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	blobPath := store.Path("account-1", hash)
	payload, err := os.ReadFile(blobPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// Flip a byte in the middle of the compressed payload.
	mid := len(payload) / 2
	payload[mid] ^= 0xFF
	if err := os.WriteFile(blobPath, payload, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := store.Verify("account-1", hash); err == nil {
		t.Fatal("Verify should have returned error for corrupt blob, got nil")
	}
	if _, err := store.Read("account-1", hash); err == nil {
		t.Fatal("Read should have returned error for corrupt blob, got nil")
	}
}

func TestBlobStore_WriteOverCorruptBlob(t *testing.T) {
	dir := t.TempDir()
	store := New(dir, nil)
	raw := []byte("From: carol@example.com\r\n\r\nRewrite test.")

	hash, err := store.Write("account-1", raw)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Corrupt the blob.
	blobPath := store.Path("account-1", hash)
	_ = os.WriteFile(blobPath, []byte("garbage"), 0o644)

	// Writing the same content again should detect the corruption, overwrite,
	// and return the correct hash.
	hash2, err := store.Write("account-1", raw)
	if err != nil {
		t.Fatalf("Write after corruption: %v", err)
	}
	if hash2 != hash {
		t.Fatalf("hash changed after rewrite: %s vs %s", hash2, hash)
	}
	if err := store.Verify("account-1", hash); err != nil {
		t.Fatalf("Verify after rewrite: %v", err)
	}
}

func TestBlobStore_EmptyInput(t *testing.T) {
	store := New(t.TempDir(), nil)
	if _, err := store.Write("account-1", nil); err == nil {
		t.Fatal("Write(nil) should return error")
	}
	if _, err := store.Write("account-1", []byte{}); err == nil {
		t.Fatal("Write([]) should return error")
	}
}

func TestBlobStore_EncryptionRoundTrip(t *testing.T) {
	key := randomKey(t)
	dir := t.TempDir()
	store := New(dir, key)
	raw := []byte("From: dave@example.com\r\n\r\nEncrypted body.")

	hash, err := store.Write("account-1", raw)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	// The on-disk file must not contain the plaintext.
	diskPayload, _ := os.ReadFile(store.Path("account-1", hash))
	if containsSubslice(diskPayload, []byte("Encrypted body.")) {
		t.Fatal("plaintext visible in encrypted blob — encryption not applied")
	}

	// A plaintext store must NOT be able to read an encrypted blob.
	plainStore := New(dir, nil)
	if _, err := plainStore.Read("account-1", hash); err == nil {
		t.Fatal("plaintext store should fail to read encrypted blob")
	}

	// The encrypted store must recover the original bytes.
	got, err := store.Read("account-1", hash)
	if err != nil {
		t.Fatalf("Read encrypted: %v", err)
	}
	if string(got) != string(raw) {
		t.Fatalf("decrypted content mismatch")
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func randomKey(t *testing.T) *[32]byte {
	t.Helper()
	var k [32]byte
	if _, err := rand.Read(k[:]); err != nil {
		t.Fatalf("rand key: %v", err)
	}
	return &k
}

func makeRandom(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("rand bytes: %v", err)
	}
	return b
}

func countBlobFiles(t *testing.T, root string) int {
	t.Helper()
	count := 0
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			count++
		}
		return nil
	})
	return count
}

func containsSubslice(haystack, needle []byte) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		match := true
		for j, b := range needle {
			if haystack[i+j] != b {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// Silence unused import warning in non-test builds.
var _ = fmt.Sprintf
