package db

import (
	"path/filepath"
	"testing"
)

func newTestJournal(t *testing.T) *Journal {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	j, err := InitJournal(dbPath)
	if err != nil {
		t.Fatalf("InitJournal: %v", err)
	}
	t.Cleanup(func() { j.Close() })
	return j
}

func sampleRecord(sourcePath string) *FileRecord {
	return &FileRecord{
		SourcePath:      sourcePath,
		FileSize:        1024,
		MediaType:       "image",
		Extension:       "jpg",
		CreationTime:    "2024-01-15 10:30:00",
		LargerDimension: 4000,
		OriginalName:    "photo.jpg",
		TimestampKey:    "20240115-103000_image_.jpg",
		Status:          StatusPending,
	}
}

func TestInsertAndRetrieve(t *testing.T) {
	j := newTestJournal(t)

	rec := sampleRecord("/tmp/photo.jpg")
	id, err := j.InsertFile(rec)
	if err != nil {
		t.Fatalf("InsertFile: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive ID, got %d", id)
	}

	// Verify via Stats
	stats, err := j.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats[StatusPending] != 1 {
		t.Errorf("expected 1 pending, got %d", stats[StatusPending])
	}
}

func TestErrAlreadyExists(t *testing.T) {
	j := newTestJournal(t)

	rec := sampleRecord("/tmp/photo.jpg")
	_, err := j.InsertFile(rec)
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}

	// Second insert with same source_path should fail
	_, err = j.InsertFile(rec)
	if err != ErrAlreadyExists {
		t.Fatalf("expected ErrAlreadyExists, got %v", err)
	}
}

func TestUpdateStatus(t *testing.T) {
	j := newTestJournal(t)

	rec := sampleRecord("/tmp/photo.jpg")
	id, _ := j.InsertFile(rec)

	err := j.UpdateStatus(id, StatusCompleted, "")
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	stats, _ := j.Stats()
	if stats[StatusCompleted] != 1 {
		t.Errorf("expected 1 completed, got %d", stats[StatusCompleted])
	}
}

func TestUpdateHash(t *testing.T) {
	j := newTestJournal(t)

	rec := sampleRecord("/tmp/photo.jpg")
	id, _ := j.InsertFile(rec)

	hash := "abc123def456"
	err := j.UpdateHash(id, hash)
	if err != nil {
		t.Fatalf("UpdateHash: %v", err)
	}

	records, err := j.GetByHash(hash)
	if err != nil {
		t.Fatalf("GetByHash: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].Hash != hash {
		t.Errorf("expected hash %q, got %q", hash, records[0].Hash)
	}
}

func TestUpdateDestPath(t *testing.T) {
	j := newTestJournal(t)

	rec := sampleRecord("/tmp/photo.jpg")
	id, _ := j.InsertFile(rec)

	err := j.UpdateDestPath(id, "/dest/photo.jpg", 2, true)
	if err != nil {
		t.Fatalf("UpdateDestPath: %v", err)
	}

	// Verify duplicate count
	count, err := j.DuplicateCount()
	if err != nil {
		t.Fatalf("DuplicateCount: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 duplicate, got %d", count)
	}
}

func TestCountByFileSize(t *testing.T) {
	j := newTestJournal(t)

	r1 := sampleRecord("/tmp/a.jpg")
	r1.FileSize = 5000
	j.InsertFile(r1)

	r2 := sampleRecord("/tmp/b.jpg")
	r2.FileSize = 5000
	j.InsertFile(r2)

	r3 := sampleRecord("/tmp/c.jpg")
	r3.FileSize = 9999
	j.InsertFile(r3)

	count, err := j.CountByFileSize(5000)
	if err != nil {
		t.Fatalf("CountByFileSize: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 files with size 5000, got %d", count)
	}
}

func TestCountByTimestampKey(t *testing.T) {
	j := newTestJournal(t)

	r1 := sampleRecord("/tmp/a.jpg")
	r1.TimestampKey = "20240115-103000_image_.jpg"
	j.InsertFile(r1)

	r2 := sampleRecord("/tmp/b.jpg")
	r2.TimestampKey = "20240115-103000_image_.jpg"
	j.InsertFile(r2)

	count, err := j.CountByTimestampKey("20240115-103000_image_.jpg")
	if err != nil {
		t.Fatalf("CountByTimestampKey: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestGetCompletedSourcePaths(t *testing.T) {
	j := newTestJournal(t)

	r1 := sampleRecord("/tmp/a.jpg")
	id1, _ := j.InsertFile(r1)
	j.UpdateStatus(id1, StatusCompleted, "")

	r2 := sampleRecord("/tmp/b.jpg")
	id2, _ := j.InsertFile(r2)
	j.UpdateStatus(id2, StatusDryRun, "")

	r3 := sampleRecord("/tmp/c.jpg")
	j.InsertFile(r3) // stays pending

	paths, err := j.GetCompletedSourcePaths()
	if err != nil {
		t.Fatalf("GetCompletedSourcePaths: %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("expected 2 completed paths, got %d", len(paths))
	}
	if !paths["/tmp/a.jpg"] || !paths["/tmp/b.jpg"] {
		t.Errorf("expected a.jpg and b.jpg in completed set")
	}
	if paths["/tmp/c.jpg"] {
		t.Errorf("c.jpg should not be in completed set")
	}
}

func TestGetPendingFiles(t *testing.T) {
	j := newTestJournal(t)

	// Pending with dest_path — should appear
	r1 := sampleRecord("/tmp/a.jpg")
	id1, _ := j.InsertFile(r1)
	j.UpdateDestPath(id1, "/dest/a.jpg", 0, false)

	// Pending without dest_path — should not appear
	r2 := sampleRecord("/tmp/b.jpg")
	j.InsertFile(r2)

	// Completed — should not appear
	r3 := sampleRecord("/tmp/c.jpg")
	id3, _ := j.InsertFile(r3)
	j.UpdateDestPath(id3, "/dest/c.jpg", 0, false)
	j.UpdateStatus(id3, StatusCompleted, "")

	pending, err := j.GetPendingFiles()
	if err != nil {
		t.Fatalf("GetPendingFiles: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}
	if pending[0].SourcePath != "/tmp/a.jpg" {
		t.Errorf("expected /tmp/a.jpg, got %s", pending[0].SourcePath)
	}
}

func TestResetFailed(t *testing.T) {
	j := newTestJournal(t)

	r1 := sampleRecord("/tmp/a.jpg")
	id1, _ := j.InsertFile(r1)
	j.UpdateStatus(id1, StatusFailed, "disk full")

	r2 := sampleRecord("/tmp/b.jpg")
	id2, _ := j.InsertFile(r2)
	j.UpdateStatus(id2, StatusFailed, "permission denied")

	count, err := j.ResetFailed()
	if err != nil {
		t.Fatalf("ResetFailed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 reset, got %d", count)
	}

	stats, _ := j.Stats()
	if stats[StatusFailed] != 0 {
		t.Errorf("expected 0 failed after reset, got %d", stats[StatusFailed])
	}
	if stats[StatusPending] != 2 {
		t.Errorf("expected 2 pending after reset, got %d", stats[StatusPending])
	}
}

func TestDropAll(t *testing.T) {
	j := newTestJournal(t)

	j.InsertFile(sampleRecord("/tmp/a.jpg"))
	j.InsertFile(sampleRecord("/tmp/b.jpg"))

	err := j.DropAll()
	if err != nil {
		t.Fatalf("DropAll: %v", err)
	}

	total, _ := j.TotalCount()
	if total != 0 {
		t.Errorf("expected 0 records after DropAll, got %d", total)
	}
}

func TestGetUnhashedByFileSize(t *testing.T) {
	j := newTestJournal(t)

	r1 := sampleRecord("/tmp/a.jpg")
	r1.FileSize = 5000
	id1, _ := j.InsertFile(r1)

	r2 := sampleRecord("/tmp/b.jpg")
	r2.FileSize = 5000
	id2, _ := j.InsertFile(r2)

	// Hash one of them
	j.UpdateHash(id1, "somehash")

	unhashed, err := j.GetUnhashedByFileSize(5000)
	if err != nil {
		t.Fatalf("GetUnhashedByFileSize: %v", err)
	}
	if len(unhashed) != 1 {
		t.Fatalf("expected 1 unhashed, got %d", len(unhashed))
	}
	if unhashed[0].ID != id2 {
		t.Errorf("expected record %d, got %d", id2, unhashed[0].ID)
	}
}

func TestGetByHashEmpty(t *testing.T) {
	j := newTestJournal(t)

	// Empty hash should return nil, nil
	records, err := j.GetByHash("")
	if err != nil {
		t.Fatalf("GetByHash empty: %v", err)
	}
	if records != nil {
		t.Errorf("expected nil for empty hash, got %v", records)
	}
}

func TestStatsEmpty(t *testing.T) {
	j := newTestJournal(t)

	stats, err := j.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if len(stats) != 0 {
		t.Errorf("expected empty stats, got %v", stats)
	}
}
