package store

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

// FsckReport summarises the result of a blob verification run.
type FsckReport struct {
	TotalBlobs    int
	Verified      int
	Corrupt       []FsckCorrupt
	MissingBlobs  []FsckMissing // in DB but not on disk
	OrphanedBlobs []string      // on disk but not in DB (informational)
}

// FsckCorrupt describes a blob whose decompressed content does not match its
// filename (SHA-256).
type FsckCorrupt struct {
	Path   string
	SHA256 string
	Err    string
}

// FsckMissing describes a message row that references a blob file not found
// on disk — the archive is incomplete for these messages.
type FsckMissing struct {
	MessageID string
	SHA256    string
	AccountID string
}

// Fsck walks every blob under the store's base directory, verifies each
// against its filename checksum, and (when db is non-nil) cross-references
// against the messages table to detect missing or orphaned blobs.
//
// It is read-only: it never deletes or modifies anything.
func (s *BlobStore) Fsck(ctx context.Context, db *sql.DB) (*FsckReport, error) {
	report := &FsckReport{}
	diskBlobs := make(map[string]struct{})

	walkErr := filepath.WalkDir(s.baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Warn().Str("path", path).Err(err).Msg("fsck: walk error, skipping")
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), blobExt) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		report.TotalBlobs++
		sha256hex := strings.TrimSuffix(d.Name(), blobExt)
		diskBlobs[sha256hex] = struct{}{}

		if err := s.verifyPath(path, sha256hex); err != nil {
			report.Corrupt = append(report.Corrupt, FsckCorrupt{
				Path:   path,
				SHA256: sha256hex,
				Err:    err.Error(),
			})
			log.Error().Str("path", path).Str("sha256", sha256hex).Err(err).Msg("fsck: corrupt blob")
		} else {
			report.Verified++
		}
		return nil
	})
	if walkErr != nil && walkErr != context.Canceled {
		return nil, fmt.Errorf("walk blobs: %w", walkErr)
	}

	if db == nil {
		return report, nil
	}

	// Cross-reference: DB vs disk.
	rows, err := db.QueryContext(ctx, `
		SELECT id, account_id, blob_sha256
		FROM   messages
		WHERE  blob_sha256 IS NOT NULL
	`)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	dbBlobs := make(map[string]struct{})
	for rows.Next() {
		var msgID, accountID, sha256hex string
		if err := rows.Scan(&msgID, &accountID, &sha256hex); err != nil {
			continue
		}
		dbBlobs[sha256hex] = struct{}{}
		if _, onDisk := diskBlobs[sha256hex]; !onDisk {
			report.MissingBlobs = append(report.MissingBlobs, FsckMissing{
				MessageID: msgID,
				SHA256:    sha256hex,
				AccountID: accountID,
			})
			log.Error().Str("message_id", msgID).Str("sha256", sha256hex).Msg("fsck: blob missing from disk")
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan messages: %w", err)
	}

	for sha256hex := range diskBlobs {
		if _, inDB := dbBlobs[sha256hex]; !inDB {
			report.OrphanedBlobs = append(report.OrphanedBlobs, sha256hex)
		}
	}

	return report, nil
}
