package db

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// FileStatus represents the processing status of a file record.
type FileStatus string

const (
	StatusPending   FileStatus = "pending"
	StatusCompleted FileStatus = "completed"
	StatusFailed    FileStatus = "failed"
	StatusDryRun    FileStatus = "dry_run"
	StatusDestIndex FileStatus = "dest_index"
)

// ErrAlreadyExists is returned when inserting a file with a source_path that already exists.
var ErrAlreadyExists = errors.New("file already exists in journal")

// FileRecord represents a row in the files table.
type FileRecord struct {
	ID               int64
	SourcePath       string
	FileSize         int64
	MediaType        string
	Extension        string
	CreationTime     string
	LargerDimension  int
	OriginalName     string
	TimestampKey     string
	Hash             string
	DestPath         string
	SequenceNum      int
	IsDuplicate      bool
	Status           FileStatus
	ErrorMessage     string
	CreatedAt        string
	UpdatedAt        string
}

// Journal wraps a SQLite database for tracking file operations.
type Journal struct {
	db *sql.DB
}

// InitJournal opens (or creates) the SQLite database and initializes the schema.
func InitJournal(dbPath string) (*Journal, error) {
	// Pass pragmas via DSN so they apply to every connection in the pool,
	// not just the first one. This prevents SQLITE_BUSY errors from connections
	// that miss the busy_timeout pragma.
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode%%3DWAL&_pragma=synchronous%%3DNORMAL&_pragma=busy_timeout%%3D5000", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open journal db: %w", err)
	}

	// Create schema
	schema := `
	CREATE TABLE IF NOT EXISTS files (
		id               INTEGER PRIMARY KEY AUTOINCREMENT,
		source_path      TEXT NOT NULL UNIQUE,
		file_size        INTEGER NOT NULL,
		media_type       TEXT NOT NULL,
		extension        TEXT NOT NULL,
		creation_time    TEXT NOT NULL,
		larger_dimension INTEGER NOT NULL DEFAULT 0,
		original_name    TEXT NOT NULL,
		timestamp_key    TEXT NOT NULL,
		hash             TEXT NOT NULL DEFAULT '',
		dest_path        TEXT NOT NULL DEFAULT '',
		sequence_num     INTEGER NOT NULL DEFAULT 0,
		is_duplicate     INTEGER NOT NULL DEFAULT 0,
		status           TEXT NOT NULL DEFAULT 'pending',
		error_message    TEXT NOT NULL DEFAULT '',
		created_at       TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at       TEXT NOT NULL DEFAULT (datetime('now'))
	);
	CREATE INDEX IF NOT EXISTS idx_files_status ON files(status);
	CREATE INDEX IF NOT EXISTS idx_files_file_size ON files(file_size);
	CREATE INDEX IF NOT EXISTS idx_files_hash ON files(hash) WHERE hash != '';
	CREATE INDEX IF NOT EXISTS idx_files_timestamp_key ON files(timestamp_key);
	`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	return &Journal{db: db}, nil
}

// Close closes the underlying database connection.
func (j *Journal) Close() error {
	return j.db.Close()
}

// InsertFile inserts a new file record. Returns ErrAlreadyExists if source_path is taken.
// On success, returns the new row ID.
func (j *Journal) InsertFile(rec *FileRecord) (int64, error) {
	isDup := 0
	if rec.IsDuplicate {
		isDup = 1
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")

	res, err := j.db.Exec(`
		INSERT INTO files (source_path, file_size, media_type, extension, creation_time,
			larger_dimension, original_name, timestamp_key, hash, dest_path,
			sequence_num, is_duplicate, status, error_message, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.SourcePath, rec.FileSize, rec.MediaType, rec.Extension, rec.CreationTime,
		rec.LargerDimension, rec.OriginalName, rec.TimestampKey, rec.Hash, rec.DestPath,
		rec.SequenceNum, isDup, string(rec.Status), rec.ErrorMessage, now, now,
	)
	if err != nil {
		// Check for UNIQUE constraint violation on source_path
		if isUniqueViolation(err) {
			return 0, ErrAlreadyExists
		}
		return 0, fmt.Errorf("insert file: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

// UpdateStatus sets the status and optional error message for a record.
func (j *Journal) UpdateStatus(id int64, status FileStatus, errMsg string) error {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := j.db.Exec(
		`UPDATE files SET status = ?, error_message = ?, updated_at = ? WHERE id = ?`,
		string(status), errMsg, now, id,
	)
	return err
}

// UpdateHash sets the hash for a record.
func (j *Journal) UpdateHash(id int64, hash string) error {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := j.db.Exec(
		`UPDATE files SET hash = ?, updated_at = ? WHERE id = ?`,
		hash, now, id,
	)
	return err
}

// UpdateDestPath sets the destination path, sequence number, and duplicate flag.
func (j *Journal) UpdateDestPath(id int64, destPath string, seqNum int, isDuplicate bool) error {
	isDup := 0
	if isDuplicate {
		isDup = 1
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := j.db.Exec(
		`UPDATE files SET dest_path = ?, sequence_num = ?, is_duplicate = ?, updated_at = ? WHERE id = ?`,
		destPath, seqNum, isDup, now, id,
	)
	return err
}

// CountByFileSize returns how many records have the given file_size.
func (j *Journal) CountByFileSize(size int64) (int, error) {
	var count int
	err := j.db.QueryRow(`SELECT COUNT(*) FROM files WHERE file_size = ?`, size).Scan(&count)
	return count, err
}

// CountByTimestampKey returns how many records share the given timestamp_key.
func (j *Journal) CountByTimestampKey(key string) (int, error) {
	var count int
	err := j.db.QueryRow(`SELECT COUNT(*) FROM files WHERE timestamp_key = ?`, key).Scan(&count)
	return count, err
}

// GetByHash returns all records with the given non-empty hash.
func (j *Journal) GetByHash(hash string) ([]*FileRecord, error) {
	if hash == "" {
		return nil, nil
	}
	rows, err := j.db.Query(`SELECT `+fileColumns+` FROM files WHERE hash = ?`, hash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRecords(rows)
}

// GetCompletedSourcePaths returns a set of source paths with status 'completed' or 'dry_run'.
func (j *Journal) GetCompletedSourcePaths() (map[string]bool, error) {
	rows, err := j.db.Query(`SELECT source_path FROM files WHERE status IN ('completed', 'dry_run')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	paths := make(map[string]bool)
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths[p] = true
	}
	return paths, rows.Err()
}

// GetPendingFiles returns all records with status 'pending' that have a dest_path set.
func (j *Journal) GetPendingFiles() ([]*FileRecord, error) {
	rows, err := j.db.Query(`SELECT `+fileColumns+` FROM files WHERE status = 'pending' AND dest_path != ''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRecords(rows)
}

// ResetFailed changes all 'failed' records back to 'pending' for retry. Returns count affected.
func (j *Journal) ResetFailed() (int64, error) {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	res, err := j.db.Exec(
		`UPDATE files SET status = 'pending', error_message = '', updated_at = ? WHERE status = 'failed'`,
		now,
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DropAll deletes all records from the files table.
func (j *Journal) DropAll() error {
	_, err := j.db.Exec(`DELETE FROM files`)
	return err
}

// Stats returns a map of status → count for all records (excluding dest_index).
func (j *Journal) Stats() (map[FileStatus]int, error) {
	rows, err := j.db.Query(`SELECT status, COUNT(*) FROM files WHERE status != 'dest_index' GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[FileStatus]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		stats[FileStatus(status)] = count
	}
	return stats, rows.Err()
}

// DuplicateCount returns the number of records marked as duplicates.
func (j *Journal) DuplicateCount() (int, error) {
	var count int
	err := j.db.QueryRow(`SELECT COUNT(*) FROM files WHERE is_duplicate = 1`).Scan(&count)
	return count, err
}

// GetUnhashedByFileSize returns records with matching file_size that have no hash set.
func (j *Journal) GetUnhashedByFileSize(size int64) ([]*FileRecord, error) {
	rows, err := j.db.Query(`SELECT `+fileColumns+` FROM files WHERE file_size = ? AND hash = ''`, size)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRecords(rows)
}

// TotalCount returns the total number of records in the journal (excluding dest_index).
func (j *Journal) TotalCount() (int, error) {
	var count int
	err := j.db.QueryRow(`SELECT COUNT(*) FROM files WHERE status != 'dest_index'`).Scan(&count)
	return count, err
}

// DestFile holds minimal info for a pre-indexed destination file.
type DestFile struct {
	Path      string
	Size      int64
	MediaType string
	Extension string
}

// ClearDestIndex removes dest_index rows that have no computed hash.
// Rows with hashes are preserved so they don't need to be re-hashed on the next run.
func (j *Journal) ClearDestIndex() error {
	_, err := j.db.Exec(`DELETE FROM files WHERE status = 'dest_index' AND hash = ''`)
	return err
}

// GetFirstByTimestampKey returns the first record (lowest ID) with the given
// timestamp_key that has sequence_num = 0 (i.e., was filed without a sequence suffix).
func (j *Journal) GetFirstByTimestampKey(key string) (*FileRecord, error) {
	row := j.db.QueryRow(
		`SELECT `+fileColumns+` FROM files WHERE timestamp_key = ? AND sequence_num = 0 AND status != 'dest_index' ORDER BY id LIMIT 1`,
		key,
	)
	r := &FileRecord{}
	var isDup int
	var status string
	err := row.Scan(
		&r.ID, &r.SourcePath, &r.FileSize, &r.MediaType, &r.Extension,
		&r.CreationTime, &r.LargerDimension, &r.OriginalName, &r.TimestampKey,
		&r.Hash, &r.DestPath, &r.SequenceNum, &isDup, &status,
		&r.ErrorMessage, &r.CreatedAt, &r.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.IsDuplicate = isDup == 1
	r.Status = FileStatus(status)
	return r, nil
}

// InsertDestFiles batch-inserts destination files as dest_index records in a single transaction.
// Returns the number of files inserted.
func (j *Journal) InsertDestFiles(files []DestFile) (int, error) {
	if len(files) == 0 {
		return 0, nil
	}

	tx, err := j.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO files (source_path, file_size, media_type, extension, creation_time,
			larger_dimension, original_name, timestamp_key, hash, dest_path,
			sequence_num, is_duplicate, status, error_message, created_at, updated_at)
		VALUES (?, ?, ?, ?, '1970-01-01 00:00:00', 0, ?, 'dest_index', '', '', 0, 0, 'dest_index', '', datetime('now'), datetime('now'))`)
	if err != nil {
		return 0, fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close()

	inserted := 0
	for _, f := range files {
		res, err := stmt.Exec(f.Path, f.Size, f.MediaType, f.Extension, filepath.Base(f.Path))
		if err != nil {
			continue
		}
		n, _ := res.RowsAffected()
		inserted += int(n)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit transaction: %w", err)
	}
	return inserted, nil
}

// --- helpers ---

const fileColumns = `id, source_path, file_size, media_type, extension, creation_time,
	larger_dimension, original_name, timestamp_key, hash, dest_path,
	sequence_num, is_duplicate, status, error_message, created_at, updated_at`

func scanRecords(rows *sql.Rows) ([]*FileRecord, error) {
	var records []*FileRecord
	for rows.Next() {
		r := &FileRecord{}
		var isDup int
		var status string
		if err := rows.Scan(
			&r.ID, &r.SourcePath, &r.FileSize, &r.MediaType, &r.Extension,
			&r.CreationTime, &r.LargerDimension, &r.OriginalName, &r.TimestampKey,
			&r.Hash, &r.DestPath, &r.SequenceNum, &isDup, &status,
			&r.ErrorMessage, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		r.IsDuplicate = isDup == 1
		r.Status = FileStatus(status)
		records = append(records, r)
	}
	return records, rows.Err()
}

func isUniqueViolation(err error) bool {
	// modernc.org/sqlite returns error messages containing "UNIQUE constraint failed"
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
