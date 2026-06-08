package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/klauspost/compress/zstd"
)

const (
	blobExt   = ".eml.zst"
	nonceSize = 12 // AES-GCM standard nonce length
)

var (
	zstdEnc *zstd.Encoder
	zstdDec *zstd.Decoder
)

func init() {
	var err error
	zstdEnc, err = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		panic("init zstd encoder: " + err.Error())
	}
	zstdDec, err = zstd.NewReader(nil)
	if err != nil {
		panic("init zstd decoder: " + err.Error())
	}
}

// BlobStore manages content-addressed, zstd-compressed email blobs.
//
// Layout: <dataDir>/blobs/<accountID>/<sha256[0:2]>/<sha256[2:4]>/<sha256>.eml.zst
//
// The SHA-256 is always computed over the raw (uncompressed) RFC822 bytes so
// identical messages across accounts/folders are stored exactly once.
type BlobStore struct {
	baseDir    string
	encryptKey *[32]byte
	pathLocks  sync.Map
}

// New creates a BlobStore rooted at <dataDir>/blobs.
// If encryptKey is non-nil every blob is encrypted with AES-256-GCM after compression.
func New(dataDir string, encryptKey *[32]byte) *BlobStore {
	return &BlobStore{
		baseDir:    filepath.Join(dataDir, "blobs"),
		encryptKey: encryptKey,
	}
}

// Write compresses raw RFC822 bytes with zstd, optionally encrypts, writes
// atomically, then verifies the on-disk copy before returning the hex SHA-256.
//
// If a valid blob with the same hash already exists it is returned immediately
// without a second write (content-addressed dedup).
func (s *BlobStore) Write(accountID string, raw []byte) (string, error) {
	if len(raw) == 0 {
		return "", errors.New("raw message is empty")
	}
	hash := sha256Hex(raw)
	blobPath := s.blobPath(accountID, hash)

	// Fast-path dedup: file exists and verifies — no write needed.
	if _, err := os.Stat(blobPath); err == nil {
		if s.verifyPath(blobPath, hash) == nil {
			return hash, nil
		}
	}

	unlock := s.lockPath(blobPath)
	defer unlock()

	// Re-check under the lock to avoid a double-write race.
	if _, err := os.Stat(blobPath); err == nil {
		if s.verifyPath(blobPath, hash) == nil {
			return hash, nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(blobPath), 0o750); err != nil {
		return "", fmt.Errorf("mkdirall: %w", err)
	}

	payload := zstdEnc.EncodeAll(raw, make([]byte, 0, len(raw)/2))

	if s.encryptKey != nil {
		var err error
		payload, err = aesgcmEncrypt(s.encryptKey, payload)
		if err != nil {
			return "", fmt.Errorf("encrypt: %w", err)
		}
	}

	if err := atomicWrite(blobPath, payload); err != nil {
		return "", fmt.Errorf("write blob: %w", err)
	}

	// Golden-rule verify: read back, decompress, re-hash.
	if err := s.verifyPath(blobPath, hash); err != nil {
		_ = os.Remove(blobPath)
		return "", fmt.Errorf("post-write verify failed: %w", err)
	}

	return hash, nil
}

// Read returns the raw RFC822 bytes for the blob identified by sha256hex.
func (s *BlobStore) Read(accountID, sha256hex string) ([]byte, error) {
	payload, err := os.ReadFile(s.blobPath(accountID, sha256hex))
	if err != nil {
		return nil, fmt.Errorf("read blob %s: %w", sha256hex, err)
	}
	return s.decode(payload)
}

// Verify re-reads, decodes, and re-hashes the blob. Returns nil if the hash
// matches the stored filename; an error if the blob is missing or corrupt.
func (s *BlobStore) Verify(accountID, sha256hex string) error {
	return s.verifyPath(s.blobPath(accountID, sha256hex), sha256hex)
}

// Path returns the filesystem path for a blob without reading it.
func (s *BlobStore) Path(accountID, sha256hex string) string {
	return s.blobPath(accountID, sha256hex)
}

// BaseDir returns the root blobs directory.
func (s *BlobStore) BaseDir() string { return s.baseDir }

// ── internal ──────────────────────────────────────────────────────────────────

func (s *BlobStore) blobPath(accountID, sha256hex string) string {
	return filepath.Join(s.baseDir, accountID, sha256hex[0:2], sha256hex[2:4], sha256hex+blobExt)
}

func (s *BlobStore) lockPath(path string) func() {
	v, _ := s.pathLocks.LoadOrStore(path, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func (s *BlobStore) verifyPath(path, expectedHash string) error {
	payload, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	raw, err := s.decode(payload)
	if err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	if got := sha256Hex(raw); got != expectedHash {
		return fmt.Errorf("checksum mismatch: on-disk %s expected %s", got, expectedHash)
	}
	return nil
}

func (s *BlobStore) decode(payload []byte) ([]byte, error) {
	if s.encryptKey != nil {
		var err error
		payload, err = aesgcmDecrypt(s.encryptKey, payload)
		if err != nil {
			return nil, fmt.Errorf("decrypt: %w", err)
		}
	}
	out, err := zstdDec.DecodeAll(payload, nil)
	if err != nil {
		return nil, fmt.Errorf("decompress: %w", err)
	}
	return out, nil
}

// ── crypto helpers ────────────────────────────────────────────────────────────

func aesgcmEncrypt(key *[32]byte, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ct := gcm.Seal(nil, nonce, plaintext, nil)
	return append(nonce, ct...), nil
}

func aesgcmDecrypt(key *[32]byte, data []byte) ([]byte, error) {
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
}

// ── fs helpers ────────────────────────────────────────────────────────────────

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// atomicWrite writes data to path via a sibling temp file followed by rename,
// guaranteeing the target path is either absent or contains complete data.
func atomicWrite(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".blob-tmp-")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, werr := tmp.Write(data)
	serr := tmp.Sync()
	cerr := tmp.Close()
	if werr != nil || serr != nil || cerr != nil {
		os.Remove(name)
		if werr != nil {
			return werr
		}
		if serr != nil {
			return serr
		}
		return cerr
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}
